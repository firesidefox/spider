package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/models"
)

type prometheusSourceStore interface {
	GetByID(id string) (*models.PrometheusSource, error)
	DecryptCredentials(src *models.PrometheusSource) (password, token string, err error)
}

type tryRequest struct {
	SourceID string            `json:"source_id"`
	Params   map[string]string `json:"params"`
}

type tryResult struct {
	Status    int    `json:"status"`
	Body      string `json:"body"`
	LatencyMs int64  `json:"latency_ms"`
}

func tryKnowledgeEntry(ds docStore, ss prometheusSourceStore, w http.ResponseWriter, r *http.Request, entryIDStr string) {
	entryID, err := strconv.Atoi(entryIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid entry id")
		return
	}

	var req tryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SourceID == "" {
		writeError(w, http.StatusBadRequest, "source_id required")
		return
	}

	entries, err := ds.FetchEntries(r.Context(), []int{entryID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(entries) == 0 {
		writeError(w, http.StatusNotFound, "entry not found")
		return
	}
	entry := entries[0]
	_, path := splitMethodPath(entry.Title)

	src, err := ss.GetByID(req.SourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if src == nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	pwd, tok, err := ss.DecryptCredentials(src)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt credentials")
		return
	}

	result, err := doProxyRequest(r.Context(), src, pwd, tok, path, req.Params)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("upstream: %s", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func doProxyRequest(ctx context.Context, src *models.PrometheusSource, pwd, tok, path string, params map[string]string) (*tryResult, error) {
	baseURL := strings.TrimRight(src.BaseURL, "/")

	resolvedPath := path
	queryParams := url.Values{}
	for k, v := range params {
		placeholder := "{" + k + "}"
		if strings.Contains(resolvedPath, placeholder) {
			resolvedPath = strings.ReplaceAll(resolvedPath, placeholder, url.PathEscape(v))
		} else {
			queryParams.Set(k, v)
		}
	}

	fullURL := baseURL + resolvedPath
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	transport := &http.Transport{}
	if src.SkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	timeout := time.Duration(src.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout, Transport: transport}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	switch src.AuthType {
	case models.PrometheusAuthBasic:
		httpReq.SetBasicAuth(src.Username, pwd)
	case models.PrometheusAuthBearer:
		httpReq.Header.Set("Authorization", "Bearer "+tok)
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	latency := time.Since(start).Milliseconds()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &tryResult{
		Status:    resp.StatusCode,
		Body:      string(body),
		LatencyMs: latency,
	}, nil
}
