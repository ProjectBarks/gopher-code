package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// Source: skills/loadSkillsDir.ts, utils/frontmatterParser.ts

func TestLoadFromDir_Empty(t *testing.T) {
	dir := t.TempDir()
	skills := loadFromDir(dir, "test")
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadFromDir_NonExistent(t *testing.T) {
	skills := loadFromDir("/nonexistent/path", "test")
	if skills != nil {
		t.Fatalf("expected nil for nonexistent dir, got %v", skills)
	}
}

func TestLoadFromDir_MarkdownFiles(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "greet.md"), []byte("Say hello to the user."), 0644)
	os.WriteFile(filepath.Join(dir, "review.txt"), []byte("Review the current PR."), 0644)
	os.WriteFile(filepath.Join(dir, "notes.yaml"), []byte("key: value"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	skills := loadFromDir(dir, "project")
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	var greet *Skill
	for i := range skills {
		if skills[i].Name == "greet" {
			greet = &skills[i]
			break
		}
	}
	if greet == nil {
		t.Fatal("expected to find 'greet' skill")
	}
	if greet.Prompt != "Say hello to the user." {
		t.Errorf("unexpected prompt: %q", greet.Prompt)
	}
	if greet.Source != "project" {
		t.Errorf("expected source 'project', got %q", greet.Source)
	}
	if greet.Description != "Skill: greet" {
		t.Errorf("unexpected description: %q", greet.Description)
	}
	// Default: user-invocable should be true
	if !greet.UserInvocable {
		t.Error("default userInvocable should be true")
	}
}

func TestLoadFromDir_Frontmatter(t *testing.T) {
	dir := t.TempDir()

	content := `---
description: Deploy the application
author: test
---
Run the deployment pipeline for the current branch.`

	os.WriteFile(filepath.Join(dir, "deploy.md"), []byte(content), 0644)

	skills := loadFromDir(dir, "user")
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Name != "deploy" {
		t.Errorf("expected name 'deploy', got %q", s.Name)
	}
	if s.Description != "Deploy the application" {
		t.Errorf("expected description 'Deploy the application', got %q", s.Description)
	}
	if s.Prompt != "Run the deployment pipeline for the current branch." {
		t.Errorf("unexpected prompt: %q", s.Prompt)
	}
	if s.Source != "user" {
		t.Errorf("expected source 'user', got %q", s.Source)
	}
}

func TestParseSkillFromMarkdown_AllFrontmatterFields(t *testing.T) {
	// Source: skills/loadSkillsDir.ts:185-265 — full frontmatter parsing
	content := `---
name: code-review
description: Review code changes for quality
allowed-tools: [Read, Grep, Glob]
argument-hint: [file_path]
arguments: [file_path, focus_area]
when_to_use: When the user asks for a code review
version: 1.2.0
model: sonnet
disable-model-invocation: false
user-invocable: true
context: fork
agent: code-reviewer
effort: medium
paths: src/**, tests/**
shell: bash
---
Review the specified files for code quality issues.`

	skill := ParseSkillFromMarkdown("code-review", content, "project", "/tmp/code-review.md", "/tmp")

	t.Run("display_name", func(t *testing.T) {
		if skill.DisplayName != "code-review" {
			t.Errorf("displayName = %q", skill.DisplayName)
		}
	})

	t.Run("description", func(t *testing.T) {
		if skill.Description != "Review code changes for quality" {
			t.Errorf("description = %q", skill.Description)
		}
	})

	t.Run("allowed_tools", func(t *testing.T) {
		// Source: loadSkillsDir.ts:242-244
		if len(skill.AllowedTools) != 3 {
			t.Fatalf("expected 3 allowed tools, got %d", len(skill.AllowedTools))
		}
		if skill.AllowedTools[0] != "Read" || skill.AllowedTools[1] != "Grep" || skill.AllowedTools[2] != "Glob" {
			t.Errorf("allowedTools = %v", skill.AllowedTools)
		}
	})

	t.Run("argument_hint", func(t *testing.T) {
		// Source: loadSkillsDir.ts:245-248
		// [file_path] is parsed as array syntax → "file_path"
		if skill.ArgumentHint != "file_path" {
			t.Errorf("argumentHint = %q", skill.ArgumentHint)
		}
	})

	t.Run("argument_names", func(t *testing.T) {
		// Source: loadSkillsDir.ts:249-251
		if len(skill.ArgumentNames) != 2 {
			t.Fatalf("expected 2 argument names, got %d", len(skill.ArgumentNames))
		}
		if skill.ArgumentNames[0] != "file_path" || skill.ArgumentNames[1] != "focus_area" {
			t.Errorf("argumentNames = %v", skill.ArgumentNames)
		}
	})

	t.Run("when_to_use", func(t *testing.T) {
		// Source: loadSkillsDir.ts:252
		if skill.WhenToUse != "When the user asks for a code review" {
			t.Errorf("whenToUse = %q", skill.WhenToUse)
		}
	})

	t.Run("version", func(t *testing.T) {
		// Source: loadSkillsDir.ts:253
		if skill.Version != "1.2.0" {
			t.Errorf("version = %q", skill.Version)
		}
	})

	t.Run("model", func(t *testing.T) {
		// Source: loadSkillsDir.ts:221-226
		if skill.Model != "sonnet" {
			t.Errorf("model = %q", skill.Model)
		}
	})

	t.Run("context_fork", func(t *testing.T) {
		// Source: loadSkillsDir.ts:260
		if skill.Context != "fork" {
			t.Errorf("context = %q, want fork", skill.Context)
		}
	})

	t.Run("agent", func(t *testing.T) {
		// Source: loadSkillsDir.ts:261
		if skill.Agent != "code-reviewer" {
			t.Errorf("agent = %q", skill.Agent)
		}
	})

	t.Run("effort", func(t *testing.T) {
		// Source: loadSkillsDir.ts:228-235
		if skill.Effort != "medium" {
			t.Errorf("effort = %q", skill.Effort)
		}
	})

	t.Run("paths", func(t *testing.T) {
		// Source: loadSkillsDir.ts:159-178 — /** suffix stripped
		if len(skill.Paths) != 2 {
			t.Fatalf("expected 2 paths, got %d", len(skill.Paths))
		}
		if skill.Paths[0] != "src" || skill.Paths[1] != "tests" {
			t.Errorf("paths = %v", skill.Paths)
		}
	})

	t.Run("shell", func(t *testing.T) {
		// Source: loadSkillsDir.ts:263
		if skill.Shell != "bash" {
			t.Errorf("shell = %q", skill.Shell)
		}
	})

	t.Run("user_invocable", func(t *testing.T) {
		if !skill.UserInvocable {
			t.Error("user-invocable should be true")
		}
		if skill.IsHidden {
			t.Error("should not be hidden when user-invocable")
		}
	})

	t.Run("prompt_body", func(t *testing.T) {
		if skill.Prompt != "Review the specified files for code quality issues." {
			t.Errorf("prompt = %q", skill.Prompt)
		}
	})
}

func TestParseSkillFromMarkdown_ModelInherit(t *testing.T) {
	// Source: loadSkillsDir.ts:222-223 — "inherit" means no override
	content := `---
description: Test
model: inherit
---
Prompt`

	skill := ParseSkillFromMarkdown("test", content, "user", "/tmp/test.md", "/tmp")
	if skill.Model != "" {
		t.Errorf("model=inherit should resolve to empty, got %q", skill.Model)
	}
}

func TestParseSkillFromMarkdown_UserInvocableFalse(t *testing.T) {
	// Source: loadSkillsDir.ts:216-219
	content := `---
description: Hidden skill
user-invocable: false
---
Internal prompt`

	skill := ParseSkillFromMarkdown("hidden", content, "user", "/tmp/hidden.md", "/tmp")
	if skill.UserInvocable {
		t.Error("should not be user-invocable")
	}
	if !skill.IsHidden {
		t.Error("should be hidden when user-invocable=false")
	}
}

func TestParseSkillFromMarkdown_DefaultUserInvocable(t *testing.T) {
	// Source: loadSkillsDir.ts:216-219 — default is true
	content := `---
description: Visible skill
---
Prompt`

	skill := ParseSkillFromMarkdown("visible", content, "user", "/tmp/visible.md", "/tmp")
	if !skill.UserInvocable {
		t.Error("default should be user-invocable=true")
	}
}

func TestParseSkillFromMarkdown_NoFrontmatter(t *testing.T) {
	content := "Just a plain prompt"

	skill := ParseSkillFromMarkdown("plain", content, "project", "/tmp/plain.md", "/tmp")
	if skill.Description != "Skill: plain" {
		t.Errorf("description = %q", skill.Description)
	}
	if skill.Prompt != "Just a plain prompt" {
		t.Errorf("prompt = %q", skill.Prompt)
	}
	if !skill.UserInvocable {
		t.Error("default should be user-invocable")
	}
}

func TestParseSkillPaths(t *testing.T) {
	// Source: loadSkillsDir.ts:159-178

	t.Run("strips_double_star_suffix", func(t *testing.T) {
		result := parseSkillPaths("src/**, tests/**")
		if len(result) != 2 || result[0] != "src" || result[1] != "tests" {
			t.Errorf("got %v", result)
		}
	})

	t.Run("all_double_star_returns_empty", func(t *testing.T) {
		// Source: loadSkillsDir.ts:173-175
		result := parseSkillPaths("**")
		if len(result) != 0 {
			t.Errorf("** should return empty, got %v", result)
		}
	})

	t.Run("preserves_non_star_paths", func(t *testing.T) {
		result := parseSkillPaths("src/components, lib")
		if len(result) != 2 {
			t.Fatalf("expected 2, got %d", len(result))
		}
		if result[0] != "src/components" || result[1] != "lib" {
			t.Errorf("got %v", result)
		}
	})

	t.Run("empty_returns_empty", func(t *testing.T) {
		result := parseSkillPaths("")
		if len(result) != 0 {
			t.Errorf("expected empty, got %v", result)
		}
	})
}

func TestParseCSVList(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		result := parseCSVList("Read, Write, Bash")
		if len(result) != 3 {
			t.Fatalf("expected 3, got %d", len(result))
		}
		if result[0] != "Read" || result[1] != "Write" || result[2] != "Bash" {
			t.Errorf("got %v", result)
		}
	})

	t.Run("single", func(t *testing.T) {
		result := parseCSVList("Read")
		if len(result) != 1 || result[0] != "Read" {
			t.Errorf("got %v", result)
		}
	})

	t.Run("empty", func(t *testing.T) {
		result := parseCSVList("")
		if len(result) != 0 {
			t.Errorf("got %v", result)
		}
	})
}

