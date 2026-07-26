package api

import (
	"log"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/database"
)

func (s *APIServer) handleListAccounts(c *gin.Context) {
	accounts, err := s.db.ListAccounts()
	if err != nil {
		log.Printf("ERROR: failed to list accounts: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, gin.H{"accounts": accounts})
}

func (s *APIServer) handleAddAccount(c *gin.Context) {
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
		log.Printf("ERROR: failed to save account: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleDeleteAccount(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.DeleteAccount(id); err != nil {
		log.Printf("ERROR: failed to delete account %s: %v", id, err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, gin.H{"success": true})
}
