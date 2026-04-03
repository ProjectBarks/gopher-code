package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Source: skills/loadSkillsDir.ts, utils/frontmatterParser.ts

// Skill represents a loaded skill (prompt-based command).
// Source: types/command.ts — PromptCommand
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"` // full markdown content after frontmatter
	Source      string `json:"source"` // "bundled", "user", "project", "managed", "plugin"
	FilePath    string `json:"file_path,omitempty"`

	// Extended frontmatter fields — Source: skills/loadSkillsDir.ts:185-265
	DisplayName            string   `json:"displayName,omitempty"`
	AllowedTools           []string `json:"allowedTools,omitempty"`     // Source: frontmatter "allowed-tools"
	ArgumentHint           string   `json:"argumentHint,omitempty"`     // Source: frontmatter "argument-hint"
	ArgumentNames          []string `json:"argumentNames,omitempty"`    // Source: frontmatter "arguments"
	WhenToUse              string   `json:"whenToUse,omitempty"`        // Source: frontmatter "when_to_use"
	Version                string   `json:"version,omitempty"`          // Source: frontmatter "version"
	Model                  string   `json:"model,omitempty"`            // Source: frontmatter "model"
	DisableModelInvocation bool     `json:"disableModelInvocation,omitempty"` // Source: frontmatter "disable-model-invocation"
	UserInvocable          bool     `json:"userInvocable"`              // Source: frontmatter "user-invocable", default true
	Context                string   `json:"context,omitempty"`          // "inline" or "fork"
	Agent                  string   `json:"agent,omitempty"`            // agent type to delegate to
	Effort                 string   `json:"effort,omitempty"`           // effort level
	Paths                  []string `json:"paths,omitempty"`            // glob patterns for activation
	Shell                  string   `json:"shell,omitempty"`            // "bash" or "powershell"
	BaseDir                string   `json:"baseDir,omitempty"`          // root directory for the skill
	IsHidden               bool     `json:"isHidden,omitempty"`         // !userInvocable
}

// LoadedFrom tracks where a skill was loaded from.
// Source: skills/loadSkillsDir.ts:67-74
type LoadedFrom string

const (
	LoadedFromSkills     LoadedFrom = "skills"
	LoadedFromPlugin     LoadedFrom = "plugin"
	LoadedFromManaged    LoadedFrom = "managed"
	LoadedFromBundled    LoadedFrom = "bundled"
	LoadedFromMCP        LoadedFrom = "mcp"
)

// LoadSkills discovers and loads skills from standard locations.
func LoadSkills(cwd string) []Skill {
	var skills []Skill

	// 1. User skills: ~/.claude/skills/
	if home, err := os.UserHomeDir(); err == nil {
		userDir := filepath.Join(home, ".claude", "skills")
		skills = append(skills, loadFromDir(userDir, "user")...)
	}

	// 2. Project skills: .claude/skills/ in CWD
	if cwd != "" {
		projectDir := filepath.Join(cwd, ".claude", "skills")
		skills = append(skills, loadFromDir(projectDir, "project")...)
	}

	return skills
}

func loadFromDir(dir, source string) []Skill {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var skills []Skill
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".md") && !strings.HasSuffix(e.Name(), ".txt")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		content := string(data)

		skill := ParseSkillFromMarkdown(name, content, source, filepath.Join(dir, e.Name()), dir)
		skills = append(skills, skill)
	}
	return skills
}

