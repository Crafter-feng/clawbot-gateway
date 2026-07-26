package api

import (
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	os.Unsetenv("CLAWBOT_LOGIN_PASSWORD")
	m.Run()
}

func TestGenerateJWT(t *testing.T) {
	secret := "test-secret-key"
	token, err := GenerateJWT(secret, 1)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateJWT returned empty token")
	}

	claims, err := VerifyJWT(secret, token)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}
	if claims.Sub != "admin" {
		t.Errorf("Sub want 'admin', got '%s'", claims.Sub)
	}
	if claims.Iat == 0 {
		t.Error("Iat should not be zero")
	}
	if claims.Exp <= claims.Iat {
		t.Error("Exp should be after Iat")
	}
}

func TestGenerateJWTExpiry(t *testing.T) {
	secret := "test-secret"
	token, err := GenerateJWT(secret, 24) // 24 hours
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	claims, err := VerifyJWT(secret, token)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}

	expectedExp := claims.Iat + 24*3600
	if claims.Exp != expectedExp {
		t.Errorf("Exp want %d (Iat+24h), got %d", expectedExp, claims.Exp)
	}
}

func TestVerifyJWTInvalidSignature(t *testing.T) {
	secret := "test-secret"
	token, err := GenerateJWT(secret, 1)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	_, err = VerifyJWT("wrong-secret", token)
	if err == nil {
		t.Error("VerifyJWT with wrong secret should fail")
	}
}

func TestVerifyJWTInvalidFormat(t *testing.T) {
	_, err := VerifyJWT("secret", "invalid-token-format")
	if err == nil {
		t.Error("VerifyJWT with invalid format should fail")
	}
}

func TestVerifyJWTExpiredToken(t *testing.T) {
	secret := "test-secret"
	// Generate a token with 0 expiry (already expired)
	token, err := GenerateJWT(secret, 0)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	// Small sleep to ensure expiry is in the past
	time.Sleep(time.Millisecond)

	_, err = VerifyJWT(secret, token)
	if err == nil {
		t.Error("VerifyJWT with expired token should fail")
	}
}

func TestJWTClaims(t *testing.T) {
	claims := JWTClaims{
		Exp: 1000,
		Iat: 500,
		Sub: "test-user",
	}
	if claims.Sub != "test-user" {
		t.Errorf("Sub want 'test-user', got '%s'", claims.Sub)
	}
	if claims.Exp != 1000 {
		t.Errorf("Exp want 1000, got %d", claims.Exp)
	}
	if claims.Iat != 500 {
		t.Errorf("Iat want 500, got %d", claims.Iat)
	}
}

func TestGenerateJWTUnique(t *testing.T) {
	secret := "test-secret"
	token1, _ := GenerateJWT(secret, 1)
	token2, _ := GenerateJWT(secret, 1)
	if token1 == token2 {
		t.Error("GenerateJWT should produce unique tokens due to different Iat")
	}
}