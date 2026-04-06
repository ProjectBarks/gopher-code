package memdir

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// mockProvider implements SelectorProvider for testing.
type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) SelectMemories(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func makeMemories(names ...string) []MemoryHeader {
	out := make([]MemoryHeader, len(names))
	for i, n := range names {
		out[i] = MemoryHeader{
			Filename:    n,
			Description: "desc for " + n,
			FilePath:    "/mem/" + n,
			MtimeMs:     float64(1000 * (i + 1)),
		}
	}
	return out
}

func selectionJSON(filenames ...string) string {
	sel := MemorySelection{SelectedMemories: filenames}
	b, _ := json.Marshal(sel)
	return string(b)
}

// --- FilterAlreadySurfaced ---

func TestFilterAlreadySurfaced_NilSet(t *testing.T) {
	memories := makeMemories("a.md", "b.md")
	got := FilterAlreadySurfaced(memories, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestFilterAlreadySurfaced_FiltersMatching(t *testing.T) {
	memories := makeMemories("a.md", "b.md", "c.md")
	surfaced := map[string]struct{}{"/mem/b.md": {}}
	got := FilterAlreadySurfaced(memories, surfaced)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	for _, m := range got {
		if m.Filename == "b.md" {
			t.Fatal("b.md should have been filtered")
		}
	}
}

func TestFilterAlreadySurfaced_AllFiltered(t *testing.T) {
	memories := makeMemories("a.md")
	surfaced := map[string]struct{}{"/mem/a.md": {}}
	got := FilterAlreadySurfaced(memories, surfaced)
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

// --- BuildUserMessage ---

func TestBuildUserMessage_NoTools(t *testing.T) {
	memories := makeMemories("go-testing.md")
	msg := BuildUserMessage("how do I test?", memories, nil)
	if !strings.HasPrefix(msg, "Query: how do I test?") {
		t.Fatal("missing query prefix")
	}
	if !strings.Contains(msg, "Available memories:") {
		t.Fatal("missing manifest header")
	}
	if !strings.Contains(msg, "- go-testing.md: desc for go-testing.md") {
		t.Fatal("missing manifest entry")
	}
	if strings.Contains(msg, "Recently used tools") {
		t.Fatal("tools section should be absent")
	}
}

func TestBuildUserMessage_WithTools(t *testing.T) {
	memories := makeMemories("a.md")
	msg := BuildUserMessage("query", memories, []string{"bash", "read"})
	if !strings.Contains(msg, "\n\nRecently used tools: bash, read") {
		t.Fatal("missing tools section")
	}
}

// --- FormatMemoryManifest ---

func TestFormatMemoryManifest_Format(t *testing.T) {
	memories := []MemoryHeader{
		{Filename: "a.md", Description: "alpha"},
		{Filename: "b.md", Description: ""},
	}
	got := FormatMemoryManifest(memories)
	if !strings.Contains(got, "- a.md: alpha\n") {
		t.Fatalf("bad format for a.md: %q", got)
	}
	// No description → no colon.
	if !strings.Contains(got, "- b.md\n") {
		t.Fatalf("bad format for b.md (no desc): %q", got)
	}
}

// --- ParseAndValidateSelection ---

func TestParseAndValidateSelection_ValidSubset(t *testing.T) {
	candidates := makeMemories("a.md", "b.md", "c.md")
	raw := selectionJSON("b.md", "c.md")
	got, err := ParseAndValidateSelection(raw, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Path != "/mem/b.md" || got[1].Path != "/mem/c.md" {
		t.Fatalf("wrong paths: %+v", got)
	}
}

func TestParseAndValidateSelection_InvalidFilenamesDropped(t *testing.T) {
	candidates := makeMemories("a.md")
	raw := selectionJSON("bogus.md", "a.md", "also-bogus.md")
	got, err := ParseAndValidateSelection(raw, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 (only valid), got %d", len(got))
	}
	if got[0].Path != "/mem/a.md" {
		t.Fatalf("wrong path: %s", got[0].Path)
	}
}

func TestParseAndValidateSelection_MaxFiveCap(t *testing.T) {
	names := make([]string, 8)
	for i := range names {
		names[i] = fmt.Sprintf("mem%d.md", i)
	}
	candidates := makeMemories(names...)
	raw := selectionJSON(names...) // all 8
	got, err := ParseAndValidateSelection(raw, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxSelectedMemories {
		t.Fatalf("expected %d (max), got %d", MaxSelectedMemories, len(got))
	}
}

func TestParseAndValidateSelection_EmptySelection(t *testing.T) {
	candidates := makeMemories("a.md")
	raw := selectionJSON() // empty list
	got, err := ParseAndValidateSelection(raw, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

func TestParseAndValidateSelection_BadJSON(t *testing.T) {
	candidates := makeMemories("a.md")
	_, err := ParseAndValidateSelection("not json", candidates)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestParseAndValidateSelection_MtimeThreaded(t *testing.T) {
	candidates := makeMemories("a.md")
	candidates[0].MtimeMs = 42.0
	raw := selectionJSON("a.md")
	got, err := ParseAndValidateSelection(raw, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].MtimeMs != 42.0 {
		t.Fatalf("mtime not threaded: %f", got[0].MtimeMs)
	}
}

// --- FindRelevantMemories integration ---

func TestFindRelevantMemories_EmptyMemories(t *testing.T) {
	provider := &mockProvider{response: selectionJSON()}
	got := FindRelevantMemories(context.Background(), provider, "query", nil, nil, nil)
	if got != nil {
		t.Fatalf("expected nil for empty memories, got %+v", got)
	}
}

func TestFindRelevantMemories_AllSurfaced(t *testing.T) {
	memories := makeMemories("a.md")
	surfaced := map[string]struct{}{"/mem/a.md": {}}
	provider := &mockProvider{response: selectionJSON("a.md")}
	got := FindRelevantMemories(context.Background(), provider, "query", memories, nil, surfaced)
	if got != nil {
		t.Fatalf("expected nil when all surfaced, got %+v", got)
	}
}

func TestFindRelevantMemories_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	memories := makeMemories("a.md")
	provider := &mockProvider{response: selectionJSON("a.md")}
	got := FindRelevantMemories(ctx, provider, "query", memories, nil, nil)
	if got != nil {
		t.Fatalf("expected nil for cancelled context, got %+v", got)
	}
}

func TestFindRelevantMemories_ProviderError(t *testing.T) {
	memories := makeMemories("a.md")
	provider := &mockProvider{err: fmt.Errorf("network down")}
	got := FindRelevantMemories(context.Background(), provider, "query", memories, nil, nil)
	if got != nil {
		t.Fatalf("expected nil on provider error, got %+v", got)
	}
}

func TestFindRelevantMemories_HappyPath(t *testing.T) {
	memories := makeMemories("a.md", "b.md", "c.md")
	provider := &mockProvider{response: selectionJSON("a.md", "c.md")}
	got := FindRelevantMemories(context.Background(), provider, "query", memories, nil, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Path != "/mem/a.md" || got[1].Path != "/mem/c.md" {
		t.Fatalf("wrong paths: %+v", got)
	}
}

func TestFindRelevantMemories_SurfacedFiltered_BeforeLLM(t *testing.T) {
	memories := makeMemories("a.md", "b.md", "c.md")
	surfaced := map[string]struct{}{"/mem/b.md": {}}
	// Provider returns b.md but it should already have been filtered
	// from candidates, so it won't match.
	provider := &mockProvider{response: selectionJSON("a.md", "b.md")}
	got := FindRelevantMemories(context.Background(), provider, "query", memories, nil, surfaced)
	if len(got) != 1 {
		t.Fatalf("expected 1 (b.md filtered), got %d", len(got))
	}
	if got[0].Path != "/mem/a.md" {
		t.Fatalf("wrong path: %s", got[0].Path)
	}
}

// --- Prompt verbatim check ---

func TestSelectMemoriesSystemPrompt_Verbatim(t *testing.T) {
	// The prompt must start and end with the expected content.
	if !strings.HasPrefix(SelectMemoriesSystemPrompt, "You are selecting memories") {
		t.Fatal("prompt does not start with expected text")
	}
	if !strings.Contains(SelectMemoriesSystemPrompt, "up to 5") {
		t.Fatal("prompt missing 5-memory budget mention")
	}
	if !strings.Contains(SelectMemoriesSystemPrompt, "warnings, gotchas, or known issues") {
		t.Fatal("prompt missing tool-gotcha instruction")
	}
}

// --- Constants ---

func TestConstants(t *testing.T) {
	if MaxSelectedMemories != 5 {
		t.Fatalf("MaxSelectedMemories should be 5, got %d", MaxSelectedMemories)
	}
	if MaxSelectorTokens != 256 {
		t.Fatalf("MaxSelectorTokens should be 256, got %d", MaxSelectorTokens)
	}
}
