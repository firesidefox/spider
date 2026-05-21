package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/spiderai/spider/internal/knowledge"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/rag"
)

// kbStore is the subset of knowledge.KnowledgePlugin used by these handlers.
type kbStore interface {
	CreateGroup(ctx context.Context, name string) (*knowledge.Group, error)
	ListGroups(ctx context.Context) ([]knowledge.Group, error)
	DeleteGroup(ctx context.Context, groupID int) error
	DeleteGroups(ctx context.Context, groupIDs []int) error
}

func listKnowledgeGroups(s kbStore, w http.ResponseWriter, r *http.Request) {
	groups, err := s.ListGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if groups == nil {
		groups = []knowledge.Group{}
	}
	writeJSON(w, http.StatusOK, groups)
}

func createKnowledgeGroup(s kbStore, w http.ResponseWriter, r *http.Request) {
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
	g, err := s.CreateGroup(r.Context(), body.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func deleteKnowledgeGroup(s kbStore, w http.ResponseWriter, r *http.Request, idStr string) {
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

func deleteKnowledgeGroupsBatch(s kbStore, w http.ResponseWriter, r *http.Request) {
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
	if err := s.DeleteGroups(r.Context(), body.IDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type docStore interface {
	ListDocuments(ctx context.Context, groupID int) ([]knowledge.Document, error)
	GetDocument(ctx context.Context, docID int) (*knowledge.Document, error)
	DeleteDocuments(ctx context.Context, docIDs []int) error
	MoveDocuments(ctx context.Context, docIDs []int, targetGroupID int) error
}

func listKnowledgeGroupDocuments(s docStore, w http.ResponseWriter, r *http.Request, groupIDStr string) {
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

func deleteKnowledgeDocuments(s docStore, w http.ResponseWriter, r *http.Request) {
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

func moveKnowledgeDocuments(s docStore, w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs     []int `json:"ids"`
		GroupID int   `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(body.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required and must be non-empty")
		return
	}
	if body.GroupID <= 0 {
		writeError(w, http.StatusBadRequest, "group_id is required")
		return
	}
	if err := s.MoveDocuments(r.Context(), body.IDs, body.GroupID); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func importKnowledgeDocument(ks *knowledge.Store, app *mcppkg.App, embedder rag.Embedder, w http.ResponseWriter, r *http.Request) {
	const maxUploadBytes = 32 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
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
	if app.AgentFactory == nil {
		writeError(w, http.StatusServiceUnavailable, "LLM provider not configured")
		return
	}
	req := knowledge.ImportRequest{
		GroupID:   groupID,
		Name:      header.Filename,
		Content:   content,
		Filename:  header.Filename,
		LLMClient: app.AgentFactory.LLMClient,
		Embedder:  embedder,
	}
	result, err := ks.ImportDocument(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func reindexKnowledgeDocuments(ks *knowledge.Store, app *mcppkg.App, embedder rag.Embedder, w http.ResponseWriter, r *http.Request) {
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
	if app.AgentFactory == nil {
		writeError(w, http.StatusServiceUnavailable, "LLM provider not configured")
		return
	}
	req := knowledge.ImportRequest{
		LLMClient: app.AgentFactory.LLMClient,
		Embedder:  embedder,
	}
	results := make([]*knowledge.ImportResult, 0, len(body.IDs))
	errs := make(map[int]string)
	for _, id := range body.IDs {
		res, err := ks.ReindexDocument(r.Context(), id, req)
		if err != nil {
			errs[id] = err.Error()
			continue
		}
		results = append(results, res)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"errors":  errs,
	})
}

