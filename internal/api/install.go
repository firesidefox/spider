package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
)

const installScriptTmpl = `#!/bin/sh
SPIDER_URL="{{.BaseURL}}"
SKILLS_DIR="$HOME/.claude/skills/spider"

set -e

RED='\033[31m'; GREEN='\033[32m'; YELLOW='\033[33m'
BLUE='\033[34m'; DIM='\033[2m'; RESET='\033[0m'

h1()      { printf "\n${BLUE}══ %s ══${RESET}\n" "$*"; }
step()    { printf "  ${BLUE}▶ %s...${RESET}\n" "$*"; }
success() { printf "  ${GREEN}✔ %s${RESET}\n" "$*"; }
warn()    { printf "  ${YELLOW}⚠ %s${RESET}\n" "$*"; }
error()   { printf "  ${RED}✖ %s${RESET}\n" "$*" >&2; }

h1 "Spider 安装"

step "下载 Skills"
mkdir -p "$SKILLS_DIR"
curl -fsSL "$SPIDER_URL/api/v1/install/skills.tar.gz" | tar -xz -C "$SKILLS_DIR"
success "Skills 已安装到 $SKILLS_DIR"

step "注册 MCP 服务器"
if ! command -v claude >/dev/null 2>&1; then
  error "未找到 claude CLI，请先安装 Claude Code"; exit 1
fi
claude mcp add --transport http spider "$SPIDER_URL/mcp"
success "已注册：spider → $SPIDER_URL/mcp"

h1 "安装完成 — 重启 Claude Code 即可使用 Spider"
`

var installTmpl = template.Must(template.New("install").Parse(installScriptTmpl))

func scriptHandler(tmpl *template.Template, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if err := tmpl.Execute(w, struct{ BaseURL string }{baseURL}); err != nil {
			http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		}
	}
}

// InstallScriptHandler serves the Claude Code client install script.
func InstallScriptHandler(baseURL string) http.HandlerFunc {
	return scriptHandler(installTmpl, baseURL)
}

// SkillsTarGzHandler streams all skills from <dataDir>/skills/ as a tar.gz.
func SkillsTarGzHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		diskDir := filepath.Join(dataDir, "skills")

		if _, err := os.Stat(diskDir); os.IsNotExist(err) {
			http.Error(w, "skills directory not found: "+diskDir, http.StatusNotFound)
			return
		}

		files := map[string][]byte{}
		if err := filepath.WalkDir(diskDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(diskDir, path)
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			files[rel] = data
			return nil
		}); err != nil {
			http.Error(w, "failed to read skills: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/gzip")
		gw := gzip.NewWriter(w)
		tw := tar.NewWriter(gw)
		for name, data := range files {
			hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}
			if err := tw.WriteHeader(hdr); err != nil {
				return
			}
			_, _ = io.Copy(tw, bytes.NewReader(data))
		}
		_ = tw.Close()
		_ = gw.Close()
	}
}
