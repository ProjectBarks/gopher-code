// Package memdir implements the core memory-directory subsystem: entrypoint
// truncation, guidance strings, directory bootstrap, and prompt building.
// Source: memdir/memdir.ts
package memdir

import (
	"fmt"
	"os"
	"strings"
)

// MEMORY.md index constants.
// Source: memdir/memdir.ts:34-38
const (
	EntrypointName     = "MEMORY.md"
	MaxEntrypointLines = 200
	MaxEntrypointBytes = 25_000
)

// EntrypointTruncation describes the result of truncating MEMORY.md.
// Source: memdir/memdir.ts:41-47
type EntrypointTruncation struct {
	Content          string
	LineCount        int
	ByteCount        int
	WasLineTruncated bool
	WasByteTruncated bool
}

// FormatFileSize formats a byte count to a human-readable string (KB, MB, GB).
// Matches the TS formatFileSize in src/utils/format.ts.
func FormatFileSize(sizeInBytes int) string {
	kb := float64(sizeInBytes) / 1024
	if kb < 1 {
		return fmt.Sprintf("%d bytes", sizeInBytes)
	}
	if kb < 1024 {
		s := fmt.Sprintf("%.1f", kb)
		s = strings.TrimSuffix(s, ".0")
		return s + "KB"
	}
	mb := kb / 1024
	if mb < 1024 {
		s := fmt.Sprintf("%.1f", mb)
		s = strings.TrimSuffix(s, ".0")
		return s + "MB"
	}
	gb := mb / 1024
	s := fmt.Sprintf("%.1f", gb)
	s = strings.TrimSuffix(s, ".0")
	return s + "GB"
}

// TruncateEntrypointContent truncates MEMORY.md to line and byte caps.
// Line-truncates first (natural boundary), then byte-truncates at the last
// newline before the cap so we don't cut mid-line.
//
// The byte check uses ORIGINAL bytes (long lines are the failure mode the
// byte cap targets; post-line-truncation size would understate the warning).
// Source: memdir/memdir.ts:57-103
func TruncateEntrypointContent(raw string) EntrypointTruncation {
	trimmed := strings.TrimSpace(raw)
	lines := strings.Split(trimmed, "\n")
	lineCount := len(lines)
	byteCount := len(trimmed)

	wasLineTruncated := lineCount > MaxEntrypointLines
	wasByteTruncated := byteCount > MaxEntrypointBytes

	if !wasLineTruncated && !wasByteTruncated {
		return EntrypointTruncation{
			Content:          trimmed,
			LineCount:        lineCount,
			ByteCount:        byteCount,
			WasLineTruncated: false,
			WasByteTruncated: false,
		}
	}

	truncated := trimmed
	if wasLineTruncated {
		truncated = strings.Join(lines[:MaxEntrypointLines], "\n")
	}
	if len(truncated) > MaxEntrypointBytes {
		cutAt := strings.LastIndex(truncated[:MaxEntrypointBytes], "\n")
		if cutAt > 0 {
			truncated = truncated[:cutAt]
		} else {
			truncated = truncated[:MaxEntrypointBytes]
		}
	}

	var reason string
	switch {
	case wasByteTruncated && !wasLineTruncated:
		reason = fmt.Sprintf("%s (limit: %s) — index entries are too long",
			FormatFileSize(byteCount), FormatFileSize(MaxEntrypointBytes))
	case wasLineTruncated && !wasByteTruncated:
		reason = fmt.Sprintf("%d lines (limit: %d)", lineCount, MaxEntrypointLines)
	default:
		reason = fmt.Sprintf("%d lines and %s", lineCount, FormatFileSize(byteCount))
	}

	return EntrypointTruncation{
		Content: truncated +
			"\n\n> WARNING: " + EntrypointName + " is " + reason +
			". Only part of it was loaded. Keep index entries to one line under ~200 chars; move detail into topic files.",
		LineCount:        lineCount,
		ByteCount:        byteCount,
		WasLineTruncated: wasLineTruncated,
		WasByteTruncated: wasByteTruncated,
	}
}

// Guidance strings appended to memory directory prompt lines.
// Shipped because Claude was burning turns on ls/mkdir -p before writing.
// Source: memdir/memdir.ts:116-119
const DirExistsGuidance = "This directory already exists — write to it directly with the Write tool (do not run mkdir or check for its existence)."
const DirsExistGuidance = "Both directories already exist — write to them directly with the Write tool (do not run mkdir or check for their existence)."

