package api

import (
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
		s.log.Warn("bad request: push send", "error", err)
		c.JSON(400, gin.H{"error": "请求参数错误"})
		return
	}

	accounts, err := s.db.ListAccounts()
	if err != nil || len(accounts) == 0 {
		c.JSON(500, gin.H{"error": "no accounts available"})
		return
	}

	var sent bool
	for _, acct := range accounts {
		if req.AccountID != "" && acct.AccountID != req.AccountID {
			continue
		}
		if acct.Token == "" || acct.BaseURL == "" {
			continue
		}
		sent = true
		err = s.connector.SendTextWithCreds(c.Request.Context(), &bot.Credentials{
			Token:   acct.Token,
			BaseURL: acct.BaseURL,
		}, req.ToUser, req.Content, "")
		if err != nil {
			s.log.Warn("failed to send message via connector", "account_id", acct.AccountID, "to_user", req.ToUser, "error", err)
		}
	}

	if !sent {
		c.JSON(404, gin.H{"error": "account not found"})
		return
	}
	c.JSON(200, gin.H{"success": true})
}