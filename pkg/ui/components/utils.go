package components

import "strings"

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
