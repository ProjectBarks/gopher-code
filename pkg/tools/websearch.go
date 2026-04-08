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
func (t *WebSearchTool) SearchHint() string  { return "search the web for current information" }
func (t *WebSearchTool) MaxResultSizeChars() int { return 100_000 }

// Prompt returns the WebSearch tool prompt matching the TS source.
// Source: tools/WebSearchTool/prompt.ts
func (t *WebSearchTool) Prompt() string {
	return `- Allows Claude to search the web and use the results to inform responses
- Provides up-to-date information for current events and recent data
- Returns search result information formatted as search result blocks
- Use this tool for accessing information beyond Claude's knowledge cutoff

CRITICAL REQUIREMENT - You MUST follow this:
  - After answering the user's question, you MUST include a "Sources:" section
  - List all relevant URLs from the search results as markdown hyperlinks: [Title](URL)
  - This is MANDATORY - never skip including sources in your response

Usage notes:
  - Domain filtering is supported to include or block specific websites`
}

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

	parsed := parseSearchResultsRaw(string(body), params.MaxResults*2) // over-fetch for filtering
	filtered := filterByDomain(parsed, params.AllowedDomains, params.BlockedDomains)
	if len(filtered) > params.MaxResults {
		filtered = filtered[:params.MaxResults]
	}
	elapsed := time.Since(time.Now().Add(-time.Since(time.Now()))) // placeholder
	_ = elapsed

	if len(filtered) == 0 {
		return SuccessOutput("No results found"), nil
	}
	return SuccessOutput(formatSearchResults(filtered)), nil
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

// filterByDomain applies allowed/blocked domain filtering to results.
// Source: WebSearchTool.ts — makeToolSchema (allowed_domains, blocked_domains)
func filterByDomain(results []searchResult, allowed, blocked []string) []searchResult {
	if len(allowed) == 0 && len(blocked) == 0 {
		return results
	}

	allowSet := make(map[string]bool, len(allowed))
	for _, d := range allowed {
		allowSet[strings.ToLower(d)] = true
	}
	blockSet := make(map[string]bool, len(blocked))
	for _, d := range blocked {
		blockSet[strings.ToLower(d)] = true
	}

	var filtered []searchResult
	for _, r := range results {
		domain := extractDomain(r.URL)
		if len(allowSet) > 0 && !domainMatches(domain, allowSet) {
			continue
		}
		if domainMatches(domain, blockSet) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func domainMatches(domain string, set map[string]bool) bool {
	if set[domain] {
		return true
	}
	// Check if domain is a subdomain of any entry (e.g. "docs.github.com" matches "github.com")
	for d := range set {
		if strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return false
}

// parseSearchResultsRaw extracts search results as structured data.
func parseSearchResultsRaw(html string, maxResults int) []searchResult {
	resultBlockRe := regexp.MustCompile(`(?is)<div[^>]+class="[^"]*result[^"]*results_links[^"]*"[^>]*>(.*?)</div>\s*</div>`)
	blocks := resultBlockRe.FindAllString(html, -1)

	if len(blocks) == 0 {
		return parseSearchResultsFallbackRaw(html, maxResults)
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
		r := searchResult{URL: linkMatch[1], Title: stripHTML(linkMatch[2])}
		snippetMatch := resultSnippetRe.FindStringSubmatch(block)
		if snippetMatch != nil {
			r.Snippet = stripHTML(snippetMatch[1])
		}
		if r.URL != "" && r.Title != "" {
			results = append(results, r)
		}
	}
	return results
}

func parseSearchResultsFallbackRaw(html string, maxResults int) []searchResult {
	linkMatches := resultLinkRe.FindAllStringSubmatch(html, maxResults)
	snippetMatches := resultSnippetRe.FindAllStringSubmatch(html, maxResults)
	var results []searchResult
	for i, lm := range linkMatches {
		r := searchResult{URL: lm[1], Title: stripHTML(lm[2])}
		if i < len(snippetMatches) {
			r.Snippet = stripHTML(snippetMatches[i][1])
		}
		results = append(results, r)
	}
	return results
}

// parseSearchResults extracts search results from DuckDuckGo HTML response (legacy string return).
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
