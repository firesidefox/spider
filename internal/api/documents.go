package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/rag"
)

func ragStore(app *mcppkg.App) (*rag.Store, error) {
	return app.GetOrBuildRagStore()
}

func listDocuments(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	vendor := r.URL.Query().Get("vendor")
	tag := r.URL.Query().Get("tag")
	groupIDStr := r.URL.Query().Get("group_id")
	var (
		docs any
		err  error
	)
	switch {
	case groupIDStr != "":
		gid, convErr := strconv.Atoi(groupIDStr)
		if convErr != nil {
			writeError(w, http.StatusBadRequest, "invalid group_id")
			return
		}
		docs, err = app.DocStore.ListByGroup(gid)
	case tag != "":
		docs, err = app.DocStore.ListByTag(tag)
	case vendor != "":
		docs, err = app.DocStore.ListByVendor(vendor)
	default:
		docs, err = app.DocStore.List()
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func deleteDocument(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := app.DocStore.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func ingestDocument(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Vendor       string   `json:"vendor"`
		Tags         []string `json:"tags"`
		Title        string   `json:"title"`
		Content      string   `json:"content"`
		SourceFile   string   `json:"source_file"`
		ChunkIndex   int      `json:"chunk_index"`
		GroupID      *int     `json:"group_id"`
		UseEmbedding *bool    `json:"use_embedding"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	useEmbed := req.UseEmbedding == nil || *req.UseEmbedding
	if useEmbed {
		rs, err := ragStore(app)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "embedding unavailable: "+err.Error())
			return
		}
		if err := rs.Ingest(r.Context(), req.Vendor, req.Tags, req.Title, req.Content, req.SourceFile, req.ChunkIndex, req.GroupID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		if err := app.DocStore.Save(req.Vendor, req.Tags, req.Title, req.Content, nil, req.SourceFile, req.ChunkIndex, req.GroupID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
}

func searchDocuments(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}
	vendor := r.URL.Query().Get("vendor")
	topK := 5
	if s := r.URL.Query().Get("top_k"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			topK = n
		}
	}
	rs, err := ragStore(app)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "embedding unavailable: "+err.Error())
		return
	}
	results, err := rs.Search(r.Context(), q, vendor, topK)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func listGroups(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	groups, err := app.GroupStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func createGroup(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	g, err := app.GroupStore.Create(req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func deleteGroup(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := app.GroupStore.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func renameGroup(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := app.GroupStore.Rename(id, req.Name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func moveDocumentToGroup(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	docID, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		GroupID *int `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := app.GroupStore.MoveDocument(docID, req.GroupID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func deleteBatchDocuments(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	if len(req.IDs) > 500 {
		writeError(w, http.StatusBadRequest, "too many ids (max 500)")
		return
	}
	if err := app.DocStore.DeleteBatch(req.IDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func deleteBatchGroups(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs             []int `json:"ids"`
		DeleteDocuments bool  `json:"delete_documents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	if len(req.IDs) > 500 {
		writeError(w, http.StatusBadRequest, "too many ids (max 500)")
		return
	}
	if err := app.GroupStore.DeleteBatch(req.IDs, req.DeleteDocuments); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func normalizeDescription(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("\n", " ", "\t", " ", "\r", " ").Replace(s)
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	runes := []rune(s)
	if len(runes) > 200 {
		runes = runes[:200]
	}
	return string(runes)
}

func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) > maxRunes {
		return string(r[:maxRunes])
	}
	return s
}

func regenerateDocDescription(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}
	doc, err := app.DocStore.GetByID(id)
	if err != nil || doc == nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if app.AgentFactory == nil {
		writeError(w, http.StatusServiceUnavailable, "llm provider unavailable")
		return
	}
	prompt := fmt.Sprintf(
		"为知识库文档生成一句话描述。文档标题: %s。文档内容摘要:\n%s\n输出一句话（≤50字）概括本文档主题。纯文本，不含换行与Markdown。",
		doc.Title, truncate(doc.Content, 500),
	)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	resp, err := app.AgentFactory.LLMClient.Chat(ctx, &llm.ChatRequest{
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 256,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "llm provider unavailable")
		return
	}
	desc := normalizeDescription(resp)
	if err := app.DocStore.UpdateDescription(r.Context(), id, desc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"description": desc})
}

func regenerateGroupDescription(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	groupID, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	groups, err := app.GroupStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var group *models.DocumentGroup
	for _, g := range groups {
		if g.ID == groupID {
			group = g
			break
		}
	}
	if group == nil {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	docs, err := app.DocStore.ListByGroup(groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(docs) == 0 {
		writeError(w, http.StatusBadRequest, "group has no documents")
		return
	}
	if app.AgentFactory == nil {
		writeError(w, http.StatusServiceUnavailable, "llm provider unavailable")
		return
	}
	var sb strings.Builder
	for _, d := range docs {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", d.Title, d.Description))
	}
	prompt := fmt.Sprintf(
		"为知识库分组生成一句话描述。分组名: %s。包含文档:\n%s\n输出一句话（≤50字）概括本组涵盖的知识范围。纯文本，不含换行与Markdown。",
		group.Name, sb.String(),
	)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	resp, err := app.AgentFactory.LLMClient.Chat(ctx, &llm.ChatRequest{
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 256,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "llm provider unavailable")
		return
	}
	desc := normalizeDescription(resp)
	if err := app.GroupStore.UpdateDescription(r.Context(), groupID, desc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"description": desc})
}

func updateDocDescription(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}
	doc, err := app.DocStore.GetByID(id)
	if err != nil || doc == nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	var body struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	desc := normalizeDescription(body.Description)
	if err := app.DocStore.UpdateDescription(r.Context(), id, desc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"description": desc})
}

func updateGroupDescription(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	groupID, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	groups, err := app.GroupStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	found := false
	for _, g := range groups {
		if g.ID == groupID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	var body struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	desc := normalizeDescription(body.Description)
	if err := app.GroupStore.UpdateDescription(r.Context(), groupID, desc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"description": desc})
}
