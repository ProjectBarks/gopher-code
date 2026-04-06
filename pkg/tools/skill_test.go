package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestSkillTool_Execute_WithArgs_NoPlaceholder(t *testing.T) {
	// When a skill has no $ARGUMENTS placeholder, args are appended.
	// Source: argumentSubstitution.ts:140-142
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
	expected := "Deploy the app.\n\nARGUMENTS: staging"
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
	// Source: SkillTool.ts:406-408
	if out.Content != "Unknown skill: nonexistent" {
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

// --- T529: New tests below ---

func TestSkillTool_Description(t *testing.T) {
	// Source: prompt.ts:174 — description should match TS
	tool := NewSkillTool(nil)
	expected := "Execute a skill within the main conversation"
	if tool.Description() != expected {
		t.Errorf("description = %q, want %q", tool.Description(), expected)
	}
}

func TestSkillTool_SearchHint(t *testing.T) {
	// Source: SkillTool.ts:333
	tool := NewSkillTool(nil)
	if tool.SearchHint() != "invoke a slash-command skill" {
		t.Errorf("searchHint = %q", tool.SearchHint())
	}
}

func TestSkillTool_MaxResultSizeChars(t *testing.T) {
	// Source: SkillTool.ts:334
	tool := NewSkillTool(nil)
	if tool.MaxResultSizeChars() != 100_000 {
		t.Errorf("maxResultSizeChars = %d", tool.MaxResultSizeChars())
	}
}

func TestSkillTool_Prompt_VerbatimText(t *testing.T) {
	// Source: tools/SkillTool/prompt.ts:173-196
	tool := NewSkillTool(nil)
	prompt := tool.Prompt()

	mustContain := []string{
		"Execute a skill within the main conversation",
		`When users reference a "slash command"`,
		`skill: "pdf"`,
		`skill: "commit", args: "-m 'Fix bug'"`,
		`skill: "review-pr", args: "123"`,
		`skill: "ms-office-suite:pdf"`,
		"BLOCKING REQUIREMENT",
		"NEVER mention a skill without actually calling this tool",
		"Do not invoke a skill that is already running",
		"Do not use this tool for built-in CLI commands",
		"<command-name>",
		"the skill has ALREADY been loaded",
	}
	for _, s := range mustContain {
		if !strings.Contains(prompt, s) {
			t.Errorf("prompt missing: %q", s)
		}
	}
}

func TestSkillTool_Execute_LeadingSlash(t *testing.T) {
	// Source: SkillTool.ts:370-372, 440, 598 — strip leading slash
	s := []skills.Skill{
		{Name: "commit", Prompt: "Create a commit."},
	}
	tool := NewSkillTool(s)

	input, _ := json.Marshal(map[string]string{"skill": "/commit"})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.IsError {
		t.Fatalf("expected success with leading slash, got error: %s", out.Content)
	}
	if out.Content != "Create a commit." {
		t.Errorf("content = %q", out.Content)
	}
}

func TestSkillTool_Execute_DisableModelInvocation(t *testing.T) {
	// Source: SkillTool.ts:412-418
	s := []skills.Skill{
		{Name: "internal", Prompt: "Internal use only.", DisableModelInvocation: true},
	}
	tool := NewSkillTool(s)

	input, _ := json.Marshal(map[string]string{"skill": "internal"})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Fatal("expected error for disable-model-invocation skill")
	}
	if !strings.Contains(out.Content, "disable-model-invocation") {
		t.Errorf("error message should mention disable-model-invocation: %q", out.Content)
	}
}

func TestSkillTool_Execute_SkillMDParsing_TempDir(t *testing.T) {
	// E2E: load a skill from a temp directory, invoke it via the tool.
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	md := `---
description: Greet a user by name
when_to_use: When the user wants to say hello
arguments: name
---
Hello $name! Welcome aboard.`

	if err := os.WriteFile(filepath.Join(skillsDir, "greet.md"), []byte(md), 0644); err != nil {
		t.Fatal(err)
	}

	loaded := skills.LoadSkills(dir)
	var greet *skills.Skill
	for i, s := range loaded {
		if s.Name == "greet" {
			greet = &loaded[i]
			break
		}
	}
	if greet == nil {
		t.Fatal("failed to load greet skill from temp dir")
	}

	tool := NewSkillTool(loaded)

	input, _ := json.Marshal(map[string]string{"skill": "greet", "args": "Alice"})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.IsError {
		t.Fatalf("expected success: %s", out.Content)
	}
	if !strings.Contains(out.Content, "Hello Alice!") {
		t.Errorf("expected named arg substitution, got: %q", out.Content)
	}
}

func TestSkillTool_Execute_SkillMD_IndexedArgs(t *testing.T) {
	// Test $ARGUMENTS[N] and $N shorthand substitution
	s := []skills.Skill{
		{Name: "test", Prompt: "First: $ARGUMENTS[0], Second: $1, All: $ARGUMENTS"},
	}
	tool := NewSkillTool(s)

	input, _ := json.Marshal(map[string]string{"skill": "test", "args": "foo bar"})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.IsError {
		t.Fatalf("expected success: %s", out.Content)
	}
	expected := "First: foo, Second: bar, All: foo bar"
	if out.Content != expected {
		t.Errorf("expected %q, got %q", expected, out.Content)
	}
}

func TestSkillTool_Execute_SkillMD_QuotedArgs(t *testing.T) {
	// Quoted arguments are kept as single tokens
	s := []skills.Skill{
		{Name: "test", Prompt: "Arg0: $0, Arg1: $1"},
	}
	tool := NewSkillTool(s)

	input, _ := json.Marshal(map[string]string{"skill": "test", "args": `"hello world" bar`})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Arg0: hello world, Arg1: bar"
	if out.Content != expected {
		t.Errorf("expected %q, got %q", expected, out.Content)
	}
}

func TestSkillTool_Execute_UnknownSkillError(t *testing.T) {
	// Source: SkillTool.ts:403-409
	s := []skills.Skill{
		{Name: "commit", Prompt: "Commit code."},
	}
	tool := NewSkillTool(s)

	input, _ := json.Marshal(map[string]string{"skill": "doesnotexist"})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Fatal("expected error for unknown skill")
	}
	if !strings.Contains(out.Content, "Unknown skill: doesnotexist") {
		t.Errorf("error message = %q", out.Content)
	}
}

