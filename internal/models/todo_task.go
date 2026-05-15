package models

import "time"

type Todo struct {
	ID             int64     `json:"id"`
	Seq            int64     `json:"seq"`
	ConversationID string    `json:"conversation_id"`
	Subject        string    `json:"subject"`
	ActiveForm     string    `json:"active_form,omitempty"`
	Description    string    `json:"description,omitempty"`
	Status         string    `json:"status"`
	Owner          string    `json:"owner,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
