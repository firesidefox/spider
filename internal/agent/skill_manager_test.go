package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSkillFrontmatter_Valid(t *testing.T) {
	content := "---\ndescription: Use when deploying the app.\n---\n\n# Body"
	meta, body, err := ParseSkillFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Description != "Use when deploying the app." {
		t.Errorf("got description %q", meta.Description)
	}
	if body != "\n# Body" {
		t.Errorf("got body %q", body)
	}
}

func TestParseSkillFrontmatter_MissingDescription(t *testing.T) {
	content := "---\n---\n\n# Body"
	_, _, err := ParseSkillFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestParseSkillFrontmatter_DescriptionTooLong(t *testing.T) {
	desc := ""
	for i := 0; i < 251; i++ {
		desc += "a"
	}
	content := "---\ndescription: " + desc + "\n---\n\n# Body"
	_, _, err := ParseSkillFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for description > 250 chars")
	}
}

func TestParseSkillFrontmatter_DescriptionTooLong_Unicode(t *testing.T) {
	// 251 Chinese characters = 251 runes but 753 bytes
	desc := strings.Repeat("中", 251)
	content := "---\ndescription: " + desc + "\n---\n\n# Body"
	_, _, err := ParseSkillFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for description > 250 runes")
	}
}

func TestParseSkillFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# Just a body"
	_, _, err := ParseSkillFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestSkillManager_LoadSkills(t *testing.T) {
	dataDir := t.TempDir()
	skillsDir := filepath.Join(dataDir, "skills")
	writeSkillFile(t, skillsDir, "deploy", "---\ndescription: Use when deploying.\n---\n# Deploy")
	writeSkillFile(t, skillsDir, "backup", "---\ndescription: Use when backing up.\n---\n# Backup")

	sm := NewSkillManager(dataDir)
	skills, err := sm.LoadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
	// sorted by name
	if skills[0].Name != "backup" {
		t.Errorf("expected first skill to be 'backup', got %q", skills[0].Name)
	}
	if skills[0].Status != "ok" {
		t.Errorf("expected status 'ok', got %q", skills[0].Status)
	}
}

func TestSkillEntry_Body(t *testing.T) {
	dataDir := t.TempDir()
	skillsDir := filepath.Join(dataDir, "skills")
	writeSkillFile(t, skillsDir, "deploy", "---\ndescription: Use when deploying.\n---\n\n# Deploy Steps")

	sm := NewSkillManager(dataDir)
	skills, _ := sm.LoadSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill")
	}
	body, err := skills[0].Body()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "# Deploy Steps") {
		t.Errorf("body missing expected content: %q", body)
	}
}

