package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Source: tools/WebFetchTool/utils.ts

// Constants matching TS source exactly.
const (
	MaxURLLength         = 2000              // Source: utils.ts:106
	MaxHTTPContentLength = 10 * 1024 * 1024  // Source: utils.ts:112
	FetchTimeoutMs       = 60_000            // Source: utils.ts:116
	MaxRedirects         = 10                // Source: utils.ts:125
	MaxMarkdownLength    = 100_000           // Source: utils.ts:128
)

// WebFetchUserAgent is the User-Agent sent with fetch requests.
// Source: utils/http.ts:56-58
const WebFetchUserAgent = "Claude-User (gopher-code; +https://support.anthropic.com/)"

// WebFetchTool fetches a URL and returns its content as Markdown.
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
		params.MaxLength = MaxMarkdownLength
	}

	// Validate URL — Source: utils.ts:139-169
	if !ValidateURL(params.URL) {
		return ErrorOutput("Invalid URL"), nil
	}

	// Upgrade http to https — Source: utils.ts:377-379
	fetchURL := params.URL
	if parsed, err := url.Parse(fetchURL); err == nil && parsed.Scheme == "http" {
		host := parsed.Hostname()
		// Don't upgrade localhost/loopback (used by test servers and local dev)
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			parsed.Scheme = "https"
			fetchURL = parsed.String()
		}
	}

	// Fetch with redirect handling — Source: utils.ts:262-329
	result, err := fetchWithRedirects(ctx, fetchURL, 0)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("fetch failed: %s", err)), nil
	}

	// Handle redirect info — Source: WebFetchTool.ts:217-249
	if result.IsRedirect {
		msg := fmt.Sprintf("The URL %s redirected to a different domain: %s (HTTP %d).\n\nTo fetch the redirected URL, please call WebFetch again with the new URL.",
			result.OriginalURL, result.RedirectURL, result.StatusCode)
		return SuccessOutput(msg), nil
	}

	if result.StatusCode >= 400 {
		return ErrorOutput(fmt.Sprintf("HTTP %d: %s", result.StatusCode, result.StatusText)), nil
	}

	// Convert HTML to Markdown or use content raw
	// Source: utils.ts:456-466
	content := result.Body
	contentType := result.ContentType
	if strings.Contains(contentType, "text/html") {
		content = htmlToMarkdown(content)
	}

	// Truncate — Source: utils.ts:128
	if len(content) > params.MaxLength {
		content = content[:params.MaxLength] + "\n\n[Content truncated due to length...]"
	}

	return SuccessOutput(content), nil
}

// fetchResult holds the outcome of a fetch.
type fetchResult struct {
	Body        string
	StatusCode  int
	StatusText  string
	ContentType string
	// Redirect info
	IsRedirect  bool
	OriginalURL string
	RedirectURL string
}

