package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ── JWT 实现（零外部依赖，HMAC-SHA256） ──

type JWTClaims struct {
	Sub string `json:"sub"` // subject: "admin"
	Exp int64  `json:"exp"` // expiry unix timestamp
	Iat int64  `json:"iat"` // issued at unix timestamp
}

// GenerateJWTSecret 生成随机 JWT 签名密钥
func GenerateJWTSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// SignJWT 签发 JWT token
func SignJWT(secret string, expiry time.Duration) (string, error) {
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

	now := time.Now()
	claims := JWTClaims{
		Sub: "admin",
		Exp: now.Add(expiry).Unix(),
		Iat: now.Unix(),
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	payload := base64URLEncode(claimsJSON)

	signature, err := signHS256(secret, header+"."+payload)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	return header + "." + payload + "." + signature, nil
}

// VerifyJWT 验证 JWT token，返回 claims 或错误
func VerifyJWT(secret, tokenString string) (*JWTClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	header, payload, signature := parts[0], parts[1], parts[2]

	// 验证签名
	expectedSig, err := signHS256(secret, header+"."+payload)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// 解析 claims
	claimsJSON, err := base64URLDecode(payload)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	// 验证过期
	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

func signHS256(secret, data string) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return base64URLEncode(mac.Sum(nil)), nil
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func base64URLDecode(s string) ([]byte, error) {
	// 补齐 padding
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}
