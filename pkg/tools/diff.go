package tools

import (
	"fmt"
	"strings"
)

// --- Structured diff types ---
//
// Mirrors Claude Code's StructuredPatchHunk shape
// (source/src/components/StructuredDiffList.tsx). Tools attach these to
// ToolOutput.Display so the UI can render a rich diff without re-parsing
// a unified-diff text blob.

// DiffHunk is one contiguous change region in a unified diff. Each entry
// in Lines is prefixed with one of " ", "+", or "-".
type DiffHunk struct {
	OldStart int      // 1-based line number in the old file
	OldLines int      // number of lines covered in the old file
	NewStart int      // 1-based line number in the new file
	NewLines int      // number of lines covered in the new file
	Lines    []string // prefixed lines (space/plus/minus + content)
}

// DiffDisplay is the structured payload that Edit/Write tools attach to
// ToolOutput.Display. UI renderers type-switch on this to draw a colored
// diff block instead of plain text.
type DiffDisplay struct {
	FilePath string
	Hunks    []DiffHunk
}

// isToolDisplay is a marker method so DiffDisplay satisfies any future
// ToolDisplay interface without breaking existing code.
func (DiffDisplay) isToolDisplay() {}

// ComputeDiffHunks produces a structured unified-diff hunk set from two
// string contents. It identifies a single contiguous change range
// (first/last differing lines) and emits one hunk with 3 lines of
// context on each side. Returns nil when contents are identical.
func ComputeDiffHunks(oldContent, newContent string) []DiffHunk {
	if oldContent == newContent {
		return nil
	}
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// First differing line.
	startOld, startNew := 0, 0
	for startOld < len(oldLines) && startNew < len(newLines) &&
		oldLines[startOld] == newLines[startNew] {
		startOld++
		startNew++
	}
	// Last differing line (scanning from the end).
	endOld := len(oldLines) - 1
	endNew := len(newLines) - 1
	for endOld >= startOld && endNew >= startNew &&
		oldLines[endOld] == newLines[endNew] {
		endOld--
		endNew--
	}

	const ctx = 3
	hunkStartOld := startOld - ctx
	if hunkStartOld < 0 {
		hunkStartOld = 0
	}
	hunkStartNew := startNew - ctx
	if hunkStartNew < 0 {
		hunkStartNew = 0
	}
	hunkEndOld := endOld + 1 + ctx
	if hunkEndOld > len(oldLines) {
		hunkEndOld = len(oldLines)
	}
	hunkEndNew := endNew + 1 + ctx
	if hunkEndNew > len(newLines) {
		hunkEndNew = len(newLines)
	}

	lines := make([]string, 0,
		(startOld-hunkStartOld)+(endOld-startOld+1)+(endNew-startNew+1)+(hunkEndOld-endOld-1))
	for i := hunkStartOld; i < startOld; i++ {
		lines = append(lines, " "+oldLines[i])
	}
	for i := startOld; i <= endOld; i++ {
		lines = append(lines, "-"+oldLines[i])
	}
	for i := startNew; i <= endNew; i++ {
		lines = append(lines, "+"+newLines[i])
	}
	for i := endOld + 1; i < hunkEndOld; i++ {
		lines = append(lines, " "+oldLines[i])
	}

	return []DiffHunk{{
		OldStart: hunkStartOld + 1,
		OldLines: hunkEndOld - hunkStartOld,
		NewStart: hunkStartNew + 1,
		NewLines: hunkEndNew - hunkStartNew,
		Lines:    lines,
	}}
}

// BuildUnifiedDiff produces a unified-diff string of oldContent vs
// newContent. Kept as a thin wrapper over ComputeDiffHunks for callers
// (like tests or the print-mode CLI) that want the raw text form.
func BuildUnifiedDiff(oldContent, newContent, filePath string) string {
	hunks := ComputeDiffHunks(oldContent, newContent)
	if len(hunks) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n", filePath)
	fmt.Fprintf(&sb, "+++ b/%s\n", filePath)
	for _, h := range hunks {
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n",
			h.OldStart, h.OldLines, h.NewStart, h.NewLines)
		for _, line := range h.Lines {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
