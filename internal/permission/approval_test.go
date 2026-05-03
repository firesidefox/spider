package permission

import (
	"context"
	"testing"
	"time"
)

func TestApprovalManager_CreateAndRespond(t *testing.T) {
	m := NewApprovalManager()
	req := m.Create("sess-1", "rm /tmp/old", "prod-01", L3Dangerous, "rm deletes files")

	if req.ID == "" {
		t.Fatal("ID empty")
	}
	if req.SessionID != "sess-1" || req.Command != "rm /tmp/old" {
		t.Fatal("fields mismatch")
	}

	// respond before Wait
	m.Respond(req.ID, true, "alice")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := m.Wait(ctx, req.ID)
	if err != nil {
		t.Fatalf("Wait error: %v", err)
	}
	if !result.Approved || result.ApprovedBy != "alice" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestApprovalManager_WaitThenRespond(t *testing.T) {
	m := NewApprovalManager()
	req := m.Create("sess-2", "kill 1234", "prod-02", L3Dangerous, "kill process")

	go func() {
		time.Sleep(20 * time.Millisecond)
		m.Respond(req.ID, false, "bob")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := m.Wait(ctx, req.ID)
	if err != nil {
		t.Fatalf("Wait error: %v", err)
	}
	if result.Approved || result.ApprovedBy != "bob" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestApprovalManager_WaitContextCancel(t *testing.T) {
	m := NewApprovalManager()
	req := m.Create("sess-3", "dd if=/dev/zero", "prod-03", L4Destroy, "destructive")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := m.Wait(ctx, req.ID)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestApprovalManager_Subscribe(t *testing.T) {
	m := NewApprovalManager()
	ch := m.Subscribe()
	defer m.Unsubscribe(ch)

	req := m.Create("sess-4", "rm -rf /logs", "prod-04", L4Destroy, "bulk delete")

	select {
	case got := <-ch:
		if got.ID != req.ID {
			t.Fatalf("got ID %s, want %s", got.ID, req.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for subscription event")
	}
}

func TestApprovalManager_Pending(t *testing.T) {
	m := NewApprovalManager()
	m.Create("sess-5", "cmd1", "h1", L3Dangerous, "r1")
	m.Create("sess-5", "cmd2", "h1", L4Destroy, "r2")

	pending := m.Pending()
	if len(pending) != 2 {
		t.Fatalf("want 2 pending, got %d", len(pending))
	}
}
