package message

import (
	"regexp"
	"strings"
)

// ExtractTextContent concatenates all text blocks with the given separator.
// Source: utils/messages.ts:2893-2901
func ExtractTextContent(blocks []ContentBlock, separator string) string {
	var texts []string
	for _, b := range blocks {
		if b.Type == ContentText {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, separator)
}

// GetContentText extracts text from content blocks, joining with newlines and trimming.
// Returns empty string if no text content is found.
// Source: utils/messages.ts:2903-2913
func GetContentText(blocks []ContentBlock) string {
	result := strings.TrimSpace(ExtractTextContent(blocks, "\n"))
	return result
}

// extractTagRE builds a regex for extractTag. The tag name is escaped for use in a regex.
func extractTagRE(tagName string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(tagName)
	// Matches opening tag (with optional attributes), non-greedy content, and closing tag.
	return regexp.MustCompile(`(?s)<` + escaped + `(?:\s[^>]*)?>` + `(.*?)` + `</` + escaped + `>`)
}

// ExtractTag extracts the content of the first top-level occurrence of the named XML tag.
// Returns empty string if not found or if html/tagName are blank.
// Source: utils/messages.ts:633-687
func ExtractTag(html, tagName string) string {
	if strings.TrimSpace(html) == "" || strings.TrimSpace(tagName) == "" {
		return ""
	}
	re := extractTagRE(tagName)
	m := re.FindStringSubmatch(html)
	if m == nil {
		return ""
	}
	return m[1]
}

// strippedPromptTagRE matches the four known prompt XML tags. Expanded rather
// than using a backreference (which RE2 does not support).
// Source: utils/messages.ts:2758-2759
var strippedPromptTagRE = regexp.MustCompile(
	`(?s)<commit_analysis>.*?</commit_analysis>\n?` +
		`|(?s)<context>.*?</context>\n?` +
		`|(?s)<function_analysis>.*?</function_analysis>\n?` +
		`|(?s)<pr_analysis>.*?</pr_analysis>\n?`,
)

// StripPromptXMLTags removes specific system-injected prompt XML tags from content.
// Source: utils/messages.ts:2761-2763
func StripPromptXMLTags(content string) string {
	return strings.TrimSpace(strippedPromptTagRE.ReplaceAllString(content, ""))
}

// IsEmptyMessageText returns true if the text is empty after stripping prompt XML tags,
// or equals the no-content placeholder.
// Source: utils/messages.ts:2753-2756
func IsEmptyMessageText(text string) bool {
	return StripPromptXMLTags(text) == "" || strings.TrimSpace(text) == NoContentMessage
}
