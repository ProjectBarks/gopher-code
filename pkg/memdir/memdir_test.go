package memdir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Source: memdir/memdir.ts

func TestMemdirConstants(t *testing.T) {
	// Source: memdir/memdir.ts:34-38
	if EntrypointName != "MEMORY.md" {
		t.Errorf("EntrypointName = %q, want %q", EntrypointName, "MEMORY.md")
	}
	if MaxEntrypointLines != 200 {
		t.Errorf("MaxEntrypointLines = %d, want 200", MaxEntrypointLines)
	}
	if MaxEntrypointBytes != 25_000 {
		t.Errorf("MaxEntrypointBytes = %d, want 25000", MaxEntrypointBytes)
	}
}

// --- Truncation tests ---
// Source: memdir/memdir.ts:57-103

func TestTruncateEntrypointContent_UnderLimits(t *testing.T) {
	content := "- [Prefs](prefs.md) — user preferences\n- [Style](style.md) — coding style"
	result := TruncateEntrypointContent(content)

	if result.WasLineTruncated || result.WasByteTruncated {
		t.Error("should not be truncated when under limits")
	}
	if result.Content != strings.TrimSpace(content) {
		t.Error("content should be trimmed but unchanged")
	}
	if result.LineCount != 2 {
		t.Errorf("lineCount = %d, want 2", result.LineCount)
	}
}

func TestTruncateEntrypointContent_LineLimitOnly(t *testing.T) {
	// 250 short lines — exceeds 200-line cap but well under byte cap.
	var lines []string
	for i := 0; i < 250; i++ {
		lines = append(lines, fmt.Sprintf("- [E%d](e.md) — short", i))
	}
	content := strings.Join(lines, "\n")
	result := TruncateEntrypointContent(content)

	if !result.WasLineTruncated {
		t.Error("should be line-truncated at 250 lines")
	}
	if result.WasByteTruncated {
		t.Error("should NOT be byte-truncated (short lines)")
	}
	if result.LineCount != 250 {
		t.Errorf("lineCount = %d, want 250", result.LineCount)
	}
	// Warning should mention line count but NOT "index entries are too long"
	if !strings.Contains(result.Content, "250 lines (limit: 200)") {
		t.Errorf("warning should say '250 lines (limit: 200)', got:\n%s",
			result.Content[len(result.Content)-200:])
	}
	if strings.Contains(result.Content, "index entries are too long") {
		t.Error("line-only truncation should NOT mention 'index entries are too long'")
	}
	// Should contain warning boilerplate
	if !strings.Contains(result.Content, "WARNING: MEMORY.md") {
		t.Error("should contain truncation warning")
	}
	if !strings.Contains(result.Content, "Keep index entries to one line under ~200 chars; move detail into topic files.") {
		t.Error("should contain guidance suffix")
	}
}

func TestTruncateEntrypointContent_ByteLimitOnly(t *testing.T) {
	// Few lines but exceeds 25KB — the byte-only path.
	// 5 lines of ~6KB each ≈ 30KB total, well under 200 lines.
	var lines []string
	for i := 0; i < 5; i++ {
		lines = append(lines, strings.Repeat("x", 6000))
	}
	content := strings.Join(lines, "\n")
	result := TruncateEntrypointContent(content)

	if result.WasLineTruncated {
		t.Error("should NOT be line-truncated (only 5 lines)")
	}
	if !result.WasByteTruncated {
		t.Error("should be byte-truncated (>25KB)")
	}
	// Warning must mention "index entries are too long" for byte-only case
	if !strings.Contains(result.Content, "index entries are too long") {
		t.Error("byte-only truncation should mention 'index entries are too long'")
	}
	// Uses FormatFileSize for the byte counts
	if !strings.Contains(result.Content, "KB") {
		t.Error("byte-only warning should use human-readable KB size")
	}
	// Limit shown as formatted size
	if !strings.Contains(result.Content, FormatFileSize(MaxEntrypointBytes)) {
		t.Errorf("should show limit as %s", FormatFileSize(MaxEntrypointBytes))
	}
}

func TestTruncateEntrypointContent_BothLimits(t *testing.T) {
	// 250 lines of 200 chars each ≈ 50KB — exceeds both caps.
	var lines []string
	for i := 0; i < 250; i++ {
		lines = append(lines, strings.Repeat("a", 200))
	}
	content := strings.Join(lines, "\n")
	result := TruncateEntrypointContent(content)

	if !result.WasLineTruncated {
		t.Error("should be line-truncated")
	}
	if !result.WasByteTruncated {
		t.Error("should be byte-truncated")
	}
	// Both: "{n} lines and {formattedSize}"
	if !strings.Contains(result.Content, "250 lines and ") {
		t.Error("both-truncation warning should say '250 lines and <size>'")
	}
	if !strings.Contains(result.Content, "KB") {
		t.Error("both-truncation warning should include formatted byte size")
	}
}

