package components

import "strings"

// UI characters matching Claude Code's visual language.
const (
	// PromptPrefix is the "❯" (U+276F) character used for input prompts and user messages.
	// Claude Code uses figures.pointer from the npm figures package.
	PromptPrefix = "❯ "

	// ResponseConnector is the "⎿" (U+23BF) character for tool results/responses.
	// Claude uses ⎿ (DENTISTRY SYMBOL LIGHT DOWN AND HORIZONTAL), not └ (U+2514).
	// Source: GrepTool/UI.tsx:66 — <Text dimColor={true}>  ⎿  </Text>
	ResponseConnector = "  ⎿  "

	// ResponseContinuation is the indent for continuation lines under a connector.
	ResponseContinuation = "    "

	// DividerChar is the light horizontal line (U+2500) for section dividers.
	// Claude Code uses ─ (light), not ━ (heavy).
	DividerChar = "─"
)

// FuzzyMatch returns true if needle is a subsequence of haystack.
func FuzzyMatch(needle, haystack string) bool {
	if needle == "" {
		return true
	}
	ni := 0
	for _, c := range haystack {
		if c == rune(needle[ni]) {
			ni++
			if ni == len(needle) {
				return true
			}
		}
	}
	return false
}

// truncateField truncates a string to maxLen, appending "…" if truncated.
func truncateField(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

// truncateLines truncates multi-line content to maxLines.
func truncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

// wrapText wraps text to fit within width using simple word-wrap.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if len(line) <= width {
			lines = append(lines, line)
			continue
		}
		words := strings.Fields(line)
		current := ""
		for _, word := range words {
			if current == "" {
				current = word
			} else if len(current)+1+len(word) <= width {
				current += " " + word
			} else {
				lines = append(lines, current)
				current = word
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return strings.Join(lines, "\n")
}
