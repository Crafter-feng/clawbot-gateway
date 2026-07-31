package bot

import (
	"testing"
)

func TestParseQRStatusResponseWait(t *testing.T) {
	body := []byte(`{"status":"wait"}`)
	status, creds, redirectHost := parseQRStatusResponse(body, "https://default.com")
	if status != "wait" {
		t.Errorf("status want 'wait', got '%s'", status)
	}
	if creds != nil {
		t.Error("creds should be nil for wait status")
	}
	if redirectHost != "" {
		t.Errorf("redirectHost want empty, got '%s'", redirectHost)
	}
}

func TestParseQRStatusResponseScanned(t *testing.T) {
	body := []byte(`{"status":"scanned","redirect_host":"scan.weixin.qq.com"}`)
	status, creds, redirectHost := parseQRStatusResponse(body, "https://default.com")
	if status != "scanned" {
		t.Errorf("status want 'scanned', got '%s'", status)
	}
	if creds != nil {
		t.Error("creds should be nil for scanned status")
	}
	if redirectHost != "scan.weixin.qq.com" {
		t.Errorf("redirectHost want 'scan.weixin.qq.com', got '%s'", redirectHost)
	}
}

func TestParseQRStatusResponseConfirmed(t *testing.T) {
	body := []byte(`{"status":"confirmed","bot_token":"test-token","ilink_bot_id":"bot001","ilink_user_id":"user001","baseurl":"https://ilink.weixin.qq.com"}`)
	status, creds, redirectHost := parseQRStatusResponse(body, "https://default.com")
	if status != "confirmed" {
		t.Errorf("status want 'confirmed', got '%s'", status)
	}
	if creds == nil {
		t.Fatal("creds should not be nil for confirmed status")
	}
	if creds.Token != "test-token" {
		t.Errorf("Token want 'test-token', got '%s'", creds.Token)
	}
	if creds.AccountID != "bot001" {
		t.Errorf("AccountID want 'bot001', got '%s'", creds.AccountID)
	}
	if creds.UserID != "user001" {
		t.Errorf("UserID want 'user001', got '%s'", creds.UserID)
	}
	if creds.BaseURL != "https://ilink.weixin.qq.com" {
		t.Errorf("BaseURL want 'https://ilink.weixin.qq.com', got '%s'", creds.BaseURL)
	}
	if redirectHost != "" {
		t.Errorf("redirectHost want empty, got '%s'", redirectHost)
	}
}

func TestParseQRStatusResponseConfirmedFallbackBaseURL(t *testing.T) {
	// When baseurl is not provided, should use defaultBaseURL
	body := []byte(`{"status":"confirmed","bot_token":"test-token","ilink_bot_id":"bot001","ilink_user_id":"user001"}`)
	status, creds, _ := parseQRStatusResponse(body, "https://default.weixin.qq.com")
	if status != "confirmed" {
		t.Errorf("status want 'confirmed', got '%s'", status)
	}
	if creds == nil {
		t.Fatal("creds should not be nil")
	}
	if creds.BaseURL != "https://default.weixin.qq.com" {
		t.Errorf("BaseURL want 'https://default.weixin.qq.com', got '%s'", creds.BaseURL)
	}
}

func TestParseQRStatusResponseExpired(t *testing.T) {
	body := []byte(`{"status":"expired"}`)
	status, creds, redirectHost := parseQRStatusResponse(body, "https://default.com")
	if status != "expired" {
		t.Errorf("status want 'expired', got '%s'", status)
	}
	if creds != nil {
		t.Error("creds should be nil for expired status")
	}
	if redirectHost != "" {
		t.Errorf("redirectHost want empty, got '%s'", redirectHost)
	}
}

func TestParseQRStatusResponseInvalidJSON(t *testing.T) {
	body := []byte(`not-json`)
	status, creds, redirectHost := parseQRStatusResponse(body, "https://default.com")
	if status != "wait" {
		t.Errorf("For invalid JSON, status want 'wait', got '%s'", status)
	}
	if creds != nil {
		t.Error("creds should be nil for invalid JSON")
	}
	if redirectHost != "" {
		t.Errorf("redirectHost want empty, got '%s'", redirectHost)
	}
}

func TestParseQRStatusResponseQRCodeResponseFormat(t *testing.T) {
	// The flat format (status at top level) is what parseQRStatusResponse handles
	body := []byte(`{"status":"confirmed","bot_token":"alt-token","ilink_bot_id":"alt-bot","ilink_user_id":"alt-user","baseurl":"https://alt.weixin.qq.com"}`)
	status, creds, _ := parseQRStatusResponse(body, "https://default.com")
	if status != "confirmed" {
		t.Errorf("status want 'confirmed', got '%s'", status)
	}
	if creds == nil {
		t.Fatal("creds should not be nil")
	}
	if creds.Token != "alt-token" {
		t.Errorf("Token want 'alt-token', got '%s'", creds.Token)
	}
	if creds.AccountID != "alt-bot" {
		t.Errorf("AccountID want 'alt-bot', got '%s'", creds.AccountID)
	}
}

func TestParseQRStatusResponseQRCodeResponseFormatFailed(t *testing.T) {
	// Invalid JSON should return "wait"
	body := []byte(`{invalid}`)
	status, creds, _ := parseQRStatusResponse(body, "https://default.com")
	if status != "wait" {
		t.Errorf("For invalid JSON, status want 'wait', got '%s'", status)
	}
	if creds != nil {
		t.Error("creds should be nil")
	}
}

func TestParseQRStatusResponseScanButRedirect(t *testing.T) {
	body := []byte(`{"status":"scaned_but_redirect","redirect_host":"new.weixin.qq.com"}`)
	status, creds, redirectHost := parseQRStatusResponse(body, "https://default.com")
	if status != "scaned_but_redirect" {
		t.Errorf("status want 'scaned_but_redirect', got '%s'", status)
	}
	if creds != nil {
		t.Error("creds should be nil")
	}
	if redirectHost != "new.weixin.qq.com" {
		t.Errorf("redirectHost want 'new.weixin.qq.com', got '%s'", redirectHost)
	}
}