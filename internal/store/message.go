package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/logger"
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
	logger.Global().Debug().Str("table", "messages").Str("op", "insert").Str("conv_id", conversationID).Str("role", role).Msg("store")
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
	logger.Global().Debug().Str("table", "messages").Str("op", "select").Str("conv_id", conversationID).Int("count", len(list)).Msg("store")
	return list, nil
}

func (s *MessageStore) DeleteByConversation(conversationID string) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE conversation_id = ?", conversationID)
	if err != nil {
		return err
	}
	logger.Global().Debug().Str("table", "messages").Str("op", "delete").Str("conv_id", conversationID).Msg("store")
	return nil
}

func (s *MessageStore) ListAfterMessage(conversationID, messageID string) ([]*models.Message, error) {
	if messageID == "" {
		return s.ListByConversation(conversationID)
	}
	rows, err := s.db.Query(`
		SELECT id, conversation_id, role, content, tool_calls, created_at
		FROM messages
		WHERE conversation_id = ?
		  AND rowid > (SELECT rowid FROM messages WHERE id = ?)
		ORDER BY rowid ASC`,
		conversationID, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []*models.Message
	for rows.Next() {
		m := &models.Message{}
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ToolCalls, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
