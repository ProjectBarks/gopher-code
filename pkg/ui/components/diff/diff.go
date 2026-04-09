// Package diff provides unified diff rendering for the TUI.
// Source: components/FileEditToolDiff.tsx, StructuredDiffList.tsx, StructuredDiff/
//
// Renders unified diffs with colored additions/deletions, line numbers,
// and context lines — similar to `git diff` output but with lipgloss styling.
package diff

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Line represents a single diff line with its type and content.
type Line struct {
	Type    LineType
	Content string
	OldNum  int // 0 = not applicable (addition)
	NewNum  int // 0 = not applicable (deletion)
}

// LineType classifies diff lines.
type LineType int

const (
	LineContext  LineType = iota // unchanged
	LineAdded                   // +
	LineRemoved                 // -
	LineHunk                    // @@ header
)

// Hunk is a group of diff lines with context.
type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []Line
}

// FileDiff represents a diff for a single file.
type FileDiff struct {
	Path    string
	OldPath string // for renames
	Mode    string // "modified", "added", "deleted", "renamed"
	Hunks   []Hunk
}

// Styles holds the lipgloss styles for diff rendering.
type Styles struct {
	Added      lipgloss.Style
	Removed    lipgloss.Style
	Context    lipgloss.Style
	HunkHeader lipgloss.Style
	LineNum    lipgloss.Style
	FilePath   lipgloss.Style
}

// DefaultStyles returns the default diff color scheme.
func DefaultStyles() Styles {
	return Styles{
		Added:      lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // green
		Removed:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")),  // red
		Context:    lipgloss.NewStyle().Faint(true),
		HunkHeader: lipgloss.NewStyle().Foreground(lipgloss.Color("6")), // cyan
		LineNum:    lipgloss.NewStyle().Faint(true).Width(4),
		FilePath:   lipgloss.NewStyle().Bold(true),
	}
}

// RenderFileDiff renders a complete file diff with header and all hunks.
func RenderFileDiff(fd FileDiff, styles Styles, width int) string {
	var sb strings.Builder

	// File header
	header := fd.Path
	if fd.OldPath != "" && fd.OldPath != fd.Path {
		header = fd.OldPath + " → " + fd.Path
	}
	sb.WriteString(styles.FilePath.Render(header))
	sb.WriteString("\n")

	// Hunks
	for _, hunk := range fd.Hunks {
		sb.WriteString(renderHunk(hunk, styles))
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func renderHunk(h Hunk, s Styles) string {
	var sb strings.Builder

	// Hunk header
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
	sb.WriteString(s.HunkHeader.Render(header))
	sb.WriteString("\n")

	// Lines
	for _, line := range h.Lines {
		sb.WriteString(renderLine(line, s))
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func renderLine(l Line, s Styles) string {
	var prefix string
	var style lipgloss.Style
	var numStr string

	switch l.Type {
	case LineAdded:
		prefix = "+"
		style = s.Added
		numStr = s.LineNum.Render(fmt.Sprintf("%4d", l.NewNum))
	case LineRemoved:
		prefix = "-"
		style = s.Removed
		numStr = s.LineNum.Render(fmt.Sprintf("%4d", l.OldNum))
	case LineContext:
		prefix = " "
		style = s.Context
		numStr = s.LineNum.Render(fmt.Sprintf("%4d", l.NewNum))
	case LineHunk:
		return s.HunkHeader.Render(l.Content)
	}

	return numStr + " " + style.Render(prefix+l.Content)
}

// ParseUnifiedDiff parses a unified diff string into FileDiffs.
// Handles standard `git diff` output format.
func ParseUnifiedDiff(diffText string) []FileDiff {
	var diffs []FileDiff
	var current *FileDiff
	var currentHunk *Hunk

	lines := strings.Split(diffText, "\n")
	oldNum, newNum := 0, 0

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			if current != nil {
				if currentHunk != nil {
					current.Hunks = append(current.Hunks, *currentHunk)
				}
				diffs = append(diffs, *current)
			}
			current = &FileDiff{Mode: "modified"}
			currentHunk = nil

		case strings.HasPrefix(line, "--- a/"):
			if current != nil {
				current.OldPath = strings.TrimPrefix(line, "--- a/")
			}

		case strings.HasPrefix(line, "+++ b/"):
			if current != nil {
				current.Path = strings.TrimPrefix(line, "+++ b/")
			}

		case strings.HasPrefix(line, "@@"):
			if current != nil && currentHunk != nil {
				current.Hunks = append(current.Hunks, *currentHunk)
			}
			currentHunk = parseHunkHeader(line)
			if currentHunk != nil {
				oldNum = currentHunk.OldStart
				newNum = currentHunk.NewStart
			}

		case currentHunk != nil:
			if strings.HasPrefix(line, "+") {
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type: LineAdded, Content: line[1:], NewNum: newNum,
				})
				newNum++
			} else if strings.HasPrefix(line, "-") {
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type: LineRemoved, Content: line[1:], OldNum: oldNum,
				})
				oldNum++
			} else if strings.HasPrefix(line, " ") {
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type: LineContext, Content: line[1:], OldNum: oldNum, NewNum: newNum,
				})
				oldNum++
				newNum++
			}
		}
	}

	// Finalize
	if current != nil {
		if currentHunk != nil {
			current.Hunks = append(current.Hunks, *currentHunk)
		}
		diffs = append(diffs, *current)
	}

	return diffs
}

func parseHunkHeader(line string) *Hunk {
	h := &Hunk{}
	_, err := fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@",
		&h.OldStart, &h.OldCount, &h.NewStart, &h.NewCount)
	if err != nil {
		// Try without counts
		fmt.Sscanf(line, "@@ -%d +%d @@", &h.OldStart, &h.NewStart)
		h.OldCount = 1
		h.NewCount = 1
	}
	return h
}