// fetchWithRedirects fetches a URL, manually following permitted redirects.
// Source: utils.ts:262-329
func fetchWithRedirects(ctx context.Context, rawURL string, depth int) (*fetchResult, error) {
	if depth > MaxRedirects {
		return nil, fmt.Errorf("too many redirects (exceeded %d)", MaxRedirects)
	}

	client := &http.Client{
		Timeout: time.Duration(FetchTimeoutMs) * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects automatically
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("Accept", "text/markdown, text/html, */*")
	req.Header.Set("User-Agent", WebFetchUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle redirects — Source: utils.ts:284-313
	if resp.StatusCode == 301 || resp.StatusCode == 302 ||
		resp.StatusCode == 307 || resp.StatusCode == 308 {
		location := resp.Header.Get("Location")
		if location == "" {
			return nil, fmt.Errorf("redirect missing Location header")
		}

		// Resolve relative URLs
		redirectURL, err := url.Parse(location)
		if err != nil {
			return nil, fmt.Errorf("invalid redirect URL: %w", err)
		}
		base, _ := url.Parse(rawURL)
		resolvedURL := base.ResolveReference(redirectURL).String()

		// Check if redirect is permitted
		if IsPermittedRedirect(rawURL, resolvedURL) {
			return fetchWithRedirects(ctx, resolvedURL, depth+1)
		}

		// Return redirect info for cross-host redirects
		return &fetchResult{
			IsRedirect:  true,
			OriginalURL: rawURL,
			RedirectURL: resolvedURL,
			StatusCode:  resp.StatusCode,
		}, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxHTTPContentLength))
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	return &fetchResult{
		Body:        string(body),
		StatusCode:  resp.StatusCode,
		StatusText:  resp.Status,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}

// ValidateURL checks if a URL is safe to fetch.
// Source: utils.ts:139-169
func ValidateURL(rawURL string) bool {
	if len(rawURL) > MaxURLLength {
		return false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	// Block URLs with credentials
	if parsed.User != nil {
		return false
	}
	// Must have a publicly resolvable hostname (at least 2 parts)
	// Exception: allow localhost/127.0.0.1 for local development/testing
	hostname := parsed.Hostname()
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return true
	}
	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		return false
	}
	return true
}

// IsPermittedRedirect checks if a redirect is safe to follow.
// Allows redirects that add/remove "www." or stay on the same host.
// Source: utils.ts:212-243
func IsPermittedRedirect(originalURL, redirectURL string) bool {
	orig, err := url.Parse(originalURL)
	if err != nil {
		return false
	}
	redir, err := url.Parse(redirectURL)
	if err != nil {
		return false
	}

	// Protocol must match
	if redir.Scheme != orig.Scheme {
		return false
	}
	// Port must match
	if redir.Port() != orig.Port() {
		return false
	}
	// No credentials in redirect
	if redir.User != nil {
		return false
	}

	// Hostname check: allow www. addition/removal
	stripWWW := func(h string) string { return strings.TrimPrefix(h, "www.") }
	return stripWWW(orig.Hostname()) == stripWWW(redir.Hostname())
}

// parseHostAndPath extracts hostname and pathname from a URL string.
func parseHostAndPath(rawURL string) (string, string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}
	return parsed.Hostname(), parsed.Path
}

// htmlToMarkdown converts HTML to Markdown using x/net/html DOM parsing.
// This is the Go equivalent of TS Turndown library.
// Source: utils.ts:85-97 (Turndown lazy singleton)
func htmlToMarkdown(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return stripHTML(htmlStr)
	}

	var sb strings.Builder
	walkNode(&sb, doc, 0)

	result := sb.String()
	// Clean up excessive newlines
	result = multiNewlineRe.ReplaceAllString(result, "\n\n")
	return strings.TrimSpace(result)
}

