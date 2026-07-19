package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/bot"
)

func (s *APIServer) handlePushSend(c *gin.Context) {
	var req struct {
		ToUser    string `json:"to_user"`
		Content   string `json:"content"`
		AccountID string `json:"account_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	accounts, err := s.db.ListAccounts()
	if err != nil || len(accounts) == 0 {
		c.JSON(500, gin.H{"error": "no accounts available"})
		return
	}

	for _, acct := range accounts {
		if req.AccountID == "" || acct.AccountID == req.AccountID {
			err = s.connector.SendTextWithCreds(context.Background(), &bot.Credentials{
				Token:   acct.Token,
				BaseURL: acct.BaseURL,
			}, req.ToUser, req.Content, "")
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"success": true})
			return
		}
	}

	c.JSON(404, gin.H{"error": "account not found"})
}
