// Package store provides data access layer for Spider's persistent storage.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/models"
)

// TaskStore provides CRUD operations for tasks.
type TaskStore struct {
	db *sql.DB
}

// NewTaskStore creates a new TaskStore.
func NewTaskStore(db *sql.DB) *TaskStore {
	return &TaskStore{db: db}
}

// Create inserts a new task into the database.
// Returns an error if task.Name is empty or task.HostIDs is empty.
func (s *TaskStore) Create(task *models.Task) (*models.Task, error) {
	if task.Name == "" {
		return nil, errors.New("task name cannot be empty")
	}
	if len(task.HostIDs) == 0 {
		return nil, errors.New("task must have at least one host")
	}

	now := time.Now()
	task.ID = uuid.New().String()
	task.CreatedAt = now
	task.UpdatedAt = now

	hostIDsJSON, err := json.Marshal(task.HostIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal host_ids for task %s: %w", task.ID, err)
	}

	_, err = s.db.Exec(`
		INSERT INTO tasks (id, name, goal, host_ids, schedule, notify_mode, run_retention_days, timeout_minutes, status, created_at, updated_at, source_conv_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.ID, task.Name, task.Goal, string(hostIDsJSON), task.Schedule, task.NotifyMode, task.RunRetentionDays, task.TimeoutMinutes, task.Status, now, now, task.SourceConvID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert task %s: %w", task.ID, err)
	}

	return task, nil
}

// Get retrieves a task by ID.
// Returns ErrNotFound if the task does not exist.
func (s *TaskStore) Get(id string) (*models.Task, error) {
	var task models.Task
	var hostIDsJSON string

	err := s.db.QueryRow(`
		SELECT id, name, goal, host_ids, schedule, notify_mode, run_retention_days, timeout_minutes, status, created_at, updated_at, source_conv_id
		FROM tasks WHERE id = ?
	`, id).Scan(&task.ID, &task.Name, &task.Goal, &hostIDsJSON, &task.Schedule, &task.NotifyMode, &task.RunRetentionDays, &task.TimeoutMinutes, &task.Status, &task.CreatedAt, &task.UpdatedAt, &task.SourceConvID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to query task %s: %w", id, err)
	}

	if err := json.Unmarshal([]byte(hostIDsJSON), &task.HostIDs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal host_ids for task %s: %w", task.ID, err)
	}

	return &task, nil
}

// List retrieves all tasks ordered by creation time (newest first).
func (s *TaskStore) List() ([]*models.Task, error) {
	rows, err := s.db.Query(`
		SELECT id, name, goal, host_ids, schedule, notify_mode, run_retention_days, timeout_minutes, status, created_at, updated_at, source_conv_id
		FROM tasks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		var hostIDsJSON string

		if err := rows.Scan(&task.ID, &task.Name, &task.Goal, &hostIDsJSON, &task.Schedule, &task.NotifyMode, &task.RunRetentionDays, &task.TimeoutMinutes, &task.Status, &task.CreatedAt, &task.UpdatedAt, &task.SourceConvID); err != nil {
			return nil, fmt.Errorf("failed to scan task row: %w", err)
		}

		if err := json.Unmarshal([]byte(hostIDsJSON), &task.HostIDs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal host_ids for task %s: %w", task.ID, err)
		}

		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating task rows: %w", err)
	}

	return tasks, nil
}
