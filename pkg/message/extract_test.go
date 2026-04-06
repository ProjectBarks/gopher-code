package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTextContent(t *testing.T) {
	t.Run("single text block", func(t *testing.T) {
		blocks := []ContentBlock{TextBlock("hello")}
		assert.Equal(t, "hello", ExtractTextContent(blocks, ""))
	})

	t.Run("multiple text blocks with separator", func(t *testing.T) {
		blocks := []ContentBlock{TextBlock("hello"), TextBlock("world")}
		assert.Equal(t, "hello\nworld", ExtractTextContent(blocks, "\n"))
	})

	t.Run("filters non-text blocks", func(t *testing.T) {
		blocks := []ContentBlock{
			TextBlock("before"),
			ToolResultBlock("t1", "result", false),
			TextBlock("after"),
		}
		assert.Equal(t, "before, after", ExtractTextContent(blocks, ", "))
	})

	t.Run("empty blocks", func(t *testing.T) {
		assert.Equal(t, "", ExtractTextContent(nil, ""))
	})

	t.Run("no text blocks", func(t *testing.T) {
		blocks := []ContentBlock{ToolResultBlock("t1", "result", false)}
		assert.Equal(t, "", ExtractTextContent(blocks, "\n"))
	})

	t.Run("empty separator", func(t *testing.T) {
		blocks := []ContentBlock{TextBlock("a"), TextBlock("b"), TextBlock("c")}
		assert.Equal(t, "abc", ExtractTextContent(blocks, ""))
	})
}

func TestGetContentText(t *testing.T) {
	t.Run("returns trimmed joined text", func(t *testing.T) {
		blocks := []ContentBlock{TextBlock("  hello  "), TextBlock("world  ")}
		assert.Equal(t, "hello  \nworld", GetContentText(blocks))
	})

	t.Run("empty blocks returns empty", func(t *testing.T) {
		assert.Equal(t, "", GetContentText(nil))
	})

	t.Run("whitespace only returns empty", func(t *testing.T) {
		blocks := []ContentBlock{TextBlock("   ")}
		assert.Equal(t, "", GetContentText(blocks))
	})
}

func TestExtractTag(t *testing.T) {
	t.Run("simple tag", func(t *testing.T) {
		assert.Equal(t, "content", ExtractTag("<foo>content</foo>", "foo"))
	})

	t.Run("tag with attributes", func(t *testing.T) {
		assert.Equal(t, "content", ExtractTag(`<foo bar="baz">content</foo>`, "foo"))
	})

	t.Run("multiline content", func(t *testing.T) {
		html := "<foo>\nline1\nline2\n</foo>"
		assert.Equal(t, "\nline1\nline2\n", ExtractTag(html, "foo"))
	})

	t.Run("no match", func(t *testing.T) {
		assert.Equal(t, "", ExtractTag("<foo>content</foo>", "bar"))
	})

	t.Run("empty html", func(t *testing.T) {
		assert.Equal(t, "", ExtractTag("", "foo"))
	})

	t.Run("empty tagName", func(t *testing.T) {
		assert.Equal(t, "", ExtractTag("<foo>content</foo>", ""))
	})

	t.Run("whitespace only html", func(t *testing.T) {
		assert.Equal(t, "", ExtractTag("   ", "foo"))
	})

	t.Run("whitespace only tagName", func(t *testing.T) {
		assert.Equal(t, "", ExtractTag("<foo>content</foo>", "  "))
	})

	t.Run("surrounding text", func(t *testing.T) {
		html := "before <foo>inner</foo> after"
		assert.Equal(t, "inner", ExtractTag(html, "foo"))
	})

	t.Run("tag with special regex chars in name", func(t *testing.T) {
		// The tag name is escaped, so dots in names are treated literally
		html := "<foo.bar>content</foo.bar>"
		assert.Equal(t, "content", ExtractTag(html, "foo.bar"))
	})
}

func TestStripPromptXMLTags(t *testing.T) {
	t.Run("strips commit_analysis", func(t *testing.T) {
		input := "<commit_analysis>some analysis</commit_analysis>\nActual content"
		assert.Equal(t, "Actual content", StripPromptXMLTags(input))
	})

	t.Run("strips context tag", func(t *testing.T) {
		input := "<context>ctx data</context>\nContent here"
		assert.Equal(t, "Content here", StripPromptXMLTags(input))
	})

	t.Run("strips function_analysis", func(t *testing.T) {
		input := "Prefix <function_analysis>analysis</function_analysis> suffix"
		assert.Equal(t, "Prefix  suffix", StripPromptXMLTags(input))
	})

	t.Run("strips pr_analysis", func(t *testing.T) {
		input := "<pr_analysis>pr data</pr_analysis>"
		assert.Equal(t, "", StripPromptXMLTags(input))
	})

	t.Run("leaves other tags alone", func(t *testing.T) {
		input := "<div>content</div>"
		assert.Equal(t, "<div>content</div>", StripPromptXMLTags(input))
	})

	t.Run("empty input", func(t *testing.T) {
		assert.Equal(t, "", StripPromptXMLTags(""))
	})
}

func TestIsEmptyMessageText(t *testing.T) {
	t.Run("empty after stripping", func(t *testing.T) {
		assert.True(t, IsEmptyMessageText("<context>stuff</context>"))
	})

	t.Run("no content placeholder", func(t *testing.T) {
		assert.True(t, IsEmptyMessageText("(no content)"))
	})

	t.Run("non-empty text", func(t *testing.T) {
		assert.False(t, IsEmptyMessageText("hello world"))
	})

	t.Run("whitespace only", func(t *testing.T) {
		assert.True(t, IsEmptyMessageText("   "))
	})
}