// walkNode recursively converts HTML DOM nodes to Markdown.
func walkNode(sb *strings.Builder, n *html.Node, depth int) {
	if n == nil {
		return
	}

	switch n.Type {
	case html.TextNode:
		text := n.Data
		// Collapse whitespace in inline context
		if n.Parent != nil && !isBlockElement(n.Parent) {
			text = collapseWhitespace(text)
		}
		sb.WriteString(text)
		return

	case html.ElementNode:
		tag := n.DataAtom

		// Skip script, style, and hidden elements
		if tag == atom.Script || tag == atom.Style || tag == atom.Noscript {
			return
		}

		switch tag {
		case atom.H1:
			sb.WriteString("\n\n# ")
			walkChildren(sb, n, depth)
			sb.WriteString("\n\n")
			return
		case atom.H2:
			sb.WriteString("\n\n## ")
			walkChildren(sb, n, depth)
			sb.WriteString("\n\n")
			return
		case atom.H3:
			sb.WriteString("\n\n### ")
			walkChildren(sb, n, depth)
			sb.WriteString("\n\n")
			return
		case atom.H4:
			sb.WriteString("\n\n#### ")
			walkChildren(sb, n, depth)
			sb.WriteString("\n\n")
			return
		case atom.H5:
			sb.WriteString("\n\n##### ")
			walkChildren(sb, n, depth)
			sb.WriteString("\n\n")
			return
		case atom.H6:
			sb.WriteString("\n\n###### ")
			walkChildren(sb, n, depth)
			sb.WriteString("\n\n")
			return

		case atom.P:
			sb.WriteString("\n\n")
			walkChildren(sb, n, depth)
			sb.WriteString("\n\n")
			return

		case atom.Br:
			sb.WriteString("\n")
			return

		case atom.Hr:
			sb.WriteString("\n\n---\n\n")
			return

		case atom.Strong, atom.B:
			sb.WriteString("**")
			walkChildren(sb, n, depth)
			sb.WriteString("**")
			return

		case atom.Em, atom.I:
			sb.WriteString("*")
			walkChildren(sb, n, depth)
			sb.WriteString("*")
			return

		case atom.Code:
			if n.Parent != nil && n.Parent.DataAtom == atom.Pre {
				// Code inside pre — handled by pre
				walkChildren(sb, n, depth)
			} else {
				sb.WriteString("`")
				walkChildren(sb, n, depth)
				sb.WriteString("`")
			}
			return

		case atom.Pre:
			sb.WriteString("\n\n```\n")
			walkChildren(sb, n, depth)
			sb.WriteString("\n```\n\n")
			return

		case atom.Blockquote:
			sb.WriteString("\n\n> ")
			walkChildren(sb, n, depth)
			sb.WriteString("\n\n")
			return

		case atom.A:
			href := getAttr(n, "href")
			if href != "" {
				sb.WriteString("[")
				walkChildren(sb, n, depth)
				sb.WriteString("](")
				sb.WriteString(href)
				sb.WriteString(")")
			} else {
				walkChildren(sb, n, depth)
			}
			return

		case atom.Img:
			alt := getAttr(n, "alt")
			src := getAttr(n, "src")
			if src != "" {
				sb.WriteString("![")
				sb.WriteString(alt)
				sb.WriteString("](")
				sb.WriteString(src)
				sb.WriteString(")")
			}
			return

		case atom.Ul:
			sb.WriteString("\n")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.DataAtom == atom.Li {
					sb.WriteString("\n- ")
					walkChildren(sb, c, depth+1)
				}
			}
			sb.WriteString("\n")
			return

		case atom.Ol:
			sb.WriteString("\n")
			idx := 1
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.DataAtom == atom.Li {
					sb.WriteString(fmt.Sprintf("\n%d. ", idx))
					walkChildren(sb, c, depth+1)
					idx++
				}
			}
			sb.WriteString("\n")
			return

		case atom.Table:
			sb.WriteString("\n\n")
			walkChildren(sb, n, depth)
			sb.WriteString("\n\n")
			return

		case atom.Tr:
			sb.WriteString("| ")
			walkChildren(sb, n, depth)
			sb.WriteString("\n")
			return

		case atom.Td, atom.Th:
			walkChildren(sb, n, depth)
			sb.WriteString(" | ")
			return

		case atom.Div, atom.Section, atom.Article, atom.Main, atom.Header, atom.Footer, atom.Nav, atom.Aside:
			sb.WriteString("\n")
			walkChildren(sb, n, depth)
			sb.WriteString("\n")
			return
		}
	}

	// Default: recurse into children
	walkChildren(sb, n, depth)
}

func walkChildren(sb *strings.Builder, n *html.Node, depth int) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkNode(sb, c, depth)
	}
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func isBlockElement(n *html.Node) bool {
	switch n.DataAtom {
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Main,
		atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6,
		atom.Ul, atom.Ol, atom.Li, atom.Blockquote, atom.Pre,
		atom.Table, atom.Tr, atom.Td, atom.Th,
		atom.Header, atom.Footer, atom.Nav, atom.Aside:
		return true
	}
	return false
}

var whitespaceRe = regexp.MustCompile(`\s+`)

func collapseWhitespace(s string) string {
	return whitespaceRe.ReplaceAllString(s, " ")
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
// Used as fallback if DOM parsing fails.
func stripHTML(htmlStr string) string {
	// Remove script and style blocks entirely
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	htmlStr = scriptRe.ReplaceAllString(htmlStr, "")
	htmlStr = styleRe.ReplaceAllString(htmlStr, "")

	// Replace block-level tags with newlines
	blockRe := regexp.MustCompile(`(?i)</(p|div|br|li|h[1-6]|tr|td|th|blockquote|pre)>`)
	htmlStr = blockRe.ReplaceAllString(htmlStr, "\n")
	brRe := regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlStr = brRe.ReplaceAllString(htmlStr, "\n")

	// Strip all remaining tags
	text := htmlTagRe.ReplaceAllString(htmlStr, "")

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
