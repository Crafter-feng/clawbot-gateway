package crypto

import (
	"testing"
)

func TestGenerateSecret(t *testing.T) {
	// Test default length
	secret, err := GenerateSecret(0)
	if err != nil {
		t.Fatal("GenerateSecret(0) unexpected error:", err)
	}
	if len(secret) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("GenerateSecret(0) length want 64, got %d", len(secret))
	}

	// Test custom length
	secret, err = GenerateSecret(16)
	if err != nil {
		t.Fatal("GenerateSecret(16) unexpected error:", err)
	}
	if len(secret) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("GenerateSecret(16) length want 32, got %d", len(secret))
	}

	// Test uniqueness
	secret1, _ := GenerateSecret(32)
	secret2, _ := GenerateSecret(32)
	if secret1 == secret2 {
		t.Error("GenerateSecret should produce unique values")
	}
}

func TestGenerateSecretCharacterSet(t *testing.T) {
	secret, err := GenerateSecret(100)
	if err != nil {
		t.Fatal("GenerateSecret(100) unexpected error:", err)
	}
	for _, c := range secret {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("GenerateSecret contains invalid character: %c", c)
			break
		}
	}
}

func TestSecureEqual(t *testing.T) {
	a := "test-secret"
	b := "test-secret"
	c := "different-secret"
	if !SecureEqual(a, b) {
		t.Error("SecureEqual should return true for equal strings")
	}
	if SecureEqual(a, c) {
		t.Error("SecureEqual should return false for different strings")
	}
	if !SecureEqual("", "") {
		t.Error("SecureEqual should return true for empty strings")
	}
}
