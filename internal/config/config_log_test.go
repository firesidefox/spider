package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultLogConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Log.Level != "info" {
		t.Errorf("want level=info, got %s", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("want format=json, got %s", cfg.Log.Format)
	}
	if cfg.Log.MaxSizeMB != 100 {
		t.Errorf("want max_size_mb=100, got %d", cfg.Log.MaxSizeMB)
	}
	if cfg.Log.MaxBackups != 7 {
		t.Errorf("want max_backups=7, got %d", cfg.Log.MaxBackups)
	}
}

func TestLogConfigYAMLParsing(t *testing.T) {
	yaml := `
log:
  level: debug
  format: text
  file: /tmp/test.log
  max_size_mb: 50
  max_backups: 3
  stderr: true
`
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("want level=debug, got %s", cfg.Log.Level)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("want format=text, got %s", cfg.Log.Format)
	}
	if cfg.Log.File != "/tmp/test.log" {
		t.Errorf("want file=/tmp/test.log, got %s", cfg.Log.File)
	}
	if cfg.Log.MaxSizeMB != 50 {
		t.Errorf("want max_size_mb=50, got %d", cfg.Log.MaxSizeMB)
	}
	if cfg.Log.MaxBackups != 3 {
		t.Errorf("want max_backups=3, got %d", cfg.Log.MaxBackups)
	}
	if !cfg.Log.Stderr {
		t.Error("want stderr=true")
	}
}