func TestSkillManager_LoadSkills_ErrorEntry(t *testing.T) {
	dataDir := t.TempDir()
	skillsDir := filepath.Join(dataDir, "skills")
	writeSkillFile(t, skillsDir, "broken", "not valid frontmatter")

	sm := NewSkillManager(dataDir)
	skills, err := sm.LoadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Status != "error" {
		t.Errorf("expected status 'error', got %q", skills[0].Status)
	}
	if skills[0].Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestSkillManager_LoadSkills_Empty(t *testing.T) {
	dataDir := t.TempDir()
	sm := NewSkillManager(dataDir)
	skills, err := sm.LoadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestSkillManager_ComputeHash_Empty(t *testing.T) {
	dataDir := t.TempDir()
	sm := NewSkillManager(dataDir)
	hash, err := sm.ComputeHash()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both subdirs don't exist → empty hash
	if hash != "" {
		t.Errorf("expected empty hash when no skill dirs exist, got %q", hash)
	}
}

func TestSkillManager_ComputeHash_NonExistentDir(t *testing.T) {
	sm := NewSkillManager("/nonexistent/path/xyz")
	hash, err := sm.ComputeHash()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash != "" {
		t.Errorf("expected empty hash for non-existent dataDir, got %q", hash)
	}
}

func TestSkillManager_ComputeHash_ChangesOnModify(t *testing.T) {
	dataDir := t.TempDir()
	skillsDir := filepath.Join(dataDir, "skills")
	writeSkillFile(t, skillsDir, "deploy", "---\ndescription: Use when deploying.\n---\n# Deploy")
	sm := NewSkillManager(dataDir)
	h1, _ := sm.ComputeHash()
	time.Sleep(10 * time.Millisecond)
	path := filepath.Join(skillsDir, "deploy", "SKILL.md")
	os.Chtimes(path, time.Now(), time.Now())
	h2, _ := sm.ComputeHash()
	if h1 == h2 {
		t.Error("expected hash to change after file modification")
	}
}

func TestSkillManager_RenderList_Normal(t *testing.T) {
	entries := []SkillEntry{
		{Name: "deploy", Description: "Use when deploying.", Status: "ok"},
		{Name: "backup", Description: "Use when backing up.", Status: "ok"},
	}
	sm := NewSkillManager("")
	list := sm.RenderList(entries)
	if !strings.Contains(list, "- deploy: Use when deploying.") {
		t.Errorf("missing deploy entry in list: %q", list)
	}
	if !strings.Contains(list, "- backup: Use when backing up.") {
		t.Errorf("missing backup entry in list: %q", list)
	}
}

func TestSkillManager_RenderList_SkipsErrorEntries(t *testing.T) {
	entries := []SkillEntry{
		{Name: "deploy", Description: "Use when deploying.", Status: "ok"},
		{Name: "broken", Status: "error", Error: "bad yaml"},
	}
	sm := NewSkillManager("")
	list := sm.RenderList(entries)
	if strings.Contains(list, "broken") {
		t.Error("error entries should not appear in rendered list")
	}
}

func TestSkillManager_RenderList_Empty(t *testing.T) {
	sm := NewSkillManager("")
	list := sm.RenderList(nil)
	if list != "" {
		t.Errorf("expected empty string for nil entries, got %q", list)
	}
}

func TestSkillManager_RenderList_BudgetDegradation(t *testing.T) {
	entries := make([]SkillEntry, 100)
	for i := range entries {
		entries[i] = SkillEntry{
			Name:        fmt.Sprintf("skill%03d", i),
			Description: strings.Repeat("x", 250),
			Status:      "ok",
		}
	}
	sm := NewSkillManager("")
	list := sm.RenderList(entries)
	if len(list) > 8192 {
		t.Errorf("rendered list %d bytes exceeds budget 8192", len(list))
	}
	if list == "" {
		t.Error("expected non-empty list even with budget pressure")
	}
}

func TestSkillManager_LoadSkills_DualDirectory(t *testing.T) {
	dataDir := t.TempDir()
	builtinDir := filepath.Join(dataDir, "skills_builtin")
	customDir := filepath.Join(dataDir, "skills")

	// Write builtin skills
	writeSkillFile(t, builtinDir, "deploy", "---\ndescription: Builtin deploy skill.\n---\n# Builtin Deploy")
	writeSkillFile(t, builtinDir, "backup", "---\ndescription: Builtin backup skill.\n---\n# Builtin Backup")

	// Write custom skills (including one that shadows builtin)
	writeSkillFile(t, customDir, "deploy", "---\ndescription: Custom deploy skill.\n---\n# Custom Deploy")
	writeSkillFile(t, customDir, "monitor", "---\ndescription: Custom monitor skill.\n---\n# Custom Monitor")

	sm := NewSkillManager(dataDir)
	skills, err := sm.LoadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 4 entries: backup(builtin), deploy(builtin), deploy(custom), monitor(custom)
	if len(skills) != 4 {
		t.Fatalf("expected 4 skills, got %d", len(skills))
	}

	// Verify sorting: name ascending, same name → custom before builtin
	expected := []struct{ name, source string }{
		{"backup", "builtin"},
		{"deploy", "custom"},
		{"deploy", "builtin"},
		{"monitor", "custom"},
	}
	for i, exp := range expected {
		if skills[i].Name != exp.name {
			t.Errorf("skills[%d].Name = %q, want %q", i, skills[i].Name, exp.name)
		}
		if skills[i].Source != exp.source {
			t.Errorf("skills[%d].Source = %q, want %q", i, skills[i].Source, exp.source)
		}
	}
}

func TestSkillManager_RenderList_ShadowsBuiltin(t *testing.T) {
	entries := []SkillEntry{
		{Name: "backup", Description: "Builtin backup.", Status: "ok", Source: "builtin"},
		{Name: "deploy", Description: "Custom deploy.", Status: "ok", Source: "custom"},
		{Name: "deploy", Description: "Builtin deploy.", Status: "ok", Source: "builtin"},
		{Name: "monitor", Description: "Custom monitor.", Status: "ok", Source: "custom"},
	}
	sm := NewSkillManager("")
	list := sm.RenderList(entries)

	// Should include custom deploy, not builtin deploy
	if !strings.Contains(list, "- deploy: Custom deploy.") {
		t.Error("missing custom deploy in rendered list")
	}
	if strings.Contains(list, "Builtin deploy") {
		t.Error("builtin deploy should be shadowed by custom")
	}

	// Should include backup and monitor
	if !strings.Contains(list, "- backup: Builtin backup.") {
		t.Error("missing builtin backup in rendered list")
	}
	if !strings.Contains(list, "- monitor: Custom monitor.") {
		t.Error("missing custom monitor in rendered list")
	}
}

func TestParseSkillFrontmatter_DescriptionWithColon(t *testing.T) {
	// description 含冒号但未加引号，当前会触发 YAML parse error
	content := "---\ndescription: Use when X. Triggers: foo、bar。\n---\n\n# Body"
	meta, body, err := ParseSkillFrontmatter(content)
	if err != nil {
		t.Fatalf("description with colon should not error: %v", err)
	}
	if meta.Description != "Use when X. Triggers: foo、bar。" {
		t.Errorf("got description %q", meta.Description)
	}
	if body != "\n# Body" {
		t.Errorf("got body %q", body)
	}
}

func TestParseSkillFrontmatter_DescriptionWithChineseColon(t *testing.T) {
	content := "---\ndescription: 用于对比配置。触发词：配置对比、漂移。\n---\n\n# Body"
	meta, _, err := ParseSkillFrontmatter(content)
	if err != nil {
		t.Fatalf("description with Chinese colon should not error: %v", err)
	}
	if meta.Description != "用于对比配置。触发词：配置对比、漂移。" {
		t.Errorf("got description %q", meta.Description)
	}
}

func writeSkillFile(t *testing.T, base, name, content string) {
	t.Helper()
	dir := filepath.Join(base, name)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644)
}
