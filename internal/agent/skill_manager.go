package agent

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxDescriptionChars = 250

type skillFrontmatter struct {
	Description string `yaml:"description"`
}

// SkillEntry represents a loaded skill with its parse status.
type SkillEntry struct {
	Name        string
	Description string
	Status      string // "ok" | "error"
	Error       string
	bodyPath    string
}

// Body reads and returns the skill body (frontmatter stripped).
// Returns error if the file cannot be read or parsed.
func (e *SkillEntry) Body() (string, error) {
	data, err := os.ReadFile(e.bodyPath)
	if err != nil {
		return "", err
	}
	_, body, err := parseSkillFrontmatter(string(data))
	if err != nil {
		return "", err
	}
	return body, nil
}

// SkillManager scans the skills directory and provides skill metadata.
type SkillManager struct {
	dir string
}

// NewSkillManager creates a SkillManager rooted at dir.
func NewSkillManager(dir string) *SkillManager {
	return &SkillManager{dir: dir}
}

// parseSkillFrontmatter splits YAML frontmatter from body and validates required fields.
func parseSkillFrontmatter(content string) (skillFrontmatter, string, error) {
	if !strings.HasPrefix(content, "---") {
		return skillFrontmatter{}, "", fmt.Errorf("missing frontmatter: file must start with ---")
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return skillFrontmatter{}, "", fmt.Errorf("malformed frontmatter: missing closing ---")
	}
	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return skillFrontmatter{}, "", fmt.Errorf("frontmatter parse error: %w", err)
	}
	if meta.Description == "" {
		return skillFrontmatter{}, "", fmt.Errorf("description is required")
	}
	if len([]rune(meta.Description)) > maxDescriptionChars {
		return skillFrontmatter{}, "", fmt.Errorf("description exceeds %d characters (%d)", maxDescriptionChars, len(meta.Description))
	}
	body := strings.TrimPrefix(parts[2], "\n")
	return meta, body, nil
}

// LoadSkills scans the skills directory and returns all skill entries.
// Entries with parse errors have Status="error"; valid entries have Status="ok".
func (sm *SkillManager) LoadSkills() ([]SkillEntry, error) {
	var entries []SkillEntry
	err := filepath.WalkDir(sm.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		dir := filepath.Dir(path)
		rel, _ := filepath.Rel(sm.dir, dir)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			entries = append(entries, SkillEntry{Name: rel, Status: "error", Error: readErr.Error()})
			return nil
		}
		meta, _, parseErr := parseSkillFrontmatter(string(data))
		if parseErr != nil {
			entries = append(entries, SkillEntry{Name: rel, Status: "error", Error: parseErr.Error()})
			return nil
		}
		entries = append(entries, SkillEntry{
			Name: rel, Description: meta.Description,
			Status: "ok", bodyPath: path,
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}
