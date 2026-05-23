package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/spiderai/spider/internal/knowledge"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/rag"
	"gopkg.in/yaml.v3"
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
	CatalogSections(ctx context.Context, scope knowledge.Scope) ([]knowledge.Section, error)
	CatalogEntries(ctx context.Context, sectionID int) ([]knowledge.EntrySummary, error)
	FetchEntries(ctx context.Context, entryIDs []int) ([]knowledge.Entry, error)
}

func getKnowledgeDocument(s docStore, w http.ResponseWriter, r *http.Request, docIDStr string) {
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}
	doc, err := s.GetDocument(r.Context(), docID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if doc == nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func getKnowledgeDocumentSections(s docStore, w http.ResponseWriter, r *http.Request, docIDStr string) {
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}
	scope := knowledge.Scope{Type: "document", ID: docID}
	sections, err := s.CatalogSections(r.Context(), scope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sections == nil {
		sections = []knowledge.Section{}
	}
	writeJSON(w, http.StatusOK, sections)
}

func getKnowledgeSectionEntries(s docStore, w http.ResponseWriter, r *http.Request, sectionIDStr string) {
	sectionID, err := strconv.Atoi(sectionIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid section id")
		return
	}
	entries, err := s.CatalogEntries(r.Context(), sectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []knowledge.EntrySummary{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func getKnowledgeEntry(s docStore, w http.ResponseWriter, r *http.Request, entryIDStr string) {
	entryID, err := strconv.Atoi(entryIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid entry id")
		return
	}
	entries, err := s.FetchEntries(r.Context(), []int{entryID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(entries) == 0 {
		writeError(w, http.StatusNotFound, "entry not found")
		return
	}
	e := entries[0]
	method, path := splitMethodPath(e.Title)
	resp := map[string]any{
		"id":          e.ID,
		"document_id": e.DocumentID,
		"section_id":  e.SectionID,
		"title":       e.Title,
		"summary":     e.Summary,
		"content":     e.Content,
		"position":    e.Position,
		"method":      method,
		"path":        path,
	}
	if op := parseOpenAPIOperation(e.Content); op != nil {
		resp["description"] = op.Description
		resp["parameters"] = op.Parameters
		resp["responses"] = op.Responses
	}
	writeJSON(w, http.StatusOK, resp)
}

func splitMethodPath(title string) (string, string) {
	parts := strings.SplitN(title, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", title
}

type openAPIParam struct {
	Name        string `json:"name"`
	In          string `json:"in,omitempty"`
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

type openAPIResponse struct {
	Description string `json:"description,omitempty"`
	Example     any    `json:"example,omitempty"`
}

type openAPIOperation struct {
	Description string                     `json:"description,omitempty"`
	Parameters  []openAPIParam             `json:"parameters"`
	Responses   map[string]openAPIResponse `json:"responses"`
}

func parseOpenAPIOperation(content string) *openAPIOperation {
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		return nil
	}
	out := &openAPIOperation{
		Parameters: []openAPIParam{},
		Responses:  map[string]openAPIResponse{},
	}
	if d, ok := raw["description"].(string); ok {
		out.Description = d
	} else if d, ok := raw["summary"].(string); ok {
		out.Description = d
	}
	if pp, ok := raw["parameters"].([]any); ok {
		for _, p := range pp {
			pm, ok := p.(map[string]any)
			if !ok {
				continue
			}
			param := openAPIParam{}
			if v, ok := pm["name"].(string); ok {
				param.Name = v
			}
			if v, ok := pm["in"].(string); ok {
				param.In = v
			}
			if v, ok := pm["required"].(bool); ok {
				param.Required = v
			}
			if v, ok := pm["description"].(string); ok {
				param.Description = v
			}
			if sch, ok := pm["schema"].(map[string]any); ok {
				if t, ok := sch["type"].(string); ok {
					param.Type = t
				}
			} else if t, ok := pm["type"].(string); ok {
				param.Type = t
			}
			out.Parameters = append(out.Parameters, param)
		}
	}
	if rr, ok := raw["responses"].(map[string]any); ok {
		for code, val := range rr {
			vm, ok := val.(map[string]any)
			if !ok {
				continue
			}
			resp := openAPIResponse{}
			if d, ok := vm["description"].(string); ok {
				resp.Description = d
			}
			if c, ok := vm["content"].(map[string]any); ok {
				for _, mt := range c {
					mtm, ok := mt.(map[string]any)
					if !ok {
						continue
					}
					if ex, ok := mtm["example"]; ok {
						resp.Example = ex
						break
					}
					if exs, ok := mtm["examples"].(map[string]any); ok {
						for _, e := range exs {
							em, ok := e.(map[string]any)
							if !ok {
								continue
							}
							if v, ok := em["value"]; ok {
								resp.Example = v
								break
							}
						}
						if resp.Example != nil {
							break
						}
					}
				}
			}
			if ex, ok := vm["example"]; ok && resp.Example == nil {
				resp.Example = ex
			}
			out.Responses[code] = resp
		}
	}
	return out
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

