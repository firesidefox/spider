package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/models"
)

type ConversationStore struct {
	db *sql.DB
}

func NewConversationStore(db *sql.DB) *ConversationStore {
	return &ConversationStore{db: db}
}

func (s *ConversationStore) Create(userID, title string) (*models.Conversation, error) {
	now := time.Now()
	if title == "" {
		title = now.Format("2006-01-02-1504")
	}
	conv := &models.Conversation{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     title,
		Status:    "idle",
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Exec(
		"INSERT INTO conversations (id, user_id, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		conv.ID, conv.UserID, conv.Title, conv.CreatedAt, conv.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert conversation: %w", err)
	}
	logger.Global().Debug().Str("conv_id", conv.ID).Str("user_id", userID).Msg("store: conversation created")
	return conv, nil
}

func (s *ConversationStore) GetByID(id string) (*models.Conversation, error) {
	row := s.db.QueryRow(
		"SELECT id, user_id, title, status, permission_mode, created_at, updated_at FROM conversations WHERE id = ?", id,
	)
	var c models.Conversation
	err := row.Scan(&c.ID, &c.UserID, &c.Title, &c.Status, &c.PermissionMode, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("scan conversation: %w", err)
	}
	logger.Global().Debug().Str("conv_id", id).Msg("store: conversation fetched")
	return &c, nil
}

func (s *ConversationStore) ListByUser(userID string) ([]*models.Conversation, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, title, status, permission_mode, created_at, updated_at FROM conversations WHERE user_id = ? ORDER BY updated_at DESC",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()
	list := make([]*models.Conversation, 0)
	for rows.Next() {
		var c models.Conversation
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.Status, &c.PermissionMode, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan conversation row: %w", err)
		}
		list = append(list, &c)
	}
	logger.Global().Debug().Str("user_id", userID).Int("count", len(list)).Msg("store: conversations listed")
	return list, nil
}

func (s *ConversationStore) UpdateTitle(id, title string) error {
	_, err := s.db.Exec(
		"UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?",
		title, time.Now().UTC(), id,
	)
	return err
}

func (s *ConversationStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return err
	}
	logger.Global().Debug().Str("conv_id", id).Msg("store: conversation deleted")
	return nil
}

func (s *ConversationStore) UpdatePermissionMode(id, mode string) error {
	_, err := s.db.Exec(
		`UPDATE conversations SET permission_mode = ?, updated_at = ? WHERE id = ?`,
		mode, time.Now().UTC(), id,
	)
	return err
}

func (s *ConversationStore) SetStatus(id, status string) error {
	_, err := s.db.Exec(
		"UPDATE conversations SET status = ?, updated_at = ? WHERE id = ?",
		status, time.Now().UTC(), id,
	)
	if err != nil {
		return err
	}
	logger.Global().Debug().Str("conv_id", id).Str("status", status).Msg("store: conversation status set")
	return nil
}
