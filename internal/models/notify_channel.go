package models

import "time"

// NotifyChannelType identifies the notification backend.
type NotifyChannelType string

const (
	NotifyChannelDingTalk NotifyChannelType = "dingtalk"
)

// NotifyChannel stores a notification destination (e.g. DingTalk webhook).
// Config is stored encrypted; the store handles encrypt/decrypt.
type NotifyChannel struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Type      NotifyChannelType `json:"type"`
	Config    string `json:"config,omitempty"` // JSON blob, decrypted at read time
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// DingTalkConfig holds the fields for a DingTalk webhook channel.
type DingTalkConfig struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret,omitempty"` // optional signing secret
}
