package models

import "time"

type TodoTask struct {
	ID             int64     `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Subject        string    `json:"subject"`
	Description    string    `json:"description,omitempty"`
	Status         string    `json:"status"`
	Owner          string    `json:"owner,omitempty"`
	BlockedBy      []int64   `json:"blocked_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
