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

func TestListDocuments(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	// Empty list
	docs, err := s.ListDocuments(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 documents, got %d", len(docs))
	}

	// Create documents directly via SQL for testing
	db := newTestDB(t)
	s2 := knowledge.NewStore(db)
	kb2, _ := s2.CreateKB(ctx, "AISG")
	g2, _ := s2.CreateGroup(ctx, kb2.ID, "v706")

	_, err = db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g2.ID, "API Spec", "openapi", "content1", "api.yaml", "ready")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g2.ID, "Guide", "markdown", "content2", "guide.md", "pending")
	if err != nil {
		t.Fatal(err)
	}

	// List documents
	docs, err = s2.ListDocuments(ctx, g2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
	if docs[0].Name != "API Spec" || docs[1].Name != "Guide" {
		t.Fatalf("unexpected document names: %s, %s", docs[0].Name, docs[1].Name)
	}
}

func TestGetDocument(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	// Insert a document
	res, err := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, entry_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "API Spec", "openapi", "content", "api.yaml", "ready", 5)
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := res.LastInsertId()

	// Get existing document
	doc, err := s.GetDocument(ctx, int(docID))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name != "API Spec" || doc.DocType != "openapi" || doc.EntryCount != 5 {
		t.Fatalf("unexpected document: %+v", doc)
	}

	// Get non-existent document
	_, err = s.GetDocument(ctx, 99999)
	if err == nil {
		t.Fatal("expected error for non-existent document")
	}
}

func TestDeleteDocuments(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	// Insert documents
	res1, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "markdown", "content1", "doc1.md", "ready")
	docID1, _ := res1.LastInsertId()

	res2, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc2", "markdown", "content2", "doc2.md", "ready")
	docID2, _ := res2.LastInsertId()

	// Delete single document
	err := s.DeleteDocuments(ctx, []int{int(docID1)})
	if err != nil {
		t.Fatal(err)
	}

	docs, _ := s.ListDocuments(ctx, g.ID)
	if len(docs) != 1 || docs[0].Name != "Doc2" {
		t.Fatalf("expected 1 document after delete, got %d", len(docs))
	}

	// Delete multiple documents
	res3, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc3", "markdown", "content3", "doc3.md", "ready")
	docID3, _ := res3.LastInsertId()

	err = s.DeleteDocuments(ctx, []int{int(docID2), int(docID3)})
	if err != nil {
		t.Fatal(err)
	}

	docs, _ = s.ListDocuments(ctx, g.ID)
	if len(docs) != 0 {
		t.Fatalf("expected 0 documents after delete, got %d", len(docs))
	}
}

func TestDeleteDocumentsCascade(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	// Insert document
	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "markdown", "content1", "doc1.md", "ready")
	docID, _ := res.LastInsertId()

	// Insert sections
	res1, _ := db.ExecContext(ctx, `INSERT INTO knowledge_sections
		(document_id, name, summary, position) VALUES (?, ?, ?, ?)`,
		docID, "Section1", "summary1", 0)
	sectionID1, _ := res1.LastInsertId()

	res2, _ := db.ExecContext(ctx, `INSERT INTO knowledge_sections
		(document_id, name, summary, position) VALUES (?, ?, ?, ?)`,
		docID, "Section2", "summary2", 1)
	sectionID2, _ := res2.LastInsertId()

	// Insert entries
	db.ExecContext(ctx, `INSERT INTO knowledge_entries
		(document_id, section_id, title, summary, content, position)
		VALUES (?, ?, ?, ?, ?, ?)`,
		docID, sectionID1, "Entry1", "sum1", "content1", 0)

	db.ExecContext(ctx, `INSERT INTO knowledge_entries
		(document_id, section_id, title, summary, content, position)
		VALUES (?, ?, ?, ?, ?, ?)`,
		docID, sectionID2, "Entry2", "sum2", "content2", 0)

	// Verify sections and entries exist
	var sectionCount, entryCount int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_sections WHERE document_id = ?`, docID).Scan(&sectionCount)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_entries WHERE document_id = ?`, docID).Scan(&entryCount)

	if sectionCount != 2 {
		t.Fatalf("expected 2 sections, got %d", sectionCount)
	}
	if entryCount != 2 {
		t.Fatalf("expected 2 entries, got %d", entryCount)
	}

	// Delete document
	err := s.DeleteDocuments(ctx, []int{int(docID)})
	if err != nil {
		t.Fatal(err)
	}

	// Verify cascade delete
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_sections WHERE document_id = ?`, docID).Scan(&sectionCount)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_entries WHERE document_id = ?`, docID).Scan(&entryCount)

	if sectionCount != 0 {
		t.Fatalf("expected sections to be cascade deleted, found %d", sectionCount)
	}
	if entryCount != 0 {
		t.Fatalf("expected entries to be cascade deleted, found %d", entryCount)
	}
}

