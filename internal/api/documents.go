package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	mcppkg "github.com/spiderai/spider/internal/mcp"
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
		Vendor     string   `json:"vendor"`
		Tags       []string `json:"tags"`
		Title      string   `json:"title"`
		Content    string   `json:"content"`
		SourceFile string   `json:"source_file"`
		ChunkIndex int      `json:"chunk_index"`
		GroupID    *int     `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	rs, err := ragStore(app)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "embedding unavailable: "+err.Error())
		return
	}
	if err := rs.Ingest(r.Context(), req.Vendor, req.Tags, req.Title, req.Content, req.SourceFile, req.ChunkIndex, req.GroupID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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
