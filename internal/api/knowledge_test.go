package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spiderai/spider/internal/knowledge"
)

// mockKBStore is an in-memory implementation of kbStore for tests.
type mockKBStore struct {
	groups  []knowledge.Group
	docs    []knowledge.Document
	nextGr  int
	nextDoc int
}

func newMockKBStore() *mockKBStore { return &mockKBStore{nextGr: 1, nextDoc: 1} }

func (m *mockKBStore) CreateGroup(_ context.Context, name string) (*knowledge.Group, error) {
	g := knowledge.Group{ID: m.nextGr, Name: name}
	m.nextGr++
	m.groups = append(m.groups, g)
	return &g, nil
}

func (m *mockKBStore) ListGroups(_ context.Context) ([]knowledge.Group, error) {
	return m.groups, nil
}

func (m *mockKBStore) DeleteGroup(_ context.Context, groupID int) error {
	for i, g := range m.groups {
		if g.ID == groupID {
			m.groups = append(m.groups[:i], m.groups[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockKBStore) DeleteGroups(_ context.Context, groupIDs []int) error {
	for _, id := range groupIDs {
		m.DeleteGroup(context.Background(), id)
	}
	return nil
}

func (m *mockKBStore) ListDocuments(_ context.Context, groupID int) ([]knowledge.Document, error) {
	var out []knowledge.Document
	for _, d := range m.docs {
		if d.GroupID == groupID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (m *mockKBStore) GetDocument(_ context.Context, docID int) (*knowledge.Document, error) {
	for _, d := range m.docs {
		if d.ID == docID {
			return &d, nil
		}
	}
	return nil, nil
}

func (m *mockKBStore) DeleteDocuments(_ context.Context, docIDs []int) error {
	for _, id := range docIDs {
		for i, d := range m.docs {
			if d.ID == id {
				m.docs = append(m.docs[:i], m.docs[i+1:]...)
				break
			}
		}
	}
	return nil
}

func (m *mockKBStore) MoveDocuments(_ context.Context, docIDs []int, targetGroupID int) error {
	for _, id := range docIDs {
		for i := range m.docs {
			if m.docs[i].ID == id {
				m.docs[i].GroupID = targetGroupID
			}
		}
	}
	return nil
}

func (m *mockKBStore) CatalogSections(_ context.Context, scope knowledge.Scope) ([]knowledge.Section, error) {
	return []knowledge.Section{}, nil
}

func (m *mockKBStore) CatalogEntries(_ context.Context, sectionID int) ([]knowledge.EntrySummary, error) {
	return []knowledge.EntrySummary{}, nil
}

func TestListKnowledgeGroups(t *testing.T) {
	mock := newMockKBStore()
	mock.CreateGroup(context.Background(), "AISG")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-groups", nil)
	w := httptest.NewRecorder()
	listKnowledgeGroups(mock, w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var groups []knowledge.Group
	json.NewDecoder(w.Body).Decode(&groups)
	if len(groups) != 1 || groups[0].Name != "AISG" {
		t.Fatalf("unexpected groups: %+v", groups)
	}
}

func TestCreateKnowledgeGroup(t *testing.T) {
	mock := newMockKBStore()
	body := bytes.NewBufferString(`{"name":"v706"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-groups", body)
	w := httptest.NewRecorder()
	createKnowledgeGroup(mock, w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	var g knowledge.Group
	json.NewDecoder(w.Body).Decode(&g)
	if g.Name != "v706" {
		t.Fatalf("unexpected group: %+v", g)
	}
}

func TestDeleteKnowledgeGroup(t *testing.T) {
	mock := newMockKBStore()
	g, _ := mock.CreateGroup(context.Background(), "v706")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/knowledge-groups/1", nil)
	w := httptest.NewRecorder()
	deleteKnowledgeGroup(mock, w, req, "1")

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	groups, _ := mock.ListGroups(context.Background())
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups after delete, got %d", len(groups))
	}
	_ = g
}

func TestListKnowledgeGroupDocuments(t *testing.T) {
	mock := newMockKBStore()
	g, _ := mock.CreateGroup(context.Background(), "v706")
	mock.docs = append(mock.docs, knowledge.Document{ID: 1, GroupID: g.ID, Name: "doc1"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-groups/1/documents", nil)
	w := httptest.NewRecorder()
	listKnowledgeGroupDocuments(mock, w, req, "1")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var docs []knowledge.Document
	json.NewDecoder(w.Body).Decode(&docs)
	if len(docs) != 1 || docs[0].Name != "doc1" {
		t.Fatalf("unexpected docs: %+v", docs)
	}
}

func TestDeleteKnowledgeDocuments(t *testing.T) {
	mock := newMockKBStore()
	g, _ := mock.CreateGroup(context.Background(), "v706")
	mock.docs = append(mock.docs, knowledge.Document{ID: 1, GroupID: g.ID, Name: "doc1"})
	mock.docs = append(mock.docs, knowledge.Document{ID: 2, GroupID: g.ID, Name: "doc2"})

	body := bytes.NewBufferString(`{"ids":[1,2]}`)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/knowledge-documents", body)
	w := httptest.NewRecorder()
	deleteKnowledgeDocuments(mock, w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if len(mock.docs) != 0 {
		t.Fatalf("expected 0 docs after delete, got %d", len(mock.docs))
	}
}

func TestMoveKnowledgeDocuments(t *testing.T) {
	mock := newMockKBStore()
	g1, _ := mock.CreateGroup(context.Background(), "v706")
	g2, _ := mock.CreateGroup(context.Background(), "v707")
	mock.docs = append(mock.docs, knowledge.Document{ID: 1, GroupID: g1.ID, Name: "doc1"})

	body := bytes.NewBufferString(`{"ids":[1],"group_id":2}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/knowledge-documents", body)
	w := httptest.NewRecorder()
	moveKnowledgeDocuments(mock, w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if mock.docs[0].GroupID != g2.ID {
		t.Fatalf("expected doc to be moved to group %d, got %d", g2.ID, mock.docs[0].GroupID)
	}
}
