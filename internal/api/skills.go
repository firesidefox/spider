package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type skillInfo struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

func isValidSkillName(name string) bool {
	if name == "" {
		return false
	}
	return !strings.ContainsAny(name, `/\.`)
}

func listSkillsHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := filepath.Join(dataDir, "skills")
		entries, err := os.ReadDir(base)
		if err != nil {
			if os.IsNotExist(err) {
				writeJSON(w, http.StatusOK, []skillInfo{})
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to read skills dir")
			return
		}
		var skills []skillInfo
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			mdPath := filepath.Join(base, e.Name(), "SKILL.md")
			if _, err := os.Stat(mdPath); err == nil {
				skills = append(skills, skillInfo{Name: e.Name(), Source: "custom"})
			}
		}
		sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
		if skills == nil {
			skills = []skillInfo{}
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
		dir := filepath.Join(dataDir, "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create skill dir")
			return
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), body, 0o644); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to write SKILL.md")
			return
		}
		writeJSON(w, http.StatusOK, skillInfo{Name: name, Source: "custom"})
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
