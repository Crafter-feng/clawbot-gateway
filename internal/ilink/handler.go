package ilink

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/bot"
)

// ── 请求/响应结构 ──

type SendMessageRequest struct {
	Msg struct {
		FromUserID   string `json:"from_user_id"`
		ToUserID     string `json:"to_user_id"`
		ClientID     string `json:"client_id"`
		MessageType  int    `json:"message_type"`
		MessageState int    `json:"message_state"`
		ItemList     []Item `json:"item_list"`
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

type GetConfigRequest struct {
	IlinkUserID  string `json:"ilink_user_id"`
	ContextToken string `json:"context_token,omitempty"`
	BaseInfo     struct {
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

// Message iLink 消息格式
type Message struct {
	MessageID    int64  `json:"message_id"`
	FromUserID   string `json:"from_user_id"`
	ToUserID     string `json:"to_user_id"`
	ClientID     string `json:"client_id"`
	MessageType  int    `json:"message_type"`
	MessageState int    `json:"message_state"`
	ItemList     []Item `json:"item_list"`
	ContextToken string `json:"context_token,omitempty"`
	Timestamp    int64  `json:"timestamp,omitempty"`
}

// Item 消息项（支持所有类型）
type Item struct {
	Type      int         `json:"type"`
	TextItem  *TextItem   `json:"text_item,omitempty"`
	ImageItem *ImageItem  `json:"image_item,omitempty"`
	VoiceItem *VoiceItem  `json:"voice_item,omitempty"`
	FileItem  *FileItem   `json:"file_item,omitempty"`
	VideoItem *VideoItem  `json:"video_item,omitempty"`
	RefMsg    *RefMsg     `json:"ref_msg,omitempty"`
}

type TextItem struct {
	Text string `json:"text"`
}

type ImageItem struct {
	Media *CDNMedia `json:"media,omitempty"`
}

type VoiceItem struct {
	Media *CDNMedia `json:"media,omitempty"`
	Text  string    `json:"text,omitempty"`
}

type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
}

type VideoItem struct {
	Media *CDNMedia `json:"media,omitempty"`
}

type RefMsg struct {
	Title       string `json:"title,omitempty"`
	MessageItem *Item  `json:"message_item,omitempty"`
}

type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"`
}

// ── 认证 ──

// validateToken 验证虚拟 Bot token
// token 就是 account_id（如 gw_a1b2c3d4）
func (s *Server) validateToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		token := auth[7:]
		if bot := s.registry.GetByToken(token); bot != nil {
			return bot.AccountID
		}
	}
	return ""
}

// ── 处理函数 ──

// handleGetUpdates 处理长轮询获取消息
func (s *Server) handleGetUpdates(c *gin.Context) {
	// 1. 验证虚拟 Bot token
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"ret": -1, "errmsg": "unauthorized"})
		return
	}

	// 2. 获取虚拟 Bot 的消息队列
	bot := s.registry.Get(accountID)
	if bot == nil {
		c.JSON(400, gin.H{"ret": -1, "errmsg": "bot not registered"})
		return
	}

	// 3. 从队列取消息（长轮询）
	timeout := 35 * time.Second
	msgs := bot.Queue.DequeueAll(timeout)

	// 4. 转换为 iLink 格式
	ilinkMsgs := make([]Message, 0, len(msgs))
	for _, msg := range msgs {
		ilinkMsgs = append(ilinkMsgs, s.convertToILinkMessage(msg))
	}

	c.JSON(200, GetUpdatesResponse{
		Ret:           0,
		Msgs:          ilinkMsgs,
		GetUpdatesBuf: bot.UpdateBuf,
	})
}

// handleSendMessage 处理消息发送请求（支持所有消息类型）
func (s *Server) handleSendMessage(c *gin.Context) {
	// 1. 验证虚拟 Bot token
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"ret": -1, "errmsg": "unauthorized"})
		return
	}

	// 2. 解析请求
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"ret": -1, "errmsg": "invalid request"})
		return
	}

	// 3. 获取凭证
	creds := s.bot.GetAccountCredentials(accountID)
	if creds == nil {
		accounts := s.bot.GetAccounts()
		if len(accounts) == 0 {
			c.JSON(500, gin.H{"ret": -1, "errmsg": "no accounts available"})
			return
		}
		creds = accounts[0].Credentials
	}

	// 4. 发送消息（支持所有类型）
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

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

	err := s.bot.SendTextWithCreds(ctx, creds, req.Msg.ToUserID, text, req.Msg.ContextToken)
	if err != nil {
		s.log.Warn("sendmessage failed", "error", err)
		c.JSON(500, gin.H{"ret": -1, "errmsg": err.Error()})
		return
	}

	c.JSON(200, gin.H{"ret": 0, "message_id": 0})
}

