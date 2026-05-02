package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DataDir != "/var/lib/spider" {
		t.Errorf("默认 DataDir = %q，期望 /var/lib/spider", cfg.DataDir)
	}
}

func TestLoad_NoFile(t *testing.T) {
	// 文件不存在时静默返回默认配置
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("文件不存在时不应返回错误: %v", err)
	}
	if cfg.DataDir != "/var/lib/spider" {
		t.Errorf("DataDir = %q，期望 /var/lib/spider", cfg.DataDir)
	}
}

func TestLoad_FileOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := "data_dir: /tmp/custom\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if cfg.DataDir != "/tmp/custom" {
		t.Errorf("DataDir = %q，期望 /tmp/custom", cfg.DataDir)
	}
}

func TestLoad_EmptyPath_UsesDefaultDataDir(t *testing.T) {
	// path="" 时尝试读取 /var/lib/spider/config.yaml，不存在则静默使用默认值
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") 不应返回错误: %v", err)
	}
	if cfg.DataDir != "/var/lib/spider" {
		t.Errorf("DataDir = %q，期望 /var/lib/spider", cfg.DataDir)
	}
}

func TestLoad_NoSPIDER_DATA_DIR(t *testing.T) {
	// 确保环境变量不再影响 DataDir
	t.Setenv("SPIDER_DATA_DIR", "/should/be/ignored")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "/var/lib/spider" {
		t.Errorf("SPIDER_DATA_DIR 不应影响 DataDir，got %q", cfg.DataDir)
	}
}
