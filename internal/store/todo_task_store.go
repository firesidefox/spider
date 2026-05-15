package store

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/models"
)

const todoColumns = `id, seq, conversation_id, subject, active_form, description, status, owner, created_at, updated_at`

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

	now := time.Now().UTC()
	var seq int64
	s.db.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM todo_tasks WHERE conversation_id = ?`, task.ConversationID).Scan(&seq)
	seq++
	res, err := s.db.Exec(
		`INSERT INTO todo_tasks (conversation_id, subject, active_form, description, status, owner, seq, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ConversationID, task.Subject, task.ActiveForm, task.Description,
		task.Status, task.Owner, seq, now, now,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	task.ID = id
	task.Seq = seq
	task.CreatedAt = now
	task.UpdatedAt = now
	return nil
}

func (s *TodoStore) Update(conversationID string, id int64, subject, activeForm, description, status, owner string) (*models.Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	setClauses := []string{"updated_at = ?"}
	args := []any{now}

	if subject != "" {
		setClauses = append(setClauses, "subject = ?")
		args = append(args, subject)
	}
	if activeForm != "" {
		setClauses = append(setClauses, "active_form = ?")
		args = append(args, activeForm)
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

	args = append(args, id, conversationID)
	_, err := s.db.Exec(
		fmt.Sprintf("UPDATE todo_tasks SET %s WHERE id = ? AND conversation_id = ?", strings.Join(setClauses, ", ")),
		args...,
	)
	if err != nil {
		return nil, err
	}

	var t models.Todo
	err = s.db.QueryRow(
		`SELECT `+todoColumns+` FROM todo_tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.Seq, &t.ConversationID, &t.Subject, &t.ActiveForm, &t.Description,
		&t.Status, &t.Owner, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *TodoStore) GetByIDs(ids []int64) ([]*models.Todo, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := sqlPlaceholders(len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(
		`SELECT `+todoColumns+` FROM todo_tasks WHERE id IN (`+placeholders+`) ORDER BY id ASC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTodos(rows)
}

func (s *TodoStore) List(conversationID string) ([]*models.Todo, error) {
	rows, err := s.db.Query(
		`SELECT `+todoColumns+` FROM todo_tasks WHERE conversation_id = ? AND status NOT IN ('completed', 'deleted') ORDER BY id ASC`,
		conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks, err := scanTodos(rows)
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *TodoStore) Get(id int64) (*models.Todo, error) {
	var t models.Todo
	err := s.db.QueryRow(
		`SELECT `+todoColumns+` FROM todo_tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.Seq, &t.ConversationID, &t.Subject, &t.ActiveForm, &t.Description,
		&t.Status, &t.Owner, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scanTodos(rows *sql.Rows) ([]*models.Todo, error) {
	var tasks []*models.Todo
	for rows.Next() {
		var t models.Todo
		if err := rows.Scan(&t.ID, &t.Seq, &t.ConversationID, &t.Subject, &t.ActiveForm, &t.Description,
			&t.Status, &t.Owner, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, &t)
	}
	return tasks, rows.Err()
}
