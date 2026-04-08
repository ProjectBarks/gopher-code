package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/memdir"
	"github.com/projectbarks/gopher-code/pkg/prompt"
)

// stubSelector implements memdir.SelectorProvider for integration testing.
type stubSelector struct {
	response string
	err      error
}

func (s *stubSelector) SelectMemories(_ context.Context, _, _ string) (string, error) {
	return s.response, s.err
}

// TestMemdirRelevance_WiredIntoBinary verifies that the memdir relevance
// selector types are reachable from the binary and work through the same
// construction path used in main.go.
func TestMemdirRelevance_WiredIntoBinary(t *testing.T) {
	// Same construction as main.go.
	selector := &memdir.MemoryRelevanceSelector{}

	// Without a provider, Select returns nil (safe no-op).
	got := selector.Select(context.Background(), "query", nil, nil)
	if got != nil {
		t.Fatalf("expected nil without provider, got %v", got)
	}

	// Wire a stub provider and test the full path.
	sel := memdir.MemorySelection{SelectedMemories: []string{"go-testing.md"}}
	respJSON, _ := json.Marshal(sel)
	selector.Provider = &stubSelector{response: string(respJSON)}

	memories := []memdir.MemoryHeader{
		{
			Filename:    "go-testing.md",
			Description: "Go testing patterns",
			FilePath:    "/mem/go-testing.md",
			MtimeMs:     1000,
		},
		{
			Filename:    "docker.md",
			Description: "Docker tips",
			FilePath:    "/mem/docker.md",
			MtimeMs:     2000,
		},
	}

	got = selector.Select(context.Background(), "how do I test in Go?", memories, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 selected memory, got %d", len(got))
	}
	if got[0].Path != "/mem/go-testing.md" {
		t.Errorf("expected /mem/go-testing.md, got %s", got[0].Path)
	}

	// AlreadySurfaced should now contain the selected path.
	if _, ok := selector.AlreadySurfaced["/mem/go-testing.md"]; !ok {
		t.Error("AlreadySurfaced should track /mem/go-testing.md after Select")
	}

	// Second call with same memories — go-testing.md should be filtered out.
	got = selector.Select(context.Background(), "another query", memories, nil)
	for _, m := range got {
		if m.Path == "/mem/go-testing.md" {
			t.Error("go-testing.md should have been filtered as already surfaced")
		}
	}
}

// TestMemdirRelevance_ConstantsReachable verifies that relevance constants
// are accessible from the binary.
func TestMemdirRelevance_ConstantsReachable(t *testing.T) {
	if memdir.MaxSelectedMemories != 5 {
		t.Errorf("MaxSelectedMemories = %d, want 5", memdir.MaxSelectedMemories)
	}
	if memdir.MaxSelectorTokens != 256 {
		t.Errorf("MaxSelectorTokens = %d, want 256", memdir.MaxSelectorTokens)
	}
	if !strings.Contains(memdir.SelectMemoriesSystemPrompt, "up to 5") {
		t.Error("SelectMemoriesSystemPrompt missing 5-memory budget")
	}
}

// TestMemdirMemorySection_WiredIntoBuildSystemPrompt verifies that the memory
// section produced by memdir.BuildMemoryPrompt integrates correctly with
// prompt.BuildSystemPrompt, the same path used in main.go.
func TestMemdirMemorySection_WiredIntoBuildSystemPrompt(t *testing.T) {
	// Create a temporary memory directory with a MEMORY.md file.
	memDir := t.TempDir()
	indexPath := filepath.Join(memDir, memdir.EntrypointName)
	indexContent := "- [Go Testing](go-testing.md) — patterns and tips\n"
	if err := os.WriteFile(indexPath, []byte(indexContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build the memory section the same way main.go does.
	section := prompt.SystemPromptSection("memory", func() *string {
		content := ""
		if data, err := os.ReadFile(indexPath); err == nil {
			content = string(data)
		}
		p := memdir.BuildMemoryPrompt("auto memory", memDir, nil, content)
		return &p
	})

	// Build the full system prompt with the memory section.
	cwd := t.TempDir()
	result := prompt.BuildSystemPrompt("base prompt", cwd, "test-model", section)

	// The system prompt should contain the memory section content.
	if !strings.Contains(result, "auto memory") {
		t.Error("system prompt should contain memory display name")
	}
	if !strings.Contains(result, "Go Testing") {
		t.Error("system prompt should contain MEMORY.md index content")
	}
	if !strings.Contains(result, "persistent, file-based memory system") {
		t.Error("system prompt should contain memory behavioural instructions")
	}
}
