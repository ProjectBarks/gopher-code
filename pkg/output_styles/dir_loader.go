// Package output_styles loads user and project output-style configs from
// .claude/output-styles/*.md files.
//
// Source: src/outputStyles/loadOutputStylesDir.ts
// Source: src/utils/markdownConfigLoader.ts (extractDescriptionFromMarkdown, loadMarkdownFilesForSubdir)
// Source: src/utils/frontmatterParser.ts (coerceDescriptionToString)
package output_styles

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/projectbarks/gopher-code/pkg/config"
)

// OutputStyleConfig describes a single output style loaded from disk or built-in.
// Source: constants/outputStyles.ts — OutputStyleConfig
type OutputStyleConfig struct {
	Name                  string              `json:"name"`
	Description           string              `json:"description"`
	Prompt                string              `json:"prompt"`
	Source                config.SettingSource `json:"source"`
	KeepCodingInstructions *bool              `json:"keepCodingInstructions,omitempty"`
	ForceForPlugin        bool                `json:"forceForPlugin,omitempty"`
}

// MarkdownFile is a parsed .md file with frontmatter and content.
type MarkdownFile struct {
	FilePath    string
	Frontmatter map[string]any
	Content     string
	Source      config.SettingSource
}

// frontmatterRe matches YAML frontmatter delimited by --- at the start of a file.
var frontmatterRe = regexp.MustCompile(`(?s)\A---\s*\n(.*?)---\s*\n?`)

// headerRe matches markdown header lines (# Heading).
var headerRe = regexp.MustCompile(`^#+\s+(.+)$`)

// ── Memoized loader ────────────────────────────────────────────────

var (
	cacheMu sync.Mutex
	cache   = map[string][]OutputStyleConfig{}
)

// GetOutputStyleDirStyles loads all output styles from .claude/output-styles/
// in both the user home and project directories (project overrides user).
// Results are memoized per cwd. Source: loadOutputStylesDir.ts — getOutputStyleDirStyles
func GetOutputStyleDirStyles(cwd string) []OutputStyleConfig {
	cacheMu.Lock()
	if cached, ok := cache[cwd]; ok {
		cacheMu.Unlock()
		return cached
	}
	cacheMu.Unlock()

	styles := loadOutputStyleDirStyles(cwd)

	cacheMu.Lock()
	cache[cwd] = styles
	cacheMu.Unlock()
	return styles
}

// ClearOutputStyleCaches clears the dir-style cache.
// Source: loadOutputStylesDir.ts — clearOutputStyleCaches
func ClearOutputStyleCaches() {
	cacheMu.Lock()
	cache = map[string][]OutputStyleConfig{}
	cacheMu.Unlock()
}

// ── Core loader ────────────────────────────────────────────────────

func loadOutputStyleDirStyles(cwd string) []OutputStyleConfig {
	mdFiles, err := LoadMarkdownFilesForSubdir("output-styles", cwd)
	if err != nil {
		slog.Debug("output_styles: failed to load markdown files", "error", err)
		return nil
	}

	var styles []OutputStyleConfig
	for _, mf := range mdFiles {
		style, err := parseStyleFromMarkdown(mf)
		if err != nil {
			slog.Debug("output_styles: skipping file", "path", mf.FilePath, "error", err)
			continue
		}
		styles = append(styles, style)
	}
	return styles
}

func parseStyleFromMarkdown(mf MarkdownFile) (OutputStyleConfig, error) {
	fileName := filepath.Base(mf.FilePath)
	styleName := strings.TrimSuffix(fileName, ".md")

	// Name resolution: frontmatter.name OR styleName
	name := styleName
	if fmName, ok := mf.Frontmatter["name"].(string); ok && fmName != "" {
		name = fmName
	}

	// Description resolution
	description := coerceDescriptionToString(mf.Frontmatter["description"], styleName)
	if description == "" {
		description = ExtractDescriptionFromMarkdown(mf.Content, fmt.Sprintf("Custom %s output style", styleName))
	}

	// Parse keep-coding-instructions (bool or string)
	keepCoding := ParseKeepCodingInstructions(mf.Frontmatter["keep-coding-instructions"])

	// Warn if force-for-plugin set on non-plugin style
	if _, hasForce := mf.Frontmatter["force-for-plugin"]; hasForce {
		slog.Warn(fmt.Sprintf("Output style %q has force-for-plugin set, but this option only applies to plugin output styles. Ignoring.", name))
	}

	return OutputStyleConfig{
		Name:                   name,
		Description:            description,
		Prompt:                 strings.TrimSpace(mf.Content),
		Source:                 mf.Source,
		KeepCodingInstructions: keepCoding,
	}, nil
}

