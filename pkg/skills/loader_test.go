package skills

import (
	"os"
	"path/filepath"
	"testing"
)

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

	// Create a simple .md skill
	os.WriteFile(filepath.Join(dir, "greet.md"), []byte("Say hello to the user."), 0644)

	// Create a .txt skill
	os.WriteFile(filepath.Join(dir, "review.txt"), []byte("Review the current PR."), 0644)

	// Create a non-skill file (should be ignored)
	os.WriteFile(filepath.Join(dir, "notes.yaml"), []byte("key: value"), 0644)

	// Create a subdirectory (should be ignored)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	skills := loadFromDir(dir, "project")
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	// Find the greet skill
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
	// Should not panic with empty CWD
	skills := LoadSkills("")
	_ = skills // just ensure no panic
}
