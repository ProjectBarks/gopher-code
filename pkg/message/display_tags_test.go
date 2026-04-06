package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripDisplayTags(t *testing.T) {
	t.Run("strips lowercase xml tags", func(t *testing.T) {
		input := "<system-reminder>some context</system-reminder>\nHello world"
		assert.Equal(t, "Hello world", StripDisplayTags(input))
	})

	t.Run("strips multiple tags", func(t *testing.T) {
		input := "<task-notification>task data</task-notification>\n<channel-message>msg</channel-message>\nActual content"
		assert.Equal(t, "Actual content", StripDisplayTags(input))
	})

	t.Run("preserves uppercase/JSX tags", func(t *testing.T) {
		input := "Fix the <Button> layout"
		assert.Equal(t, "Fix the <Button> layout", StripDisplayTags(input))
	})

	t.Run("returns original if all content is tags", func(t *testing.T) {
		input := "<foo>bar</foo>"
		// After stripping, result is empty, so original is returned
		assert.Equal(t, "<foo>bar</foo>", StripDisplayTags(input))
	})

	t.Run("strips tags with attributes", func(t *testing.T) {
		input := `<ide_opened_file path="/foo/bar.go">content</ide_opened_file>Hello`
		assert.Equal(t, "Hello", StripDisplayTags(input))
	})

	t.Run("handles multiline tag content", func(t *testing.T) {
		input := "<foo>\nline1\nline2\n</foo>\nVisible text"
		assert.Equal(t, "Visible text", StripDisplayTags(input))
	})

	t.Run("preserves unpaired angle brackets", func(t *testing.T) {
		input := "when x < y"
		assert.Equal(t, "when x < y", StripDisplayTags(input))
	})

	t.Run("empty input", func(t *testing.T) {
		// Empty after strip returns original (which is empty)
		assert.Equal(t, "", StripDisplayTags(""))
	})

	t.Run("only whitespace after stripping", func(t *testing.T) {
		input := "<foo>bar</foo>   "
		// After strip and trim, empty => return original
		assert.Equal(t, "<foo>bar</foo>   ", StripDisplayTags(input))
	})

	t.Run("tag with hyphen in name", func(t *testing.T) {
		input := "<system-reminder>ctx</system-reminder> Hello"
		assert.Equal(t, "Hello", StripDisplayTags(input))
	})
}

func TestStripDisplayTagsAllowEmpty(t *testing.T) {
	t.Run("returns empty when all content is tags", func(t *testing.T) {
		input := "<foo>bar</foo>"
		assert.Equal(t, "", StripDisplayTagsAllowEmpty(input))
	})

	t.Run("returns content when mixed", func(t *testing.T) {
		input := "<foo>bar</foo>\nVisible"
		assert.Equal(t, "Visible", StripDisplayTagsAllowEmpty(input))
	})
}

func TestStripIdeContextTags(t *testing.T) {
	t.Run("strips ide_opened_file", func(t *testing.T) {
		input := `<ide_opened_file path="/foo">content</ide_opened_file>Hello world`
		assert.Equal(t, "Hello world", StripIdeContextTags(input))
	})

	t.Run("strips ide_selection", func(t *testing.T) {
		input := `<ide_selection file="/foo" lines="1-5">selected code</ide_selection>Fix this`
		assert.Equal(t, "Fix this", StripIdeContextTags(input))
	})

	t.Run("preserves other lowercase tags", func(t *testing.T) {
		input := "<foo>bar</foo> Hello"
		assert.Equal(t, "<foo>bar</foo> Hello", StripIdeContextTags(input))
	})

	t.Run("preserves user HTML content", func(t *testing.T) {
		input := "<code>foo</code>"
		assert.Equal(t, "<code>foo</code>", StripIdeContextTags(input))
	})

	t.Run("strips multiple IDE tags", func(t *testing.T) {
		input := `<ide_opened_file path="/a">a</ide_opened_file>` +
			`<ide_selection file="/b">b</ide_selection>` +
			"User query"
		assert.Equal(t, "User query", StripIdeContextTags(input))
	})
}
