package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadModelConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
data_dir: /tmp/test
model:
  active_provider: my-claude
  active_model: claude-sonnet-4-6
  providers:
    - id: my-claude
      type: claude
      api_key: sk-ant-test
    - id: my-openai
      type: openai
      api_key: sk-test
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
	if cfg.Model.ActiveProvider != "my-claude" {
		t.Errorf("ActiveProvider = %q, want %q", cfg.Model.ActiveProvider, "my-claude")
	}
	if cfg.Model.ActiveModel != "claude-sonnet-4-6" {
		t.Errorf("ActiveModel = %q, want %q", cfg.Model.ActiveModel, "claude-sonnet-4-6")
	}
	if len(cfg.Model.Providers) != 2 {
		t.Fatalf("Providers len = %d, want 2", len(cfg.Model.Providers))
	}
	if cfg.Model.Providers[0].Type != "claude" {
		t.Errorf("Providers[0].Type = %q, want %q", cfg.Model.Providers[0].Type, "claude")
	}
	if cfg.Embedding.Active != "openai-small" {
		t.Errorf("Embedding.Active = %q, want %q", cfg.Embedding.Active, "openai-small")
	}
}

func TestGetActiveProvider(t *testing.T) {
	cfg := &ModelConfig{
		ActiveProvider: "my-openai",
		Providers: []ProviderConfig{
			{ID: "my-claude", Type: "claude"},
			{ID: "my-openai", Type: "openai"},
		},
	}
	p := cfg.GetActiveProvider()
	if p == nil || p.ID != "my-openai" {
		t.Errorf("GetActiveProvider = %v, want my-openai", p)
	}
	cfg.ActiveProvider = "nonexistent"
	if cfg.GetActiveProvider() != nil {
		t.Error("GetActiveProvider should return nil for nonexistent")
	}
}

func TestProviderResolveAPIKey(t *testing.T) {
	p := &ProviderConfig{ID: "test", APIKey: "from-config"}
	if p.ResolveAPIKey() != "from-config" {
		t.Errorf("ResolveAPIKey = %q, want from-config", p.ResolveAPIKey())
	}
	t.Setenv("SPIDER_PROVIDER_APIKEY_test", "from-env")
	if p.ResolveAPIKey() != "from-env" {
		t.Errorf("ResolveAPIKey = %q, want from-env", p.ResolveAPIKey())
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
