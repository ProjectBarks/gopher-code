package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/skills"
)

func TestSkillTool_Execute_Found(t *testing.T) {
	s := []skills.Skill{
		{Name: "greet", Prompt: "Say hello to the user.", Description: "Greet skill"},
		{Name: "review", Prompt: "Review the code.", Description: "Review skill"},
	}
	tool := NewSkillTool(s)

	input, _ := json.Marshal(map[string]string{"skill": "greet"})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.IsError {
		t.Fatalf("expected success, got error: %s", out.Content)
	}
	if out.Content != "Say hello to the user." {
		t.Errorf("unexpected content: %q", out.Content)
	}
}

func TestSkillTool_Execute_WithArgs(t *testing.T) {
	s := []skills.Skill{
		{Name: "deploy", Prompt: "Deploy the app."},
	}
	tool := NewSkillTool(s)

	input, _ := json.Marshal(map[string]string{"skill": "deploy", "args": "staging"})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.IsError {
		t.Fatalf("expected success, got error: %s", out.Content)
	}
	expected := "Deploy the app.\n\nArguments: staging"
	if out.Content != expected {
		t.Errorf("expected %q, got %q", expected, out.Content)
	}
}

func TestSkillTool_Execute_NotFound(t *testing.T) {
	tool := NewSkillTool(nil)

	input, _ := json.Marshal(map[string]string{"skill": "nonexistent"})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Fatal("expected error output for missing skill")
	}
	if out.Content != "skill not found: nonexistent" {
		t.Errorf("unexpected error message: %q", out.Content)
	}
}

func TestSkillTool_Execute_EmptyName(t *testing.T) {
	tool := NewSkillTool(nil)

	input, _ := json.Marshal(map[string]string{"skill": ""})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Fatal("expected error output for empty skill name")
	}
}

func TestSkillTool_Metadata(t *testing.T) {
	tool := NewSkillTool(nil)
	if tool.Name() != "Skill" {
		t.Errorf("expected name 'Skill', got %q", tool.Name())
	}
	if !tool.IsReadOnly() {
		t.Error("expected Skill tool to be read-only")
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(tool.InputSchema(), &schema); err != nil {
		t.Fatalf("invalid input schema: %v", err)
	}
}
