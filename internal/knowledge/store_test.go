package knowledge_test

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/rag"
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

func TestGroupCRUD(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	g, err := s.CreateGroup(ctx, "AISG")
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "AISG" || g.ID == 0 {
		t.Fatalf("unexpected group: %+v", g)
	}

	groups, err := s.ListGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].ID != g.ID {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	if err := s.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	groups, _ = s.ListGroups(ctx)
	if len(groups) != 0 {
		t.Fatal("expected 0 groups after delete")
	}
}

func TestCascadeDelete(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	g1, _ := s.CreateGroup(ctx, "v706")
	_, _ = s.CreateGroup(ctx, "v707")

	groups, _ := s.ListGroups(ctx)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Delete group should cascade delete documents
	if err := s.DeleteGroup(ctx, g1.ID); err != nil {
		t.Fatal(err)
	}

	// Verify one group remains
	groups, _ = s.ListGroups(ctx)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group after delete, got %d", len(groups))
	}

	// Verify deleted group is gone
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_groups WHERE id = ?`, g1.ID).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected group to be deleted, found %d", count)
	}
}

func TestListDocuments(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	g, _ := s.CreateGroup(ctx, "v706")

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
	g2, _ := s2.CreateGroup(ctx, "v706")

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

	g, _ := s.CreateGroup(ctx, "v706")

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

	g, _ := s.CreateGroup(ctx, "v706")

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

	g, _ := s.CreateGroup(ctx, "v706")

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

	g, _ := s.CreateGroup(ctx, "v706")

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

	g, _ := s.CreateGroup(ctx, "v706")

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

func TestCatalogSections(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	// Create 2 groups
	g1, _ := s.CreateGroup(ctx, "v706")
	g2, _ := s.CreateGroup(ctx, "v808")

	// Create 2 documents: doc1 in g1, doc2 in g2
	res1, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g1.ID, "Doc1", "markdown", "content1", "doc1.md", "ready")
	doc1ID, _ := res1.LastInsertId()

	res2, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g2.ID, "Doc2", "markdown", "content2", "doc2.md", "ready")
	doc2ID, _ := res2.LastInsertId()

	// Create sections: 2 in doc1, 1 in doc2
	sec1, _ := s.CreateSection(ctx, int(doc1ID), "Introduction", "intro summary", 0)
	sec2, _ := s.CreateSection(ctx, int(doc1ID), "Conclusion", "conclusion summary", 1)
	_, _ = s.CreateSection(ctx, int(doc2ID), "Overview", "overview summary", 0)

	// Create entries: 2 in sec1, 1 in sec2, 0 in sec3
	s.CreateEntry(ctx, int(doc1ID), &sec1.ID, "Entry1", "sum1", "content1", []byte("emb1"), 0)
	s.CreateEntry(ctx, int(doc1ID), &sec1.ID, "Entry2", "sum2", "content2", []byte("emb2"), 1)
	s.CreateEntry(ctx, int(doc1ID), &sec2.ID, "Entry3", "sum3", "content3", []byte("emb3"), 0)

	// Test scope: group (should return sections from both docs in g1)
	sections, err := s.CatalogSections(ctx, knowledge.Scope{Type: "group", ID: g1.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("group scope: expected 2 sections, got %d", len(sections))
	}

	// Verify entry counts by section name
	sectionMap := make(map[string]int)
	for _, sec := range sections {
		sectionMap[sec.Name] = sec.EntryCount
	}

	if sectionMap["Introduction"] != 2 {
		t.Fatalf("kb scope: expected Introduction to have 2 entries, got %d", sectionMap["Introduction"])
	}
	if sectionMap["Conclusion"] != 1 {
		t.Fatalf("kb scope: expected Conclusion to have 1 entry, got %d", sectionMap["Conclusion"])
	}
	if sectionMap["Overview"] != 0 {
		t.Fatalf("kb scope: expected Overview to have 0 entries, got %d", sectionMap["Overview"])
	}

	// Test scope: group (should return 2 sections for g1)
	sections, err = s.CatalogSections(ctx, knowledge.Scope{Type: "group", ID: g1.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("group scope: expected 2 sections, got %d", len(sections))
	}
	if sections[0].Name != "Introduction" || sections[1].Name != "Conclusion" {
		t.Fatalf("group scope: unexpected section names: %s, %s", sections[0].Name, sections[1].Name)
	}

	// Test scope: document (should return 2 sections for doc1)
	sections, err = s.CatalogSections(ctx, knowledge.Scope{Type: "document", ID: int(doc1ID)})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("document scope: expected 2 sections, got %d", len(sections))
	}
	if sections[0].Name != "Introduction" || sections[1].Name != "Conclusion" {
		t.Fatalf("document scope: unexpected section names: %s, %s", sections[0].Name, sections[1].Name)
	}

	// Test invalid scope type
	_, err = s.CatalogSections(ctx, knowledge.Scope{Type: "invalid", ID: 1})
	if err == nil {
		t.Fatal("expected error for invalid scope type")
	}
}

func TestSectionEntryCount(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	g, _ := s.CreateGroup(ctx, "v706")

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

func TestCatalogEntries(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	g, _ := s.CreateGroup(ctx, "v706")

	// Insert document
	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "markdown", "content1", "doc1.md", "ready")
	docID, _ := res.LastInsertId()

	// Create section
	sec, _ := s.CreateSection(ctx, int(docID), "Section1", "summary1", 0)

	// Create 3 entries in the section
	e1, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "Entry1", "Summary of entry 1", "Full content 1", []byte("emb1"), 0)
	e2, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "Entry2", "Summary of entry 2", "Full content 2", []byte("emb2"), 1)
	e3, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "Entry3", "Summary of entry 3", "Full content 3", []byte("emb3"), 2)

	// Call CatalogEntries
	entries, err := s.CatalogEntries(ctx, sec.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify returns 3 entries
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify correct title and summary (not full content)
	if entries[0].ID != e1.ID || entries[0].Title != "Entry1" || entries[0].Summary != "Summary of entry 1" {
		t.Fatalf("entry 0 mismatch: got ID=%d Title=%s Summary=%s", entries[0].ID, entries[0].Title, entries[0].Summary)
	}
	if entries[1].ID != e2.ID || entries[1].Title != "Entry2" || entries[1].Summary != "Summary of entry 2" {
		t.Fatalf("entry 1 mismatch: got ID=%d Title=%s Summary=%s", entries[1].ID, entries[1].Title, entries[1].Summary)
	}
	if entries[2].ID != e3.ID || entries[2].Title != "Entry3" || entries[2].Summary != "Summary of entry 3" {
		t.Fatalf("entry 2 mismatch: got ID=%d Title=%s Summary=%s", entries[2].ID, entries[2].Title, entries[2].Summary)
	}
}

func TestFetchEntries(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	g, _ := s.CreateGroup(ctx, "v706")

	// Insert document
	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "markdown", "content1", "doc1.md", "ready")
	docID, _ := res.LastInsertId()

	// Create section
	sec, _ := s.CreateSection(ctx, int(docID), "Section1", "summary1", 0)

	// Create 2 entries with full content
	e1, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "Entry1", "Summary 1", "Full content of entry 1", []byte("emb1"), 0)
	e2, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "Entry2", "Summary 2", "Full content of entry 2", []byte("emb2"), 1)

	// Test: Fetch specific entries
	entries, err := s.FetchEntries(ctx, []int{e1.ID, e2.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].ID != e1.ID || entries[0].Title != "Entry1" || entries[0].Content != "Full content of entry 1" {
		t.Fatalf("entry 0 mismatch: got ID=%d Title=%s Content=%s", entries[0].ID, entries[0].Title, entries[0].Content)
	}
	if entries[1].ID != e2.ID || entries[1].Title != "Entry2" || entries[1].Content != "Full content of entry 2" {
		t.Fatalf("entry 1 mismatch: got ID=%d Title=%s Content=%s", entries[1].ID, entries[1].Title, entries[1].Content)
	}

	// Test: Empty input returns empty slice
	emptyEntries, err := s.FetchEntries(ctx, []int{})
	if err != nil {
		t.Fatal(err)
	}
	if len(emptyEntries) != 0 {
		t.Fatalf("expected empty slice for empty input, got %d entries", len(emptyEntries))
	}

	// Test: Non-existent IDs return empty slice
	nonExistent, err := s.FetchEntries(ctx, []int{99999, 88888})
	if err != nil {
		t.Fatal(err)
	}
	if len(nonExistent) != 0 {
		t.Fatalf("expected empty slice for non-existent IDs, got %d entries", len(nonExistent))
	}
}

func TestSearch(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	g, _ := s.CreateGroup(ctx, "v706")

	// Insert document
	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "markdown", "content1", "doc1.md", "ready")
	docID, _ := res.LastInsertId()

	// Create section
	sec, _ := s.CreateSection(ctx, int(docID), "Section1", "summary1", 0)

	// Create 2 entries with mock embeddings (16 bytes = 4 floats)
	// Vector 1: [1.0, 0.0, 0.0, 0.0] - should be closer to query [1.0, 0.0, 0.0, 0.0]
	emb1 := make([]byte, 16)
	binary.LittleEndian.PutUint32(emb1[0:4], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(emb1[4:8], math.Float32bits(0.0))
	binary.LittleEndian.PutUint32(emb1[8:12], math.Float32bits(0.0))
	binary.LittleEndian.PutUint32(emb1[12:16], math.Float32bits(0.0))

	// Vector 2: [0.0, 1.0, 0.0, 0.0] - should be farther from query
	emb2 := make([]byte, 16)
	binary.LittleEndian.PutUint32(emb2[0:4], math.Float32bits(0.0))
	binary.LittleEndian.PutUint32(emb2[4:8], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(emb2[8:12], math.Float32bits(0.0))
	binary.LittleEndian.PutUint32(emb2[12:16], math.Float32bits(0.0))

	e1, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "Entry1", "Summary 1", "Full content of entry 1", emb1, 0)
	_, _ = s.CreateEntry(ctx, int(docID), &sec.ID, "Entry2", "Summary 2", "Full content of entry 2", emb2, 1)

	// Query embedding: [1.0, 0.0, 0.0, 0.0] - should match e1 better
	queryEmb := make([]byte, 16)
	binary.LittleEndian.PutUint32(queryEmb[0:4], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(queryEmb[4:8], math.Float32bits(0.0))
	binary.LittleEndian.PutUint32(queryEmb[8:12], math.Float32bits(0.0))
	binary.LittleEndian.PutUint32(queryEmb[12:16], math.Float32bits(0.0))

	// Test: Search within group scope
	entries, err := s.Search(ctx, queryEmb, knowledge.Scope{Type: "group", ID: g.ID}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 1 {
		t.Fatalf("expected at least 1 entry, got %d", len(entries))
	}
	// First result should be e1 (exact match)
	if entries[0].ID != e1.ID {
		t.Fatalf("expected first result to be e1 (ID=%d), got ID=%d", e1.ID, entries[0].ID)
	}
	// Verify full content is returned
	if entries[0].Content != "Full content of entry 1" {
		t.Fatalf("expected full content, got: %s", entries[0].Content)
	}

	// Test: Empty query embedding returns error
	_, err = s.Search(ctx, []byte{}, knowledge.Scope{Type: "group", ID: g.ID}, 5)
	if err == nil {
		t.Fatal("expected error for empty query embedding")
	}

	// Test: Invalid scope type returns error
	_, err = s.Search(ctx, queryEmb, knowledge.Scope{Type: "invalid", ID: g.ID}, 5)
	if err == nil {
		t.Fatal("expected error for invalid scope type")
	}

	// Test: Search within group scope
	groupEntries, err := s.Search(ctx, queryEmb, knowledge.Scope{Type: "group", ID: g.ID}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(groupEntries) < 1 {
		t.Fatalf("expected at least 1 entry in group scope, got %d", len(groupEntries))
	}

	// Test: Search within document scope
	docEntries, err := s.Search(ctx, queryEmb, knowledge.Scope{Type: "document", ID: int(docID)}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(docEntries) < 1 {
		t.Fatalf("expected at least 1 entry in document scope, got %d", len(docEntries))
	}
}

// mockLLMClient returns a fixed JSON clustering response for any input.
type mockLLMClient struct {
	response string
}

func (m *mockLLMClient) Chat(ctx context.Context, req *llm.ChatRequest) (string, error) {
	return m.response, nil
}

func (m *mockLLMClient) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *mockLLMClient) CountTokens(ctx context.Context, msgs []llm.Message) (int, error) {
	return 0, nil
}

// mockEmbedder returns a fixed 3-dim embedding for any text.
type mockEmbedder struct{}

func (e *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (e *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3}
	}
	return out, nil
}

func (e *mockEmbedder) Dimensions() int { return 3 }

// Compile-time checks that mocks satisfy interfaces.
var _ llm.Client = (*mockLLMClient)(nil)
var _ rag.Embedder = (*mockEmbedder)(nil)

func TestImportDocumentOpenAPI(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	g, _ := s.CreateGroup(ctx, "TestGroup")

	content := []byte(`openapi: "3.0.0"
