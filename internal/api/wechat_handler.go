package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/store"
)

// ══════════════════════════════════════════════════════════════════════
//  微信扫码登录 + 账号管理
// ══════════════════════════════════════════════════════════════════════

func (s *APIServer) handleGetQRCode(c *gin.Context) {
	qrData, err := s.connector.GetQRCode(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": err.Error()})
		return
	}
	_ = s.connector.QRManager().CreateScan(context.Background(), qrData.QRCode)
	c.JSON(200, gin.H{
		"success": true,
		"data": gin.H{
			"qrcode_url": qrData.QRCodeURL,
			"qrcode":     qrData.QRCode,
		},
	})
}

func (s *APIServer) handleQRStatus(c *gin.Context) {
	var req struct {
		QRCode string `json:"qrcode" form:"qrcode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "missing qrcode"})
		return
	}

	state := s.connector.QRManager().CheckStatus(req.QRCode)
	if state.Status == "unknown" {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"status": "expired"}})
		return
	}

	if state.Status == "scaned_but_redirect" {
		c.JSON(200, gin.H{"success": true, "data": gin.H{
			"status":        "scaned_but_redirect",
			"redirect_host": state.RedirectHost,
		}})
		return
	}

	if state.Status == "confirmed" && state.Creds != nil {
		creds := state.Creds
		_ = s.accountStore.Save(store.StoredCredential{
			AccountID: creds.AccountID,
			Token:     creds.Token,
			BaseURL:   creds.BaseURL,
			UserID:    creds.UserID,
			LoginAt:   creds.LoginAt,
		})
		if err := s.connector.AddAccount(context.Background(), creds); err != nil {
			if s.connector.AccountExists(creds.AccountID) {
				s.log.Info("account already exists, re-using", "account_id", creds.AccountID)
			} else {
				c.JSON(200, gin.H{"success": true, "data": gin.H{"status": "expired", "error": err.Error()}})
				return
			}
		}
		c.JSON(200, gin.H{
			"success": true,
			"data": gin.H{
				"status": "confirmed",
				"account_id": creds.AccountID,
				"user_id":    creds.UserID,
				"baseurl":    creds.BaseURL,
			},
		})
		return
	}

	c.JSON(200, gin.H{"success": true, "data": gin.H{"status": state.Status}})
}

func (s *APIServer) handleListAccounts(c *gin.Context) {
	accounts := s.connector.GetAccounts()
	result := make([]gin.H, 0)
	for _, a := range accounts {
		result = append(result, gin.H{
			"account_id": a.Credentials.AccountID,
			"user_id":    a.Credentials.UserID,
			"login_at":   a.Credentials.LoginAt,
			"status":     a.Status,
		})
	}
	c.JSON(200, gin.H{"success": true, "accounts": result})
}

func (s *APIServer) handleDisconnectAccount(c *gin.Context) {
	id := c.Param("id")
	if err := s.connector.RemoveAccount(id); err != nil {
		c.JSON(404, gin.H{"success": false, "error": err.Error()})
		return
	}
	_ = s.accountStore.Remove(id)
	s.log.Info("account disconnected", "account_id", id)
	c.JSON(200, gin.H{"success": true, "message": "disconnected"})
}
