package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/models"
)

// TaskRunStore provides CRUD operations for task runs.
type TaskRunStore struct {
	db *sql.DB
}

// NewTaskRunStore creates a new TaskRunStore.
func NewTaskRunStore(db *sql.DB) *TaskRunStore {
	return &TaskRunStore{db: db}
}

// Create inserts a new task run into the database.
// Returns an error if taskRun.TaskID is empty or taskRun.Status is empty.
func (s *TaskRunStore) Create(taskRun *models.TaskRun) (*models.TaskRun, error) {
	if taskRun.TaskID == "" {
		return nil, errors.New("task_id cannot be empty")
	}
	if taskRun.Status == "" {
		return nil, errors.New("status cannot be empty")
	}

	taskRun.ID = uuid.New().String()

	var finishedAt interface{}
	if taskRun.FinishedAt != nil {
		finishedAt = *taskRun.FinishedAt
	}

	_, err := s.db.Exec(`
		INSERT INTO task_runs (id, task_id, started_at, finished_at, status, raw_output, summary, alerted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, taskRun.ID, taskRun.TaskID, taskRun.StartedAt, finishedAt, taskRun.Status, taskRun.RawOutput, taskRun.Summary, taskRun.Alerted)
	if err != nil {
		return nil, fmt.Errorf("failed to insert task run for task %s: %w", taskRun.TaskID, err)
	}

	return taskRun, nil
}

// Get retrieves a task run by ID.
// Returns ErrNotFound if the task run does not exist.
func (s *TaskRunStore) Get(id string) (*models.TaskRun, error) {
	var taskRun models.TaskRun
	var finishedAt sql.NullTime
	var alerted int

	err := s.db.QueryRow(`
		SELECT id, task_id, started_at, finished_at, status, raw_output, summary, alerted
		FROM task_runs WHERE id = ?
	`, id).Scan(&taskRun.ID, &taskRun.TaskID, &taskRun.StartedAt, &finishedAt, &taskRun.Status, &taskRun.RawOutput, &taskRun.Summary, &alerted)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to query task run %s: %w", id, err)
	}

	if finishedAt.Valid {
		taskRun.FinishedAt = &finishedAt.Time
	}
	taskRun.Alerted = alerted != 0

	return &taskRun, nil
}

// ListByTaskID retrieves all task runs for a given task ID, ordered by started_at DESC (newest first).
func (s *TaskRunStore) ListByTaskID(taskID string) ([]*models.TaskRun, error) {
	rows, err := s.db.Query(`
		SELECT id, task_id, started_at, finished_at, status, raw_output, summary, alerted
		FROM task_runs WHERE task_id = ?
		ORDER BY started_at DESC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query task runs for task %s: %w", taskID, err)
	}
	defer rows.Close()

	var taskRuns []*models.TaskRun
	for rows.Next() {
		var taskRun models.TaskRun
		var finishedAt sql.NullTime
		var alerted int

		if err := rows.Scan(&taskRun.ID, &taskRun.TaskID, &taskRun.StartedAt, &finishedAt, &taskRun.Status, &taskRun.RawOutput, &taskRun.Summary, &alerted); err != nil {
			return nil, fmt.Errorf("failed to scan task run row: %w", err)
		}

		if finishedAt.Valid {
			taskRun.FinishedAt = &finishedAt.Time
		}
		taskRun.Alerted = alerted != 0

		taskRuns = append(taskRuns, &taskRun)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating task run rows for task %s: %w", taskID, err)
	}

	return taskRuns, nil
}
