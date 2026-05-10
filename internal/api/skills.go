package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spiderai/spider/internal/agent"
)

type skillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

func isValidSkillName(name string) bool {
	if name == "" {
		return false
	}
	// 允许一级斜杠（如 spider/cron），但不允许 .. 或多级
	parts := strings.SplitN(name, "/", 3)
	if len(parts) > 2 {
		return false
	}
	for _, p := range parts {
		if p == "" || strings.ContainsAny(p, `\.`) {
			return false
		}
	}
	return true
}

func listSkillsHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sm := agent.NewSkillManager(filepath.Join(dataDir, "skills"))
		entries, err := sm.LoadSkills()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read skills dir")
			return
		}
		skills := make([]skillInfo, len(entries))
		for i, e := range entries {
			skills[i] = skillInfo{
				Name:        e.Name,
				Description: e.Description,
				Status:      e.Status,
				Error:       e.Error,
			}
		}
		writeJSON(w, http.StatusOK, skills)
	}
}

func uploadSkillHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !isValidSkillName(name) {
			writeError(w, http.StatusBadRequest, "invalid skill name")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read body")
			return
		}
		if _, _, err := agent.ParseSkillFrontmatter(string(body)); err != nil {
			writeError(w, http.StatusBadRequest, "invalid SKILL.md: "+err.Error())
			return
		}
		dir := filepath.Join(dataDir, "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create skill dir")
			return
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), body, 0o644); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to write SKILL.md")
			return
		}
		writeJSON(w, http.StatusOK, skillInfo{Name: name, Status: "ok"})
	}
}

func deleteSkillHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !isValidSkillName(name) {
			writeError(w, http.StatusBadRequest, "invalid skill name")
			return
		}
		dir := filepath.Join(dataDir, "skills", name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete skill")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func getSkillHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !isValidSkillName(name) {
			writeError(w, http.StatusBadRequest, "invalid skill name")
			return
		}
		mdPath := filepath.Join(dataDir, "skills", filepath.FromSlash(name), "SKILL.md")
		data, err := os.ReadFile(mdPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeError(w, http.StatusNotFound, "skill not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to read skill")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	}
}