// --- SubstituteArguments tests ---

func TestSubstituteArguments_FullArgs(t *testing.T) {
	result := SubstituteArguments("Run: $ARGUMENTS", "foo bar", true, nil)
	if result != "Run: foo bar" {
		t.Errorf("got %q", result)
	}
}

func TestSubstituteArguments_IndexedArgs(t *testing.T) {
	result := SubstituteArguments("$ARGUMENTS[0] and $ARGUMENTS[1]", "first second", true, nil)
	if result != "first and second" {
		t.Errorf("got %q", result)
	}
}

func TestSubstituteArguments_ShorthandArgs(t *testing.T) {
	result := SubstituteArguments("$0 and $1", "first second", true, nil)
	if result != "first and second" {
		t.Errorf("got %q", result)
	}
}

func TestSubstituteArguments_NamedArgs(t *testing.T) {
	result := SubstituteArguments("Hello $name, welcome to $place!", "Alice Wonderland", true, []string{"name", "place"})
	if result != "Hello Alice, welcome to Wonderland!" {
		t.Errorf("got %q", result)
	}
}

func TestSubstituteArguments_NoPlaceholder_Appends(t *testing.T) {
	result := SubstituteArguments("Plain prompt", "myarg", true, nil)
	expected := "Plain prompt\n\nARGUMENTS: myarg"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSubstituteArguments_NoPlaceholder_NoAppend(t *testing.T) {
	result := SubstituteArguments("Plain prompt", "myarg", false, nil)
	if result != "Plain prompt" {
		t.Errorf("expected unchanged, got %q", result)
	}
}

func TestSubstituteArguments_EmptyArgs(t *testing.T) {
	result := SubstituteArguments("Hello $ARGUMENTS", "", true, nil)
	if result != "Hello $ARGUMENTS" {
		t.Errorf("expected unchanged for empty args, got %q", result)
	}
}

func TestSubstituteArguments_OutOfBoundsIndex(t *testing.T) {
	// Out-of-bounds indexed args should resolve to empty string
	result := SubstituteArguments("$ARGUMENTS[5]", "only", true, nil)
	if result != "" {
		t.Errorf("expected empty for out-of-bounds, got %q", result)
	}
}

// --- parseShellArguments tests ---

func TestParseShellArguments(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"foo bar baz", []string{"foo", "bar", "baz"}},
		{`foo "hello world" baz`, []string{"foo", "hello world", "baz"}},
		{`foo 'hello world' baz`, []string{"foo", "hello world", "baz"}},
		{"", nil},
		{"  ", nil},
		{"single", []string{"single"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseShellArguments(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d args, got %d: %v", len(tt.expected), len(result), result)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("arg[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// --- Budget / listing format tests ---

func TestGetCharBudget_ContextWindow(t *testing.T) {
	// Source: prompt.ts:31-41
	budget := GetCharBudget(200_000) // 200k token window
	expected := 8_000                // 200k * 4 * 0.01
	if budget != expected {
		t.Errorf("budget = %d, want %d", budget, expected)
	}
}

func TestGetCharBudget_Default(t *testing.T) {
	budget := GetCharBudget(0)
	if budget != DefaultCharBudget {
		t.Errorf("budget = %d, want %d", budget, DefaultCharBudget)
	}
}

func TestGetSkillDescription_Truncation(t *testing.T) {
	// Source: prompt.ts:43-50
	long := strings.Repeat("a", 300)
	s := skills.Skill{Description: long}
	desc := GetSkillDescription(s)
	runeCount := len([]rune(desc))
	if runeCount != MaxListingDescChars {
		t.Errorf("rune len = %d, want %d", runeCount, MaxListingDescChars)
	}
	if !strings.HasSuffix(desc, "\u2026") {
		t.Error("should end with ellipsis")
	}
}

func TestGetSkillDescription_WithWhenToUse(t *testing.T) {
	s := skills.Skill{Description: "Review code", WhenToUse: "When asked for review"}
	desc := GetSkillDescription(s)
	if desc != "Review code - When asked for review" {
		t.Errorf("desc = %q", desc)
	}
}

func TestFormatSkillListing(t *testing.T) {
	// Source: prompt.ts:52-66 — "- {name}: {description}"
	s := skills.Skill{Name: "commit", Description: "Create a commit"}
	listing := FormatSkillListing(s)
	if listing != "- commit: Create a commit" {
		t.Errorf("listing = %q", listing)
	}
}

func TestFormatSkillsWithinBudget_FitsAll(t *testing.T) {
	ss := []skills.Skill{
		{Name: "a", Description: "desc-a"},
		{Name: "b", Description: "desc-b"},
	}
	// Large context window means everything fits
	result := FormatSkillsWithinBudget(ss, 1_000_000)
	if !strings.Contains(result, "- a: desc-a") {
		t.Errorf("missing a: %q", result)
	}
	if !strings.Contains(result, "- b: desc-b") {
		t.Errorf("missing b: %q", result)
	}
}

func TestFormatSkillsWithinBudget_Empty(t *testing.T) {
	result := FormatSkillsWithinBudget(nil, 200_000)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestFormatSkillsWithinBudget_BundledProtected(t *testing.T) {
	// Bundled skills always keep full descriptions.
	// Source: prompt.ts:92-101
	bundled := skills.Skill{Name: "core-skill", Description: "A very important bundled skill", Source: "bundled"}
	user := skills.Skill{Name: "my-long-name-skill", Description: strings.Repeat("x", 200), Source: "user"}
	ss := []skills.Skill{bundled, user}

	// Use a very small context window to force truncation
	result := FormatSkillsWithinBudget(ss, 100) // budget = 100 * 4 * 0.01 = 4 chars
	if !strings.Contains(result, "A very important bundled skill") {
		t.Errorf("bundled description should be preserved: %q", result)
	}
}

// --- normalizeSkillName tests ---

func TestNormalizeSkillName(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"commit", "commit"},
		{"/commit", "commit"},
		{"  /review-pr  ", "review-pr"},
		{"ms-office:pdf", "ms-office:pdf"},
	}
	for _, tt := range tests {
		if got := normalizeSkillName(tt.input); got != tt.expected {
			t.Errorf("normalizeSkillName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- Constants tests ---

func TestBudgetConstants(t *testing.T) {
	// Source: prompt.ts:21-29
	if SkillBudgetContextPercent != 0.01 {
		t.Errorf("SkillBudgetContextPercent = %f", SkillBudgetContextPercent)
	}
	if CharsPerToken != 4 {
		t.Errorf("CharsPerToken = %d", CharsPerToken)
	}
	if DefaultCharBudget != 8_000 {
		t.Errorf("DefaultCharBudget = %d", DefaultCharBudget)
	}
	if MaxListingDescChars != 250 {
		t.Errorf("MaxListingDescChars = %d", MaxListingDescChars)
	}
	if MinDescLength != 20 {
		t.Errorf("MinDescLength = %d", MinDescLength)
	}
}

func TestInputSchema_SkillDescription(t *testing.T) {
	// Source: SkillTool.ts:292-297 — schema descriptions match TS
	tool := NewSkillTool(nil)
	var schema struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(tool.InputSchema(), &schema); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(schema.Properties["skill"].Description, "commit") {
		t.Errorf("skill description should mention commit: %q", schema.Properties["skill"].Description)
	}
	if !strings.Contains(schema.Properties["args"].Description, "Optional") {
		t.Errorf("args description should say Optional: %q", schema.Properties["args"].Description)
	}
}
