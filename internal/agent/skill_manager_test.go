package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkillFrontmatter_Valid(t *testing.T) {
	content := "---\ndescription: Use when deploying the app.\n---\n\n# Body"
	meta, body, err := parseSkillFrontmatter(content)
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
	_, _, err := parseSkillFrontmatter(content)
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
	_, _, err := parseSkillFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for description > 250 chars")
	}
}

func TestParseSkillFrontmatter_DescriptionTooLong_Unicode(t *testing.T) {
	// 251 Chinese characters = 251 runes but 753 bytes
	desc := strings.Repeat("中", 251)
	content := "---\ndescription: " + desc + "\n---\n\n# Body"
	_, _, err := parseSkillFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for description > 250 runes")
	}
}

func TestParseSkillFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# Just a body"
	_, _, err := parseSkillFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestSkillManager_LoadSkills(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "deploy", "---\ndescription: Use when deploying.\n---\n# Deploy")
	writeSkillFile(t, dir, "backup", "---\ndescription: Use when backing up.\n---\n# Backup")

	sm := NewSkillManager(dir)
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
	dir := t.TempDir()
	writeSkillFile(t, dir, "deploy", "---\ndescription: Use when deploying.\n---\n\n# Deploy Steps")

	sm := NewSkillManager(dir)
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
	dir := t.TempDir()
	writeSkillFile(t, dir, "broken", "not valid frontmatter")

	sm := NewSkillManager(dir)
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
	dir := t.TempDir()
	sm := NewSkillManager(dir)
	skills, err := sm.LoadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func writeSkillFile(t *testing.T, base, name, content string) {
	t.Helper()
	dir := filepath.Join(base, name)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644)
}
