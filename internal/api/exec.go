package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
)

type execRequest struct {
	HostID         string `json:"host_id"`
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type execResult struct {
	Host       string `json:"host"`
	Command    string `json:"command"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

func execTimeout(seconds int, cfg interface{ GetDefaultTimeout() int }) time.Duration {
	if seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 30 * time.Second
}

func runExec(ctx context.Context, app *mcppkg.App, host *models.Host, command string, timeout time.Duration) execResult {
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := app.Pool.Get(host, app.HostStore)
	if err != nil {
		return execResult{Host: host.Name, Command: command, Error: fmt.Sprintf("SSH 连接失败: %v", err)}
	}
	defer app.Pool.Release(host.ID)

	result, err := client.Execute(execCtx, command)
	if err != nil {
		return execResult{Host: host.Name, Command: command, Error: fmt.Sprintf("执行失败: %v", err)}
	}

	_ = app.LogStore.Save(&models.ExecutionLog{
		HostID:      host.ID,
		Command:     command,
		Stdout:      result.Stdout,
		Stderr:      result.Stderr,
		ExitCode:    result.ExitCode,
		DurationMs:  result.Duration.Milliseconds(),
		TriggeredBy: "web",
	})

	return execResult{
		Host:       host.Name,
		Command:    command,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		ExitCode:   result.ExitCode,
		DurationMs: result.Duration.Milliseconds(),
	}
}

func execCommand(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req execRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.HostID == "" || req.Command == "" {
		writeError(w, http.StatusBadRequest, "host_id 和 command 不能为空")
		return
	}
	host, err := app.HostStore.GetByIDOrName(req.HostID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(app.Config.SSH.DefaultTimeout) * time.Second
	}
	res := runExec(r.Context(), app, host, req.Command, timeout)
	writeJSON(w, http.StatusOK, res)
}

func streamCommand(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	hostID := q.Get("host_id")
	command := q.Get("command")
	timeoutSec, _ := strconv.Atoi(q.Get("timeout"))
	if hostID == "" || command == "" {
		writeError(w, http.StatusBadRequest, "host_id 和 command 不能为空")
		return
	}
	host, err := app.HostStore.GetByIDOrName(hostID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	timeout := time.Duration(timeoutSec) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(app.Config.SSH.DefaultTimeout) * time.Second
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "不支持 SSE")
		return
	}

	sendEvent := func(data any) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	execCtx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	start := time.Now()
	client, err := app.Pool.Get(host, app.HostStore)
	if err != nil {
		sendEvent(map[string]any{"type": "done", "exit_code": -1, "error": err.Error()})
		return
	}
	defer app.Pool.Release(host.ID)

	result, err := client.Execute(execCtx, command)
	if err != nil {
		sendEvent(map[string]any{"type": "done", "exit_code": -1, "error": err.Error()})
		return
	}

	if result.Stdout != "" {
		sendEvent(map[string]any{"type": "stdout", "data": result.Stdout})
	}
	if result.Stderr != "" {
		sendEvent(map[string]any{"type": "stderr", "data": result.Stderr})
	}
	durationMs := time.Since(start).Milliseconds()
	sendEvent(map[string]any{"type": "done", "exit_code": result.ExitCode, "duration_ms": durationMs})

	_ = app.LogStore.Save(&models.ExecutionLog{
		HostID:      host.ID,
		Command:     command,
		Stdout:      result.Stdout,
		Stderr:      result.Stderr,
		ExitCode:    result.ExitCode,
		DurationMs:  durationMs,
		TriggeredBy: "web",
	})
}

type batchRequest struct {
	HostIDs        string `json:"host_ids"`
	Tag            string `json:"tag"`
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func execBatch(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command 不能为空")
		return
	}

	var hosts []*models.Host
	if req.Tag != "" {
		hs, err := app.HostStore.List(req.Tag)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		hosts = hs
	} else if req.HostIDs != "" {
		for _, id := range strings.Split(req.HostIDs, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			h, err := app.HostStore.GetByIDOrName(id)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("主机不存在: %s", id))
				return
			}
			hosts = append(hosts, h)
		}
	} else {
		writeError(w, http.StatusBadRequest, "必须提供 host_ids 或 tag")
		return
	}

	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(app.Config.SSH.DefaultTimeout) * time.Second
	}

	results := make([]execResult, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		go func(idx int, host *models.Host) {
			defer wg.Done()
			results[idx] = runExec(r.Context(), app, host, req.Command, timeout)
		}(i, h)
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, results)
}
