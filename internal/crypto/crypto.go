package crypto

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateSecret 生成加密安全的随机密钥
func GenerateSecret(length int) string {
	if length <= 0 {
		length = 32
	}
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}
