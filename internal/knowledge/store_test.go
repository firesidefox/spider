package knowledge_test

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/knowledge"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqldb, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	// Explicitly enable foreign keys
	if _, err := sqldb.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(sqldb); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqldb.Close() })
	return sqldb
}

func TestKBCRUD(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	kb, err := s.CreateKB(ctx, "AISG")
	if err != nil {
		t.Fatal(err)
	}
	if kb.Name != "AISG" || kb.ID == 0 {
		t.Fatalf("unexpected kb: %+v", kb)
	}

	kbs, err := s.ListKBs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(kbs) != 1 || kbs[0].ID != kb.ID {
		t.Fatalf("expected 1 kb, got %d", len(kbs))
	}

	if err := s.DeleteKB(ctx, kb.ID); err != nil {
		t.Fatal(err)
	}
	kbs, _ = s.ListKBs(ctx)
	if len(kbs) != 0 {
		t.Fatal("expected 0 kbs after delete")
	}
}

func TestGroupCRUD(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, err := s.CreateGroup(ctx, kb.ID, "v706")
	if err != nil {
		t.Fatal(err)
	}
	if g.KBID != kb.ID || g.Name != "v706" {
		t.Fatalf("unexpected group: %+v", g)
	}

	groups, err := s.ListGroups(ctx, kb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	if err := s.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	groups, _ = s.ListGroups(ctx, kb.ID)
	if len(groups) != 0 {
		t.Fatal("expected 0 groups after delete")
	}
}

func TestCascadeDelete(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g1, _ := s.CreateGroup(ctx, kb.ID, "v706")
	g2, _ := s.CreateGroup(ctx, kb.ID, "v707")

	groups, _ := s.ListGroups(ctx, kb.ID)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Delete KB should cascade delete groups
	if err := s.DeleteKB(ctx, kb.ID); err != nil {
		t.Fatal(err)
	}

	// Verify groups are deleted
	groups, _ = s.ListGroups(ctx, kb.ID)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups after KB delete, got %d", len(groups))
	}

	// Verify we can't find the groups by ID
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_groups WHERE id IN (?, ?)`, g1.ID, g2.ID).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected groups to be cascade deleted, found %d", count)
	}
}
