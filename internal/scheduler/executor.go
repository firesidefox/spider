package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/agent"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/notify"
	"github.com/spiderai/spider/internal/store"
)

// ErrAlreadyRunning is returned when a task already has a running TaskRun.
var ErrAlreadyRunning = errors.New("task is already running")

// Executor runs tasks headlessly using the agent.
type Executor struct {
	taskStore           *store.TaskStore
	taskRunStore        *store.TaskRunStore
	hostStore           *store.HostStore
	agentFactory        *agent.Factory
	notifyChannelStore  *store.NotifyChannelStore
}

// NewExecutor creates a new Executor.
func NewExecutor(
	taskStore *store.TaskStore,
	taskRunStore *store.TaskRunStore,
	hostStore *store.HostStore,
	agentFactory *agent.Factory,
	notifyChannelStore *store.NotifyChannelStore,
) *Executor {
	return &Executor{
		taskStore:          taskStore,
		taskRunStore:       taskRunStore,
		hostStore:          hostStore,
		agentFactory:       agentFactory,
		notifyChannelStore: notifyChannelStore,
	}
}

// Execute starts a task run asynchronously. Returns the created TaskRun immediately.
// Returns error if task not found, already running, or run creation fails.
func (e *Executor) Execute(ctx context.Context, taskID string) (*models.TaskRun, error) {
	task, err := e.taskStore.Get(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	hasRunning, err := e.taskRunStore.HasRunning(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to check running tasks: %w", err)
	}
	if hasRunning {
		return nil, ErrAlreadyRunning
	}

	run := &models.TaskRun{
		TaskID:    taskID,
		StartedAt: time.Now(),
		Status:    models.TaskRunStatusRunning,
	}
	created, err := e.taskRunStore.Create(run)
	if err != nil {
		return nil, fmt.Errorf("failed to create task run: %w", err)
	}

	go e.executeAsync(context.Background(), task, created)
	return created, nil
}

func (e *Executor) executeAsync(ctx context.Context, task *models.Task, run *models.TaskRun) {
	log := logger.Global().With().Str("task_id", task.ID).Str("run_id", run.ID).Logger()

	validHostIDs := e.filterValidHosts(task.HostIDs)
	if len(validHostIDs) == 0 {
		now := time.Now()
		run.Status = models.TaskRunStatusFailed
		run.RawOutput = fmt.Sprintf("all hosts invalid: %v", task.HostIDs)
		run.Alerted = true
		run.FinishedAt = &now
		if err := e.taskRunStore.Update(run); err != nil {
			log.Error().Err(err).Msg("failed to update task run")
		}
		e.sendNotifications(context.Background(), task, run)
		return
	}

	execCtx := ctx
	if task.TimeoutMinutes > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(task.TimeoutMinutes)*time.Minute)
		defer cancel()
	}

	var hostLines []string
	for _, id := range validHostIDs {
		if host, err := e.hostStore.GetByID(id); err == nil {
			hostLines = append(hostLines, fmt.Sprintf("- %s (%s)", host.Name, host.IP))
		}
	}
	systemPrompt := fmt.Sprintf(
		"You are executing an automated task.\n\nTask: %s\n\nTarget hosts:\n%s\n\nExecute the task and report results.",
		task.Goal, strings.Join(hostLines, "\n"),
	)

	convID := "task-run-" + run.ID
	ag := e.agentFactory.NewAgent(systemPrompt, convID)
	events, err := ag.Run(execCtx, convID, task.Goal, nil)
	if err != nil {
		now := time.Now()
		run.Status = models.TaskRunStatusFailed
		run.RawOutput = fmt.Sprintf("agent start failed: %v", err)
		run.Alerted = true
		run.FinishedAt = &now
		if uerr := e.taskRunStore.Update(run); uerr != nil {
			log.Error().Err(uerr).Msg("failed to update task run")
		}
		e.sendNotifications(context.Background(), task, run)
		return
	}

	var outputParts []string
	for ev := range events {
		if ev.Type == agent.EventTextDelta {
			if s, ok := ev.Content["text"].(string); ok {
				outputParts = append(outputParts, s)
			}
		}
	}
	timedOut := execCtx.Err() != nil

	now := time.Now()
	run.RawOutput = strings.Join(outputParts, "")
	run.FinishedAt = &now
	if timedOut {
		run.Status = models.TaskRunStatusFailed
		run.RawOutput += fmt.Sprintf("\nexecution timeout after %dm", task.TimeoutMinutes)
		run.Alerted = true
	} else {
		run.Status = models.TaskRunStatusCompleted
	}
	if err := e.taskRunStore.Update(run); err != nil {
		log.Error().Err(err).Msg("failed to update task run")
	}
	log.Info().Str("status", string(run.Status)).Msg("task run complete")
	e.sendNotifications(context.Background(), task, run)
}

// filterValidHosts returns only host IDs that exist in the host store.
func (e *Executor) filterValidHosts(hostIDs []string) []string {
	valid := make([]string, 0, len(hostIDs))
	for _, id := range hostIDs {
		if _, err := e.hostStore.GetByID(id); err == nil {
			valid = append(valid, id)
		}
	}
	return valid
}

// sendNotifications sends the task run result to all configured notify channels.
func (e *Executor) sendNotifications(ctx context.Context, task *models.Task, run *models.TaskRun) {
	if e.notifyChannelStore == nil || task.NotifyMode == models.NotifyNone || task.NotifyMode == "" {
		return
	}
	switch task.NotifyMode {
	case models.NotifyFailure:
		if run.Status != models.TaskRunStatusFailed {
			return
		}
	case models.NotifyAnomaly:
		if !run.Alerted {
			return
		}
	case models.NotifyComplete:
		// always send
	}
	channels, err := e.notifyChannelStore.List()
	if err != nil {
		logger.Global().Error().Err(err).Str("task_id", task.ID).Msg("failed to list notify channels")
		return
	}
	msg := notify.FormatMessage(task, run)
	for _, ch := range channels {
		sender, err := notify.NewSender(ch)
		if err != nil {
			logger.Global().Warn().Err(err).Int64("channel_id", ch.ID).Msg("skip unsupported channel")
			continue
		}
		if err := sender.Send(ctx, msg); err != nil {
			logger.Global().Error().Err(err).Int64("channel_id", ch.ID).Msg("notification send failed")
		}
	}
}
