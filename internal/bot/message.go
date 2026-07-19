package bot

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// ── iLink API 常量 ──

const (
	ILinkAppID          = "bot"
	appClientVersionVal = (2 << 16) | (2 << 8) | 0 // 131584
	qrPollDeadlineSecs  = 480
	maxQRRefreshCount   = 3
)

// ── 消息模型 ──

type NormalizedMessage struct {
	MsgID        string          `json:"msg_id"`
	FromUser     string          `json:"from_user"`
	ToUser       string          `json:"to_user"`
	AccountID    string          `json:"account_id"` // 消息来源的微信账号
	Type         int             `json:"type"`       // 1=text, 3=image, 34=voice, 49=file ...
	Content      string          `json:"content"`
	Timestamp    int64           `json:"timestamp"`
	ContextToken string          `json:"context_token,omitempty"`
	Raw          json.RawMessage `json:"raw,omitempty"`
}

type BaseInfo struct {
	ChannelVersion string `json:"channel_version"`
}

func BuildBaseInfo() BaseInfo {
	return BaseInfo{
		ChannelVersion: "1.0.2",
	}
}

// ── 登录凭证 ──

type Credentials struct {
	Token       string `json:"token"`
	BaseURL     string `json:"base_url"`
	AccountID   string `json:"account_id"`
	UserID      string `json:"user_id"`
	AccountName string `json:"account_name,omitempty"`
	LoginAt     int64  `json:"login_at"`
}

type QRCodeResponse struct {
	Success bool   `json:"success"`
	Data    QRData `json:"data"`
}

type QRData struct {
	QRCodeURL string `json:"qrcode_url"`
	QRCode    string `json:"qrcode"`
}

type QRStatusResponse struct {
	Success bool         `json:"success"`
	Data    QRStatusData `json:"data"`
}

type QRStatusData struct {
	Status      string           `json:"status"` // wait, scaned, confirmed, expired
	Credentials *CredentialsData `json:"credentials,omitempty"`
	BaseURL     string           `json:"baseurl,omitempty"`
}

type CredentialsData struct {
	BotToken    string `json:"bot_token"`
	IlinkBotID  string `json:"ilink_bot_id"`
	IlinkUserID string `json:"ilink_user_id"`
}

// ── 获取消息 ──

type GetUpdatesReq struct {
	GetUpdatesBuf string   `json:"get_updates_buf"`
	BaseInfo      BaseInfo `json:"base_info"`
}

type GetUpdatesResp struct {
	Ret           int              `json:"ret"`
	Errcode       int              `json:"errcode,omitempty"`
	Errmsg        string           `json:"errmsg,omitempty"`
	Msgs          []RawMessageItem `json:"msgs"`
	GetUpdatesBuf string           `json:"get_updates_buf"`
}

type RawMessageItem struct {
	MsgID        json.Number           `json:"message_id"`
	FromUserid   string                `json:"from_user_id"`
	ToUserid     string                `json:"to_user_id"`
	MsgType      int                   `json:"msg_type"`
	ItemList     []RawMessageItem_Item `json:"item_list"`
	Timestamp    int64                 `json:"timestamp"`
	ContextToken string                `json:"context_token,omitempty"`
}

type RawMessageItem_Item struct {
	Type      int                    `json:"type"`
	TextItem  *RawMessageItem_TextItem `json:"text_item,omitempty"`
	VoiceItem *RawMessageItem_TextItem `json:"voice_item,omitempty"`
	RefMsg    *RawMessageItem_RefMsg   `json:"ref_msg,omitempty"`
}

type RawMessageItem_TextItem struct {
	Text string `json:"text"`
}

type RawMessageItem_RefMsg struct {
	Title       string              `json:"title,omitempty"`
	MessageItem *RawMessageItem_Item `json:"message_item,omitempty"`
}

// ── 消息解析 ──

// normalize 将原始 iLink 消息转换为标准化格式
func normalize(raw RawMessageItem) NormalizedMessage {
	content := extractText(raw.ItemList)
	return NormalizedMessage{
		MsgID:        raw.MsgID.String(),
		FromUser:     raw.FromUserid,
		ToUser:       raw.ToUserid,
		Type:         raw.MsgType,
		Content:      content,
		Timestamp:    raw.Timestamp,
		ContextToken: raw.ContextToken,
	}
}

// extractText 从 item_list 提取文本内容（与 Python _extract_text 逻辑对齐）
func extractText(items []RawMessageItem_Item) string {
	// 优先提取文本类型 (type=1)
	for _, item := range items {
		if item.Type == 1 && item.TextItem != nil {
			text := item.TextItem.Text
			// 处理引用消息
			if item.RefMsg != nil {
				refText := ""
				if item.RefMsg.MessageItem != nil {
					refText = extractText([]RawMessageItem_Item{*item.RefMsg.MessageItem})
				}
				title := item.RefMsg.Title
				if title != "" || refText != "" {
					prefix := ""
					if title != "" && refText != "" {
						prefix = "[引用: " + title + " | " + refText + "]\n"
					} else if title != "" {
						prefix = "[引用: " + title + "]\n"
					} else {
						prefix = "[引用: " + refText + "]\n"
					}
					text = prefix + text
				}
			}
			return strings.TrimSpace(text)
		}
	}
	// 回退：提取语音转文字 (type=3)
	for _, item := range items {
		if item.Type == 3 && item.VoiceItem != nil && item.VoiceItem.Text != "" {
			return item.VoiceItem.Text
		}
	}
	return ""
}

// GenerateClientID 生成客户端 ID（格式：openclaw-weixin:<ms>-<rand>）
func GenerateClientID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "openclaw-weixin:" + time.Now().Format("20060102150405.000") + "-" + hex.EncodeToString(b)
}
