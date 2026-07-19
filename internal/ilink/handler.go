package ilink

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// ── 请求/响应结构 ──

type SendMessageRequest struct {
	Msg struct {
		FromUserID   string `json:"from_user_id"`
		ToUserID     string `json:"to_user_id"`
		ClientID     string `json:"client_id"`
		MessageType  int    `json:"message_type"`
		MessageState int    `json:"message_state"`
		ItemList     []struct {
			Type      int `json:"type"`
			TextItem  *struct {
				Text string `json:"text"`
			} `json:"text_item,omitempty"`
		} `json:"item_list"`
		ContextToken string `json:"context_token,omitempty"`
	} `json:"msg"`
	BaseInfo struct {
		ChannelVersion string `json:"channel_version"`
	} `json:"base_info"`
}

type SendMessageResponse struct {
	MessageID int64 `json:"message_id"`
}

type SendTypingRequest struct {
	IlinkUserID  string `json:"ilink_user_id"`
	TypingTicket string `json:"typing_ticket"`
	Status       int    `json:"status"`
	BaseInfo     struct {
		ChannelVersion string `json:"channel_version"`
	} `json:"base_info"`
}

type GetUpdatesRequest struct {
	GetUpdatesBuf string `json:"get_updates_buf"`
	BaseInfo      struct {
		ChannelVersion string `json:"channel_version"`
	} `json:"base_info"`
}

type GetUpdatesResponse struct {
	Ret           int        `json:"ret"`
	Errcode       int        `json:"errcode,omitempty"`
	Errmsg        string     `json:"errmsg,omitempty"`
	Msgs          []Message  `json:"msgs"`
	GetUpdatesBuf string     `json:"get_updates_buf"`
}

type Message struct {
	MessageID    int64  `json:"message_id"`
	FromUserID   string `json:"from_user_id"`
	ToUserID     string `json:"to_user_id"`
	ClientID     string `json:"client_id"`
	MessageType  int    `json:"message_type"`
	MessageState int    `json:"message_state"`
	ItemList     []struct {
		Type      int `json:"type"`
		TextItem  *struct {
			Text string `json:"text"`
		} `json:"text_item,omitempty"`
	} `json:"item_list"`
	ContextToken string `json:"context_token,omitempty"`
}

// ── 处理函数 ──

// handleSendMessage 处理消息发送请求
func (s *Server) handleSendMessage(c *gin.Context) {
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"ret": -1, "errmsg": "invalid request"})
		return
	}

	// 提取文本内容
	text := ""
	for _, item := range req.Msg.ItemList {
		if item.Type == 1 && item.TextItem != nil {
			text = item.TextItem.Text
			break
		}
	}

	if text == "" {
		c.JSON(400, gin.H{"ret": -1, "errmsg": "empty message"})
		return
	}

	// 转发到 bot 发送
	creds := s.bot.GetAccountCredentials(req.Msg.ToUserID)
	if creds == nil {
		// 尝试使用默认账号
		accounts := s.bot.GetAccounts()
		if len(accounts) == 0 {
			c.JSON(500, gin.H{"ret": -1, "errmsg": "no accounts available"})
			return
		}
		creds = accounts[0].Credentials
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	err := s.bot.SendTextWithCreds(ctx, creds, req.Msg.ToUserID, text, req.Msg.ContextToken)
	if err != nil {
		s.log.Warn("sendmessage failed", "error", err)
		c.JSON(500, gin.H{"ret": -1, "errmsg": err.Error()})
		return
	}

	c.JSON(200, SendMessageResponse{
		MessageID: 0, // iLink 返回 message_id
	})
}

// handleSendTyping 处理输入状态请求
func (s *Server) handleSendTyping(c *gin.Context) {
	var req SendTypingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"ret": -1, "errmsg": "invalid request"})
		return
	}

	creds := s.bot.GetAccountCredentials(req.IlinkUserID)
	if creds == nil {
		accounts := s.bot.GetAccounts()
		if len(accounts) == 0 {
			c.JSON(500, gin.H{"ret": -1, "errmsg": "no accounts available"})
			return
		}
		creds = accounts[0].Credentials
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	err := s.bot.SendTypingWithCreds(ctx, creds, req.IlinkUserID)
	if err != nil {
		s.log.Warn("sendtyping failed", "error", err)
		c.JSON(500, gin.H{"ret": -1, "errmsg": err.Error()})
		return
	}

	c.JSON(200, gin.H{"ret": 0})
}

// handleGetUpdates 处理长轮询获取消息
func (s *Server) handleGetUpdates(c *gin.Context) {
	var req GetUpdatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// getupdates 可能是 GET 请求，参数在 query string
		req.GetUpdatesBuf = c.Query("get_updates_buf")
	}

	// 等待消息（带超时）
	timeout := 35 * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case msg, ok := <-s.bot.Messages():
		if !ok {
			c.JSON(200, GetUpdatesResponse{Ret: 0, Msgs: []Message{}})
			return
		}

		// 转换为 iLink 格式
		ilinkMsg := Message{
			MessageID:    0,
			FromUserID:   msg.FromUser,
			ToUserID:     msg.ToUser,
			MessageType:  1,
			MessageState: 2,
			ContextToken: msg.ContextToken,
			ItemList: []struct {
				Type      int `json:"type"`
				TextItem  *struct {
					Text string `json:"text"`
				} `json:"text_item,omitempty"`
			}{
				{
					Type: 1,
					TextItem: &struct {
						Text string `json:"text"`
					}{Text: msg.Content},
				},
			},
		}

		c.JSON(200, GetUpdatesResponse{
			Ret:           0,
			Msgs:          []Message{ilinkMsg},
			GetUpdatesBuf: "",
		})

	case <-timer.C:
		// 超时，返回空消息
		c.JSON(200, GetUpdatesResponse{
			Ret:           0,
			Msgs:          []Message{},
			GetUpdatesBuf: "",
		})

	case <-c.Request.Context().Done():
		c.JSON(200, GetUpdatesResponse{Ret: 0, Msgs: []Message{}})
	}
}

// handleGetQRCode 获取二维码
func (s *Server) handleGetQRCode(c *gin.Context) {
	botType := 3
	if bt := c.Query("bot_type"); bt != "" {
		fmt.Sscanf(bt, "%d", &botType)
	}

	// 获取 QR 码（这里简化处理，实际需要调用 iLink API）
	// 对于代理模式，我们可能需要转发到真正的 iLink API
	c.JSON(200, gin.H{
		"ret":                0,
		"qrcode":             "",
		"qrcode_img_content": "",
	})
}

// handleGetQRCodeStatus 检查二维码状态
func (s *Server) handleGetQRCodeStatus(c *gin.Context) {
	qrcode := c.Query("qrcode")
	if qrcode == "" {
		c.JSON(400, gin.H{"ret": -1, "errmsg": "missing qrcode"})
		return
	}

	// 检查 QR 扫码状态
	state := s.bot.QRManager().CheckStatus(qrcode)

	resp := gin.H{
		"ret":    0,
		"status": state.Status,
	}

	if state.Creds != nil {
		resp["bot_token"] = state.Creds.Token
		resp["ilink_bot_id"] = state.Creds.AccountID
		resp["ilink_user_id"] = state.Creds.UserID
		resp["baseurl"] = state.Creds.BaseURL
	}

	c.JSON(200, resp)
}

// handleGetUploadURL 获取上传 URL
func (s *Server) handleGetUploadURL(c *gin.Context) {
	// 简化处理 - 返回错误，因为代理模式不直接处理媒体上传
	c.JSON(501, gin.H{"ret": -1, "errmsg": "upload not supported in proxy mode"})
}
