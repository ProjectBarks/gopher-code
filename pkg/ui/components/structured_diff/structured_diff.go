// Package structured_diff provides structured diff rendering with file context.
// Source: components/StructuredDiffList.tsx, components/StructuredDiff/
//
// Renders a file's diff hunks with syntax-highlighted context, line numbers,
// and ellipsis separators between non-adjacent hunks.
package structured_diff

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/components/diff"
)

// RenderStructuredDiff renders a list of diff hunks for a file with
// ellipsis separators between non-adjacent hunks.
// Source: StructuredDiffList.tsx
func RenderStructuredDiff(fd diff.FileDiff, width int, dim bool) string {
	styles := diff.DefaultStyles()
	if dim {
		styles.Added = styles.Added.Faint(true)
		styles.Removed = styles.Removed.Faint(true)
		styles.Context = styles.Context.Faint(true)
	}

	var sb strings.Builder

	// File header
	headerStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	sb.WriteString(headerStyle.Render(fd.Path))
	sb.WriteString("\n")

	for i, hunk := range fd.Hunks {
		if i > 0 {
			// Ellipsis separator between non-adjacent hunks
			sb.WriteString(lipgloss.NewStyle().Faint(true).Render("  ..."))
			sb.WriteString("\n")
		}
		sb.WriteString(renderHunk(hunk, styles))
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func renderHunk(h diff.Hunk, s diff.Styles) string {
	var sb strings.Builder

	// Hunk header
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
	sb.WriteString(s.HunkHeader.Render(header))
	sb.WriteString("\n")

	for _, line := range h.Lines {
		sb.WriteString(renderLine(line, s))
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func renderLine(l diff.Line, s diff.Styles) string {
	switch l.Type {
	case diff.LineAdded:
		num := s.LineNum.Render(fmt.Sprintf("%4d", l.NewNum))
		return num + " " + s.Added.Render("+"+l.Content)
	case diff.LineRemoved:
		num := s.LineNum.Render(fmt.Sprintf("%4d", l.OldNum))
		return num + " " + s.Removed.Render("-"+l.Content)
	case diff.LineContext:
		num := s.LineNum.Render(fmt.Sprintf("%4d", l.NewNum))
		return num + " " + s.Context.Render(" "+l.Content)
	default:
		return l.Content
	}
}

// RenderDiffSummary returns a one-line summary like "+3 -1" for a file diff.
func RenderDiffSummary(fd diff.FileDiff) string {
	added, removed := 0, 0
	for _, h := range fd.Hunks {
		for _, l := range h.Lines {
			switch l.Type {
			case diff.LineAdded:
				added++
			case diff.LineRemoved:
				removed++
			}
		}
	}

	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	rmStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	parts := []string{}
	if added > 0 {
		parts = append(parts, addStyle.Render(fmt.Sprintf("+%d", added)))
	}
	if removed > 0 {
		parts = append(parts, rmStyle.Render(fmt.Sprintf("-%d", removed)))
	}
	if len(parts) == 0 {
		return "no changes"
	}
	return strings.Join(parts, " ")
}
