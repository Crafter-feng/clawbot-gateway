// ── 自定义 JWT 实现 ──
// 这是一个轻量级的自定义 JWT 实现，用于替代第三方库。
// 使用 HMAC-SHA256 签名，支持 HS256 算法。
// 密钥通过配置文件动态加载，而非硬编码。
// 注意：这不是标准 JWT 库的完整实现，仅满足本项目的认证需求。

package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type JWTClaims struct {
	Exp int64  `json:"exp"`
	Iat int64  `json:"iat"`
	Sub string `json:"sub"`
	Jti string `json:"jti"` // JWT ID - 随机生成，确保唯一性
}

func GenerateJWT(secret string, expiryHours int) (string, error) {
	now := time.Now()
	iat := now.Unix()
	// 当 expiryHours <= 0 时，生成已过期的 token（用于测试）
	exp := iat
	if expiryHours > 0 {
		exp = now.Add(time.Duration(expiryHours) * time.Hour).Unix()
	} else {
		exp = iat - 3600 // 1小时前已过期
	}
	// 随机 jti 确保 token 唯一性
	jtiBytes := make([]byte, 8)
	if _, err := rand.Read(jtiBytes); err != nil {
		jtiBytes = []byte(fmt.Sprintf("%d", now.UnixNano()))
	}
	claims := JWTClaims{
		Exp: exp,
		Iat: iat,
		Sub: "admin",
		Jti: fmt.Sprintf("%x", jtiBytes),
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(claims)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)

	signingInput := header + "." + payloadB64
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

func VerifyJWT(secret, tokenStr string) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expectedSig)) != 1 {
		return nil, fmt.Errorf("invalid signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}
