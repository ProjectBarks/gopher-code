package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// TestFileEditTool_WiredIntoBinary verifies the FileEditTool is registered
// and executes through the real code path.
func TestFileEditTool_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	edit := registry.Get("Edit")
	if edit == nil {
		t.Fatal("Edit tool should be registered")
	}
	if edit.Name() != "Edit" {
		t.Errorf("Name() = %q, want Edit", edit.Name())
	}

	// Verify prompt contains key phrases from the TS source.
	prompt := tools.GetToolPrompt(edit)
	for _, phrase := range []string{"old_string", "new_string", "replace_all"} {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("Prompt() should contain %q", phrase)
		}
	}

	// Execute a real edit on a temp file.
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\ngoodbye world\n"), 0644)

	rfs := tools.NewReadFileState()
	rfs.Record(testFile, "hello world\ngoodbye world\n", false)
	tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
	input, _ := json.Marshal(map[string]string{
		"file_path":  testFile,
		"old_string": "hello",
		"new_string": "greetings",
	})

	out, err := edit.Execute(context.Background(), tc, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil {
		t.Fatal("Execute() returned nil output")
	}

	// Verify the file was actually modified.
	content, _ := os.ReadFile(testFile)
	if !strings.Contains(string(content), "greetings world") {
		t.Errorf("file should contain 'greetings world', got %q", string(content))
	}
}