// EnsureMemoryDirExists creates a memory directory (and parents) if needed.
// Idempotent — called once per session so the model can write without
// checking existence first. Source: memdir/memdir.ts:129-147
func EnsureMemoryDirExists(memoryDir string) error {
	return os.MkdirAll(memoryDir, 0755)
}

// BuildMemoryLines builds the typed-memory behavioral instructions (without
// MEMORY.md content). Returns the lines that form the memory section of the
// system prompt.
// Source: memdir/memdir.ts:199-266
func BuildMemoryLines(displayName, memoryDir string, extraGuidelines []string) []string {
	lines := []string{
		"# " + displayName,
		"",
		fmt.Sprintf("You have a persistent, file-based memory system at `%s`. %s", memoryDir, DirExistsGuidance),
		"",
		"You should build up this memory system over time so that future conversations can have a complete picture of who the user is, how they'd like to collaborate with you, what behaviors to avoid or repeat, and the context behind the work the user gives you.",
		"",
		"If the user explicitly asks you to remember something, save it immediately as whichever type fits best. If they ask you to forget something, find and remove the relevant entry.",
		"",
	}

	// How to save section (with index)
	lines = append(lines,
		"## How to save memories",
		"",
		"Saving a memory is a two-step process:",
		"",
		fmt.Sprintf("**Step 2** — add a pointer to that file in `%s`. `%s` is an index, not a memory — each entry should be one line, under ~150 characters: `- [Title](file.md) — one-line hook`. It has no frontmatter. Never write memory content directly into `%s`.",
			EntrypointName, EntrypointName, EntrypointName),
		"",
		fmt.Sprintf("- `%s` is always loaded into your conversation context — lines after %d will be truncated, so keep the index concise",
			EntrypointName, MaxEntrypointLines),
		"- Keep the name, description, and type fields in memory files up-to-date with the content",
		"- Organize memory semantically by topic, not chronologically",
		"- Update or remove memories that turn out to be wrong or outdated",
		"- Do not write duplicate memories. First check if there is an existing memory you can update before writing a new one.",
		"",
	)

	// Memory and other forms of persistence
	lines = append(lines,
		"## Memory and other forms of persistence",
		"Memory is one of several persistence mechanisms available to you as you assist the user in a given conversation. The distinction is often that memory can be recalled in future conversations and should not be used for persisting information that is only useful within the scope of the current conversation.",
		"- When to use or update a plan instead of memory: If you are about to start a non-trivial implementation task and would like to reach alignment with the user on your approach you should use a Plan rather than saving this information to memory. Similarly, if you already have a plan within the conversation and you have changed your approach persist that change by updating the plan rather than saving a memory.",
		"- When to use or update tasks instead of memory: When you need to break your work in current conversation into discrete steps or keep track of your progress use tasks instead of saving to memory. Tasks are great for persisting information about the work that needs to be done in the current conversation, but memory should be reserved for information that will be useful in future conversations.",
		"",
	)

	if len(extraGuidelines) > 0 {
		lines = append(lines, extraGuidelines...)
		lines = append(lines, "")
	}

	return lines
}

// BuildMemoryPrompt builds the full memory prompt including MEMORY.md content.
// Source: memdir/memdir.ts:272-316
func BuildMemoryPrompt(displayName, memoryDir string, extraGuidelines []string, entrypointContent string) string {
	lines := BuildMemoryLines(displayName, memoryDir, extraGuidelines)

	trimmed := strings.TrimSpace(entrypointContent)
	if trimmed != "" {
		t := TruncateEntrypointContent(entrypointContent)
		lines = append(lines, "## "+EntrypointName, "", t.Content)
	} else {
		lines = append(lines,
			"## "+EntrypointName,
			"",
			fmt.Sprintf("Your %s is currently empty. When you save new memories, they will appear here.", EntrypointName),
		)
	}

	return strings.Join(lines, "\n")
}

// BuildSearchingPastContextSection builds the "Searching past context"
// section lines. Source: memdir/memdir.ts:375-407
func BuildSearchingPastContextSection(autoMemDir, projectDir string) []string {
	return []string{
		"## Searching past context",
		"",
		"When looking for past context:",
		"1. Search topic files in your memory directory:",
		"```",
		fmt.Sprintf("Grep with pattern=\"<search term>\" path=\"%s\" glob=\"*.md\"", autoMemDir),
		"```",
		"2. Session transcript logs (last resort — large files, slow):",
		"```",
		fmt.Sprintf("Grep with pattern=\"<search term>\" path=\"%s/\" glob=\"*.jsonl\"", projectDir),
		"```",
		"Use narrow search terms (error messages, file paths, function names) rather than broad keywords.",
		"",
	}
}
