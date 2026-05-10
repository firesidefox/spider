package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".spider", "data")
	if cfg.DataDir != want {
		t.Errorf("默认 DataDir = %q，期望 %q", cfg.DataDir, want)
	}
}

func TestLoad_NoFile(t *testing.T) {
	// 文件不存在时静默返回默认配置
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("文件不存在时不应返回错误: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".spider", "data")
	if cfg.DataDir != want {
		t.Errorf("DataDir = %q，期望 %q", cfg.DataDir, want)
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
	// path="" 时尝试读取 DataDir/config.yaml，不存在则静默使用默认值
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") 不应返回错误: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".spider", "data")
	if cfg.DataDir != want {
		t.Errorf("DataDir = %q，期望 %q", cfg.DataDir, want)
	}
}

func TestLoadConfigWithRules(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
data_dir: /tmp/test
agent:
  permission_mode: auto
  approval_timeout: 120
  rules:
    - pattern: "^docker\\s+rm"
      level: L3
      description: "docker remove"
    - pattern: "^ansible"
      level: L2
`
	os.WriteFile(cfgPath, []byte(content), 0644)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Agent.Rules) != 2 {
		t.Fatalf("want 2 rules, got %d", len(cfg.Agent.Rules))
	}
	if cfg.Agent.Rules[0].Pattern != `^docker\s+rm` {
		t.Errorf("rule[0].Pattern = %q", cfg.Agent.Rules[0].Pattern)
	}
	if cfg.Agent.Rules[0].Level != "L3" {
		t.Errorf("rule[0].Level = %q", cfg.Agent.Rules[0].Level)
	}
	if cfg.Agent.Rules[0].Description != "docker remove" {
		t.Errorf("rule[0].Description = %q", cfg.Agent.Rules[0].Description)
	}
	if cfg.Agent.Rules[1].Pattern != "^ansible" {
		t.Errorf("rule[1].Pattern = %q", cfg.Agent.Rules[1].Pattern)
	}
	if cfg.Agent.Rules[1].Level != "L2" {
		t.Errorf("rule[1].Level = %q", cfg.Agent.Rules[1].Level)
	}
}

func TestLoad_NoSPIDER_DATA_DIR(t *testing.T) {
	// 确保环境变量不影响 DataDir
	t.Setenv("SPIDER_DATA_DIR", "/should/be/ignored")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".spider", "data")
	if cfg.DataDir != want {
		t.Errorf("SPIDER_DATA_DIR 不应影响 DataDir，got %q", cfg.DataDir)
	}
}
