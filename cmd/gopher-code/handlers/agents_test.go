package handlers_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/cmd/gopher-code/handlers"
	"github.com/projectbarks/gopher-code/pkg/agents"
)

func TestAgentsHandler_NoDirs(t *testing.T) {
	t.Parallel()
	// Use a temp dir that has no .claude/agents anywhere.
	tmp := t.TempDir()
	var buf bytes.Buffer
	handlers.AgentsHandler(&buf, tmp)
	got := buf.String()
	if !strings.Contains(got, "No agents found.") {
		t.Fatalf("expected 'No agents found.' for empty dirs, got:\n%s", got)
	}
}

func TestAgentsHandler_MarkdownAgents(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Create project-level agents dir with two .md files.
	agentsDir := filepath.Join(tmp, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "code-review.md"), []byte("# Code Review\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "deploy.md"), []byte("# Deploy\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handlers.AgentsHandler(&buf, tmp)
	got := buf.String()

	// Verify header format.
	if !strings.HasPrefix(got, "2 active agents\n") {
		t.Errorf("expected '2 active agents' header, got:\n%s", got)
	}

	// Verify group label.
	if !strings.Contains(got, "Project agents:") {
		t.Errorf("expected 'Project agents:' group header, got:\n%s", got)
	}

	// Verify 2-space indent on agent lines.
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "code-review") || strings.Contains(line, "deploy") {
			if !strings.HasPrefix(line, "  ") {
				t.Errorf("expected 2-space indent on agent line, got: %q", line)
			}
		}
	}

	// Verify alphabetical order (code-review before deploy).
	crIdx := strings.Index(got, "code-review")
	dIdx := strings.Index(got, "deploy")
	if crIdx < 0 || dIdx < 0 || crIdx >= dIdx {
		t.Errorf("expected code-review before deploy in output:\n%s", got)
	}
}

func TestAgentsHandler_JSONAgents(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	agentsDir := filepath.Join(tmp, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	defs := map[string]map[string]string{
		"tester": {
			"description": "Runs tests",
			"model":       "claude-sonnet-4-6",
			"memory":      "user",
		},
	}
	data, _ := json.Marshal(defs)
	if err := os.WriteFile(filepath.Join(agentsDir, "agents.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handlers.AgentsHandler(&buf, tmp)
	got := buf.String()

	// Verify middle-dot format: "tester · claude-sonnet-4-6 · user memory"
	if !strings.Contains(got, "tester \u00b7 claude-sonnet-4-6 \u00b7 user memory") {
		t.Errorf("expected formatted agent line with middle dots, got:\n%s", got)
	}

	if !strings.HasPrefix(got, "1 active agents\n") {
		t.Errorf("expected '1 active agents' header, got:\n%s", got)
	}
}

func TestAgentsHandler_ShadowedAgent(t *testing.T) {
	t.Parallel()

	userDir := t.TempDir()
	projectDir := t.TempDir()

	// Create the same agent in both user and project dirs.
	if err := os.WriteFile(filepath.Join(userDir, "reviewer.md"), []byte("# Reviewer\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "reviewer.md"), []byte("# Reviewer\n"), 0644); err != nil {
		t.Fatal(err)
	}

	dirs := map[agents.Source]string{
		agents.SourceUser:    userDir,
		agents.SourceProject: projectDir,
	}

	var buf bytes.Buffer
	handlers.AgentsHandlerWithDirs(&buf, dirs)
	got := buf.String()

	// Project overrides user, so user should be shadowed.
	if !strings.Contains(got, "(shadowed by project) reviewer") {
		t.Errorf("expected user agent to be shadowed by project, got:\n%s", got)
	}

	// Only 1 active (the project one).
	if !strings.HasPrefix(got, "1 active agents\n") {
		t.Errorf("expected '1 active agents' header, got:\n%s", got)
	}
}

func TestAgentsHandler_OutputEndsClean(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	agentsDir := filepath.Join(tmp, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "alpha.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handlers.AgentsHandler(&buf, tmp)
	got := buf.String()

	// Output should end with a single newline (from Fprintln), not multiple trailing newlines.
	trimmed := strings.TrimRight(got, "\n")
	trailing := got[len(trimmed):]
	if trailing != "\n" {
		t.Errorf("expected output to end with exactly one newline, got %d trailing newlines", len(trailing))
	}
}
