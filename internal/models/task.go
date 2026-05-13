package models

import "time"

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	TaskStatusActive   TaskStatus = "active"
	TaskStatusPaused   TaskStatus = "paused"
	TaskStatusArchived TaskStatus = "archived"
)

// NotifyMode defines when to send notifications for task runs.
type NotifyMode string

const (
	NotifyNone     NotifyMode = "none"
	NotifyFailure  NotifyMode = "failure"
	NotifyComplete NotifyMode = "complete"
	NotifyAnomaly  NotifyMode = "anomaly"
)

// TaskRunStatus represents the execution state of a task run.
type TaskRunStatus string

const (
	TaskRunStatusRunning   TaskRunStatus = "running"
	TaskRunStatusCompleted TaskRunStatus = "completed"
	TaskRunStatusFailed    TaskRunStatus = "failed"
	TaskRunStatusTimeout   TaskRunStatus = "timeout"
)

// Task represents a scheduled automation task.
type Task struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Goal             string     `json:"goal"`
	HostIDs          []string   `json:"host_ids"`
	Schedule         string     `json:"schedule,omitempty"`
	NotifyMode       NotifyMode `json:"notify_mode,omitempty"`
	RunRetentionDays int        `json:"run_retention_days"`
	TimeoutMinutes   int        `json:"timeout_minutes"`
	Status           TaskStatus `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	SourceConvID     string     `json:"source_conv_id,omitempty"`
}

// TaskRun represents a single execution of a task.
type TaskRun struct {
	ID         string         `json:"id"`
	TaskID     string         `json:"task_id"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Status     TaskRunStatus  `json:"status"`
	RawOutput  string         `json:"raw_output,omitempty"`
	Summary    string         `json:"summary,omitempty"`
	Alerted    bool           `json:"alerted"`
}