// ── T28: LoadMarkdownFilesForSubdir ───────────────────────────────

// LoadMarkdownFilesForSubdir loads .md files from ~/.claude/{subdir} (user)
// and {cwd}/.claude/{subdir} (project). Project files override user files.
// Source: utils/markdownConfigLoader.ts — loadMarkdownFilesForSubdir (simplified)
func LoadMarkdownFilesForSubdir(subdir string, cwd string) ([]MarkdownFile, error) {
	var all []MarkdownFile

	// User styles: ~/.claude/{subdir}
	home, err := os.UserHomeDir()
	if err == nil {
		userDir := filepath.Join(home, ".claude", subdir)
		userFiles := loadMarkdownFiles(userDir, config.SourceUser)
		all = append(all, userFiles...)
	}

	// Project styles: {cwd}/.claude/{subdir}
	projectDir := filepath.Join(cwd, ".claude", subdir)
	projectFiles := loadMarkdownFiles(projectDir, config.SourceProject)
	all = append(all, projectFiles...)

	return all, nil
}

// loadMarkdownFiles reads all .md files from a directory, parsing frontmatter.
func loadMarkdownFiles(dir string, source config.SettingSource) []MarkdownFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Directory doesn't exist or can't be read — not an error
		return nil
	}

	var files []MarkdownFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Debug("output_styles: failed to read file", "path", path, "error", err)
			continue
		}

		fm, content := parseFrontmatter(string(data))
		files = append(files, MarkdownFile{
			FilePath:    path,
			Frontmatter: fm,
			Content:     content,
			Source:      source,
		})
	}
	return files
}

// parseFrontmatter extracts YAML frontmatter and body content from markdown.
// Source: utils/frontmatterParser.ts — parseFrontmatter
func parseFrontmatter(raw string) (map[string]any, string) {
	match := frontmatterRe.FindStringSubmatchIndex(raw)
	if match == nil {
		return map[string]any{}, raw
	}

	fmText := raw[match[2]:match[3]]
	content := raw[match[1]:]

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
		slog.Debug("output_styles: YAML parse error in frontmatter", "error", err)
		return map[string]any{}, content
	}
	if fm == nil {
		fm = map[string]any{}
	}
	return fm, content
}

// ── T29: ParseKeepCodingInstructions ──────────────────────────────

// ParseKeepCodingInstructions handles the bool-or-string frontmatter field.
// Returns *bool: true/false for recognized values, nil for anything else.
// Source: loadOutputStylesDir.ts — keepCodingInstructions parse logic
func ParseKeepCodingInstructions(raw any) *bool {
	switch v := raw.(type) {
	case bool:
		return &v
	case string:
		switch strings.ToLower(v) {
		case "true":
			t := true
			return &t
		case "false":
			f := false
			return &f
		}
	}
	return nil
}

// ── T30: ExtractDescriptionFromMarkdown ───────────────────────────

// ExtractDescriptionFromMarkdown returns the first non-empty line of markdown
// content as a description. Headers have their prefix stripped. Result is
// truncated to 100 chars. Falls back to defaultDesc.
// Source: utils/markdownConfigLoader.ts — extractDescriptionFromMarkdown
func ExtractDescriptionFromMarkdown(content string, defaultDesc string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Strip markdown header prefix
		if m := headerRe.FindStringSubmatch(trimmed); m != nil {
			trimmed = m[1]
		}
		if len(trimmed) > 100 {
			return trimmed[:97] + "..."
		}
		return trimmed
	}
	return defaultDesc
}

// ── helpers ────────────────────────────────────────────────────────

// coerceDescriptionToString validates and coerces a frontmatter description.
// Source: utils/frontmatterParser.ts — coerceDescriptionToString
func coerceDescriptionToString(value any, componentName string) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case int, int64, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		slog.Debug("output_styles: description invalid, omitting", "component", componentName)
		return ""
	}
}
