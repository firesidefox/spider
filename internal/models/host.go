package models

import "time"

// AuthType 定义 SSH 认证方式。
type AuthType string

const (
	AuthPassword    AuthType = "password"
	AuthKey         AuthType = "key"
	AuthKeyPassword AuthType = "key_password" // 带 passphrase 的私钥
)

// Host 表示一台被管理的远程主机。
type Host struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	IP                   string    `json:"ip"`
	Port                 int       `json:"port"`
	Username             string    `json:"username"`
	AuthType             AuthType  `json:"auth_type"`
	// EncryptedCredential 是 AES-GCM 加密后的 base64 字符串（密码或私钥内容）。
	EncryptedCredential  string    `json:"-"`
	// EncryptedPassphrase 仅 key_password 模式使用。
	EncryptedPassphrase  string    `json:"-"`
	ProxyHostID          string    `json:"proxy_host_id,omitempty"` // 跳板机 FK
	Tags                 []string  `json:"tags"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// SafeHost 是对外展示的安全版本（不含凭据）。
type SafeHost struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	IP          string    `json:"ip"`
	Port        int       `json:"port"`
	Username    string    `json:"username"`
	AuthType    AuthType  `json:"auth_type"`
	ProxyHostID string    `json:"proxy_host_id,omitempty"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Safe 返回不含敏感字段的版本。
func (h *Host) Safe() *SafeHost {
	return &SafeHost{
		ID:          h.ID,
		Name:        h.Name,
		IP:          h.IP,
		Port:        h.Port,
		Username:    h.Username,
		AuthType:    h.AuthType,
		ProxyHostID: h.ProxyHostID,
		Tags:        h.Tags,
		CreatedAt:   h.CreatedAt,
		UpdatedAt:   h.UpdatedAt,
	}
}

// AddHostRequest 是添加主机的请求参数。
type AddHostRequest struct {
	Name        string   `json:"name"`
	IP          string   `json:"ip"`
	Port        int      `json:"port"`
	Username    string   `json:"username"`
	AuthType    AuthType `json:"auth_type"`
	Credential  string   `json:"credential"`   // 明文密码 or 私钥内容
	Passphrase  string   `json:"passphrase"`   // 私钥 passphrase（可选）
	ProxyHostID string   `json:"proxy_host_id"`
	Tags        []string `json:"tags"`
}

// UpdateHostRequest 是更新主机的请求参数（所有字段可选）。
type UpdateHostRequest struct {
	Name        *string   `json:"name"`
	IP          *string   `json:"ip"`
	Port        *int      `json:"port"`
	Username    *string   `json:"username"`
	AuthType    *AuthType `json:"auth_type"`
	Credential  *string   `json:"credential"`
	Passphrase  *string   `json:"passphrase"`
	ProxyHostID *string   `json:"proxy_host_id"`
	Tags        []string  `json:"tags"`
}
