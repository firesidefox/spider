package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/spiderai/spider/internal/knowledge"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

func listHosts(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	hosts, err := app.HostStore.List(tag)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if hosts == nil {
		hosts = []*models.Host{}
	}
	resp, err := enrichHosts(r.Context(), app, hosts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func addHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req models.AddHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	h, err := app.HostStore.Add(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, h)
}

func getHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	h, err := app.HostStore.GetByID(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "host not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	resp, err := enrichHost(r.Context(), app, h)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func updateHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	var req models.UpdateHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	h, err := app.HostStore.Update(id, &req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "host not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func deleteHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if err := app.HostStore.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "host not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}

// Access face handlers

func listAccessFaces(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string) {
	faces, err := app.AccessFaceStore.ListByHost(hostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if faces == nil {
		faces = []*models.AccessFace{}
	}
	resp, err := enrichAccessFaces(r.Context(), app, faces)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func addAccessFace(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string) {
	var req models.AddAccessFaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateKnowledgeRefs(r.Context(), app, req.KBMode, req.KnowledgeSources); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	f, err := app.AccessFaceStore.Add(hostID, &req)
	if err != nil {
		writeError(w, accessFaceErrorStatus(err), err.Error())
		return
	}
	resp, err := enrichAccessFace(r.Context(), app, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func updateAccessFace(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID, faceID string) {
	existing, err := app.AccessFaceStore.GetByID(faceID)
	if err != nil || existing == nil || existing.HostID != hostID {
		writeError(w, http.StatusNotFound, "access face not found")
		return
	}
	var req models.UpdateAccessFaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateKnowledgeRefs(r.Context(), app, derefString(req.KBMode, existing.KBMode), mergeKnowledgeSources(req.KnowledgeSources, existing.KnowledgeSources)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	f, err := app.AccessFaceStore.Update(faceID, &req)
	if err != nil {
		writeError(w, accessFaceErrorStatus(err), err.Error())
		return
	}
	resp, err := enrichAccessFace(r.Context(), app, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func deleteAccessFace(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID, faceID string) {
	existing, err := app.AccessFaceStore.GetByID(faceID)
	if err != nil || existing == nil || existing.HostID != hostID {
		writeError(w, http.StatusNotFound, "access face not found")
		return
	}
	if err := app.AccessFaceStore.Delete(faceID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "access face not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}

// Fingerprint handler

func getFingerprint(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string) {
	fp, err := app.FingerprintStore.Get(hostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if fp == nil {
		writeError(w, http.StatusNotFound, "fingerprint not found")
		return
	}
	writeJSON(w, http.StatusOK, fp)
}

// Memory handlers

func listMemories(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string) {
	mems, err := app.MemoryStore.ListByHost(hostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if mems == nil {
		mems = []*models.Memory{}
	}
	writeJSON(w, http.StatusOK, mems)
}

func addMemory(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string) {
	var req models.AddMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// set created_by from auth context if not provided
	if req.CreatedBy == "" {
		req.CreatedBy = "user"
	}
	m, err := app.MemoryStore.Add(hostID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func deleteMemory(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string, memID int) {
	if err := app.MemoryStore.Delete(hostID, memID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "memory not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}

// parseMemID parses a string memory ID to int, returns -1 on failure.
func parseMemID(s string) int {
	id, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return id
}

type hostResponse struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	IP             string               `json:"ip"`
	Notes          string               `json:"notes,omitempty"`
	Tags           []string             `json:"tags"`
	Vendor         string               `json:"vendor,omitempty"`
	ProductName    string               `json:"product_name,omitempty"`
	ProductVersion string               `json:"product_version,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	AccessFaces    []accessFaceResponse `json:"access_faces,omitempty"`
	Fingerprint    *models.Fingerprint  `json:"fingerprint,omitempty"`
	Memories       []models.Memory      `json:"memories,omitempty"`
}

type accessFaceResponse struct {
	ID               string                              `json:"id"`
	HostID           string                              `json:"host_id"`
	Type             models.AccessFaceType               `json:"type"`
	IP               string                              `json:"ip"`
	Port             int                                 `json:"port"`
	Username         string                              `json:"username,omitempty"`
	SSHAuthType      models.SSHAuthType                  `json:"ssh_auth_type,omitempty"`
	SSHKeyID         string                              `json:"ssh_key_id,omitempty"`
	SSHLegacy        bool                                `json:"ssh_legacy,omitempty"`
	SSHLoginInput    string                              `json:"ssh_login_input,omitempty"`
	BaseURL          string                              `json:"base_url,omitempty"`
	RESTScheme       string                              `json:"rest_scheme,omitempty"`
	RESTAuthType     models.RESTAuthType                 `json:"rest_auth_type,omitempty"`
	RESTUsername     string                              `json:"rest_username,omitempty"`
	HeaderName       string                              `json:"header_name,omitempty"`
	HMACAlgo         string                              `json:"hmac_algo,omitempty"`
	KBMode           string                              `json:"kb_mode"`
	KnowledgeSources []models.KnowledgeSourceRefEnriched `json:"knowledge_sources"`
	ProbePort        int                                 `json:"probe_port,omitempty"`
	PrometheusSourceID string                            `json:"prometheus_source_id,omitempty"`
	CreatedAt        time.Time                           `json:"created_at"`
	UpdatedAt        time.Time                           `json:"updated_at"`
}

func enrichHosts(ctx context.Context, app *mcppkg.App, hosts []*models.Host) ([]hostResponse, error) {
	if err := hydrateHostAccessFaces(app, hosts); err != nil {
		return nil, err
	}
	cache, err := buildKnowledgeRefCache(ctx, app, hosts)
	if err != nil {
		return nil, err
	}
	out := make([]hostResponse, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, makeHostResponse(h, cache))
	}
	return out, nil
}

func enrichHost(ctx context.Context, app *mcppkg.App, h *models.Host) (hostResponse, error) {
	if err := hydrateHostAccessFaces(app, []*models.Host{h}); err != nil {
		return hostResponse{}, err
	}
	cache, err := buildKnowledgeRefCache(ctx, app, []*models.Host{h})
	if err != nil {
		return hostResponse{}, err
	}
	return makeHostResponse(h, cache), nil
}

func hydrateHostAccessFaces(app *mcppkg.App, hosts []*models.Host) error {
	for _, h := range hosts {
		faces, err := app.AccessFaceStore.ListByHost(h.ID)
		if err != nil {
			return err
		}
		h.AccessFaces = make([]models.AccessFace, 0, len(faces))
		for _, f := range faces {
			h.AccessFaces = append(h.AccessFaces, *f)
		}
	}
	return nil
}

func enrichAccessFace(ctx context.Context, app *mcppkg.App, f *models.AccessFace) (accessFaceResponse, error) {
	h := &models.Host{AccessFaces: []models.AccessFace{*f}}
	cache, err := buildKnowledgeRefCache(ctx, app, []*models.Host{h})
	if err != nil {
		return accessFaceResponse{}, err
	}
	return makeAccessFaceResponse(*f, cache), nil
}

func enrichAccessFaces(ctx context.Context, app *mcppkg.App, faces []*models.AccessFace) ([]accessFaceResponse, error) {
	h := &models.Host{AccessFaces: make([]models.AccessFace, 0, len(faces))}
	for _, f := range faces {
		h.AccessFaces = append(h.AccessFaces, *f)
	}
	cache, err := buildKnowledgeRefCache(ctx, app, []*models.Host{h})
	if err != nil {
		return nil, err
	}
	out := make([]accessFaceResponse, 0, len(faces))
	for _, f := range faces {
		out = append(out, makeAccessFaceResponse(*f, cache))
	}
	return out, nil
}

type knowledgeRefCache struct {
	groups map[int]knowledge.Group
	docs   map[int]knowledge.Document
}

func buildKnowledgeRefCache(ctx context.Context, app *mcppkg.App, hosts []*models.Host) (knowledgeRefCache, error) {
	groupIDs := map[int]struct{}{}
	docIDs := map[int]struct{}{}
	for _, h := range hosts {
		for _, f := range h.AccessFaces {
			if f.KBMode == "none" {
				continue
			}
			for _, src := range f.KnowledgeSources {
				switch src.Type {
				case "group":
					groupIDs[src.ID] = struct{}{}
				case "doc":
					docIDs[src.ID] = struct{}{}
				}
			}
		}
	}
	docs, err := app.KnowledgeStore.GetDocumentsByIDs(ctx, keysInt(docIDs))
	if err != nil {
		return knowledgeRefCache{}, err
	}
	for _, d := range docs {
		groupIDs[d.GroupID] = struct{}{}
	}
	groups, err := app.KnowledgeStore.GetGroupsByIDs(ctx, keysInt(groupIDs))
	if err != nil {
		return knowledgeRefCache{}, err
	}
	cache := knowledgeRefCache{
		groups: make(map[int]knowledge.Group, len(groups)),
		docs:   make(map[int]knowledge.Document, len(docs)),
	}
	for _, g := range groups {
		cache.groups[g.ID] = g
	}
	for _, d := range docs {
		cache.docs[d.ID] = d
	}
	return cache, nil
}

func makeHostResponse(h *models.Host, cache knowledgeRefCache) hostResponse {
	resp := hostResponse{
		ID:             h.ID,
		Name:           h.Name,
		IP:             h.IP,
		Notes:          h.Notes,
		Tags:           h.Tags,
		Vendor:         h.Vendor,
		ProductName:    h.ProductName,
		ProductVersion: h.ProductVersion,
		CreatedAt:      h.CreatedAt,
		UpdatedAt:      h.UpdatedAt,
		Fingerprint:    h.Fingerprint,
		Memories:       h.Memories,
	}
	for _, f := range h.AccessFaces {
		resp.AccessFaces = append(resp.AccessFaces, makeAccessFaceResponse(f, cache))
	}
	return resp
}

func makeAccessFaceResponse(f models.AccessFace, cache knowledgeRefCache) accessFaceResponse {
	return accessFaceResponse{
		ID:               f.ID,
		HostID:           f.HostID,
		Type:             f.Type,
		IP:               f.IP,
		Port:             f.Port,
		Username:         f.Username,
		SSHAuthType:      f.SSHAuthType,
		SSHKeyID:         f.SSHKeyID,
		SSHLegacy:        f.SSHLegacy,
		SSHLoginInput:    f.SSHLoginInput,
		BaseURL:          f.BaseURL,
		RESTScheme:       f.RESTScheme,
		RESTAuthType:     f.RESTAuthType,
		RESTUsername:     f.RESTUsername,
		HeaderName:       f.HeaderName,
		HMACAlgo:         f.HMACAlgo,
		KBMode:           f.KBMode,
		KnowledgeSources: enrichKnowledgeSources(f, cache),
		ProbePort:        f.ProbePort,
		PrometheusSourceID: f.PrometheusSourceID,
		CreatedAt:        f.CreatedAt,
		UpdatedAt:        f.UpdatedAt,
	}
}

func enrichKnowledgeSources(f models.AccessFace, cache knowledgeRefCache) []models.KnowledgeSourceRefEnriched {
	if f.KBMode == "none" {
		return []models.KnowledgeSourceRefEnriched{}
	}
	out := make([]models.KnowledgeSourceRefEnriched, 0, len(f.KnowledgeSources))
	for _, src := range f.KnowledgeSources {
		enriched := models.KnowledgeSourceRefEnriched{Type: src.Type, ID: src.ID}
		switch src.Type {
		case "group":
			if g, ok := cache.groups[src.ID]; ok {
				enriched.Name = g.Name
			}
		case "doc":
			if d, ok := cache.docs[src.ID]; ok {
				enriched.Title = d.Name
				enriched.GroupID = d.GroupID
				if g, ok := cache.groups[d.GroupID]; ok {
					enriched.GroupName = g.Name
				}
			}
		}
		out = append(out, enriched)
	}
	return out
}

func validateKnowledgeRefs(ctx context.Context, app *mcppkg.App, mode string, sources []models.KnowledgeSourceRef) error {
	if mode == "" {
		mode = "none"
	}
	if mode == "none" {
		return nil
	}
	groupIDs := map[int]struct{}{}
	docIDs := map[int]struct{}{}
	for _, src := range sources {
		switch src.Type {
		case "group":
			groupIDs[src.ID] = struct{}{}
		case "doc":
			docIDs[src.ID] = struct{}{}
		}
	}
	groups, err := app.KnowledgeStore.GetGroupsByIDs(ctx, keysInt(groupIDs))
	if err != nil {
		return err
	}
	docs, err := app.KnowledgeStore.GetDocumentsByIDs(ctx, keysInt(docIDs))
	if err != nil {
		return err
	}
	if len(groups) != len(groupIDs) {
		return fmt.Errorf("knowledge group not found")
	}
	if len(docs) != len(docIDs) {
		return fmt.Errorf("knowledge document not found")
	}
	return nil
}

func keysInt(m map[int]struct{}) []int {
	out := make([]int, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out
}

func derefString(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}

func mergeKnowledgeSources(next, current []models.KnowledgeSourceRef) []models.KnowledgeSourceRef {
	if next == nil {
		return current
	}
	return next
}

func accessFaceErrorStatus(err error) int {
	msg := err.Error()
	if msg == "invalid kb_mode" ||
		msg == "kb_mode=specific requires at least one knowledge_source" ||
		msg == "knowledge_sources exceeds limit of 10" ||
		msg == "invalid knowledge_source type" ||
		msg == "invalid knowledge_source id" {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
