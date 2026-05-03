package models

import "time"

type Conversation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	ToolCalls      string    `json:"tool_calls,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type PendingConfirmation struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	ToolName       string     `json:"tool_name"`
	ToolInput      string     `json:"tool_input"`
	RiskLevel      string     `json:"risk_level"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}
