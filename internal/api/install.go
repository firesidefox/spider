package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed scripts/client-install.sh
var installScriptTmpl string

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
			// 目录不存在时返回空 tar.gz，不中断客户端安装
			w.Header().Set("Content-Type", "application/gzip")
			gw := gzip.NewWriter(w)
			tw := tar.NewWriter(gw)
			_ = tw.Close()
			_ = gw.Close()
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
