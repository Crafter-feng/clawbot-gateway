package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
)

// GenerateSecret 生成加密安全的随机密钥
func GenerateSecret(length int) (string, error) {
	if length <= 0 {
		length = 32
	}
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// SecureEqual 使用 constant-time 方式比较两个字符串
func SecureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// MustGenerateSecret 生成密钥，失败时 panic（适用于启动期初始化）
func MustGenerateSecret(length int) string {
	s, err := GenerateSecret(length)
	if err != nil {
		panic("crypto: " + err.Error())
	}
	return s
}
