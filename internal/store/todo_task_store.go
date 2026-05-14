package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/logger"

	"github.com/spiderai/spider/internal/models"
)

type TodoStore struct {
	db *sql.DB
	mu sync.Mutex
}

func NewTodoStore(db *sql.DB) *TodoStore {
	return &TodoStore{db: db}
}

func (s *TodoStore) Create(task *models.Todo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	blockedBy, _ := json.Marshal(task.BlockedBy)
	if blockedBy == nil {
		blockedBy = []byte("[]")
	}
	now := time.Now().UTC()
	res, err := s.db.Exec(
		`INSERT INTO todo_tasks (conversation_id, turn_id, subject, description, status, owner, blocked_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ConversationID, task.TurnID, task.Subject, task.Description,
		task.Status, task.Owner, string(blockedBy), now, now,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	task.ID = id
	task.CreatedAt = now
	task.UpdatedAt = now
	logger.Global().Debug().Str("table", "todo_tasks").Str("op", "insert").Int64("task_id", task.ID).Str("conv_id", task.ConversationID).Msg("store")
	return nil
}

func (s *TodoStore) Update(conversationID string, id int64, subject, description, status, owner string, blockedBy []int64) (*models.Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	setClauses := []string{"updated_at = ?"}
	args := []any{now}

	if subject != "" {
		setClauses = append(setClauses, "subject = ?")
		args = append(args, subject)
	}
	if description != "" {
		setClauses = append(setClauses, "description = ?")
		args = append(args, description)
	}
	if status != "" {
		setClauses = append(setClauses, "status = ?")
		args = append(args, status)
	}
	if owner != "" {
		setClauses = append(setClauses, "owner = ?")
		args = append(args, owner)
	}
	if blockedBy != nil {
		b, _ := json.Marshal(blockedBy)
		setClauses = append(setClauses, "blocked_by = ?")
		args = append(args, string(b))
	}

	args = append(args, id, conversationID)
	_, err := s.db.Exec(
		fmt.Sprintf("UPDATE todo_tasks SET %s WHERE id = ? AND conversation_id = ?", strings.Join(setClauses, ", ")),
		args...,
	)
	if err != nil {
		return nil, err
	}

	var t models.Todo
	var blockedByJSON string
	err = s.db.QueryRow(
		`SELECT id, conversation_id, turn_id, subject, description, status, owner, blocked_by, created_at, updated_at
		 FROM todo_tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.ConversationID, &t.TurnID, &t.Subject, &t.Description,
		&t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(blockedByJSON), &t.BlockedBy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal blocked_by for task %d: %w", t.ID, err)
	}
	logger.Global().Debug().Str("table", "todo_tasks").Str("op", "update").Int64("task_id", id).Str("status", status).Msg("store")
	return &t, nil
}

func (s *TodoStore) List(conversationID string) ([]*models.Todo, error) {
	// Only return tasks from turns that have at least one non-completed task.
	rows, err := s.db.Query(
		`SELECT id, conversation_id, turn_id, subject, description, status, owner, blocked_by, created_at, updated_at
		 FROM todo_tasks
		 WHERE conversation_id = ?
		   AND status != 'deleted'
		   AND turn_id IN (
		       SELECT DISTINCT turn_id FROM todo_tasks
		       WHERE conversation_id = ?
		         AND status NOT IN ('completed', 'deleted')
		   )
		 ORDER BY id ASC`,
		conversationID, conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Todo
	for rows.Next() {
		var t models.Todo
		var blockedByJSON string
		if err := rows.Scan(&t.ID, &t.ConversationID, &t.TurnID, &t.Subject, &t.Description,
			&t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(blockedByJSON), &t.BlockedBy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal blocked_by for task %d: %w", t.ID, err)
		}
		tasks = append(tasks, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	logger.Global().Debug().Str("table", "todo_tasks").Str("op", "select").Str("conv_id", conversationID).Int("count", len(tasks)).Msg("store")
	return tasks, nil
}

func (s *TodoStore) ListByTurn(turnID string) ([]*models.Todo, error) {
	rows, err := s.db.Query(
		`SELECT id, conversation_id, turn_id, subject, description, status, owner, blocked_by, created_at, updated_at
		 FROM todo_tasks WHERE turn_id = ? AND status != 'deleted' ORDER BY id ASC`,
		turnID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Todo
	for rows.Next() {
		var t models.Todo
		var blockedByJSON string
		if err := rows.Scan(&t.ID, &t.ConversationID, &t.TurnID, &t.Subject, &t.Description,
			&t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(blockedByJSON), &t.BlockedBy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal blocked_by for task %d: %w", t.ID, err)
		}
		tasks = append(tasks, &t)
	}
	return tasks, rows.Err()
}

func (s *TodoStore) Get(id int64) (*models.Todo, error) {
	var t models.Todo
	var blockedByJSON string
	err := s.db.QueryRow(
		`SELECT id, conversation_id, turn_id, subject, description, status, owner, blocked_by, created_at, updated_at
		 FROM todo_tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.ConversationID, &t.TurnID, &t.Subject, &t.Description,
		&t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(blockedByJSON), &t.BlockedBy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal blocked_by for task %d: %w", t.ID, err)
	}
	return &t, nil
}
