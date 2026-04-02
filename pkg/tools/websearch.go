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
)

// WebSearchTool searches the web using DuckDuckGo HTML search.
type WebSearchTool struct{}

type webSearchInput struct {
	Query          string   `json:"query"`
	AllowedDomains []string `json:"allowed_domains"`
	BlockedDomains []string `json:"blocked_domains"`
	MaxResults     int      `json:"max_results"`
}

func (t *WebSearchTool) Name() string        { return "WebSearch" }
func (t *WebSearchTool) Description() string { return "Search the web and return results" }
func (t *WebSearchTool) IsReadOnly() bool    { return true }

func (t *WebSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query"},
			"allowed_domains": {"type": "array", "items": {"type": "string"}, "description": "Only include results from these domains"},
			"blocked_domains": {"type": "array", "items": {"type": "string"}, "description": "Exclude results from these domains"}
		},
		"required": ["query"],
		"additionalProperties": false
	}`)
}

func (t *WebSearchTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params webSearchInput
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Query == "" {
		return ErrorOutput("query is required"), nil
	}
	if params.MaxResults <= 0 {
		params.MaxResults = 5
	}

	// Use DuckDuckGo HTML search
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(params.Query))
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to create request: %s", err)), nil
	}
	req.Header.Set("User-Agent", "gopher-code/0.1")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("search failed: %s", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ErrorOutput(fmt.Sprintf("search returned HTTP %d", resp.StatusCode)), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to read response: %s", err)), nil
	}

	results := parseSearchResults(string(body), params.MaxResults)
	if results == "" {
		return SuccessOutput("No results found"), nil
	}
	return SuccessOutput(results), nil
}

// searchResult holds a single parsed search result.
type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

// resultLinkRe matches DuckDuckGo result links.
var resultLinkRe = regexp.MustCompile(`(?is)<a[^>]+class="result__a"[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)

// resultSnippetRe matches DuckDuckGo result snippets.
var resultSnippetRe = regexp.MustCompile(`(?is)<a[^>]+class="result__snippet"[^>]*>(.*?)</a>`)

// parseSearchResults extracts search results from DuckDuckGo HTML response.
func parseSearchResults(html string, maxResults int) string {
	// Split by result blocks
	resultBlockRe := regexp.MustCompile(`(?is)<div[^>]+class="[^"]*result[^"]*results_links[^"]*"[^>]*>(.*?)</div>\s*</div>`)
	blocks := resultBlockRe.FindAllString(html, -1)

	// If the block regex doesn't match, try finding individual links and snippets
	if len(blocks) == 0 {
		return parseSearchResultsFallback(html, maxResults)
	}

	var results []searchResult
	for _, block := range blocks {
		if len(results) >= maxResults {
			break
		}

		linkMatch := resultLinkRe.FindStringSubmatch(block)
		if linkMatch == nil {
			continue
		}

		r := searchResult{
			URL:   linkMatch[1],
			Title: stripHTML(linkMatch[2]),
		}

		snippetMatch := resultSnippetRe.FindStringSubmatch(block)
		if snippetMatch != nil {
			r.Snippet = stripHTML(snippetMatch[1])
		}

		results = append(results, r)
	}

	return formatSearchResults(results)
}

// parseSearchResultsFallback uses a simpler approach when block parsing fails.
func parseSearchResultsFallback(html string, maxResults int) string {
	linkMatches := resultLinkRe.FindAllStringSubmatch(html, maxResults)
	snippetMatches := resultSnippetRe.FindAllStringSubmatch(html, maxResults)

	var results []searchResult
	for i, lm := range linkMatches {
		r := searchResult{
			URL:   lm[1],
			Title: stripHTML(lm[2]),
		}
		if i < len(snippetMatches) {
			r.Snippet = stripHTML(snippetMatches[i][1])
		}
		results = append(results, r)
	}

	return formatSearchResults(results)
}

func formatSearchResults(results []searchResult) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, r := range results {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n   %s", i+1, r.Title, r.URL))
		if r.Snippet != "" {
			sb.WriteString(fmt.Sprintf("\n   %s", r.Snippet))
		}
	}
	return sb.String()
}
