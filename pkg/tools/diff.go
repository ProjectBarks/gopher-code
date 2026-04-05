package tools

import (
	"fmt"
	"strings"
)

// BuildUnifiedDiff produces a unified diff of oldContent vs newContent with
// 3 lines of context around the changed region. Returns "" when the two
// contents are identical.
//
// The diff identifies a single contiguous change range (the first and last
// differing lines) and emits one hunk covering that range. This keeps diffs
// tight for the common case of Edit tool (single string replacement) while
// remaining a valid unified diff for multi-change cases (replace_all,
// large Write operations).
func BuildUnifiedDiff(oldContent, newContent, filePath string) string {
	if oldContent == newContent {
		return ""
	}
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Find first differing line.
	startOld, startNew := 0, 0
	for startOld < len(oldLines) && startNew < len(newLines) &&
		oldLines[startOld] == newLines[startNew] {
		startOld++
		startNew++
	}
	// Find last differing line (scanning from the end).
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

	oldCount := hunkEndOld - hunkStartOld
	newCount := hunkEndNew - hunkStartNew

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n", filePath)
	fmt.Fprintf(&sb, "+++ b/%s\n", filePath)
	fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n",
		hunkStartOld+1, oldCount, hunkStartNew+1, newCount)

	// Leading context.
	for i := hunkStartOld; i < startOld; i++ {
		sb.WriteString(" ")
		sb.WriteString(oldLines[i])
		sb.WriteString("\n")
	}
	// Removals.
	for i := startOld; i <= endOld; i++ {
		sb.WriteString("-")
		sb.WriteString(oldLines[i])
		sb.WriteString("\n")
	}
	// Additions.
	for i := startNew; i <= endNew; i++ {
		sb.WriteString("+")
		sb.WriteString(newLines[i])
		sb.WriteString("\n")
	}
	// Trailing context.
	for i := endOld + 1; i < hunkEndOld; i++ {
		sb.WriteString(" ")
		sb.WriteString(oldLines[i])
		sb.WriteString("\n")
	}
	return sb.String()
}
