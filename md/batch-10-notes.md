# Batch 10 Notes — Core Services

## What was done

### Query loop retry limit fix (BEHAVIORAL BUG)
Go had `maxRetries = 3` for all retryable API errors. TS has two limits:
- `DEFAULT_MAX_RETRIES = 10` — for 429 rate limits and 5xx server errors
- `MAX_529_RETRIES = 3` — for 529 overloaded errors specifically

The Go value of 3 matched the 529-specific limit but applied it to ALL retries, meaning rate-limited users would give up after only 3 attempts instead of 10. Fixed by adding separate `max529Retries` constant and checking for 529/overloaded in the retry logic.

### services/api (pkg/provider/errors.go): Already at parity
Error classification is comprehensive:
- All 18 APIErrorType constants match TS (rate_limit, server_overload, prompt_too_long, etc.)
- ClassifyHTTPError handles 429, 529, 401, 403, 400, 408, 409, 5xx correctly
- Retry backoff uses exponential backoff with 25% jitter (BASE_DELAY_MS=500, max=32000)
- Context overflow parsing extracts input_tokens, max_tokens, context_limit
- ShouldRetry, IsRetryableError, Is529Error, IsRateLimitError helpers all present

### services/compact (pkg/compact/): Already at parity
- **MicroCompact**: CompactableTools matches TS (Read, Bash, Grep, Glob, WebSearch, WebFetch, Edit, Write). MicroCompactMessages correctly clears old tool results keeping keepRecent most recent. EstimateToolResultTokens uses ~4 chars/token heuristic with 4/3 padding.
- **Budget**: ShouldCompact at 80% of InputBudget (ContextWindow - MaxOutputTokens). DefaultBudget uses 200K context, 16K output.
- **AutoCompact**: BudgetTracker with continuation count, diminishing returns detection (3+ continuations with <500 token deltas).
- **Prompt**: Full compaction prompt with 9-section analysis structure, no-tools preamble, and trailing reinforcement. FormatCompactSummary strips <analysis> and extracts <summary>. GetCompactUserSummaryMessage builds the post-compaction continuation message.

### services/mcp (pkg/mcp/): Basic parity
- Manager handles Connect/RegisterTools/CloseAll lifecycle
- MCPClient does JSON-RPC over stdio
- Config loads and merges from user/project/local scopes
- Missing: SSE/HTTP transports, OAuth flow, elicitation, reconnection logic

### services/oauth (pkg/auth/): API key auth only
- GetAPIKey resolves from env → keyring → auth.json (3-tier)
- SaveAPIKey/DeleteAPIKey manage both keyring and plaintext
- Full OAuth flow not implemented (TS has browser flow, token refresh, profile fetching)

### services/tools (pkg/tools/orchestrator.go): At parity
- ExecuteBatch correctly separates concurrent (read-only) and sequential (mutating) calls
- Pre-tool hooks can block execution
- Post-tool hooks fire after execution
- Permission checks (Allow/Deny/Ask) applied for non-read-only tools

## What's NOT done (deferred)

### Full OAuth flow
TS has complete OAuth implementation: browser flow, refresh tokens, profile fetching, organization UUID resolution. Go only has API key auth.

### MCP advanced features
- SSE/HTTP/WebSocket transports (Go only has stdio)
- OAuth authentication for MCP servers
- Elicitation (server-to-user prompts)
- Reconnection with session recovery
- LRU caching for tool/resource lists

### Streaming tool execution
TS has StreamingToolExecutor for real-time progress during tool execution. Go executes tools synchronously.

### Prompt cache break detection
TS tracks and reports prompt cache effectiveness. Go doesn't monitor cache behavior.

## Patterns noticed

1. **Retry limits matter for UX**: The 3 vs 10 retry difference would cause users to see "API error" messages much more frequently during high-load periods. Rate limits (429) are common and the 10-retry window with exponential backoff gives the API time to recover.

2. **Compact infrastructure is solid**: The Go compact package is one of the most complete subsystems — microcompact, auto-compact, budget tracking, and the full summarization prompt are all faithful ports of the TS code.

3. **Error classification is comprehensive**: All 18 error types are present with correct HTTP status code mapping. The retry backoff formula (500ms * 2^attempt + 25% jitter, max 32s) matches TS exactly.
