package message

import (
	"regexp"
	"strings"
)

// xmlOpenTagRE matches the opening of a lowercase XML tag, capturing the tag name.
// Only matches lowercase tag names so JSX like <Button> passes through.
// Source: utils/displayTags.ts:15
var xmlOpenTagRE = regexp.MustCompile(`<([a-z][\w-]*)(?:\s[^>]*)?>`)

// stripMatchedTags removes all <tag ...>...</tag> blocks where the opening tag
// matches the given pattern. Uses a simple scan-and-match approach since Go's
// regexp engine (RE2) doesn't support backreferences.
func stripMatchedTags(text string, openRE *regexp.Regexp) string {
	var b strings.Builder
	remaining := text
	for {
		loc := openRE.FindStringSubmatchIndex(remaining)
		if loc == nil {
			b.WriteString(remaining)
			break
		}
		// loc[0..1] = full match, loc[2..3] = tag name capture
		tagName := remaining[loc[2]:loc[3]]
		closingTag := "</" + tagName + ">"
		closeIdx := strings.Index(remaining[loc[1]:], closingTag)
		if closeIdx < 0 {
			// No matching close tag — keep everything and move past opening tag
			b.WriteString(remaining[:loc[1]])
			remaining = remaining[loc[1]:]
			continue
		}
		// Write everything before the opening tag
		b.WriteString(remaining[:loc[0]])
		// Skip past closing tag + optional trailing newline
		endIdx := loc[1] + closeIdx + len(closingTag)
		if endIdx < len(remaining) && remaining[endIdx] == '\n' {
			endIdx++
		}
		remaining = remaining[endIdx:]
	}
	return b.String()
}

// ideOpenTagRE matches IDE-injected context tags only.
// Source: utils/displayTags.ts:41-42
var ideOpenTagRE = regexp.MustCompile(`<(ide_opened_file|ide_selection)(?:\s[^>]*)?>`)

// StripDisplayTags removes XML-like tag blocks from text for UI titles.
// If stripping would result in empty text, returns the original unchanged.
// Source: utils/displayTags.ts:26-29
func StripDisplayTags(text string) string {
	result := strings.TrimSpace(stripMatchedTags(text, xmlOpenTagRE))
	if result == "" {
		return text
	}
	return result
}

// StripDisplayTagsAllowEmpty is like StripDisplayTags but returns empty string
// when all content is tags. Used by getLogDisplayTitle.
// Source: utils/displayTags.ts:37-39
func StripDisplayTagsAllowEmpty(text string) string {
	return strings.TrimSpace(stripMatchedTags(text, xmlOpenTagRE))
}

// StripIdeContextTags removes only IDE-injected context tags (ide_opened_file, ide_selection).
// Used by UP-arrow resubmit to preserve user-typed content while dropping IDE noise.
// Source: utils/displayTags.ts:49-51
func StripIdeContextTags(text string) string {
	return strings.TrimSpace(stripMatchedTags(text, ideOpenTagRE))
}
