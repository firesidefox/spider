package config

import "testing"

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
