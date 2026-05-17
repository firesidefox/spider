package agent

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxDescriptionChars = 250
const skillListBudgetBytes = 8192

type skillFrontmatter struct {
	Description string `yaml:"description"`
}

// SkillEntry represents a loaded skill with its parse status.
type SkillEntry struct {
	Name        string
	Description string
	Status      string // "ok" | "error"
	Error       string
	Source      string // "builtin" | "custom"
	bodyPath    string
}

// Body reads and returns the skill body (frontmatter stripped).
// Returns error if the file cannot be read or parsed.
func (e *SkillEntry) Body() (string, error) {
	data, err := os.ReadFile(e.bodyPath)
	if err != nil {
		return "", err
	}
	_, body, err := ParseSkillFrontmatter(string(data))
	if err != nil {
		return "", err
	}
	return body, nil
}

// SkillManager scans the skills directory and provides skill metadata.
type SkillManager struct {
	builtinDir string
	customDir  string
}

// NewSkillManager creates a SkillManager rooted at dataDir.
// It scans both dataDir/skills_builtin/ and dataDir/skills/.
func NewSkillManager(dataDir string) *SkillManager {
	return &SkillManager{
		builtinDir: filepath.Join(dataDir, "skills_builtin"),
		customDir:  filepath.Join(dataDir, "skills"),
	}
}

// LookupSkill resolves a skill by name. Custom shadows builtin.
func (sm *SkillManager) LookupSkill(name string) (string, error) {
	for _, dir := range []string{sm.customDir, sm.builtinDir} {
		mdPath := filepath.Join(dir, filepath.FromSlash(name), "SKILL.md")
		entry := SkillEntry{Name: name, bodyPath: mdPath}
		if body, err := entry.Body(); err == nil {
			return body, nil
		}
	}
	return "", fmt.Errorf("skill %q not found", name)
}

// SourceDir returns the directory for the given source ("builtin" or "custom").
func (sm *SkillManager) SourceDir(source string) string {
	if source == "builtin" {
		return sm.builtinDir
	}
	return sm.customDir
}

// ParseSkillFrontmatter splits YAML frontmatter from body and validates required fields.
// description values with colons or other special characters do not need manual quoting.
func ParseSkillFrontmatter(content string) (skillFrontmatter, string, error) {
	if !strings.HasPrefix(content, "---") {
		return skillFrontmatter{}, "", fmt.Errorf("missing frontmatter: file must start with ---")
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return skillFrontmatter{}, "", fmt.Errorf("malformed frontmatter: missing closing ---")
	}
	normalized := normalizeDescriptionLine(parts[1])
	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(normalized), &meta); err != nil {
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

// normalizeDescriptionLine finds the description: line in a YAML frontmatter block and
// wraps its value in double quotes if not already quoted, preventing colons in the value
// from being misinterpreted as YAML mapping syntax.
func normalizeDescriptionLine(frontmatter string) string {
	lines := strings.Split(frontmatter, "\n")
	for i, line := range lines {
		key, val, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) != "description" {
			continue
		}
		val = strings.TrimLeft(val, " ")
		if strings.HasPrefix(val, `"`) || strings.HasPrefix(val, `'`) {
			break
		}
		escaped := strings.ReplaceAll(val, `"`, `\"`)
		lines[i] = strings.TrimRight(key, " ") + `: "` + escaped + `"`
		break
	}
	return strings.Join(lines, "\n")
}

// LoadSkills scans both builtin and custom skills directories.
// Returns all entries sorted by name ascending, with custom before builtin for same name.
func (sm *SkillManager) LoadSkills() ([]SkillEntry, error) {
	var entries []SkillEntry

	// Load builtin skills
	builtinEntries, err := sm.loadFromDir(sm.builtinDir, "builtin")
	if err != nil {
		return nil, err
	}
	entries = append(entries, builtinEntries...)

	// Load custom skills
	customEntries, err := sm.loadFromDir(sm.customDir, "custom")
	if err != nil {
		return nil, err
	}
	entries = append(entries, customEntries...)

	// Sort: name ascending, same name → custom before builtin
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Source == "custom" && entries[j].Source == "builtin"
	})

	return entries, nil
}

// loadFromDir scans a single directory and returns entries with the given source.
func (sm *SkillManager) loadFromDir(dir, source string) ([]SkillEntry, error) {
	var entries []SkillEntry
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		skillDir := filepath.Dir(path)
		rel, _ := filepath.Rel(dir, skillDir)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			entries = append(entries, SkillEntry{
				Name:   rel,
				Status: "error",
				Error:  readErr.Error(),
				Source: source,
			})
			return nil
		}
		meta, _, parseErr := ParseSkillFrontmatter(string(data))
		if parseErr != nil {
			entries = append(entries, SkillEntry{
				Name:   rel,
				Status: "error",
				Error:  parseErr.Error(),
				Source: source,
			})
			return nil
		}
		entries = append(entries, SkillEntry{
			Name:        rel,
			Description: meta.Description,
			Status:      "ok",
			Source:      source,
			bodyPath:    path,
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return entries, nil
}

func (sm *SkillManager) ComputeHash() (string, error) {
	builtinExists := true
	customExists := true
	if _, err := os.Stat(sm.builtinDir); os.IsNotExist(err) {
		builtinExists = false
	}
	if _, err := os.Stat(sm.customDir); os.IsNotExist(err) {
		customExists = false
	}
	if !builtinExists && !customExists {
		return "", nil
	}

	var parts []string
	for _, dir := range []string{sm.builtinDir, sm.customDir} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return filepath.SkipAll
				}
				return err
			}
			if d.IsDir() || d.Name() != "SKILL.md" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			parts = append(parts, fmt.Sprintf("%s:%d\n", path, info.ModTime().UnixNano()))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(parts)
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (sm *SkillManager) RenderList(entries []SkillEntry) string {
	// Entries are sorted custom-before-builtin for same name.
	// Single pass: first occurrence of each name wins (custom shadows builtin).
	seen := make(map[string]bool)
	var ok []SkillEntry
	for _, e := range entries {
		if e.Status != "ok" || seen[e.Name] {
			continue
		}
		seen[e.Name] = true
		ok = append(ok, e)
	}

	if len(ok) == 0 {
		return ""
	}
	if s := renderLines(ok, func(e SkillEntry) string {
		return fmt.Sprintf("- %s: %s", e.Name, e.Description)
	}); len(s) <= skillListBudgetBytes {
		return s
	}
	if s := renderLines(ok, func(e SkillEntry) string {
		desc := e.Description
		if len([]rune(desc)) > 80 {
			desc = string([]rune(desc)[:79]) + "…"
		}
		return fmt.Sprintf("- %s: %s", e.Name, desc)
	}); len(s) <= skillListBudgetBytes {
		return s
	}
	s := renderLines(ok, func(e SkillEntry) string {
		return fmt.Sprintf("- %s", e.Name)
	})
	if len(s) > skillListBudgetBytes {
		s = s[:skillListBudgetBytes]
	}
	return s
}

func renderLines(entries []SkillEntry, format func(SkillEntry) string) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(format(e))
		sb.WriteByte('\n')
	}
	return sb.String()
}
