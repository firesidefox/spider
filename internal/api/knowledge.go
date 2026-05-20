package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/spiderai/spider/internal/knowledge"
)

// kbStore is the subset of knowledge.KnowledgePlugin used by these handlers.
type kbStore interface {
	CreateKB(ctx context.Context, name string) (*knowledge.KnowledgeBase, error)
	ListKBs(ctx context.Context) ([]knowledge.KnowledgeBase, error)
	DeleteKB(ctx context.Context, kbID int) error
	CreateGroup(ctx context.Context, kbID int, name string) (*knowledge.Group, error)
	ListGroups(ctx context.Context, kbID int) ([]knowledge.Group, error)
	DeleteGroup(ctx context.Context, groupID int) error
}

func listKBs(s kbStore, w http.ResponseWriter, r *http.Request) {
	kbs, err := s.ListKBs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kbs == nil {
		kbs = []knowledge.KnowledgeBase{}
	}
	writeJSON(w, http.StatusOK, kbs)
}

func createKB(s kbStore, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	kb, err := s.CreateKB(r.Context(), body.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, kb)
}

func deleteKB(s kbStore, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid kb id")
		return
	}
	if err := s.DeleteKB(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listKBGroups(s kbStore, w http.ResponseWriter, r *http.Request, kbIDStr string) {
	kbID, err := strconv.Atoi(kbIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid kb id")
		return
	}
	groups, err := s.ListGroups(r.Context(), kbID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if groups == nil {
		groups = []knowledge.Group{}
	}
	writeJSON(w, http.StatusOK, groups)
}

func createKBGroup(s kbStore, w http.ResponseWriter, r *http.Request, kbIDStr string) {
	kbID, err := strconv.Atoi(kbIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid kb id")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	g, err := s.CreateGroup(r.Context(), kbID, body.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func deleteKBGroup(s kbStore, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	if err := s.DeleteGroup(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
