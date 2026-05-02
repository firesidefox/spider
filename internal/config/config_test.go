package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLLMConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
data_dir: /tmp/test
llm:
  active: claude-sonnet
  models:
    - id: claude-sonnet
      provider: claude
      api_key: sk-ant-test
      model: claude-sonnet-4-6
      max_tokens: 4096
    - id: gpt4o
      provider: openai
      api_key: sk-test
      model: gpt-4o
      max_tokens: 4096
embedding:
  active: openai-small
  models:
    - id: openai-small
      provider: openai
      api_key: sk-test
      model: text-embedding-3-small
      dimensions: 1536
`
	os.WriteFile(cfgPath, []byte(content), 0600)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.LLM.Active != "claude-sonnet" {
		t.Errorf("LLM.Active = %q, want %q", cfg.LLM.Active, "claude-sonnet")
	}
	if len(cfg.LLM.Models) != 2 {
		t.Fatalf("LLM.Models len = %d, want 2", len(cfg.LLM.Models))
	}
	if cfg.LLM.Models[0].Provider != "claude" {
		t.Errorf("Models[0].Provider = %q, want %q", cfg.LLM.Models[0].Provider, "claude")
	}
	if cfg.Embedding.Active != "openai-small" {
		t.Errorf("Embedding.Active = %q, want %q", cfg.Embedding.Active, "openai-small")
	}
	if cfg.Embedding.Models[0].Dimensions != 1536 {
		t.Errorf("Dimensions = %d, want 1536", cfg.Embedding.Models[0].Dimensions)
	}
}

func TestActiveModel(t *testing.T) {
	cfg := &LLMConfig{
		Active: "gpt4o",
		Models: []LLMModelConfig{
			{ID: "claude-sonnet", Provider: "claude"},
			{ID: "gpt4o", Provider: "openai"},
		},
	}
	m := cfg.ActiveModel()
	if m == nil || m.ID != "gpt4o" {
		t.Errorf("ActiveModel = %v, want gpt4o", m)
	}
	cfg.Active = "nonexistent"
	if cfg.ActiveModel() != nil {
		t.Error("ActiveModel should return nil for nonexistent")
	}
}

func TestResolveAPIKey(t *testing.T) {
	m := &LLMModelConfig{ID: "test", APIKey: "from-config"}
	if m.ResolveAPIKey() != "from-config" {
		t.Errorf("ResolveAPIKey = %q, want from-config", m.ResolveAPIKey())
	}
	t.Setenv("SPIDER_LLM_APIKEY_test", "from-env")
	if m.ResolveAPIKey() != "from-env" {
		t.Errorf("ResolveAPIKey = %q, want from-env", m.ResolveAPIKey())
	}
}

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
