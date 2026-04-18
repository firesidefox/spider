package models

import "time"

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

type User struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Password  string     `json:"-"` // bcrypt hash，不序列化
	Role      Role       `json:"role"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	LastLogin *time.Time `json:"last_login"`
}

type ApiToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	TokenHash string     `json:"-"` // SHA-256 hex，不序列化
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used"`
}

// UserInfo 是对外展示的用户信息（不含密码）
type UserInfo struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Role      Role       `json:"role"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	LastLogin *time.Time `json:"last_login"`
}

// TokenInfo 是对外展示的 Token 信息（不含明文）
type TokenInfo struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used"`
}

func (u *User) ToInfo() *UserInfo {
	return &UserInfo{
		ID:        u.ID,
		Username:  u.Username,
		Role:      u.Role,
		Enabled:   u.Enabled,
		CreatedAt: u.CreatedAt,
		LastLogin: u.LastLogin,
	}
}

func (t *ApiToken) ToInfo() *TokenInfo {
	return &TokenInfo{
		ID:        t.ID,
		Name:      t.Name,
		ExpiresAt: t.ExpiresAt,
		CreatedAt: t.CreatedAt,
		LastUsed:  t.LastUsed,
	}
}
