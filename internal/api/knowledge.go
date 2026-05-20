package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/spiderai/spider/internal/knowledge"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/rag"
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

type docStore interface {
	ListDocuments(ctx context.Context, groupID int) ([]knowledge.Document, error)
	GetDocument(ctx context.Context, docID int) (*knowledge.Document, error)
	DeleteDocuments(ctx context.Context, docIDs []int) error
}

func listGroupDocuments(s docStore, w http.ResponseWriter, r *http.Request, groupIDStr string) {
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	docs, err := s.ListDocuments(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if docs == nil {
		docs = []knowledge.Document{}
	}
	writeJSON(w, http.StatusOK, docs)
}

func deleteDocuments(s docStore, w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(body.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required and must be non-empty")
		return
	}
	if err := s.DeleteDocuments(r.Context(), body.IDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func importKnowledgeDocument(ks *knowledge.Store, app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	groupIDStr := r.FormValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil || groupID <= 0 {
		writeError(w, http.StatusBadRequest, "group_id is required")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	// Get LLM client from AgentFactory
	if app.AgentFactory == nil {
		writeError(w, http.StatusServiceUnavailable, "LLM provider not configured")
		return
	}
	llmClient := app.AgentFactory.LLMClient

	// Get embedder from RagConfigStore
	var embedder rag.Embedder
	cfg, err := app.RagConfigStore.Get()
	if err == nil && cfg != nil && cfg.Model != "" {
		embedder, err = rag.NewEmbedder(cfg.Type, cfg.APIKey, cfg.Model, cfg.BaseURL, 0)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create embedder: %v", err))
			return
		}
	}

	req := knowledge.ImportRequest{
		GroupID:   groupID,
		Name:      header.Filename,
		Content:   content,
		Filename:  header.Filename,
		LLMClient: llmClient,
		Embedder:  embedder,
	}
	result, err := ks.ImportDocument(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
