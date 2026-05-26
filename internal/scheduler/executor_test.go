package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/spiderai/spider/internal/agent"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type cancelBlockingClient struct {
	started chan struct{}
}

func (c *cancelBlockingClient) ChatStream(ctx context.Context, _ *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(c.started)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

func (c *cancelBlockingClient) Chat(context.Context, *llm.ChatRequest) (string, error) {
	return "", nil
}

func (c *cancelBlockingClient) CountTokens(context.Context, []llm.Message) (int, error) {
	return 0, nil
}

func TestExecutorStopCancelsUnlimitedTaskOnShutdown(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	hosts := store.NewHostStore(database)
	host, err := hosts.Add(&models.AddHostRequest{Name: "test-host", IP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	tasks := store.NewTaskStore(database)
	task, err := tasks.Create(&models.Task{
		Name:           "blocking-task",
		Goal:           "wait forever",
		HostIDs:        []string{host.ID},
		NotifyMode:     models.NotifyNone,
		TimeoutMinutes: 0,
		Status:         models.TaskStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	client := &cancelBlockingClient{started: make(chan struct{})}
	factory := &agent.Factory{LLMClient: client, Hosts: hosts}
	shutdownCtx, cancel := context.WithCancel(context.Background())
	exec := NewExecutor(shutdownCtx, tasks, store.NewTaskRunStore(database), hosts, factory, nil)

	if _, err := exec.Execute(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("task did not start")
	}

	cancel()
	stopped := make(chan struct{})
	go func() {
		exec.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Executor.Stop blocked after shutdown cancellation")
	}

	runs, err := store.NewTaskRunStore(database).ListByTaskID(task.ID, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("task runs = %d, want 1", len(runs))
	}
	run := runs[0]
	if run.RawOutput != "\nexecution canceled: service shutting down" {
		t.Fatalf("RawOutput = %q, want shutdown cancellation message", run.RawOutput)
	}
}
