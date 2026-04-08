// Package shell provides rendering components for shell command output.
// Source: components/shell/ — OutputLine.tsx, ShellProgressMessage.tsx, ShellTimeDisplay.tsx
package shell

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// OutputLine renders a single line of shell output with optional line number.
// Source: components/shell/OutputLine.tsx
func OutputLine(lineNum int, text string, dimLineNumbers bool) string {
	if lineNum <= 0 {
		return text
	}
	numStyle := lipgloss.NewStyle().Faint(dimLineNumbers)
	return numStyle.Render(fmt.Sprintf("%4d", lineNum)) + " " + text
}

// ShellProgressMessage renders a progress indicator for a running command.
// Source: components/shell/ShellProgressMessage.tsx
func ShellProgressMessage(command string, elapsed time.Duration) string {
	elapsedStr := FormatElapsed(elapsed)
	return fmt.Sprintf("⏺ Running: %s %s", command, elapsedStr)
}

// FormatElapsed formats a duration for shell output display.
// Source: components/shell/ShellTimeDisplay.tsx
func FormatElapsed(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 60 {
		return fmt.Sprintf("(%ds)", secs)
	}
	mins := secs / 60
	remainSecs := secs % 60
	return fmt.Sprintf("(%dm%ds)", mins, remainSecs)
}

// TruncateOutput truncates shell output to maxLines, showing first and last.
func TruncateOutput(output string, maxLines int) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= maxLines {
		return output
	}
	half := maxLines / 2
	first := lines[:half]
	last := lines[len(lines)-half:]
	omitted := len(lines) - maxLines
	return strings.Join(first, "\n") +
		fmt.Sprintf("\n... (%d lines omitted) ...\n", omitted) +
		strings.Join(last, "\n")
}
