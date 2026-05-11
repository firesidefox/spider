package agent

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestSyncBuiltinSkills_WritesFiles(t *testing.T) {
	mockFS := fstest.MapFS{
		"skills/cron/SKILL.md":    {Data: []byte("---\ndescription: Cron skill.\n---\n# Cron")},
		"skills/monitor/SKILL.md": {Data: []byte("---\ndescription: Monitor skill.\n---\n# Monitor")},
	}
	dataDir := t.TempDir()
	if err := SyncBuiltinSkills(dataDir, mockFS); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{"cron", "monitor"} {
		p := filepath.Join(dataDir, "skills_builtin", name, "SKILL.md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to exist: %v", p, err)
		}
	}
}

func TestSyncBuiltinSkills_OverwritesExisting(t *testing.T) {
	mockFS := fstest.MapFS{
		"skills/cron/SKILL.md": {Data: []byte("---\ndescription: New cron.\n---\n# New")},
	}
	dataDir := t.TempDir()
	dir := filepath.Join(dataDir, "skills_builtin", "cron")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("old content"), 0o644)

	if err := SyncBuiltinSkills(dataDir, mockFS); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if string(data) != "---\ndescription: New cron.\n---\n# New" {
		t.Errorf("expected overwrite, got: %s", data)
	}
}
