package models

import "time"

// SSHKey 表示一个 SSH 密钥。
type SSHKey struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id"`
	Name                string    `json:"name"`
	EncryptedPrivateKey string    `json:"-"`
	EncryptedPassphrase string    `json:"-"`
	Fingerprint         string    `json:"fingerprint"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// SafeSSHKey 是对外展示的安全版本（不含私钥）。
type SafeSSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Safe 返回不含敏感字段的版本。
func (k *SSHKey) Safe() *SafeSSHKey {
	return &SafeSSHKey{
		ID:          k.ID,
		Name:        k.Name,
		Fingerprint: k.Fingerprint,
		CreatedAt:   k.CreatedAt,
		UpdatedAt:   k.UpdatedAt,
	}
}

// AddSSHKeyRequest 是添加 SSH 密钥的请求参数。
type AddSSHKeyRequest struct {
	Name       string `json:"name"`
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase"`
}
