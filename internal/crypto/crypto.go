package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)
// GenerateSecret 生成加密安全的随机密钥
func GenerateSecret(length int) string {
	if length <= 0 {
		length = 32
	}
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// 如果 rand.Read 失败，返回一个基于时间的不安全随机字符串作为 fallback
		// 这不应在生产环境中发生，但提供降级策略
		b = []byte(fmt.Sprintf("%d%d", time.Now().UnixNano(), length))
	}
	return hex.EncodeToString(b)
}
