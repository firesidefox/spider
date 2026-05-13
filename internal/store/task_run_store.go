package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

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

// Update updates mutable fields of an existing task run.
// Updates: finished_at, status, raw_output, summary, alerted.
// Does NOT update: id, task_id, started_at (immutable).
// Returns ErrNotFound if the task run does not exist.
func (s *TaskRunStore) Update(run *models.TaskRun) error {
	if run.ID == "" {
		return errors.New("id cannot be empty")
	}

	var finishedAt interface{}
	if run.FinishedAt != nil {
		finishedAt = *run.FinishedAt
	}

	result, err := s.db.Exec(`
		UPDATE task_runs
		SET finished_at = ?, status = ?, raw_output = ?, summary = ?, alerted = ?
		WHERE id = ?
	`, finishedAt, run.Status, run.RawOutput, run.Summary, run.Alerted, run.ID)
	if err != nil {
		return fmt.Errorf("failed to update task run %s: %w", run.ID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected for task run %s: %w", run.ID, err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// HasRunning reports whether the task has any run currently in running status.
func (s *TaskRunStore) HasRunning(taskID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM task_runs WHERE task_id = ? AND status = 'running'`, taskID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check running task runs for %s: %w", taskID, err)
	}
	return count > 0, nil
}

// LastStartedAt returns the started_at of the most recent run for a task, or nil if no runs exist.
func (s *TaskRunStore) LastStartedAt(taskID string) (*time.Time, error) {
	var t time.Time
	err := s.db.QueryRow(
		`SELECT started_at FROM task_runs WHERE task_id = ? ORDER BY started_at DESC LIMIT 1`, taskID,
	).Scan(&t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query last started_at for task %s: %w", taskID, err)
	}
	return &t, nil
}

// ListByTaskID retrieves task runs for a given task ID with pagination, ordered by started_at DESC (newest first).
func (s *TaskRunStore) ListByTaskID(taskID string, limit, offset int) ([]*models.TaskRun, error) {
	rows, err := s.db.Query(`
		SELECT id, task_id, started_at, finished_at, status, raw_output, summary, alerted
		FROM task_runs WHERE task_id = ?
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?
	`, taskID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query task runs for task %s: %w", taskID, err)
	}
	defer rows.Close()

	taskRuns := make([]*models.TaskRun, 0)
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

// MarkStaleRunsFailed marks all running task runs older than olderThan as failed.
// Called on startup to clean up runs orphaned by a previous crash.
func (s *TaskRunStore) MarkStaleRunsFailed(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	res, err := s.db.Exec(
		`UPDATE task_runs SET status='failed', finished_at=?, raw_output=raw_output||?
		 WHERE status='running' AND started_at < ?`,
		time.Now().UTC(), "\n[interrupted: process restarted]", cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to mark stale runs failed: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// DeleteOldRuns deletes task runs for a task started before the given time.
// Returns the number of rows deleted.
func (s *TaskRunStore) DeleteOldRuns(taskID string, before time.Time) (int64, error) {
	res, err := s.db.Exec(
		`DELETE FROM task_runs WHERE task_id = ? AND started_at < ?`, taskID, before,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old runs for task %s: %w", taskID, err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
