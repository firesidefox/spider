package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/models"
)

type MessageStore struct {
	db *sql.DB
}

func NewMessageStore(db *sql.DB) *MessageStore {
	return &MessageStore{db: db}
}

func (s *MessageStore) Save(conversationID, role, content, toolCalls string) error {
	_, err := s.db.Exec(
		"INSERT INTO messages (id, conversation_id, role, content, tool_calls, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		uuid.New().String(), conversationID, role, content, toolCalls, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

func (s *MessageStore) ListByConversation(conversationID string) ([]*models.Message, error) {
	rows, err := s.db.Query(
		"SELECT id, conversation_id, role, content, tool_calls, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC",
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	var list []*models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ToolCalls, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		list = append(list, &m)
	}
	return list, nil
}

func (s *MessageStore) DeleteByConversation(conversationID string) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE conversation_id = ?", conversationID)
	return err
}