func TestSectionCRUD(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	// Insert document
	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "markdown", "content1", "doc1.md", "ready")
	docID, _ := res.LastInsertId()

	// Create sections
	sec1, err := s.CreateSection(ctx, int(docID), "Introduction", "Intro summary", 0)
	if err != nil {
		t.Fatal(err)
	}
	if sec1.DocumentID != int(docID) || sec1.Name != "Introduction" || sec1.Summary != "Intro summary" || sec1.Position != 0 {
		t.Fatalf("unexpected section: %+v", sec1)
	}

	_, err = s.CreateSection(ctx, int(docID), "Conclusion", "Conclusion summary", 1)
	if err != nil {
		t.Fatal(err)
	}

	// List sections
	sections, err := s.ListSections(ctx, int(docID))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if sections[0].Name != "Introduction" || sections[1].Name != "Conclusion" {
		t.Fatalf("unexpected section order: %s, %s", sections[0].Name, sections[1].Name)
	}

	// Empty list for non-existent document
	sections, err = s.ListSections(ctx, 99999)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 0 {
		t.Fatalf("expected 0 sections for non-existent document, got %d", len(sections))
	}
}

func TestEntryCRUD(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	// Insert document
	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "markdown", "content1", "doc1.md", "ready")
	docID, _ := res.LastInsertId()

	// Create section
	sec, _ := s.CreateSection(ctx, int(docID), "Section1", "summary", 0)

	// Create entry with section
	entry1, err := s.CreateEntry(ctx, int(docID), &sec.ID, "Entry 1", "Summary 1", "Content 1", []byte("embedding1"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if entry1.DocumentID != int(docID) || entry1.SectionID == nil || *entry1.SectionID != sec.ID {
		t.Fatalf("unexpected entry: %+v", entry1)
	}
	if entry1.Title != "Entry 1" || entry1.Summary != "Summary 1" || entry1.Content != "Content 1" || entry1.Position != 0 {
		t.Fatalf("unexpected entry fields: %+v", entry1)
	}

	// Create entry without section (nil sectionID)
	entry2, err := s.CreateEntry(ctx, int(docID), nil, "Entry 2", "Summary 2", "Content 2", []byte("embedding2"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if entry2.SectionID != nil {
		t.Fatalf("expected nil sectionID, got %v", entry2.SectionID)
	}

	// List entries
	entries, err := s.ListEntries(ctx, int(docID))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Title != "Entry 1" || entries[1].Title != "Entry 2" {
		t.Fatalf("unexpected entry order: %s, %s", entries[0].Title, entries[1].Title)
	}

	// Verify ordering by position
	if entries[0].Position != 0 || entries[1].Position != 1 {
		t.Fatalf("unexpected positions: %d, %d", entries[0].Position, entries[1].Position)
	}

	// Empty list for non-existent document
	entries, err = s.ListEntries(ctx, 99999)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for non-existent document, got %d", len(entries))
	}
}

func TestSectionEntryCount(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	// Insert document
	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "markdown", "content1", "doc1.md", "ready")
	docID, _ := res.LastInsertId()

	// Create sections
	sec1, _ := s.CreateSection(ctx, int(docID), "Section1", "summary1", 0)
	sec2, _ := s.CreateSection(ctx, int(docID), "Section2", "summary2", 1)
	_, _ = s.CreateSection(ctx, int(docID), "Section3", "summary3", 2)

	// Create entries: 2 in sec1, 1 in sec2, 0 in sec3
	s.CreateEntry(ctx, int(docID), &sec1.ID, "Entry1", "sum1", "content1", []byte("emb1"), 0)
	s.CreateEntry(ctx, int(docID), &sec1.ID, "Entry2", "sum2", "content2", []byte("emb2"), 1)
	s.CreateEntry(ctx, int(docID), &sec2.ID, "Entry3", "sum3", "content3", []byte("emb3"), 0)

	// List sections and verify EntryCount
	sections, err := s.ListSections(ctx, int(docID))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	if sections[0].EntryCount != 2 {
		t.Fatalf("expected Section1 to have 2 entries, got %d", sections[0].EntryCount)
	}
	if sections[1].EntryCount != 1 {
		t.Fatalf("expected Section2 to have 1 entry, got %d", sections[1].EntryCount)
	}
	if sections[2].EntryCount != 0 {
		t.Fatalf("expected Section3 to have 0 entries, got %d", sections[2].EntryCount)
	}
}
