package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// WebFetchTool fetches a URL and returns clean text content.
type WebFetchTool struct{}

type webFetchInput struct {
	URL       string `json:"url"`
	Prompt    string `json:"prompt"`
	MaxLength int    `json:"max_length"`
}

func (t *WebFetchTool) Name() string        { return "WebFetch" }
func (t *WebFetchTool) Description() string { return "Fetch a URL and return its content as text" }
func (t *WebFetchTool) IsReadOnly() bool    { return true }

func (t *WebFetchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "The URL to fetch"},
			"prompt": {"type": "string", "description": "A prompt to guide content extraction"}
		},
		"required": ["url", "prompt"],
		"additionalProperties": false
	}`)
}

func (t *WebFetchTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params webFetchInput
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.URL == "" {
		return ErrorOutput("url is required"), nil
	}
	if params.MaxLength <= 0 {
		params.MaxLength = 10000
	}

	req, err := http.NewRequestWithContext(ctx, "GET", params.URL, nil)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("invalid URL: %s", err)), nil
	}
	req.Header.Set("User-Agent", "gopher-code/0.1")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("fetch failed: %s", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ErrorOutput(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(params.MaxLength*2)))
	if err != nil {
		return ErrorOutput(fmt.Sprintf("read failed: %s", err)), nil
	}

	text := stripHTML(string(body))
	if len(text) > params.MaxLength {
		text = text[:params.MaxLength] + "...[truncated]"
	}
	return SuccessOutput(text), nil
}

// htmlTagRe matches HTML tags including self-closing tags.
var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

// htmlEntityRe matches common HTML entities.
var htmlEntityRe = regexp.MustCompile(`&(amp|lt|gt|quot|apos|nbsp|#\d+|#x[0-9a-fA-F]+);`)

// multiSpaceRe matches runs of whitespace that should be collapsed.
var multiSpaceRe = regexp.MustCompile(`[ \t]+`)

// multiNewlineRe matches runs of 3+ newlines.
var multiNewlineRe = regexp.MustCompile(`\n{3,}`)

// stripHTML removes HTML tags and decodes common entities to produce clean text.
func stripHTML(html string) string {
	// Remove script and style blocks entirely
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = scriptRe.ReplaceAllString(html, "")
	html = styleRe.ReplaceAllString(html, "")

	// Replace block-level tags with newlines
	blockRe := regexp.MustCompile(`(?i)</(p|div|br|li|h[1-6]|tr|td|th|blockquote|pre)>`)
	html = blockRe.ReplaceAllString(html, "\n")
	brRe := regexp.MustCompile(`(?i)<br\s*/?>`)
	html = brRe.ReplaceAllString(html, "\n")

	// Strip all remaining tags
	text := htmlTagRe.ReplaceAllString(html, "")

	// Decode HTML entities
	text = htmlEntityRe.ReplaceAllStringFunc(text, decodeHTMLEntity)

	// Collapse whitespace
	text = multiSpaceRe.ReplaceAllString(text, " ")
	text = multiNewlineRe.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}

func decodeHTMLEntity(entity string) string {
	switch entity {
	case "&amp;":
		return "&"
	case "&lt;":
		return "<"
	case "&gt;":
		return ">"
	case "&quot;":
		return "\""
	case "&apos;":
		return "'"
	case "&nbsp;":
		return " "
	default:
		// Numeric entities: &#123; or &#x1F;
		inner := entity[2 : len(entity)-1]
		if strings.HasPrefix(inner, "x") || strings.HasPrefix(inner, "X") {
			var n int
			fmt.Sscanf(inner[1:], "%x", &n)
			if n > 0 {
				return string(rune(n))
			}
		} else {
			var n int
			fmt.Sscanf(inner, "%d", &n)
			if n > 0 {
				return string(rune(n))
			}
		}
		return entity
	}
}
