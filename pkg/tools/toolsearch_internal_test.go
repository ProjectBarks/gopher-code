package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestToolSearchTool_SelectMode(t *testing.T) {
	reg := NewRegistry()
	RegisterDefaults(reg)
	ts := NewToolSearchTool(reg)

	input, _ := json.Marshal(map[string]any{"query": "select:Read,Grep", "max_results": 5})
	out, err := ts.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out.Content, "Read") {
		t.Error("should find Read tool")
	}
	if !strings.Contains(out.Content, "Grep") {
		t.Error("should find Grep tool")
	}
}

func TestToolSearchTool_KeywordSearch(t *testing.T) {
	reg := NewRegistry()
	RegisterDefaults(reg)
	ts := NewToolSearchTool(reg)

	input, _ := json.Marshal(map[string]any{"query": "file", "max_results": 10})
	out, err := ts.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out.Content, "Found") {
		t.Errorf("should find some tools, got: %s", out.Content)
	}
}

func TestToolSearchTool_RequiredKeyword(t *testing.T) {
	reg := NewRegistry()
	RegisterDefaults(reg)
	ts := NewToolSearchTool(reg)

	input, _ := json.Marshal(map[string]any{"query": "+Bash command", "max_results": 5})
	out, err := ts.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out.Content, "Bash") {
		t.Errorf("should find Bash with +Bash filter, got: %s", out.Content)
	}
}

func TestToolSearchTool_NoResults(t *testing.T) {
	reg := NewRegistry()
	RegisterDefaults(reg)
	ts := NewToolSearchTool(reg)

	input, _ := json.Marshal(map[string]any{"query": "xyznonexistent123"})
	out, err := ts.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out.Content, "No tools found") {
		t.Errorf("should report no match, got: %s", out.Content)
	}
}

func TestParseToolNameParts(t *testing.T) {
	tests := map[string]string{
		"FileReadTool":            "file read tool",
		"mcp__github__list_repos": "github list repos",
		"Bash":                    "bash",
		"NotebookEdit":            "notebook edit",
	}
	for input, want := range tests {
		got := parseToolNameParts(input)
		if got != want {
			t.Errorf("parseToolNameParts(%q) = %q, want %q", input, got, want)
		}
	}
}
