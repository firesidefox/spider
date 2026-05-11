package agent

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func SyncBuiltinSkills(dataDir string, fsys fs.FS) error {
	destBase := filepath.Join(dataDir, "skills_builtin")
	return fs.WalkDir(fsys, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, "skills/")
		dest := filepath.Join(destBase, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
}