// ParseSkillFromMarkdown parses a skill from markdown content with frontmatter.
// Source: skills/loadSkillsDir.ts:185-265
func ParseSkillFromMarkdown(name, content, source, filePath, baseDir string) Skill {
	desc := fmt.Sprintf("Skill: %s", name)
	prompt := content
	skill := Skill{
		Name:          name,
		Source:        source,
		FilePath:      filePath,
		BaseDir:       baseDir,
		UserInvocable: true, // Source: loadSkillsDir.ts:216-219 — default true
	}

	if strings.HasPrefix(content, "---\n") {
		parts := strings.SplitN(content[4:], "---\n", 2)
		if len(parts) == 2 {
			fm := parseFrontmatterFields(parts[0])
			prompt = parts[1]

			// description — Source: loadSkillsDir.ts:208-214
			if d, ok := fm["description"]; ok && d != "" {
				desc = d
			}

			// displayName — Source: loadSkillsDir.ts:238-239
			if d, ok := fm["name"]; ok && d != "" {
				skill.DisplayName = d
			}

			// allowed-tools — Source: loadSkillsDir.ts:242-244
			if tools, ok := fm["allowed-tools"]; ok && tools != "" {
				skill.AllowedTools = parseCSVList(tools)
			}

			// argument-hint — Source: loadSkillsDir.ts:245-248
			if ah, ok := fm["argument-hint"]; ok && ah != "" {
				skill.ArgumentHint = ah
			}

			// arguments — Source: loadSkillsDir.ts:249-251
			if args, ok := fm["arguments"]; ok && args != "" {
				skill.ArgumentNames = parseCSVList(args)
			}

			// when_to_use — Source: loadSkillsDir.ts:252
			if w, ok := fm["when_to_use"]; ok && w != "" {
				skill.WhenToUse = w
			}

			// version — Source: loadSkillsDir.ts:253
			if v, ok := fm["version"]; ok && v != "" {
				skill.Version = v
			}

			// model — Source: loadSkillsDir.ts:221-226
			if m, ok := fm["model"]; ok && m != "" {
				if strings.EqualFold(m, "inherit") {
					// "inherit" means no override — Source: loadSkillsDir.ts:222-223
					skill.Model = ""
				} else {
					skill.Model = m
				}
			}

			// disable-model-invocation — Source: loadSkillsDir.ts:255-257
			if d, ok := fm["disable-model-invocation"]; ok {
				skill.DisableModelInvocation = parseBool(d)
			}

			// user-invocable — Source: loadSkillsDir.ts:216-219
			if u, ok := fm["user-invocable"]; ok {
				skill.UserInvocable = parseBool(u)
			}

			// context — Source: loadSkillsDir.ts:260
			if c, ok := fm["context"]; ok && c == "fork" {
				skill.Context = "fork"
			}

			// agent — Source: loadSkillsDir.ts:261
			if a, ok := fm["agent"]; ok && a != "" {
				skill.Agent = a
			}

			// effort — Source: loadSkillsDir.ts:228-235
			if e, ok := fm["effort"]; ok && e != "" {
				skill.Effort = e
			}

			// paths — Source: loadSkillsDir.ts:159-178
			if p, ok := fm["paths"]; ok && p != "" {
				parsed := parseSkillPaths(p)
				if len(parsed) > 0 {
					skill.Paths = parsed
				}
			}

			// shell — Source: loadSkillsDir.ts:263
			if s, ok := fm["shell"]; ok && (s == "bash" || s == "powershell") {
				skill.Shell = s
			}
		}
	}

	skill.Description = desc
	skill.Prompt = strings.TrimSpace(prompt)
	skill.IsHidden = !skill.UserInvocable

	return skill
}

// parseFrontmatterFields parses simple key: value frontmatter lines.
func parseFrontmatterFields(fmBlock string) map[string]string {
	fm := make(map[string]string)
	for _, line := range strings.Split(fmBlock, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Strip quotes
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		// Handle array syntax [a, b, c] → keep as comma-separated string
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			inner := value[1 : len(value)-1]
			// Strip quotes from each element
			parts := strings.Split(inner, ",")
			var cleaned []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if (strings.HasPrefix(p, "\"") && strings.HasSuffix(p, "\"")) ||
					(strings.HasPrefix(p, "'") && strings.HasSuffix(p, "'")) {
					p = p[1 : len(p)-1]
				}
				if p != "" {
					cleaned = append(cleaned, p)
				}
			}
			value = strings.Join(cleaned, ", ")
		}

		fm[key] = value
	}
	return fm
}

// parseCSVList splits a comma-separated string into a trimmed slice.
func parseCSVList(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseBool parses a boolean from a frontmatter string.
func parseBool(s string) bool {
	return strings.EqualFold(s, "true") || s == "1"
}

// parseSkillPaths parses paths frontmatter, removing /** suffixes.
// Source: skills/loadSkillsDir.ts:159-178
func parseSkillPaths(value string) []string {
	patterns := parseCSVList(value)
	var result []string
	for _, p := range patterns {
		// Remove /** suffix — Source: loadSkillsDir.ts:166-168
		if strings.HasSuffix(p, "/**") {
			p = p[:len(p)-3]
		}
		if p != "" && p != "**" {
			result = append(result, p)
		}
	}
	return result
}
