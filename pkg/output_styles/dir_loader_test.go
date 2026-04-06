package output_styles

import (
	"os"
	"path/filepath"
	"testing"
)

// ── T26: GetOutputStyleDirStyles ──────────────────────────────────

func TestGetOutputStyleDirStyles_LoadsFromProjectDir(t *testing.T) {
	dir := t.TempDir()
	stylesDir := filepath.Join(dir, ".claude", "output-styles")
	os.MkdirAll(stylesDir, 0755)

	os.WriteFile(filepath.Join(stylesDir, "concise.md"), []byte(`---
name: Concise
description: Short and sweet
---
Be extremely brief in all responses.
`), 0644)

	ClearOutputStyleCaches()
	styles := GetOutputStyleDirStyles(dir)
	if len(styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(styles))
	}
	if styles[0].Name != "Concise" {
		t.Errorf("name = %q, want %q", styles[0].Name, "Concise")
	}
	if styles[0].Description != "Short and sweet" {
		t.Errorf("description = %q, want %q", styles[0].Description, "Short and sweet")
	}
	if styles[0].Prompt != "Be extremely brief in all responses." {
		t.Errorf("prompt = %q, want trimmed content", styles[0].Prompt)
	}
	if styles[0].Source != "projectSettings" {
		t.Errorf("source = %q, want projectSettings", styles[0].Source)
	}
}

func TestGetOutputStyleDirStyles_NameFallsBackToFilename(t *testing.T) {
	dir := t.TempDir()
	stylesDir := filepath.Join(dir, ".claude", "output-styles")
	os.MkdirAll(stylesDir, 0755)

	os.WriteFile(filepath.Join(stylesDir, "verbose.md"), []byte("Explain everything in detail.\n"), 0644)

	ClearOutputStyleCaches()
	styles := GetOutputStyleDirStyles(dir)
	if len(styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(styles))
	}
	if styles[0].Name != "verbose" {
		t.Errorf("name = %q, want %q (filename without .md)", styles[0].Name, "verbose")
	}
}

func TestGetOutputStyleDirStyles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	ClearOutputStyleCaches()
	styles := GetOutputStyleDirStyles(dir)
	if len(styles) != 0 {
		t.Errorf("expected 0 styles for missing dir, got %d", len(styles))
	}
}

// ── T27: ClearOutputStyleCaches ───────────────────────────────────

func TestClearOutputStyleCaches_ReloadsAfterClear(t *testing.T) {
	dir := t.TempDir()
	stylesDir := filepath.Join(dir, ".claude", "output-styles")
	os.MkdirAll(stylesDir, 0755)

	ClearOutputStyleCaches()

	// First load: empty
	styles := GetOutputStyleDirStyles(dir)
	if len(styles) != 0 {
		t.Fatalf("expected 0 styles initially, got %d", len(styles))
	}

	// Add a file
	os.WriteFile(filepath.Join(stylesDir, "new.md"), []byte("New style content\n"), 0644)

	// Still cached: should be 0
	styles = GetOutputStyleDirStyles(dir)
	if len(styles) != 0 {
		t.Fatalf("expected 0 styles from cache, got %d", len(styles))
	}

	// Clear and reload
	ClearOutputStyleCaches()
	styles = GetOutputStyleDirStyles(dir)
	if len(styles) != 1 {
		t.Fatalf("expected 1 style after clear, got %d", len(styles))
	}
}

// ── T28: LoadMarkdownFilesForSubdir (project + user merge) ───────

func TestLoadMarkdownFilesForSubdir_ProjectDir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, ".claude", "output-styles")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "test.md"), []byte("---\nname: Test\n---\nHello\n"), 0644)

	files, err := LoadMarkdownFilesForSubdir("output-styles", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should find at least the project file
	found := false
	for _, f := range files {
		if f.Frontmatter["name"] == "Test" {
			found = true
			if f.Source != "projectSettings" {
				t.Errorf("source = %q, want projectSettings", f.Source)
			}
		}
	}
	if !found {
		t.Error("project file not found in results")
	}
}

func TestLoadMarkdownFilesForSubdir_IgnoresNonMd(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, ".claude", "output-styles")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "readme.txt"), []byte("not markdown"), 0644)
	os.WriteFile(filepath.Join(subDir, "style.md"), []byte("markdown"), 0644)

	files, err := LoadMarkdownFilesForSubdir("output-styles", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mdCount := 0
	for _, f := range files {
		if filepath.Ext(f.FilePath) == ".md" {
			mdCount++
		}
	}
	if mdCount < 1 {
		t.Error("expected at least 1 .md file")
	}
	for _, f := range files {
		if filepath.Ext(f.FilePath) != ".md" {
			t.Errorf("non-.md file included: %s", f.FilePath)
		}
	}
}

// ── T29: ParseKeepCodingInstructions ──────────────────────────────

func TestParseKeepCodingInstructions_BoolTrue(t *testing.T) {
	got := ParseKeepCodingInstructions(true)
	if got == nil || !*got {
		t.Error("expected *true for bool true")
	}
}

func TestParseKeepCodingInstructions_BoolFalse(t *testing.T) {
	got := ParseKeepCodingInstructions(false)
	if got == nil || *got {
		t.Error("expected *false for bool false")
	}
}

func TestParseKeepCodingInstructions_StringTrue(t *testing.T) {
	got := ParseKeepCodingInstructions("true")
	if got == nil || !*got {
		t.Error("expected *true for string 'true'")
	}
}

func TestParseKeepCodingInstructions_StringFalse(t *testing.T) {
	got := ParseKeepCodingInstructions("false")
	if got == nil || *got {
		t.Error("expected *false for string 'false'")
	}
}