func TestTruncateEntrypointContent_ByteTruncCutsAtNewline(t *testing.T) {
	// Ensure byte truncation doesn't cut mid-line: cuts at last newline
	// before the byte cap.
	line := strings.Repeat("y", 5000) // 5KB per line
	content := strings.Join([]string{line, line, line, line, line, line}, "\n") // 6 lines ~30KB
	result := TruncateEntrypointContent(content)

	// The truncated content (before warning) should end at a newline boundary
	beforeWarning := strings.SplitN(result.Content, "\n\n> WARNING:", 2)[0]
	// Each remaining line should be exactly 5000 chars of 'y'
	for _, l := range strings.Split(beforeWarning, "\n") {
		if len(l) > 0 && l != strings.Repeat("y", 5000) {
			t.Errorf("truncated line has unexpected length %d (expected 0 or 5000)", len(l))
		}
	}
}

// --- FormatFileSize ---

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0 bytes"},
		{512, "512 bytes"},
		{1023, "1023 bytes"},
		{1024, "1KB"},
		{1536, "1.5KB"},
		{25000, "24.4KB"},
		{1048576, "1MB"},
		{1572864, "1.5MB"},
		{1073741824, "1GB"},
	}
	for _, tt := range tests {
		got := FormatFileSize(tt.input)
		if got != tt.expected {
			t.Errorf("FormatFileSize(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- Guidance strings verbatim ---
// Source: memdir/memdir.ts:116-119

func TestDirExistsGuidance_Verbatim(t *testing.T) {
	expected := "This directory already exists — write to it directly with the Write tool (do not run mkdir or check for its existence)."
	if DirExistsGuidance != expected {
		t.Errorf("DirExistsGuidance mismatch:\ngot:  %q\nwant: %q", DirExistsGuidance, expected)
	}
}

func TestDirsExistGuidance_Verbatim(t *testing.T) {
	expected := "Both directories already exist — write to them directly with the Write tool (do not run mkdir or check for their existence)."
	if DirsExistGuidance != expected {
		t.Errorf("DirsExistGuidance mismatch:\ngot:  %q\nwant: %q", DirsExistGuidance, expected)
	}
}

// --- EnsureMemoryDirExists ---

func TestEnsureMemoryDirExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "c", "memory")
	if err := EnsureMemoryDirExists(dir); err != nil {
		t.Fatalf("EnsureMemoryDirExists failed: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
	// Idempotent — calling again should not error
	if err := EnsureMemoryDirExists(dir); err != nil {
		t.Fatalf("second call should be idempotent: %v", err)
	}
}

// --- BuildMemoryLines / BuildMemoryPrompt ---
// Source: memdir/memdir.ts:199-316

func TestBuildMemoryLines_SingleDir(t *testing.T) {
	lines := BuildMemoryLines("auto memory", "/home/user/.claude/memory", nil)
	joined := strings.Join(lines, "\n")

	// Header
	if !strings.Contains(joined, "# auto memory") {
		t.Error("should start with display name header")
	}
	// DirExistsGuidance embedded
	if !strings.Contains(joined, DirExistsGuidance) {
		t.Error("should contain DirExistsGuidance")
	}
	// Memory dir path
	if !strings.Contains(joined, "/home/user/.claude/memory") {
		t.Error("should contain memory dir path")
	}
	// How to save section
	if !strings.Contains(joined, "## How to save memories") {
		t.Error("should contain 'How to save memories' section")
	}
	// MEMORY.md references
	if !strings.Contains(joined, EntrypointName) {
		t.Error("should reference MEMORY.md")
	}
	// Line limit mentioned
	if !strings.Contains(joined, fmt.Sprintf("lines after %d will be truncated", MaxEntrypointLines)) {
		t.Error("should mention line truncation limit")
	}
}

func TestBuildMemoryLines_ExtraGuidelines(t *testing.T) {
	extra := []string{"Custom guideline from cowork."}
	lines := BuildMemoryLines("auto memory", "/mem", extra)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "Custom guideline from cowork.") {
		t.Error("should include extra guidelines")
	}
}

func TestBuildMemoryPrompt_WithContent(t *testing.T) {
	content := "- [Prefs](prefs.md) — user preferences"
	prompt := BuildMemoryPrompt("auto memory", "/mem", nil, content)

	if !strings.Contains(prompt, "## MEMORY.md") {
		t.Error("should contain MEMORY.md section header")
	}
	if !strings.Contains(prompt, "user preferences") {
		t.Error("should contain the entrypoint content")
	}
	// Should NOT show "currently empty"
	if strings.Contains(prompt, "currently empty") {
		t.Error("should NOT say 'currently empty' when content exists")
	}
}

func TestBuildMemoryPrompt_Empty(t *testing.T) {
	prompt := BuildMemoryPrompt("auto memory", "/mem", nil, "")

	if !strings.Contains(prompt, "## MEMORY.md") {
		t.Error("should contain MEMORY.md section header")
	}
	if !strings.Contains(prompt, "currently empty") {
		t.Error("should say 'currently empty' when no content")
	}
}

func TestBuildMemoryPrompt_WhitespaceOnlyContent(t *testing.T) {
	prompt := BuildMemoryPrompt("auto memory", "/mem", nil, "   \n  \n  ")

	if !strings.Contains(prompt, "currently empty") {
		t.Error("whitespace-only content should be treated as empty")
	}
}

func TestBuildMemoryPrompt_TruncatedContent(t *testing.T) {
	// 250 short lines — triggers line truncation
	var lines []string
	for i := 0; i < 250; i++ {
		lines = append(lines, fmt.Sprintf("- [E%d](e.md) — entry", i))
	}
	content := strings.Join(lines, "\n")
	prompt := BuildMemoryPrompt("auto memory", "/mem", nil, content)

	if !strings.Contains(prompt, "WARNING: MEMORY.md") {
		t.Error("should contain truncation warning in prompt")
	}
}

// --- BuildSearchingPastContextSection ---

func TestBuildSearchingPastContextSection(t *testing.T) {
	section := BuildSearchingPastContextSection("/mem", "/project")
	joined := strings.Join(section, "\n")

	if !strings.Contains(joined, "## Searching past context") {
		t.Error("should have section header")
	}
	if !strings.Contains(joined, "/mem") {
		t.Error("should reference memory dir")
	}
	if !strings.Contains(joined, "/project") {
		t.Error("should reference project dir")
	}
	if !strings.Contains(joined, "*.md") {
		t.Error("should search md files in memory dir")
	}
	if !strings.Contains(joined, "*.jsonl") {
		t.Error("should search jsonl files in project dir")
	}
}

// --- Two memory dirs (team + auto) prompt ---

func TestBuildMemoryPrompt_TwoMemoryDirs(t *testing.T) {
	// When team memory is enabled, the combined prompt should reference
	// DirsExistGuidance. For now, test that we can build prompts for two
	// separate directories and that the guidance strings are distinct.
	autoPrompt := BuildMemoryPrompt("auto memory", "/auto", nil, "auto content")
	teamPrompt := BuildMemoryPrompt("team memory", "/team", nil, "team content")

	if !strings.Contains(autoPrompt, "/auto") {
		t.Error("auto prompt should reference auto dir")
	}
	if !strings.Contains(teamPrompt, "/team") {
		t.Error("team prompt should reference team dir")
	}
	if !strings.Contains(autoPrompt, "auto content") {
		t.Error("auto prompt should include auto content")
	}
	if !strings.Contains(teamPrompt, "team content") {
		t.Error("team prompt should include team content")
	}
	// Both should include DirExistsGuidance (singular)
	if !strings.Contains(autoPrompt, DirExistsGuidance) {
		t.Error("auto prompt should contain DirExistsGuidance")
	}
	if !strings.Contains(teamPrompt, DirExistsGuidance) {
		t.Error("team prompt should contain DirExistsGuidance")
	}
}

// --- Warning template tests ---

func TestTruncationWarningTemplate_LineOnly(t *testing.T) {
	// Line-only: "{n} lines (limit: 200)"
	var lines []string
	for i := 0; i < 210; i++ {
		lines = append(lines, "short")
	}
	result := TruncateEntrypointContent(strings.Join(lines, "\n"))
	if !strings.Contains(result.Content, "210 lines (limit: 200)") {
		t.Errorf("expected '210 lines (limit: 200)' in warning, got:\n%s", lastN(result.Content, 200))
	}
}

func TestTruncationWarningTemplate_ByteOnly(t *testing.T) {
	// Byte-only: "{size} (limit: {maxSize}) — index entries are too long"
	content := strings.Repeat("x", 30_000)
	result := TruncateEntrypointContent(content)
	expected := fmt.Sprintf("%s (limit: %s) — index entries are too long",
		FormatFileSize(30_000), FormatFileSize(MaxEntrypointBytes))
	if !strings.Contains(result.Content, expected) {
		t.Errorf("expected %q in warning, got:\n%s", expected, lastN(result.Content, 300))
	}
}

func TestTruncationWarningTemplate_Both(t *testing.T) {
	// Both: "{n} lines and {size}"
	var lines []string
	for i := 0; i < 250; i++ {
		lines = append(lines, strings.Repeat("b", 200))
	}
	content := strings.Join(lines, "\n")
	result := TruncateEntrypointContent(content)
	byteCount := len(strings.TrimSpace(content))
	expected := fmt.Sprintf("250 lines and %s", FormatFileSize(byteCount))
	if !strings.Contains(result.Content, expected) {
		t.Errorf("expected %q in warning, got:\n%s", expected, lastN(result.Content, 300))
	}
}

func TestTruncationWarningSuffix(t *testing.T) {
	// All truncation warnings should end with the standard suffix
	var lines []string
	for i := 0; i < 210; i++ {
		lines = append(lines, "short line")
	}
	result := TruncateEntrypointContent(strings.Join(lines, "\n"))
	suffix := "Only part of it was loaded. Keep index entries to one line under ~200 chars; move detail into topic files."
	if !strings.Contains(result.Content, suffix) {
		t.Error("warning should contain standard suffix")
	}
}

// lastN returns the last n characters of s (or all of s if shorter).
func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
