package store

import (
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/models"
)

type TaskStore struct {
	db *sql.DB
	mu sync.Mutex
}

func NewTaskStore(db *sql.DB) *TaskStore {
	return &TaskStore{db: db}
}

func (s *TaskStore) Create(task *models.Task) (*models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	task.ID = uuid.New().String()
	task.CreatedAt = now
	task.UpdatedAt = now

	hostIDsJSON, err := json.Marshal(task.HostIDs)
	if err != nil {
		return nil, err
	}

	_, err = s.db.Exec(`
		INSERT INTO tasks (id, name, goal, host_ids, schedule, notify_mode, run_retention_days, timeout_minutes, status, created_at, updated_at, source_conv_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.ID, task.Name, task.Goal, string(hostIDsJSON), task.Schedule, task.NotifyMode, task.RunRetentionDays, task.TimeoutMinutes, task.Status, now, now, task.SourceConvID)
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (s *TaskStore) Get(id string) (*models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var task models.Task
	var hostIDsJSON string

	err := s.db.QueryRow(`
		SELECT id, name, goal, host_ids, schedule, notify_mode, run_retention_days, timeout_minutes, status, created_at, updated_at, source_conv_id
		FROM tasks WHERE id = ?
	`, id).Scan(&task.ID, &task.Name, &task.Goal, &hostIDsJSON, &task.Schedule, &task.NotifyMode, &task.RunRetentionDays, &task.TimeoutMinutes, &task.Status, &task.CreatedAt, &task.UpdatedAt, &task.SourceConvID)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(hostIDsJSON), &task.HostIDs); err != nil {
		return nil, err
	}

	return &task, nil
}

func (s *TaskStore) List() ([]*models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
		SELECT id, name, goal, host_ids, schedule, notify_mode, run_retention_days, timeout_minutes, status, created_at, updated_at, source_conv_id
		FROM tasks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		var hostIDsJSON string

		if err := rows.Scan(&task.ID, &task.Name, &task.Goal, &hostIDsJSON, &task.Schedule, &task.NotifyMode, &task.RunRetentionDays, &task.TimeoutMinutes, &task.Status, &task.CreatedAt, &task.UpdatedAt, &task.SourceConvID); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(hostIDsJSON), &task.HostIDs); err != nil {
			return nil, err
		}

		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}
