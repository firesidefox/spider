package store

import (
	"testing"
)

func TestSummaryStore_GetNotFound(t *testing.T) {
	database := setupTestDB(t)
	ss := NewSummaryStore(database)

	got, err := ss.Get("nonexistent-conv")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestSummaryStore_UpsertAndGet(t *testing.T) {
	database := setupTestDB(t)
	ss := NewSummaryStore(database)

	err := ss.Upsert("conv1", "msg10", []string{"chunk1", "chunk2"})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := ss.Get("conv1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil summary")
	}
	if got.UpToMessageID != "msg10" {
		t.Errorf("UpToMessageID = %q, want %q", got.UpToMessageID, "msg10")
	}
	if len(got.Chunks) != 2 || got.Chunks[0] != "chunk1" || got.Chunks[1] != "chunk2" {
		t.Errorf("Chunks = %v, want [chunk1 chunk2]", got.Chunks)
	}
}

func TestSummaryStore_UpsertOverwrite(t *testing.T) {
	database := setupTestDB(t)
	ss := NewSummaryStore(database)

	if err := ss.Upsert("conv1", "msg10", []string{"chunk1"}); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	first, err := ss.Get("conv1")
	if err != nil {
		t.Fatalf("Get after first Upsert: %v", err)
	}
	createdAt := first.CreatedAt

	if err := ss.Upsert("conv1", "msg20", []string{"chunk1", "chunk2"}); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	got, err := ss.Get("conv1")
	if err != nil {
		t.Fatalf("Get after second Upsert: %v", err)
	}
	if got.UpToMessageID != "msg20" {
		t.Errorf("UpToMessageID = %q, want %q", got.UpToMessageID, "msg20")
	}
	if len(got.Chunks) != 2 {
		t.Errorf("Chunks len = %d, want 2", len(got.Chunks))
	}
	if got.UpdatedAt.Before(createdAt) {
		t.Errorf("updated_at %v is before created_at %v", got.UpdatedAt, createdAt)
	}
}
