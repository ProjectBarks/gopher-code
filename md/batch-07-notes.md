# Batch 7 Notes — Web & MCP Tools

## What was done

### WebFetchTool: Already at parity
All constants match TS exactly:
- MaxURLLength = 2000
- MaxHTTPContentLength = 10 * 1024 * 1024 (10MB)
- FetchTimeoutMs = 60,000ms
- MaxRedirects = 10
- MaxMarkdownLength = 100,000

Preapproved hosts list matches TS (130+ domains). URL validation, redirect handling (www. add/remove, same-host redirects), and HTML→Markdown conversion are all implemented correctly.

### WebSearchTool: Architectural difference (not a bug)
TS WebSearchTool uses the **Anthropic API's built-in web_search_20250305 tool type** — it sends the search query to Claude's API which performs the search and returns structured results. Go uses **DuckDuckGo HTML scraping**. Both approaches work but use fundamentally different backends. The Go approach is standalone (no API dependency for search) while the TS approach leverages the API.

### MCP tools: Stubs present, match TS structure
Go has McpAuth, ListMcpResources, ReadMcpResource with correct schemas and basic behavior. McpAuth correctly returns "not implemented" for login. ListMcpResources and ReadMcpResource are stubs since MCP resource support depends on the MCP service layer (Batch 10).

## What's NOT done (deferred)

### WebFetch: Prompt-based content extraction
TS uses `applyPromptToMarkdown()` which makes a **secondary LLM call** using a small fast model (Haiku) to extract relevant content from the page based on the user's prompt. Go returns the raw markdown directly. This optimization reduces token waste but requires an extra API call. It's most beneficial for large pages where only a small portion is relevant.

### WebFetch: URL caching
TS has an LRU cache for fetched URLs (`URL_CACHE`) that avoids re-fetching the same URL within a session. Go re-fetches every time.

### WebFetch: Domain blocklist preflight
TS checks a centralized domain blocklist (via claude.ai API) before fetching. This prevents fetching from known-malicious or blocked domains. Go skips this check.

### WebSearch: API-based search
TS uses the Anthropic API's web_search tool which returns structured search results with URLs and titles. Go scrapes DuckDuckGo HTML. If the Anthropic API ever deprecates the web_search tool type, both implementations would need updating.

### MCPTool: Dynamic tool forwarding
TS MCPTool dynamically discovers tools from MCP servers and forwards tool calls to them. This is a complex tool that generates tool definitions at runtime. Go has MCP client support in pkg/mcp/ but the dynamic tool registration through MCPTool is simpler.

### McpAuthTool: OAuth flow
TS McpAuthTool implements full OAuth flow for MCP servers (redirect URI, token exchange). Go returns "not implemented".

## Patterns noticed

1. **WebFetch localhost exception**: Go correctly allows http (not https) for localhost/127.0.0.1/::1, while TS upgrades all URLs including localhost to https. Go's behavior is actually more correct for local development. Keep this intentional divergence.

2. **Go-only extra schema parameters**: Go's WebFetchTool has `max_length` and WebSearchTool has `max_results` parameters that TS doesn't have. These are harmless additions — the model won't send them unprompted if TS's system prompt doesn't mention them.

3. **HTML→Markdown quality**: Go uses golang.org/x/net/html for DOM-based markdown conversion, while TS uses the Turndown library. Both produce reasonable markdown but with different formatting details. The Go version handles headings, lists, links, images, code blocks, tables, and block quotes.
