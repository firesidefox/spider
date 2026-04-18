package api

import (
	"io"
	"io/fs"
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
		base := filepath.Join(dataDir, "skills")
		var skills []skillInfo
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return filepath.SkipAll
				}
				return err
			}
			if d.IsDir() || d.Name() != "SKILL.md" {
				return nil
			}
			dir := filepath.Dir(path)
			rel, _ := filepath.Rel(base, dir)
			skills = append(skills, skillInfo{Name: rel, Source: "custom"})
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			writeError(w, http.StatusInternalServerError, "failed to read skills dir")
			return
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
