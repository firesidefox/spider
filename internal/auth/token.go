package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const tokenPrefix = "spd_"

// Generate 生成 API Token 明文：spd_ + 32字节随机 hex
func Generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return tokenPrefix + hex.EncodeToString(b), nil
}

// Hash 计算 token 的 SHA-256 hex（用于存储）
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// IsAPIToken 判断是否为 API Token（spd_ 前缀）
func IsAPIToken(token string) bool {
	return len(token) > len(tokenPrefix) && token[:len(tokenPrefix)] == tokenPrefix
}
