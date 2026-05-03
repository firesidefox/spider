package store

import (
	"database/sql"
	"testing"

	"github.com/spiderai/spider/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestConversationStore_CreateAndGet(t *testing.T) {
	database := setupTestDB(t)
	s := NewConversationStore(database)

	conv, err := s.Create("user-1", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if conv.ID == "" || conv.Title == "" {
		t.Errorf("unexpected conv: %+v", conv)
	}

	got, err := s.GetByID(conv.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != conv.Title {
		t.Errorf("Title = %q, want %q", got.Title, conv.Title)
	}
}

func TestConversationStore_ListByUser(t *testing.T) {
	database := setupTestDB(t)
	s := NewConversationStore(database)

	s.Create("user-1", "")
	s.Create("user-1", "")
	s.Create("user-2", "")

	list, err := s.ListByUser("user-1")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}

func TestConversationStore_Delete(t *testing.T) {
	database := setupTestDB(t)
	s := NewConversationStore(database)

	conv, _ := s.Create("user-1", "")
	err := s.Delete(conv.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = s.GetByID(conv.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}
