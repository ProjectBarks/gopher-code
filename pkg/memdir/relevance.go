// Package memdir implements the memory directory system for selecting and
// managing memory files used to augment Claude Code's context.
package memdir

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// Source: src/memdir/findRelevantMemories.ts

// SelectMemoriesSystemPrompt is the verbatim system prompt used for the
// LLM-based memory relevance selector.
// Source: src/memdir/findRelevantMemories.ts:18-24
const SelectMemoriesSystemPrompt = `You are selecting memories that will be useful to Claude Code as it processes a user's query. You will be given the user's query and a list of available memory files with their filenames and descriptions.

Return a list of filenames for the memories that will clearly be useful to Claude Code as it processes the user's query (up to 5). Only include memories that you are certain will be helpful based on their name and description.
- If you are unsure if a memory will be useful in processing the user's query, then do not include it in your list. Be selective and discerning.
- If there are no memories in the list that would clearly be useful, feel free to return an empty list.
- If a list of recently-used tools is provided, do not select memories that are usage reference or API documentation for those tools (Claude Code is already exercising them). DO still select memories containing warnings, gotchas, or known issues about those tools — active use is exactly when those matter.
`

// MaxSelectedMemories is the budget for the selector: at most 5 memories per query.
const MaxSelectedMemories = 5

// MaxSelectorTokens is the max_tokens budget for the Sonnet selector call.
const MaxSelectorTokens = 256

// RelevantMemory is a selected memory file with its modification time threaded
// through so callers can surface freshness without a second stat.
// Source: src/memdir/findRelevantMemories.ts:13-16
type RelevantMemory struct {
	Path    string  `json:"path"`
	MtimeMs float64 `json:"mtimeMs"`
}

// MemoryHeader describes a scanned memory file's metadata.
// This mirrors the TS MemoryHeader from memoryScan.ts.
type MemoryHeader struct {
	Filename    string  // e.g. "go-testing.md"
	Description string  // from YAML frontmatter
	FilePath    string  // absolute path on disk
	MtimeMs     float64 // modification time in ms since epoch
}

// MemorySelection is the JSON schema output from the LLM selector.
type MemorySelection struct {
	SelectedMemories []string `json:"selected_memories"`
}

// SelectorProvider abstracts the LLM call so the relevance logic can be
// tested without a real provider. Implementations should honour ctx
// cancellation and return the raw JSON text of the MemorySelection.
type SelectorProvider interface {
	SelectMemories(ctx context.Context, system string, userMessage string) (string, error)
}

// FindRelevantMemories scans memory files, filters out already-surfaced paths,
// and asks the LLM to pick up to 5 relevant memories for the given query.
//
// Returns selected paths + mtimes. Returns nil (not error) when cancelled,
// when no memories are available, or when the LLM call fails (best-effort).
// Source: src/memdir/findRelevantMemories.ts:39-75
func FindRelevantMemories(
	ctx context.Context,
	provider SelectorProvider,
	query string,
	memories []MemoryHeader,
	recentTools []string,
	alreadySurfaced map[string]struct{},
) []RelevantMemory {
	// Filter out already-surfaced paths before the LLM call so the 5-slot
	// budget is spent on fresh candidates.
	filtered := FilterAlreadySurfaced(memories, alreadySurfaced)
	if len(filtered) == 0 {
		return nil
	}

	// Check context before making LLM call.
	if ctx.Err() != nil {
		return nil
	}

	userMsg := BuildUserMessage(query, filtered, recentTools)

	raw, err := provider.SelectMemories(ctx, SelectMemoriesSystemPrompt, userMsg)
	if err != nil {
		if ctx.Err() != nil {
			return nil // silent on cancellation
		}
		slog.Warn(fmt.Sprintf("[memdir] selectRelevantMemories failed: %v", err))
		return nil
	}

	selected, err := ParseAndValidateSelection(raw, filtered)
	if err != nil {
		slog.Warn(fmt.Sprintf("[memdir] selectRelevantMemories parse failed: %v", err))
		return nil
	}

	return selected
}

// FilterAlreadySurfaced removes memories whose FilePath is in the
// alreadySurfaced set. This saves the 5-slot budget for fresh candidates.
func FilterAlreadySurfaced(memories []MemoryHeader, alreadySurfaced map[string]struct{}) []MemoryHeader {
	if len(alreadySurfaced) == 0 {
		return memories
	}
	out := make([]MemoryHeader, 0, len(memories))
	for _, m := range memories {
		if _, skip := alreadySurfaced[m.FilePath]; !skip {
			out = append(out, m)
		}
	}
	return out
}

// BuildUserMessage constructs the user message for the selector LLM call.
// Format: "Query: {query}\n\nAvailable memories:\n{manifest}{toolsSection}"
// Source: src/memdir/findRelevantMemories.ts:105-106
func BuildUserMessage(query string, memories []MemoryHeader, recentTools []string) string {
	manifest := FormatMemoryManifest(memories)

	var toolsSection string
	if len(recentTools) > 0 {
		toolsSection = "\n\nRecently used tools: " + strings.Join(recentTools, ", ")
	}

	return fmt.Sprintf("Query: %s\n\nAvailable memories:\n%s%s", query, manifest, toolsSection)
}

// FormatMemoryManifest formats memory headers into a manifest string for the
// LLM selector. Each entry is "- {filename}: {description}".
func FormatMemoryManifest(memories []MemoryHeader) string {
	var sb strings.Builder
	for _, m := range memories {
		sb.WriteString("- ")
		sb.WriteString(m.Filename)
		if m.Description != "" {
			sb.WriteString(": ")
			sb.WriteString(m.Description)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ParseAndValidateSelection parses the raw JSON from the LLM, validates
// filenames against the candidate set, and enforces the 5-memory max.
func ParseAndValidateSelection(raw string, candidates []MemoryHeader) ([]RelevantMemory, error) {
	var sel MemorySelection
	if err := json.Unmarshal([]byte(raw), &sel); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}

	// Build lookup by filename.
	byFilename := make(map[string]MemoryHeader, len(candidates))
	for _, m := range candidates {
		byFilename[m.Filename] = m
	}

	// Filter to valid filenames and enforce max budget.
	var result []RelevantMemory
	for _, fname := range sel.SelectedMemories {
		if m, ok := byFilename[fname]; ok {
			result = append(result, RelevantMemory{
				Path:    m.FilePath,
				MtimeMs: m.MtimeMs,
			})
		}
		if len(result) >= MaxSelectedMemories {
			break
		}
	}

	return result, nil
}
