package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/database"
)
// handleGetQRCode 获取微信登录二维码（通过 Connector 调用真实 iLink API）
func (s *APIServer) handleGetQRCode(c *gin.Context) {
	qrManager := s.connector.QRManager()
	if qrManager == nil {
		c.JSON(500, gin.H{"error": "QR manager not initialized"})
		return
	}

	// 获取 iLink base URL
	creds := s.connector.GetCredentials()
	if creds == nil {
		accounts, _ := s.db.ListAccounts()
		if len(accounts) > 0 {
			creds = &bot.Credentials{BaseURL: accounts[0].BaseURL}
		} else {
			creds = &bot.Credentials{BaseURL: s.config.ClawBot.BaseURL}
		}
	}

	// 调用真实 iLink API 获取二维码
	qrData, err := s.connector.GetQRCode(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("获取二维码失败: %v", err)})
		return
	}

	// 启动二维码状态轮询（使用独立 context，不随请求结束）
	scanCtx, scanCancel := context.WithCancel(context.Background())
	defer scanCancel()
	if err := qrManager.CreateScan(scanCtx, qrData.QRCode); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("创建扫描会话失败: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"data": gin.H{
			"qrcode_url": qrData.QRCodeURL,
			"qrcode":     qrData.QRCode,
		},
	})
}

// handleQRStatus 检查二维码状态
func (s *APIServer) handleQRStatus(c *gin.Context) {
	var req struct {
		QRCode string `json:"qrcode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	qrManager := s.connector.QRManager()
	state := qrManager.CheckStatus(req.QRCode)

	resp := gin.H{
		"success": true,
		"data": gin.H{
			"status": state.Status,
		},
	}

	// 如果已确认，保存账号
	if state.Status == "confirmed" && state.Creds != nil {
		acct := database.Account{
			AccountID: state.Creds.AccountID,
			UserID:    state.Creds.UserID,
			Token:     state.Creds.Token,
			BaseURL:   state.Creds.BaseURL,
			LoginAt:   time.Now().Unix(),
		}
		if err := s.db.SaveAccount(acct); err != nil {
			s.log.Warn("save account failed", "error", err)
		}

		if err := s.connector.AddAccount(context.Background(), state.Creds); err != nil {
			s.log.Warn("add account failed", "error", err)
		}

		resp["data"].(gin.H)["account_id"] = state.Creds.AccountID
		resp["data"].(gin.H)["user_id"] = state.Creds.UserID
	}

	c.JSON(200, resp)
}

// handleDisconnectAccount 解绑微信账号
func (s *APIServer) handleDisconnectAccount(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.DeleteAccount(id); err != nil {
		log.Printf("ERROR: failed to delete account %s: %v", id, err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	s.connector.RemoveAccount(id)
	c.JSON(200, gin.H{"success": true})
}

// handleListWechatAccounts 列出微信账号
func (s *APIServer) handleListWechatAccounts(c *gin.Context) {
	accounts, err := s.db.ListAccounts()
	if err != nil {
		log.Printf("ERROR: failed to list wechat accounts: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	result := make([]gin.H, 0)
	for _, acct := range accounts {
		status := "offline"
		for _, a := range s.connector.GetAccounts() {
			if a.Credentials.AccountID == acct.AccountID {
				status = "online"
				break
			}
		}
		result = append(result, gin.H{
			"account_id": acct.AccountID,
			"user_id":    acct.UserID,
			"login_at":   acct.LoginAt,
			"status":     status,
		})
	}

	c.JSON(200, gin.H{"accounts": result})
}

// handleSaveWechatAccount 保存微信账号
func (s *APIServer) handleSaveWechatAccount(c *gin.Context) {
	var req struct {
		AccountID   string `json:"account_id"`
		UserID      string `json:"user_id"`
		Token       string `json:"token"`
		BaseURL     string `json:"base_url"`
		AccountName string `json:"account_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	acct := database.Account{
		AccountID:   req.AccountID,
		UserID:      req.UserID,
		Token:       req.Token,
		BaseURL:     req.BaseURL,
		AccountName: req.AccountName,
	}
	if err := s.db.SaveAccount(acct); err != nil {
		log.Printf("ERROR: failed to save wechat account: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	creds := &bot.Credentials{
		Token:     req.Token,
		BaseURL:   req.BaseURL,
		AccountID: req.AccountID,
		UserID:    req.UserID,
	}
	if err := s.connector.AddAccount(c.Request.Context(), creds); err != nil {
		s.log.Warn("add account failed", "error", err)
	}

	c.JSON(200, gin.H{"success": true})
}
