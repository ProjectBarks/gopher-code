package util

import (
	"regexp"
	"strings"
	"unicode"
)

// Source: utils/stringUtils.ts — general string utilities

// EscapeRegExp escapes special regex characters so the string can be used
// as a literal pattern in a regexp.
// Source: stringUtils.ts:9-11
func EscapeRegExp(s string) string {
	return regexp.QuoteMeta(s)
}

// Capitalize uppercases the first character, leaving the rest unchanged.
// Unlike strings.Title, this does NOT lowercase remaining characters.
// Source: stringUtils.ts:20-22
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// Plural returns the singular or plural form based on count.
// Source: stringUtils.ts:32-38
func Plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// PluralCustom returns the singular or custom plural form based on count.
func PluralCustom(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// FirstLineOf returns the first line without allocating a split array.
// Source: stringUtils.ts:44-47
func FirstLineOf(s string) string {
	nl := strings.IndexByte(s, '\n')
	if nl == -1 {
		return s
	}
	return s[:nl]
}

// CountChar counts occurrences of a byte in a string.
// Source: stringUtils.ts:54-65
func CountChar(s string, ch byte) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			count++
		}
	}
	return count
}

// TruncateMiddle truncates a string to maxLen, replacing the middle with "…".
func TruncateMiddle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	half := (maxLen - 1) / 2
	return s[:half] + "…" + s[len(s)-half:]
}
