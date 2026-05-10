package store

import (
	"database/sql"
	"encoding/json"
	"time"
)

type ConversationSummary struct {
	ID             int64
	ConversationID string
	UpToMessageID  string
	Chunks         []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type SummaryStore struct {
	db *sql.DB
}

func NewSummaryStore(db *sql.DB) *SummaryStore {
	return &SummaryStore{db: db}
}

// Get 取摘要缓存，不存在返回 nil, nil
func (s *SummaryStore) Get(conversationID string) (*ConversationSummary, error) {
	row := s.db.QueryRow(`
		SELECT id, conversation_id, up_to_message_id, chunks, created_at, updated_at
		FROM conversation_summaries
		WHERE conversation_id = ?`, conversationID)
	var cs ConversationSummary
	var chunksJSON string
	err := row.Scan(&cs.ID, &cs.ConversationID, &cs.UpToMessageID, &chunksJSON, &cs.CreatedAt, &cs.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(chunksJSON), &cs.Chunks); err != nil {
		return nil, err
	}
	return &cs, nil
}

// Upsert 写入或更新摘要缓存
func (s *SummaryStore) Upsert(conversationID, upToMessageID string, chunks []string) error {
	chunksJSON, err := json.Marshal(chunks)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, up_to_message_id, chunks)
		VALUES (?, ?, ?)
		ON CONFLICT(conversation_id) DO UPDATE SET
			up_to_message_id = excluded.up_to_message_id,
			chunks           = excluded.chunks,
			updated_at       = CURRENT_TIMESTAMP`,
		conversationID, upToMessageID, string(chunksJSON))
	return err
}