// handleSendTyping 处理输入状态请求
func (s *Server) handleSendTyping(c *gin.Context) {
	// 1. 验证虚拟 Bot token
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"ret": -1, "errmsg": "unauthorized"})
		return
	}

	// 2. 解析请求
	var req SendTypingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"ret": -1, "errmsg": "invalid request"})
		return
	}

	// 3. 获取凭证
	creds := s.bot.GetAccountCredentials(accountID)
	if creds == nil {
		accounts := s.bot.GetAccounts()
		if len(accounts) == 0 {
			c.JSON(500, gin.H{"ret": -1, "errmsg": "no accounts available"})
			return
		}
		creds = accounts[0].Credentials
	}

	// 4. 发送输入状态
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

// handleGetConfig 获取配置（如 typing_ticket）
func (s *Server) handleGetConfig(c *gin.Context) {
	// 1. 验证虚拟 Bot token
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"ret": -1, "errmsg": "unauthorized"})
		return
	}

	// 2. 解析请求
	var req GetConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"ret": -1, "errmsg": "invalid request"})
		return
	}

	// 3. 返回配置
	c.JSON(200, gin.H{
		"ret":            0,
		"typing_ticket":  "",
		"channel_version": "1.0.2",
	})
}

// handleGetQRCode 获取二维码（我们就是服务，返回空数据）

// handleGetQRCodeStatus 检查二维码状态

// handleGetUploadURL 获取上传 URL
func (s *Server) handleGetUploadURL(c *gin.Context) {
	c.JSON(501, gin.H{"ret": -1, "errmsg": "upload not supported in proxy mode"})
}

// ── 消息转换 ──

// convertToILinkMessage 将标准化消息转换为 iLink 格式
func (s *Server) convertToILinkMessage(msg bot.NormalizedMessage) Message {
	items := make([]Item, 0, len(msg.Items))

	for _, item := range msg.Items {
		i := Item{Type: item.Type}

		if item.TextItem != nil {
			i.TextItem = &TextItem{Text: item.TextItem.Text}
		}
		if item.ImageItem != nil {
			i.ImageItem = &ImageItem{
				Media: convertCDNMedia(item.ImageItem.Media),
			}
		}
		if item.VoiceItem != nil {
			i.VoiceItem = &VoiceItem{
				Media: convertCDNMedia(item.VoiceItem.Media),
				Text:  item.VoiceItem.Text,
			}
		}
		if item.FileItem != nil {
			i.FileItem = &FileItem{
				Media:    convertCDNMedia(item.FileItem.Media),
				FileName: item.FileItem.FileName,
			}
		}
		if item.VideoItem != nil {
			i.VideoItem = &VideoItem{
				Media: convertCDNMedia(item.VideoItem.Media),
			}
		}
		if item.RefMsg != nil {
			ref := &RefMsg{Title: item.RefMsg.Title}
			if item.RefMsg.MessageItem != nil {
				converted := convertMessageItem(item.RefMsg.MessageItem)
				ref.MessageItem = &converted
			}
			i.RefMsg = ref
		}

		items = append(items, i)
	}

	// 如果没有消息项，创建文本项
	if len(items) == 0 && msg.Content != "" {
		items = []Item{{
			Type:     1,
			TextItem: &TextItem{Text: msg.Content},
		}}
	}

	return Message{
		MessageID:    0,
		FromUserID:   msg.FromUser,
		ToUserID:     msg.ToUser,
		MessageType:  1,
		MessageState: 2,
		ItemList:     items,
		ContextToken: msg.ContextToken,
		Timestamp:    msg.Timestamp,
	}
}

func convertMessageItem(item *bot.MessageItem) Item {
	i := Item{Type: item.Type}
	if item.TextItem != nil {
		i.TextItem = &TextItem{Text: item.TextItem.Text}
	}
	if item.ImageItem != nil {
		i.ImageItem = &ImageItem{Media: convertCDNMedia(item.ImageItem.Media)}
	}
	if item.VoiceItem != nil {
		i.VoiceItem = &VoiceItem{Media: convertCDNMedia(item.VoiceItem.Media), Text: item.VoiceItem.Text}
	}
	if item.FileItem != nil {
		i.FileItem = &FileItem{Media: convertCDNMedia(item.FileItem.Media), FileName: item.FileItem.FileName}
	}
	if item.VideoItem != nil {
		i.VideoItem = &VideoItem{Media: convertCDNMedia(item.VideoItem.Media)}
	}
	return i
}

func convertCDNMedia(media *bot.CDNMedia) *CDNMedia {
	if media == nil {
		return nil
	}
	return &CDNMedia{
		EncryptQueryParam: media.EncryptQueryParam,
		AESKey:            media.AESKey,
	}
}
