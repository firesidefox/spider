package agent

import (
	"io/fs"
	"os"
	"path/filepath"
)

func SyncBuiltinSkills(dataDir string, fsys fs.FS) error {
	sub, err := fs.Sub(fsys, "skills")
	if err != nil {
		return err
	}
	destBase := filepath.Join(dataDir, "skills_builtin")
	return fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		dest := filepath.Join(destBase, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		data, err := fs.ReadFile(sub, path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
}
