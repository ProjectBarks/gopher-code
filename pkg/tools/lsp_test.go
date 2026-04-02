package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLSPTool_Metadata(t *testing.T) {
	tool := &LSPTool{}
	if tool.Name() != "LSP" {
		t.Errorf("expected name 'LSP', got %q", tool.Name())
	}
	if !tool.IsReadOnly() {
		t.Error("expected LSP tool to be read-only")
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(tool.InputSchema(), &schema); err != nil {
		t.Fatalf("invalid input schema: %v", err)
	}
}

func TestLSPTool_ExtractSymbols_GoFile(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "example.go")
	content := `package example

type Foo struct {
	Name string
}

func Bar() string {
	return "bar"
}

func (f *Foo) Method() {}

const MaxSize = 100
`
	os.WriteFile(goFile, []byte(content), 0644)

	tool := &LSPTool{}
	input, _ := json.Marshal(map[string]interface{}{
		"command":   "symbols",
		"file_path": goFile,
	})
	out, err := tool.Execute(context.Background(), &ToolContext{CWD: dir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.IsError {
		t.Fatalf("expected success, got error: %s", out.Content)
	}

	// Should find type, func, and const declarations
	if len(out.Content) == 0 {
		t.Fatal("expected symbols output, got empty")
	}

	// Check specific symbols are found
	for _, expected := range []string{"type Foo struct", "func Bar()", "const MaxSize"} {
		found := false
		for _, line := range splitLines(out.Content) {
			if containsStr(line, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find symbol %q in output:\n%s", expected, out.Content)
		}
	}
}

func TestLSPTool_ExtractSymbols_NonExistentFile(t *testing.T) {
	tool := &LSPTool{}
	input, _ := json.Marshal(map[string]interface{}{
		"command":   "symbols",
		"file_path": "/nonexistent/file.go",
	})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLSPTool_ExtractSymbols_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty.go")
	os.WriteFile(emptyFile, []byte(""), 0644)

	tool := &LSPTool{}
	input, _ := json.Marshal(map[string]interface{}{
		"command":   "symbols",
		"file_path": emptyFile,
	})
	out, err := tool.Execute(context.Background(), &ToolContext{CWD: dir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Content != "No symbols found" {
		t.Errorf("expected 'No symbols found', got %q", out.Content)
	}
}

func TestLSPTool_UnsupportedCommand(t *testing.T) {
	tool := &LSPTool{}
	input, _ := json.Marshal(map[string]interface{}{
		"command":   "hover",
		"file_path": "test.go",
		"line":      10,
		"character": 5,
	})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Fatal("expected error for unsupported command")
	}
}

func TestLSPTool_DiagnosticsUnsupportedExt(t *testing.T) {
	tool := &LSPTool{}
	input, _ := json.Marshal(map[string]interface{}{
		"command":   "diagnostics",
		"file_path": "test.rs",
	})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.IsError {
		t.Fatalf("expected success for unsupported extension, got error: %s", out.Content)
	}
	if out.Content != "No diagnostics available for .rs files" {
		t.Errorf("unexpected output: %q", out.Content)
	}
}

func TestLSPTool_MissingParams(t *testing.T) {
	tool := &LSPTool{}

	// Missing command
	input, _ := json.Marshal(map[string]interface{}{
		"file_path": "test.go",
	})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Fatal("expected error for missing command")
	}

	// Missing file_path
	input, _ = json.Marshal(map[string]interface{}{
		"command": "symbols",
	})
	out, err = tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Fatal("expected error for missing file_path")
	}
}

// Helper functions for tests
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
