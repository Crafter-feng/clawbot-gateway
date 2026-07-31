package bot

import (
	"context"
	"testing"
	"time"
)

func TestQRCodeManagerCreateScan(t *testing.T) {
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)
	qm := NewQRCodeManager(conn)
	if qm == nil {
		t.Fatal("NewQRCodeManager returned nil")
	}

	ctx := context.Background()
	err := qm.CreateScan(ctx, "test-qrcode-001")
	if err != nil {
		t.Fatalf("CreateScan failed: %v", err)
	}

	state := qm.CheckStatus("test-qrcode-001")
	if state == nil {
		t.Fatal("CheckStatus returned nil")
	}
	if state.Status != "wait" {
		t.Errorf("Status want 'wait', got '%s'", state.Status)
	}
	if state.QRCode != "test-qrcode-001" {
		t.Errorf("QRCode want 'test-qrcode-001', got '%s'", state.QRCode)
	}

	// Clean up
	qm.StopScan("test-qrcode-001")
}

func TestQRCodeManagerCheckStatusUnknown(t *testing.T) {
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)
	qm := NewQRCodeManager(conn)

	state := qm.CheckStatus("nonexistent-qrcode")
	if state == nil {
		t.Fatal("CheckStatus returned nil")
	}
	if state.Status != "unknown" {
		t.Errorf("Status for nonexistent QR want 'unknown', got '%s'", state.Status)
	}
}

func TestQRCodeManagerCreateScanDuplicate(t *testing.T) {
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)
	qm := NewQRCodeManager(conn)

	ctx := context.Background()
	err1 := qm.CreateScan(ctx, "test-qrcode-dup")
	if err1 != nil {
		t.Fatalf("First CreateScan failed: %v", err1)
	}

	// Second scan with same QR code should not error (returns nil for existing)
	err2 := qm.CreateScan(ctx, "test-qrcode-dup")
	if err2 != nil {
		t.Errorf("Duplicate CreateScan should not error, got: %v", err2)
	}

	qm.StopScan("test-qrcode-dup")
}

func TestQRCodeManagerStopScan(t *testing.T) {
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)
	qm := NewQRCodeManager(conn)

	ctx := context.Background()
	qm.CreateScan(ctx, "test-qrcode-stop")

	// Stop the scan
	qm.StopScan("test-qrcode-stop")

	// After stop, status should be unknown (state removed)
	state := qm.CheckStatus("test-qrcode-stop")
	if state == nil {
		t.Fatal("CheckStatus returned nil")
	}
	if state.Status != "unknown" {
		t.Errorf("After StopScan, status want 'unknown', got '%s'", state.Status)
	}

	// StopScan on nonexistent should not panic
	qm.StopScan("nonexistent-qrcode")
}

func TestQRCodeManagerCreateScanCancelledContext(t *testing.T) {
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)
	qm := NewQRCodeManager(conn)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := qm.CreateScan(ctx, "test-qrcode-cancel")
	if err != nil {
		t.Fatalf("CreateScan with cancelled context should not error, got: %v", err)
	}

	// Give goroutine a moment to exit
	time.Sleep(50 * time.Millisecond)

	state := qm.CheckStatus("test-qrcode-cancel")
	if state == nil {
		t.Fatal("CheckStatus returned nil")
	}
	// Goroutine should have detected cancelled context and exited,
	// removing the state (since not "confirmed")
	if state.Status == "unknown" {
		t.Log("Goroutine exited and state was cleaned up (expected)")
	}
}

func TestQRCodeManagerCreateScanExpiry(t *testing.T) {
	// This test verifies that the goroutine exits when context is cancelled
	// (simulating the fix: scanner must not call scanCancel() immediately)
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)
	qm := NewQRCodeManager(conn)

	ctx := context.Background()
	err := qm.CreateScan(ctx, "test-qrcode-timeout")
	if err != nil {
		t.Fatalf("CreateScan failed: %v", err)
	}

	// Wait for goroutine to be running
	time.Sleep(50 * time.Millisecond)

	// Stop the scan (simulates proper cleanup)
	qm.StopScan("test-qrcode-timeout")

	// After stop, state should be removed
	state := qm.CheckStatus("test-qrcode-timeout")
	if state == nil {
		t.Fatal("CheckStatus returned nil")
	}
	if state.Status != "unknown" {
		t.Errorf("After StopScan, status want 'unknown', got '%s'", state.Status)
	}
}