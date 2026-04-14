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
SKILLS_DIR="$HOME/.claude/plugins/spider"
SETTINGS="$HOME/.claude/settings.json"

set -e

echo "Installing Spider Skills..."
mkdir -p "$SKILLS_DIR"
curl -fsSL "$SPIDER_URL/api/v1/install/skills.tar.gz" | tar -xz -C "$SKILLS_DIR"

echo "Configuring MCP Server..."
if command -v node >/dev/null 2>&1; then
  node -e "
    const fs=require('fs'),p='$SETTINGS';
    const c=fs.existsSync(p)?JSON.parse(fs.readFileSync(p,'utf8')):{};
    c.mcpServers=Object.assign({},c.mcpServers,{spider:{type:'http',url:'$SPIDER_URL/mcp'}});
    fs.writeFileSync(p,JSON.stringify(c,null,2));
  "
elif command -v python3 >/dev/null 2>&1; then
  python3 -c "
import json,os
p='$SETTINGS'
c=json.load(open(p)) if os.path.exists(p) else {}
c.setdefault('mcpServers',{})['spider']={'type':'http','url':'$SPIDER_URL/mcp'}
json.dump(c,open(p,'w'),indent=2)
  "
else
  echo 'Error: node or python3 is required' >&2; exit 1
fi

echo "Done. Restart Claude Code to activate spider MCP."
`

var installTmpl = template.Must(template.New("install").Parse(installScriptTmpl))

// InstallScriptHandler returns a handler that serves a dynamic shell install script.
func InstallScriptHandler(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		data := struct{ BaseURL string }{BaseURL: baseURL}
		if err := installTmpl.Execute(w, data); err != nil {
			http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		}
	}
}

// SkillsTarGzHandler streams all skills from <dataDir>/skills/ as a tar.gz.
func SkillsTarGzHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		diskDir := filepath.Join(dataDir, "skills")
		files := map[string][]byte{}

		_ = filepath.WalkDir(diskDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(diskDir, path)
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				files[rel] = data
			}
			return nil
		})

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
