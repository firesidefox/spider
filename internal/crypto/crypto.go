package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const keyFile = "master.key"

// Manager 负责 AES-256-GCM 加解密，密钥存储在 dataDir/master.key。
type Manager struct {
	key []byte
}

// NewManager 加载或生成 master.key。
func NewManager(dataDir string) (*Manager, error) {
	keyPath := filepath.Join(dataDir, keyFile)

	// 尝试加载已有密钥
	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) == 32 {
		return &Manager{key: data}, nil
	}

	// 首次运行：生成 32 字节随机密钥
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("生成 master key 失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 写入密钥文件（仅 owner 可读写）
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("写入 master key 失败: %w", err)
	}
	return &Manager{key: key}, nil
}

// Encrypt 使用 AES-256-GCM 加密明文，返回 base64(nonce+ciphertext)。
func (m *Manager) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return "", fmt.Errorf("创建 AES cipher 失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 Encrypt 返回的 base64 字符串。
func (m *Manager) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return "", fmt.Errorf("创建 AES cipher 失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("密文太短")
	}
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("AES-GCM 解密失败: %w", err)
	}
	return string(plaintext), nil
}
