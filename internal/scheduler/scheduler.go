package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

// Scheduler polls for due cron tasks every minute and triggers execution.
type Scheduler struct {
	taskStore    *store.TaskStore
	taskRunStore *store.TaskRunStore
	executor     *Executor
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewScheduler creates a new Scheduler.
func NewScheduler(
	taskStore *store.TaskStore,
	taskRunStore *store.TaskRunStore,
	executor *Executor,
) *Scheduler {
	return &Scheduler{
		taskStore:    taskStore,
		taskRunStore: taskRunStore,
		executor:     executor,
		stopCh:       make(chan struct{}),
	}
}

// Start begins the scheduler loop in a background goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(2)
	go s.run(ctx)
	go s.runCleanup(ctx)
}

// Stop signals the scheduler to stop and waits for it to exit.
func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Scheduler) tick() {
	tasks, err := s.taskStore.List()
	if err != nil {
		logger.ForModule("scheduler").Error().Err(err).Msg("scheduler: failed to list tasks")
		return
	}
	for _, task := range tasks {
		if task.Status != models.TaskStatusActive || task.Schedule == "" {
			continue
		}
		if s.isDue(task) {
			s.tryTrigger(task)
		}
	}
}

func (s *Scheduler) isDue(task *models.Task) bool {
	sched, err := cron.ParseStandard(task.Schedule)
	if err != nil {
		logger.ForModule("scheduler").Warn().Err(err).Str("task_id", task.ID).Msg("scheduler: invalid cron schedule")
		return false
	}
	lastRun, err := s.taskRunStore.LastStartedAt(task.ID)
	if err != nil {
		logger.ForModule("scheduler").Error().Err(err).Str("task_id", task.ID).Msg("scheduler: failed to get last run time")
		return false
	}
	base := task.CreatedAt
	if lastRun != nil {
		base = *lastRun
	}
	return time.Now().After(sched.Next(base))
}

func (s *Scheduler) tryTrigger(task *models.Task) {
	hasRunning, err := s.taskRunStore.HasRunning(task.ID)
	if err != nil {
		logger.ForModule("scheduler").Error().Err(err).Str("task_id", task.ID).Msg("scheduler: failed to check running")
		return
	}
	if hasRunning {
		logger.ForModule("scheduler").Info().Str("task_id", task.ID).Msg("scheduler: skipped, previous run still running")
		return
	}
	// The executor owns task lifecycle cancellation through the service shutdown context.
	if _, err := s.executor.Execute(context.Background(), task.ID); err != nil {
		logger.ForModule("scheduler").Error().Err(err).Str("task_id", task.ID).Msg("scheduler: failed to trigger task")
	}
}

// runCleanup fires at midnight daily to purge old task runs.
func (s *Scheduler) runCleanup(ctx context.Context) {
	defer s.wg.Done()
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-s.stopCh:
			timer.Stop()
			return
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.cleanupOldRuns()
		}
	}
}

// cleanupOldRuns deletes task runs older than each task's RunRetentionDays.
func (s *Scheduler) cleanupOldRuns() {
	tasks, err := s.taskStore.List()
	if err != nil {
		logger.ForModule("scheduler").Error().Err(err).Msg("scheduler: cleanup: failed to list tasks")
		return
	}
	for _, task := range tasks {
		if task.RunRetentionDays <= 0 {
			continue
		}
		before := time.Now().AddDate(0, 0, -task.RunRetentionDays)
		n, err := s.taskRunStore.DeleteOldRuns(task.ID, before)
		if err != nil {
			logger.ForModule("scheduler").Error().Err(err).Str("task_id", task.ID).Msg("scheduler: cleanup: failed to delete old runs")
			continue
		}
		if n > 0 {
			logger.ForModule("scheduler").Info().Str("task_id", task.ID).Int64("deleted", n).Msg("scheduler: cleanup: deleted old runs")
		}
	}
}