func TestLoadSkills_ProjectDir(t *testing.T) {
	cwd := t.TempDir()
	skillsDir := filepath.Join(cwd, ".claude", "skills")
	os.MkdirAll(skillsDir, 0755)
	os.WriteFile(filepath.Join(skillsDir, "test-skill.md"), []byte("Test prompt"), 0644)

	skills := LoadSkills(cwd)
	found := false
	for _, s := range skills {
		if s.Name == "test-skill" && s.Source == "project" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find project skill 'test-skill'")
	}
}

func TestLoadSkills_EmptyCWD(t *testing.T) {
	skills := LoadSkills("")
	_ = skills
}

func TestLoadedFromConstants(t *testing.T) {
	// Source: skills/loadSkillsDir.ts:67-74
	if LoadedFromSkills != "skills" {
		t.Error("wrong")
	}
	if LoadedFromPlugin != "plugin" {
		t.Error("wrong")
	}
	if LoadedFromManaged != "managed" {
		t.Error("wrong")
	}
	if LoadedFromBundled != "bundled" {
		t.Error("wrong")
	}
	if LoadedFromMCP != "mcp" {
		t.Error("wrong")
	}
}

func TestParseFrontmatterFields_ArraySyntax(t *testing.T) {
	fm := parseFrontmatterFields(`allowed-tools: [Read, "Write", 'Bash']
model: haiku`)

	if fm["allowed-tools"] != "Read, Write, Bash" {
		t.Errorf("allowed-tools = %q", fm["allowed-tools"])
	}
	if fm["model"] != "haiku" {
		t.Errorf("model = %q", fm["model"])
	}
}

