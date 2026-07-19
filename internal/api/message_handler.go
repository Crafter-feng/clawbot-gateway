package api

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/adapter"
	"clawbot-gateway/internal/store"
)

// ══════════════════════════════════════════════════════════════════════
//  ★ 消息发送 API（核心对外接口）★
//  1. POST /api/v1/message/send        → 发消息给指定用户
//  2. POST /api/v1/message/broadcast    → 广播到所有绑定的微信
//  3. POST /api/v1/message/send-by-account → 通过指定账号发消息
//  4. GET  /api/v1/message/accounts     → 账号列表
// ══════════════════════════════════════════════════════════════════════

// handlePushSend 外部系统通过此接口向 ClawBot 发送消息
// 请求: POST /api/v1/message/send
//
//	Body: {
//	  "to_user": "微信用户ID",
//	  "content": "消息内容",
//	  "msg_type": 1,          (1=text, 3=image, 34=voice, 49=file, 默认1)
//	  "wait_reply": false     (是否等待后端回复)
//	}
func (s *APIServer) handlePushSend(c *gin.Context) {
	var req struct {
		ToUser    string `json:"to_user"`
		Content   string `json:"content"`
		MsgType   int    `json:"msg_type"`
		WaitReply bool   `json:"wait_reply"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Content == "" {
		c.JSON(400, gin.H{"error": "content is required"})
		return
	}
	if req.MsgType == 0 {
		req.MsgType = 1
	}

	// 保存到历史
	if err := s.store.SaveMessage(store.StoredMessage{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		ToUser:    req.ToUser,
		Content:   req.Content,
		MsgType:   req.MsgType,
		Timestamp: time.Now().Unix(),
		Direction: "outgoing",
	}); err != nil {
		s.log.Warn("failed to save message", "error", err)
	}

	reply := ""
	backendID := ""

	if req.WaitReply {
		dec := s.router.Route(req.Content, "api_msg_"+req.ToUser)
		if adp, ok := s.adapters.Get(dec.BackendID); ok {
			resp, err := adp.Handle(c.Request.Context(), &adapter.ChatRequest{
				Message: req.Content,
				UserID:  "api_msg_" + req.ToUser,
			})
			if err == nil {
				reply = resp.Text
				backendID = resp.Backend
			}
		}
	} else if req.ToUser != "" && s.connector.IsRunning() {
		if err := s.connector.SendMessage(c.Request.Context(), req.ToUser, req.Content, req.MsgType, ""); err != nil {
			s.log.Warn("send message error", "error", err, "to_user", req.ToUser)
		}
	}

	c.JSON(200, gin.H{
		"success":    true,
		"message_id": fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		"reply":      reply,
		"backend":    backendID,
	})
}

// handleSendByAccount 通过指定微信账号发送消息
// POST /api/v1/message/send-by-account
// Body: {"account_id": "微信账号ID", "to_user": "接收方UserID", "content": "你好", "msg_type": 1}
func (s *APIServer) handleSendByAccount(c *gin.Context) {
	var req struct {
		AccountID string `json:"account_id"`
		ToUser    string `json:"to_user"`
		Content   string `json:"content"`
		MsgType   int    `json:"msg_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.ToUser == "" || req.Content == "" {
		c.JSON(400, gin.H{"error": "to_user and content are required"})
		return
	}
	if !s.connector.IsRunning() {
		c.JSON(503, gin.H{"error": "clawbot not connected"})
		return
	}
	if req.MsgType == 0 {
		req.MsgType = 1
	}

	// 如果传了 account_id，切换到指定账号发送
	if req.AccountID != "" {
		// 查找对应账号凭证
		creds := s.accountStore.List()
		found := false
		for _, cred := range creds {
			if cred.AccountID == req.AccountID {
				found = true
				break
			}
		}
		if !found {
			c.JSON(404, gin.H{"error": "account not found: " + req.AccountID})
			return
		}
	}

	if err := s.connector.SendMessage(c.Request.Context(), req.ToUser, req.Content, req.MsgType, req.AccountID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if err := s.store.SaveMessage(store.StoredMessage{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		ToUser:    req.ToUser,
		Content:   req.Content,
		MsgType:   req.MsgType,
		AccountID: req.AccountID,
		Timestamp: time.Now().Unix(),
		Direction: "outgoing",
	}); err != nil {
		s.log.Warn("failed to save message", "error", err)
	}

	c.JSON(200, gin.H{"success": true, "message": "sent"})
}

// handleMessageAccounts 获取可发送消息的账号列表
// GET /api/v1/message/accounts
func (s *APIServer) handleMessageAccounts(c *gin.Context) {
	type AccountEntry struct {
		AccountID string `json:"account_id"`
		UserID    string `json:"user_id"`
		LoginAt   int64  `json:"login_at"`
	}
	accounts := make([]AccountEntry, 0)
	for _, a := range s.connector.GetAccounts() {
		accounts = append(accounts, AccountEntry{
			AccountID: a.Credentials.AccountID,
			UserID:    a.Credentials.UserID,
			LoginAt:   a.Credentials.LoginAt,
		})
	}
	c.JSON(200, gin.H{"accounts": accounts})
}

// handlePushBroadcast 广播消息到所有绑定的微信账号
// POST /api/v1/message/broadcast
func (s *APIServer) handlePushBroadcast(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
		MsgType int    `json:"msg_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if !s.connector.IsRunning() {
		c.JSON(503, gin.H{"error": "clawbot not connected"})
		return
	}
	if req.MsgType == 0 {
		req.MsgType = 1
	}

	creds := s.accountStore.List()
	sent := 0
	for _, cred := range creds {
		if err := s.connector.SendMessage(c.Request.Context(), cred.UserID, req.Content, req.MsgType, ""); err != nil {
			s.log.Warn("broadcast send error", "error", err, "account_id", cred.AccountID)
			continue
		}
		sent++
	}

	c.JSON(200, gin.H{"success": true, "sent": sent, "total": len(creds)})
}
