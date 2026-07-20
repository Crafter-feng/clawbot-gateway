package crypto

import (
	"testing"
)

func TestGenerateSecret(t *testing.T) {
	// Test default length
	secret := GenerateSecret(0)
	if len(secret) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("GenerateSecret(0) length want 64, got %d", len(secret))
	}

	// Test custom length
	secret = GenerateSecret(16)
	if len(secret) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("GenerateSecret(16) length want 32, got %d", len(secret))
	}

	// Test uniqueness
	secret1 := GenerateSecret(32)
	secret2 := GenerateSecret(32)
	if secret1 == secret2 {
		t.Error("GenerateSecret should produce unique values")
	}
}

func TestGenerateSecretCharacterSet(t *testing.T) {
	secret := GenerateSecret(100)
	for _, c := range secret {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("GenerateSecret contains invalid character: %c", c)
			break
		}
	}
}