func TestParseKeepCodingInstructions_StringTrueUpperCase(t *testing.T) {
	got := ParseKeepCodingInstructions("True")
	if got == nil || !*got {
		t.Error("expected *true for string 'True'")
	}
}

func TestParseKeepCodingInstructions_UnknownValue(t *testing.T) {
	got := ParseKeepCodingInstructions("maybe")
	if got != nil {
		t.Error("expected nil for unrecognized value")
	}
}

func TestParseKeepCodingInstructions_Nil(t *testing.T) {
	got := ParseKeepCodingInstructions(nil)
	if got != nil {
		t.Error("expected nil for nil input")
	}
}

func TestParseKeepCodingInstructions_Int(t *testing.T) {
	got := ParseKeepCodingInstructions(42)
	if got != nil {
		t.Error("expected nil for int input")
	}
}

// ── T30: ExtractDescriptionFromMarkdown ───────────────────────────

func TestExtractDescriptionFromMarkdown_FirstLine(t *testing.T) {
	got := ExtractDescriptionFromMarkdown("First line\nSecond line", "default")
	if got != "First line" {
		t.Errorf("got %q, want %q", got, "First line")
	}
}

func TestExtractDescriptionFromMarkdown_HeaderStripped(t *testing.T) {
	got := ExtractDescriptionFromMarkdown("## My Heading\nBody", "default")
	if got != "My Heading" {
		t.Errorf("got %q, want %q", got, "My Heading")
	}
}

func TestExtractDescriptionFromMarkdown_EmptyContentUsesDefault(t *testing.T) {
	got := ExtractDescriptionFromMarkdown("", "Custom item")
	if got != "Custom item" {
		t.Errorf("got %q, want %q", got, "Custom item")
	}
}

func TestExtractDescriptionFromMarkdown_BlankLinesSkipped(t *testing.T) {
	got := ExtractDescriptionFromMarkdown("\n\n  \nActual content", "default")
	if got != "Actual content" {
		t.Errorf("got %q, want %q", got, "Actual content")
	}
}

func TestExtractDescriptionFromMarkdown_LongLineTruncated(t *testing.T) {
	long := "A" + string(make([]byte, 150)) // > 100 chars
	// Actually, let's make a proper long string
	long = ""
	for i := 0; i < 110; i++ {
		long += "x"
	}
	got := ExtractDescriptionFromMarkdown(long, "default")
	if len(got) > 100 {
		t.Errorf("expected truncated to 100 chars, got len %d", len(got))
	}
	if got[len(got)-3:] != "..." {
		t.Errorf("expected trailing '...', got %q", got[len(got)-3:])
	}
}

// ── T31 preview: force-for-plugin warn ────────────────────────────

func TestForceForPlugin_NonPluginStyleLogsWarning(t *testing.T) {
	dir := t.TempDir()
	stylesDir := filepath.Join(dir, ".claude", "output-styles")
	os.MkdirAll(stylesDir, 0755)

	os.WriteFile(filepath.Join(stylesDir, "forced.md"), []byte(`---
name: ForcedStyle
force-for-plugin: true
---
Forced content
`), 0644)

	ClearOutputStyleCaches()
	styles := GetOutputStyleDirStyles(dir)
	if len(styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(styles))
	}
	// The style should still load (warn only, not fail)
	if styles[0].Name != "ForcedStyle" {
		t.Errorf("name = %q, want ForcedStyle", styles[0].Name)
	}
	// ForceForPlugin should NOT be set on non-plugin styles
	if styles[0].ForceForPlugin {
		t.Error("ForceForPlugin should be false for non-plugin styles")
	}
}

// ── Frontmatter parsing ──────────────────────────────────────────

func TestParseFrontmatter_ValidYAML(t *testing.T) {
	input := "---\nname: Test\ndescription: A test\n---\nBody content\n"
	fm, content := parseFrontmatter(input)
	if fm["name"] != "Test" {
		t.Errorf("name = %v, want Test", fm["name"])
	}
	if fm["description"] != "A test" {
		t.Errorf("description = %v, want 'A test'", fm["description"])
	}
	if content != "Body content\n" {
		t.Errorf("content = %q, want 'Body content\\n'", content)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	input := "Just plain markdown\n"
	fm, content := parseFrontmatter(input)
	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
	if content != input {
		t.Errorf("content should be the whole input")
	}
}

func TestParseFrontmatter_KeepCodingBoolField(t *testing.T) {
	input := "---\nkeep-coding-instructions: true\n---\nContent\n"
	fm, _ := parseFrontmatter(input)
	result := ParseKeepCodingInstructions(fm["keep-coding-instructions"])
	if result == nil || !*result {
		t.Errorf("expected *true for YAML bool true, got %v", result)
	}
}

// ── coerceDescriptionToString ─────────────────────────────────────

func TestCoerceDescriptionToString_String(t *testing.T) {
	got := coerceDescriptionToString("  hello  ", "test")
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestCoerceDescriptionToString_Nil(t *testing.T) {
	got := coerceDescriptionToString(nil, "test")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestCoerceDescriptionToString_Number(t *testing.T) {
	got := coerceDescriptionToString(42, "test")
	if got != "42" {
		t.Errorf("got %q, want %q", got, "42")
	}
}

func TestCoerceDescriptionToString_Bool(t *testing.T) {
	got := coerceDescriptionToString(true, "test")
	if got != "true" {
		t.Errorf("got %q, want %q", got, "true")
	}
}

func TestCoerceDescriptionToString_SliceReturnsEmpty(t *testing.T) {
	got := coerceDescriptionToString([]string{"a", "b"}, "test")
	if got != "" {
		t.Errorf("got %q, want empty for non-scalar", got)
	}
}
