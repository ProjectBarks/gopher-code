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

// TestFileReadTool_WiredIntoBinary verifies FileReadTool executes through
// the real code path from the binary.
func TestFileReadTool_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	read := registry.Get("Read")
	if read == nil {
		t.Fatal("Read tool should be registered")
	}

	// Verify schema mentions key parameters.
	schema := string(read.InputSchema())
	for _, phrase := range []string{"file_path", "offset", "limit"} {
		if !strings.Contains(schema, phrase) {
			t.Errorf("InputSchema() should contain %q", phrase)
		}
	}

	// Execute a real file read.
	dir := t.TempDir()
	testFile := filepath.Join(dir, "readme.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	tc := &tools.ToolContext{CWD: dir}
	input, _ := json.Marshal(map[string]any{
		"file_path": testFile,
		"limit":     3,
	})

	out, err := read.Execute(context.Background(), tc, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil {
		t.Fatal("Execute() returned nil output")
	}
	// Output should contain lines from the file.
	if !strings.Contains(out.Content, "line1") {
		t.Errorf("output should contain 'line1', got %q", out.Content)
	}
}