func TestParseFrontmatterFields_QuotedValues(t *testing.T) {
	fm := parseFrontmatterFields(`description: "A quoted description"
name: 'single-quoted'`)

	if fm["description"] != "A quoted description" {
		t.Errorf("description = %q", fm["description"])
	}
	if fm["name"] != "single-quoted" {
		t.Errorf("name = %q", fm["name"])
	}
}

func TestParseSkillFromMarkdown_DisableModelInvocation(t *testing.T) {
	// Source: loadSkillsDir.ts:255-257
	content := `---
description: No model
disable-model-invocation: true
---
Prompt`

	skill := ParseSkillFromMarkdown("nomodel", content, "user", "/tmp/nm.md", "/tmp")
	if !skill.DisableModelInvocation {
		t.Error("should have disable-model-invocation=true")
	}
}

func TestParseSkillFromMarkdown_ShellPowershell(t *testing.T) {
	// Source: loadSkillsDir.ts:263
	content := `---
description: PS skill
shell: powershell
---
Prompt`

	skill := ParseSkillFromMarkdown("ps", content, "user", "/tmp/ps.md", "/tmp")
	if skill.Shell != "powershell" {
		t.Errorf("shell = %q, want powershell", skill.Shell)
	}
}

func TestParseSkillFromMarkdown_InvalidShellIgnored(t *testing.T) {
	content := `---
description: Bad shell
shell: zsh
---
Prompt`

	skill := ParseSkillFromMarkdown("bad", content, "user", "/tmp/bad.md", "/tmp")
	if skill.Shell != "" {
		t.Errorf("invalid shell should be ignored, got %q", skill.Shell)
	}
}
