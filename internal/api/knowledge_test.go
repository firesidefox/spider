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
	kbs    []knowledge.KnowledgeBase
	groups []knowledge.Group
	nextKB int
	nextGr int
}

func newMockKBStore() *mockKBStore { return &mockKBStore{nextKB: 1, nextGr: 1} }

func (m *mockKBStore) CreateKB(_ context.Context, name string) (*knowledge.KnowledgeBase, error) {
	kb := knowledge.KnowledgeBase{ID: m.nextKB, Name: name}
	m.nextKB++
	m.kbs = append(m.kbs, kb)
	return &kb, nil
}

func (m *mockKBStore) ListKBs(_ context.Context) ([]knowledge.KnowledgeBase, error) {
	return m.kbs, nil
}

func (m *mockKBStore) DeleteKB(_ context.Context, kbID int) error {
	for i, kb := range m.kbs {
		if kb.ID == kbID {
			m.kbs = append(m.kbs[:i], m.kbs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockKBStore) CreateGroup(_ context.Context, kbID int, name string) (*knowledge.Group, error) {
	g := knowledge.Group{ID: m.nextGr, KBID: kbID, Name: name}
	m.nextGr++
	m.groups = append(m.groups, g)
	return &g, nil
}

func (m *mockKBStore) ListGroups(_ context.Context, kbID int) ([]knowledge.Group, error) {
	var out []knowledge.Group
	for _, g := range m.groups {
		if g.KBID == kbID {
			out = append(out, g)
		}
	}
	return out, nil
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

// --- Tests ---

func TestListKBs(t *testing.T) {
	s := newMockKBStore()
	s.CreateKB(context.Background(), "alpha") //nolint
	w := httptest.NewRecorder()
	listKBs(s, w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var kbs []knowledge.KnowledgeBase
	json.NewDecoder(w.Body).Decode(&kbs)
	if len(kbs) != 1 || kbs[0].Name != "alpha" {
		t.Fatalf("unexpected kbs: %+v", kbs)
	}
}

func TestCreateKB(t *testing.T) {
	s := newMockKBStore()
	body := bytes.NewBufferString(`{"name":"beta"}`)
	w := httptest.NewRecorder()
	createKB(s, w, httptest.NewRequest(http.MethodPost, "/", body))
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
	var kb knowledge.KnowledgeBase
	json.NewDecoder(w.Body).Decode(&kb)
	if kb.Name != "beta" {
		t.Fatalf("unexpected name: %s", kb.Name)
	}
}

func TestCreateKBMissingName(t *testing.T) {
	s := newMockKBStore()
	body := bytes.NewBufferString(`{"name":""}`)
	w := httptest.NewRecorder()
	createKB(s, w, httptest.NewRequest(http.MethodPost, "/", body))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestListKBGroups(t *testing.T) {
	s := newMockKBStore()
	s.CreateGroup(context.Background(), 1, "g1") //nolint
	s.CreateGroup(context.Background(), 2, "g2") //nolint
	w := httptest.NewRecorder()
	listKBGroups(s, w, httptest.NewRequest(http.MethodGet, "/", nil), "1")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var groups []knowledge.Group
	json.NewDecoder(w.Body).Decode(&groups)
	if len(groups) != 1 || groups[0].Name != "g1" {
		t.Fatalf("unexpected groups: %+v", groups)
	}
}

func TestCreateKBGroup(t *testing.T) {
	s := newMockKBStore()
	body := bytes.NewBufferString(`{"name":"mygroup"}`)
	w := httptest.NewRecorder()
	createKBGroup(s, w, httptest.NewRequest(http.MethodPost, "/", body), "1")
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
	var g knowledge.Group
	json.NewDecoder(w.Body).Decode(&g)
	if g.Name != "mygroup" || g.KBID != 1 {
		t.Fatalf("unexpected group: %+v", g)
	}
}

func TestDeleteKBGroup(t *testing.T) {
	s := newMockKBStore()
	s.CreateGroup(context.Background(), 1, "todelete") //nolint
	w := httptest.NewRecorder()
	deleteKBGroup(s, w, httptest.NewRequest(http.MethodDelete, "/", nil), "1")
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}
	if len(s.groups) != 0 {
		t.Fatalf("expected group deleted, got %+v", s.groups)
	}
}
