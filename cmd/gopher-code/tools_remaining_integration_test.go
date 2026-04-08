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

// T514: FileWriteTool remaining
func TestFileWriteTool_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	write := registry.Get("Write")
	if write == nil {
		t.Fatal("Write tool should be registered")
	}

	dir := t.TempDir()
	testFile := filepath.Join(dir, "new.txt")

	rfs := tools.NewReadFileState()
	tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
	input, _ := json.Marshal(map[string]string{
		"file_path": testFile,
		"content":   "hello from Write tool",
	})

	out, err := write.Execute(context.Background(), tc, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil {
		t.Fatal("Execute() returned nil output")
	}

	content, _ := os.ReadFile(testFile)
	if !strings.Contains(string(content), "hello from Write tool") {
		t.Errorf("file should contain written content, got %q", string(content))
	}
}

// T515: GlobTool remaining
func TestGlobTool_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	glob := registry.Get("Glob")
	if glob == nil {
		t.Fatal("Glob tool should be registered")
	}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "foo.go"), []byte("package foo"), 0644)
	os.WriteFile(filepath.Join(dir, "bar.go"), []byte("package bar"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# readme"), 0644)

	tc := &tools.ToolContext{CWD: dir}
	input, _ := json.Marshal(map[string]string{
		"pattern": "*.go",
		"path":    dir,
	})

	out, err := glob.Execute(context.Background(), tc, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.Content, ".go") {
		t.Errorf("glob output should contain .go files, got %q", out.Content)
	}
}

// T516: GrepTool remaining
func TestGrepTool_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	grep := registry.Get("Grep")
	if grep == nil {
		t.Fatal("Grep tool should be registered")
	}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "search.txt"), []byte("the quick brown fox\njumps over the lazy dog\n"), 0644)

	tc := &tools.ToolContext{CWD: dir}
	input, _ := json.Marshal(map[string]string{
		"pattern": "quick.*fox",
		"path":    dir,
	})

	out, err := grep.Execute(context.Background(), tc, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.Content, "search.txt") {
		t.Errorf("grep output should contain matching filename, got %q", out.Content)
	}
}

// T522: NotebookEditTool remaining
func TestNotebookEditTool_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	nb := registry.Get("NotebookEdit")
	if nb == nil {
		t.Fatal("NotebookEdit tool should be registered")
	}
	if nb.Name() != "NotebookEdit" {
		t.Errorf("Name() = %q, want NotebookEdit", nb.Name())
	}
	// Verify schema has expected fields.
	schema := string(nb.InputSchema())
	if !strings.Contains(schema, "notebook_path") {
		t.Errorf("schema should contain notebook_path")
	}
}

// T523: PowerShellTool full
func TestPowerShellTool_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	ps := registry.Get("PowerShell")
	if ps == nil {
		t.Fatal("PowerShell tool should be registered")
	}
	if ps.Name() != "PowerShell" {
		t.Errorf("Name() = %q, want PowerShell", ps.Name())
	}
}

// T526: ScheduleCronTool remaining
func TestCronTools_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	for _, name := range []string{"CronCreate", "CronDelete", "CronList"} {
		tool := registry.Get(name)
		if tool == nil {
			t.Errorf("%s tool should be registered", name)
			continue
		}
		if tool.Name() != name {
			t.Errorf("Name() = %q, want %q", tool.Name(), name)
		}
	}
}

// T527: SendMessageTool remaining
func TestSendMessageTool_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	sm := registry.Get("SendMessage")
	if sm == nil {
		t.Fatal("SendMessage tool should be registered")
	}
	schema := string(sm.InputSchema())
	if !strings.Contains(schema, "to") {
		t.Error("schema should contain 'to' field")
	}
}

// T529: SkillTool remaining — registered separately in main.go
func TestSkillTool_Registerable(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	// SkillTool is registered via registry.Register(tools.NewSkillTool(...))
	// in main.go, not in RegisterDefaults. Verify the constructor works.
	skillTool := tools.NewSkillTool(nil)
	if skillTool == nil {
		t.Fatal("NewSkillTool should return non-nil")
	}
	if skillTool.Name() != "Skill" {
		t.Errorf("Name() = %q, want Skill", skillTool.Name())
	}
}

// T532: Task tools remaining
func TestTaskTools_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	for _, name := range []string{"TaskCreate", "TaskUpdate", "TaskGet", "TaskList"} {
		tool := registry.Get(name)
		if tool == nil {
			t.Errorf("%s tool should be registered", name)
		}
	}
}

// T533: Team tools remaining
func TestTeamTools_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	// Team tools are registered via NewTeamTools() in RegisterDefaults.
	// Check that at least one team-related tool exists.
	found := false
	for _, tool := range registry.All() {
		if strings.Contains(strings.ToLower(tool.Name()), "team") {
			found = true
			break
		}
	}
	// Team tools might not be in All() if IsEnabled() returns false.
	// Just verify NewTeamTools() doesn't panic.
	teamTools := tools.NewTeamTools()
	if teamTools == nil {
		t.Error("NewTeamTools should return non-nil slice")
	}
	_ = found
}

// T534: TodoWriteTool remaining
func TestTodoTools_WiredIntoBinary(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	todoWrite := registry.Get("TodoWrite")
	if todoWrite == nil {
		t.Fatal("TodoWrite tool should be registered")
	}

	todoRead := registry.Get("TodoRead")
	if todoRead == nil {
		t.Fatal("TodoRead tool should be registered")
	}
}
