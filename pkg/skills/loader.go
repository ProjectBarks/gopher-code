package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Skill represents a loaded skill (prompt-based command).
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Source      string `json:"source"` // "bundled", "user", "project"
	FilePath    string `json:"file_path,omitempty"`
}

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
		// Parse frontmatter if present (---\nkey: value\n---\n)
		content := string(data)
		desc := fmt.Sprintf("Skill: %s", name)

		if strings.HasPrefix(content, "---\n") {
			parts := strings.SplitN(content[4:], "---\n", 2)
			if len(parts) == 2 {
				// Parse simple key: value frontmatter
				for _, line := range strings.Split(parts[0], "\n") {
					if strings.HasPrefix(line, "description:") {
						desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
					}
				}
				content = parts[1]
			}
		}

		skills = append(skills, Skill{
			Name:        name,
			Description: desc,
			Prompt:      strings.TrimSpace(content),
			Source:      source,
			FilePath:    filepath.Join(dir, e.Name()),
		})
	}
	return skills
}
