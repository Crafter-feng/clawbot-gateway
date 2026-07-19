package bot

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ── 发送媒体文件（完整流程：加密上传 → 发送） ──

func (c *Connector) SendMediaMessage(ctx context.Context, toUser, fileKey string, msgType int) error {
	accounts := c.GetAccounts()
	if len(accounts) == 0 {
		return fmt.Errorf("no accounts available")
	}
	creds := accounts[0].Credentials

	clientID := GenerateClientID()
	var payload sendMessagePayload
	payload.Msg.FromUserID = ""
	payload.Msg.ToUserID = toUser
	payload.Msg.ClientID = clientID
	payload.Msg.MessageType = 2
	payload.Msg.MessageState = 2

	itemType := 1
	switch msgType {
	case 3:
		itemType = 3
	case 34:
		itemType = 34
	case 49:
		itemType = 49
	}
	payload.Msg.ItemList = []struct {
		Type      int `json:"type"`
		TextItem  *struct {
			Text string `json:"text"`
		} `json:"text_item,omitempty"`
	}{{
		Type: itemType,
		TextItem: &struct {
			Text string `json:"text"`
		}{Text: fileKey},
	}}
	payload.BaseInfo = BaseInfo{
		ChannelVersion: "1.0.2",
	}

	return c.sendMessage(ctx, creds, &payload)
}

func (c *Connector) UploadAndSendMedia(ctx context.Context, accountID, toUser string, fileData []byte, fileName, mimeType string) error {
	creds := c.GetAccountCredentials(accountID)
	if creds == nil {
		return fmt.Errorf("account %s not found", accountID)
	}
	baseURL := creds.BaseURL
	token := creds.Token

	// 1. 计算文件 MD5 和大小
	hasher := md5.New()
	hasher.Write(fileData)
	fileMD5 := hex.EncodeToString(hasher.Sum(nil))
	fileSize := len(fileData)

	// 2. 生成随机 AES Key
	aesKey := make([]byte, 16)
	rand.Read(aesKey)

	// 3. 加密文件数据
	encrypted, err := aesEncrypt(fileData, aesKey)
	if err != nil {
		return fmt.Errorf("encrypt failed: %w", err)
	}

	// 4. 获取上传 URL
	uploadReq := struct {
		FileKey    string   `json:"filekey"`
		MediaType  int      `json:"media_type"`
		ToUserID   string   `json:"to_user_id"`
		RawSize    int      `json:"rawsize"`
		RawFileMD5 string   `json:"rawfilemd5"`
		FileSize   int      `json:"filesize"`
		AESKey     string   `json:"aeskey"`
		BaseInfo   BaseInfo `json:"base_info"`
	}{
		FileKey:    fileName,
		MediaType:  4,
		ToUserID:   toUser,
		RawSize:    fileSize,
		RawFileMD5: fileMD5,
		FileSize:   len(encrypted),
		AESKey:     base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(aesKey))),
		BaseInfo:   BuildBaseInfo(),
	}
	uploadJSON, _ := json.Marshal(uploadReq)
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/ilink/bot/getuploadurl", bytes.NewReader(uploadJSON))
	if err != nil {
		return err
	}
	c.setFullHeaders(req, token, string(uploadJSON))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var uploadResp struct {
		URL     string `json:"url"`
		AESKey  string `json:"aeskey"`
		FileKey string `json:"filekey"`
		Token   string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return fmt.Errorf("parse upload url response: %w", err)
	}

	// 5. 上传加密文件到 CDN
	uploadReq2, _ := http.NewRequestWithContext(ctx, "POST", uploadResp.URL, bytes.NewReader(encrypted))
	uploadReq2.Header.Set("Content-Type", "application/octet-stream")
	if uploadResp.Token != "" {
		uploadReq2.Header.Set("Authorization", uploadResp.Token)
	}
	upResp, err := c.client.Do(uploadReq2)
	if err != nil {
		return err
	}
	upResp.Body.Close()

	// 6. 发送媒体消息
	sendMediaReq := struct {
		ToUserid string `json:"to_userid"`
		MsgType  int    `json:"msg_type"`
		Content  struct {
			AESKey  string `json:"aeskey"`
			FileKey string `json:"filekey"`
			MD5     string `json:"md5"`
			Size    int    `json:"size"`
		} `json:"content"`
		BaseInfo BaseInfo `json:"base_info"`
	}{
		ToUserid: toUser,
		MsgType:  3,
		BaseInfo: BuildBaseInfo(),
	}
	sendMediaReq.Content.AESKey = uploadResp.AESKey
	sendMediaReq.Content.FileKey = uploadResp.FileKey
	sendMediaReq.Content.MD5 = fileMD5
	sendMediaReq.Content.Size = fileSize
	sendJSON, _ := json.Marshal(sendMediaReq)

	req3, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/ilink/bot/sendmessage", bytes.NewReader(sendJSON))
	if err != nil {
		return err
	}
	c.setFullHeaders(req3, token, string(sendJSON))
	resp3, err := c.client.Do(req3)
	if err != nil {
		return err
	}
	resp3.Body.Close()

	// 解析响应体中的 errcode/errmsg
	var sendResp struct {
		Ret     int    `json:"ret"`
		Errcode int    `json:"errcode,omitempty"`
		Errmsg  string `json:"errmsg,omitempty"`
	}
	respBody3, _ := io.ReadAll(resp3.Body)
	if err := json.Unmarshal(respBody3, &sendResp); err == nil {
		if sendResp.Ret != 0 || sendResp.Errcode != 0 {
			return fmt.Errorf("sendMediaMessage api error: ret=%d, errcode=%d, errmsg=%s", sendResp.Ret, sendResp.Errcode, sendResp.Errmsg)
		}
	}
	return nil
}

// ── AES 加密 ──

func aesEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padtext := append(plaintext, bytes.Repeat([]byte{byte(padding)}, padding)...)

	ciphertext := make([]byte, len(padtext))
	for i := 0; i < len(padtext); i += aes.BlockSize {
		block.Encrypt(ciphertext[i:i+aes.BlockSize], padtext[i:i+aes.BlockSize])
	}
	return ciphertext, nil
}