info:
  title: Test API
  version: "1.0"
paths:
  /users:
    get:
      summary: List users
      operationId: listUsers
  /users/{id}:
    get:
      summary: Get user by ID
      operationId: getUser
`)

	req := knowledge.ImportRequest{
		GroupID:  g.ID,
		Name:     "Test API",
		Content:  content,
		Filename: "api.yaml",
		Embedder: &mockEmbedder{},
	}

	result, err := s.ImportDocument(ctx, req)
	if err != nil {
		t.Fatalf("ImportDocument failed: %v", err)
	}

	if result.DocumentID == 0 {
		t.Fatal("expected non-zero DocumentID")
	}
	if result.EntryCount != 2 {
		t.Fatalf("expected 2 entries, got %d", result.EntryCount)
	}
	if result.SectionCount != 1 {
		t.Fatalf("expected 1 section (no LLM), got %d", result.SectionCount)
	}

	doc, err := s.GetDocument(ctx, result.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Status != "ready" {
		t.Fatalf("expected status=ready, got %s", doc.Status)
	}
	if doc.EntryCount != 2 {
		t.Fatalf("expected entry_count=2, got %d", doc.EntryCount)
	}
}

// sequentialLLMClient returns responses in order, cycling through the list.
type sequentialLLMClient struct {
	responses []string
	idx       int
}

func (m *sequentialLLMClient) Chat(ctx context.Context, req *llm.ChatRequest) (string, error) {
	r := m.responses[m.idx%len(m.responses)]
	m.idx++
	return r, nil
}

func (m *sequentialLLMClient) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *sequentialLLMClient) CountTokens(ctx context.Context, msgs []llm.Message) (int, error) {
	return 0, nil
}

func TestImportDocumentMarkdown(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	g, _ := s.CreateGroup(ctx, "TestGroup")

	// First call: markdown parse response; second call: clustering response.
	parseResp, _ := json.Marshal(map[string]interface{}{
		"entries": []map[string]string{
			{"title": "Introduction", "summary": "Intro section", "content": "# Introduction\nWelcome."},
			{"title": "Setup", "summary": "Setup guide", "content": "## Setup\nRun npm install."},
		},
	})
	clusterResp, _ := json.Marshal(map[string]interface{}{
		"sections": []map[string]interface{}{
			{"name": "Getting Started", "summary": "Intro and setup", "entry_ids": []int{0, 1}},
		},
	})

	mockLLM := &sequentialLLMClient{
		responses: []string{string(parseResp), string(clusterResp)},
	}

	content := []byte("# Introduction\nWelcome.\n\n## Setup\nRun npm install.\n")
	req := knowledge.ImportRequest{
		GroupID:   g.ID,
		Name:      "Guide",
		Content:   content,
		Filename:  "guide.md",
		LLMClient: mockLLM,
	}

	result, err := s.ImportDocument(ctx, req)
	if err != nil {
		t.Fatalf("ImportDocument failed: %v", err)
	}

	if result.EntryCount != 2 {
		t.Fatalf("expected 2 entries, got %d", result.EntryCount)
	}
	if result.SectionCount != 1 {
		t.Fatalf("expected 1 section, got %d", result.SectionCount)
	}

	doc, err := s.GetDocument(ctx, result.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Status != "ready" {
		t.Fatalf("expected status=ready, got %s", doc.Status)
	}
}
