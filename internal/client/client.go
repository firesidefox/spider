package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/spiderai/spider/internal/models"
)

// Client 是 spider HTTP API 的客户端。
type Client struct {
	base string
	http *http.Client
}

// New 创建客户端，baseURL 如 "http://localhost:8000"。
func New(baseURL string) *Client {
	return &Client{base: baseURL, http: &http.Client{}}
}

// ── hosts ─────────────────────────────────────────────────────────────────────

func (c *Client) ListHosts(tag string) ([]*models.SafeHost, error) {
	u := c.base + "/api/v1/hosts"
	if tag != "" {
		u += "?tag=" + url.QueryEscape(tag)
	}
	var hosts []*models.SafeHost
	return hosts, c.get(u, &hosts)
}

func (c *Client) AddHost(req *models.AddHostRequest) (*models.SafeHost, error) {
	var h models.SafeHost
	return &h, c.post(c.base+"/api/v1/hosts", req, &h)
}

func (c *Client) GetHost(idOrName string) (*models.SafeHost, error) {
	var h models.SafeHost
	return &h, c.get(c.base+"/api/v1/hosts/"+url.PathEscape(idOrName), &h)
}

func (c *Client) UpdateHost(id string, req *models.UpdateHostRequest) (*models.SafeHost, error) {
	var h models.SafeHost
	return &h, c.put(c.base+"/api/v1/hosts/"+url.PathEscape(id), req, &h)
}

func (c *Client) DeleteHost(id string) error {
	return c.delete(c.base + "/api/v1/hosts/" + url.PathEscape(id))
}

// PingResult is the response from POST /api/v1/hosts/:id/ping.
type PingResult struct {
	Connected bool   `json:"connected"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

func (c *Client) PingHost(idOrName string) (*PingResult, error) {
	var r PingResult
	return &r, c.post(c.base+"/api/v1/hosts/"+url.PathEscape(idOrName)+"/ping", nil, &r)
}

// ── exec ──────────────────────────────────────────────────────────────────────

type ExecRequest struct {
	HostID         string `json:"host_id"`
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type ExecResult struct {
	Host       string `json:"host"`
	Command    string `json:"command"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

func (c *Client) Exec(req *ExecRequest) (*ExecResult, error) {
	var r ExecResult
	return &r, c.post(c.base+"/api/v1/exec", req, &r)
}

// ── logs ──────────────────────────────────────────────────────────────────────

func (c *Client) ListLogs(hostID string, limit int) ([]*models.ExecutionLog, error) {
	u := c.base + "/api/v1/logs?limit=" + strconv.Itoa(limit)
	if hostID != "" {
		u += "&host_id=" + url.QueryEscape(hostID)
	}
	var logs []*models.ExecutionLog
	return logs, c.get(u, &logs)
}

// ── http helpers ──────────────────────────────────────────────────────────────

func (c *Client) get(u string, out any) error {
	return c.do(http.MethodGet, u, nil, out)
}

func (c *Client) post(u string, body, out any) error {
	return c.do(http.MethodPost, u, body, out)
}

func (c *Client) put(u string, body, out any) error {
	return c.do(http.MethodPut, u, body, out)
}

func (c *Client) delete(u string) error {
	return c.do(http.MethodDelete, u, nil, nil)
}

func (c *Client) do(method, u string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	return c.decode(resp, out)
}

func (c *Client) decode(resp *http.Response, out any) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
