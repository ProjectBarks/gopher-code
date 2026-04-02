# Go Package Recommendations for Gopher Code

Based on a full analysis of Claude Code v2.1.88 (~513K LoC TypeScript, 32 subsystems).
All versions verified as of April 2026.

---

## Go Version

Bump from `go 1.22` to **`go 1.24`** to get:
- `log/slog` improvements (structured logging without third-party dep)
- `range over func` iterators
- Enhanced `net/http` routing patterns
- Stdlib `maps` and `slices` packages fully stable

---

## Tier 1 — Terminal UI (Charm Stack v2)

The entire Charmbracelet ecosystem shipped a coordinated v2 release in March 2026.
All import paths require the `/v2` suffix.

| Package | Import Path | Version | Purpose |
|---------|------------|---------|---------|
| Bubble Tea | `github.com/charmbracelet/bubbletea/v2` | v2.0.2 | Elm-architecture TUI framework. Replaces Ink/React renderer |
| Lip Gloss | `github.com/charmbracelet/lipgloss/v2` | v2.0.2 | ANSI styling, borders, layout. Replaces chalk + yoga |
| Glamour | `github.com/charmbracelet/glamour/v2` | v2.0.0 | Markdown rendering in terminal. Code blocks, tables, lists |
| Bubbles | `github.com/charmbracelet/bubbles/v2` | v2.1.0 | Spinner, text input, viewport, paginator, progress bar |
| Huh | `github.com/charmbracelet/huh/v2` | v2.0.3 | Interactive forms and prompts (permission dialogs, confirmations) |
| Log | `github.com/charmbracelet/log/v2` | v2.0.0 | Pretty logging for CLI output |
| x/ansi | `github.com/charmbracelet/x/ansi` | v0.11.6 | Low-level ANSI escape code utilities |
| x/term | `github.com/charmbracelet/x/term` | v0.2.2 | Terminal detection, size, capabilities |

**Recommendation:** Use the full Charm v2 stack. It is the only production-grade Go TUI
ecosystem and maps cleanly to what Ink/React does in Claude Code's TypeScript. Bubble Tea's
Elm architecture (Model/Update/View) is a natural fit for the REPL loop. Huh replaces the
permission prompt dialogs. Glamour handles assistant markdown output.

---

## Tier 2 — API & Streaming

| Package | Import Path | Version | Purpose |
|---------|------------|---------|---------|
| Retryable HTTP | `github.com/hashicorp/go-retryablehttp` | latest | Exponential backoff for Anthropic API calls |
| SSE client | `github.com/tmaxmax/go-sse` | latest | Server-Sent Events for streaming API responses |
| WebSocket | `github.com/coder/websocket` | v1.8.14 | MCP WebSocket transport, remote connectivity |
| JWT | `github.com/golang-jwt/jwt/v5` | v5.3.1 | Token handling for bridge/remote auth |

**Recommendation:** Use stdlib `net/http` as the base client, wrap with `go-retryablehttp` for
the Anthropic API. For SSE streaming, `go-sse` is cleaner than hand-rolling a `bufio.Scanner`
over the response body. The Anthropic API streams `content_block_delta` events — SSE is the
correct transport, not WebSocket. WebSocket is only needed for MCP transport.

---

## Tier 3 — MCP (Model Context Protocol)

| Package | Import Path | Version | Purpose |
|---------|------------|---------|---------|
| MCP Go SDK | `github.com/mark3labs/mcp-go` | v0.46.0 | MCP client: tool listing, tool calling, resources, prompts |
| JSON-RPC | (included in mcp-go) | — | Underlying MCP protocol framing |

**Recommendation:** `mcp-go` is pre-v1 but is the de facto Go MCP SDK and actively developed
(46 minor releases). It handles all three transports (stdio, SSE, WebSocket) and the JSON-RPC
framing. Pin to a specific version and expect breaking changes. Claude Code's MCP integration
spans ~40 files and is one of the most complex subsystems — having a solid SDK matters.

---

## Tier 4 — Shell Execution & File Operations

| Package | Import Path | Version | Purpose |
|---------|------------|---------|---------|
| Shell parser | `mvdan.cc/sh/v3` | v3.13.0 | Parse bash to AST for security analysis |
| Doublestar | `github.com/bmatcuk/doublestar/v4` | v4.10.0 | `**` glob support (stdlib lacks this) |
| Chroma | `github.com/alecthomas/chroma/v2` | v2.23.1 | Syntax highlighting in terminal output |
| Diff | `github.com/sergi/go-diff` | latest | Compute diffs for the Edit tool |
| File watcher | `github.com/fsnotify/fsnotify` | latest | File modification detection for cache invalidation |
| Atomic writes | `github.com/google/renameio` | latest | Safe temp-then-rename file writes |
| Encoding detect | `github.com/saintfish/chardet` | latest | Detect UTF-8, UTF-16, etc. |
| Gitignore | `github.com/sabhiram/go-gitignore` | latest | Respect .gitignore in file search |

**Recommendation:** `mvdan.cc/sh/v3` is critical — Claude Code parses every bash command to
detect dangerous operations (rm -rf, fork bombs, etc.) before execution. The vanity import
path is `mvdan.cc/sh/v3`, NOT `github.com/mvdan/sh`. For ripgrep: vendor the ripgrep binary
like Claude Code does rather than reimplementing in Go. Use `doublestar` for the Glob tool —
stdlib `filepath.Glob` does not support `**` patterns.

---

## Tier 5 — Configuration & Validation

| Package | Import Path | Version | Purpose |
|---------|------------|---------|---------|
| Koanf | `github.com/knadh/koanf/v2` | v2.3.4 | Multi-source config (file, env, flags, defaults) |
| YAML | `github.com/goccy/go-yaml` | latest | MCP configs, settings files |
| JSON Schema | `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 | Validate tool input schemas, settings |
| Cobra | `github.com/spf13/cobra` | latest | CLI argument parsing, subcommands |

**Recommendation:** Koanf v2 over Viper — it is lighter, more composable, and doesn't pull in
the entire spf13 ecosystem. JSON Schema validation is important because Claude Code validates
every tool input against a schema before execution. Cobra for the CLI entry point is standard.

---

## Tier 6 — Git & GitHub

| Package | Import Path | Version | Purpose |
|---------|------------|---------|---------|
| go-git | `github.com/go-git/go-git/v5` | latest | Pure Go git: status, diff, log, branch, worktree |
| GitHub API | `github.com/google/go-github/v84` | v84.0.0 | PR creation, issue linking, review comments |
| OAuth2 | `golang.org/x/oauth2` | latest | GitHub/Anthropic OAuth flows |

**Recommendation:** `go-github` bumps its major version with every GitHub API cycle — pin to
latest at time of `go get` (currently v84). For simple git operations (status, diff), shelling
out to `git` is faster to implement than go-git. Use go-git when you need programmatic access
to git objects (worktree isolation, diff computation). Consider a hybrid: shell out for simple
commands, use go-git for complex operations.

---

## Tier 7 — Concurrency & State

| Package | Import Path | Version | Purpose |
|---------|------------|---------|---------|
| errgroup | `golang.org/x/sync/errgroup` | latest | Parallel tool execution with error collection |
| semaphore | `golang.org/x/sync/semaphore` | latest | Concurrency limits for tool execution |
| LRU cache | `github.com/hashicorp/golang-lru/v2` | v2.0.7 | File state cache, tool result cache |
| UUID | `github.com/google/uuid` | v1.6.0 | Session IDs, task IDs (already in go.mod) |

**Recommendation:** Use `errgroup` + `semaphore` from `x/sync` over `sourcegraph/conc`
(which hasn't been updated since 2023). Go's stdlib concurrency primitives (goroutines,
channels, context, sync.Mutex) cover 90% of needs. The LRU cache is for the file state
cache that prevents re-reading unchanged files during a session.

---

## Tier 8 — Observability

| Package | Import Path | Version | Purpose |
|---------|------------|---------|---------|
| slog | `log/slog` (stdlib) | go1.24 | Structured logging — no third-party needed |
| OpenTelemetry | `go.opentelemetry.io/otel` | latest | Traces, metrics, spans |
| GrowthBook | `github.com/growthbook/growthbook-golang` | latest | Feature flags, A/B testing |

**Recommendation:** Use stdlib `log/slog` for all logging — it is sufficient and avoids a dep.
OpenTelemetry is a later-phase add for production observability. GrowthBook integration can
wait until the feature flag system is needed.

---

## Tier 9 — Security & Auth

| Package | Import Path | Version | Purpose |
|---------|------------|---------|---------|
| Keyring | `github.com/zalando/go-keyring` | latest | Store API keys in OS keychain |
| crypto/tls | stdlib | go1.24 | mTLS for MCP server auth |
| crypto/sha256 | stdlib | go1.24 | Content-addressed storage, dedup |

**Recommendation:** `go-keyring` provides cross-platform keychain access (macOS Keychain,
Linux secret-service, Windows Credential Manager). Use it for API key storage instead of
plaintext config files. Stdlib crypto covers everything else.

---

## Tier 10 — Specialized (Later Phases)

| Package | Import Path | Version | Purpose |
|---------|------------|---------|---------|
| PDF reading | `github.com/ledongthuc/pdf` | latest | Read tool PDF support |
| Image processing | `github.com/disintegration/imaging` | latest | Image resize/convert (replaces sharp) |
| LSP client | Custom over JSON-RPC | — | Language Server Protocol for code intelligence |
| Badger (KV store) | `github.com/dgraph-io/badger/v4` | latest | Session persistence if JSON files aren't enough |

**Recommendation:** These are all later-phase. PDF and image support can be deferred until
the Read tool needs them. LSP is custom work regardless. Badger is only needed if
JSON-file-based session storage becomes a bottleneck.

---

## Packages NOT Recommended

| Package | Why Not |
|---------|---------|
| `github.com/spf13/viper` | Heavy, pulls in too many deps. Use koanf instead |
| `github.com/sourcegraph/conc` | Unmaintained since 2023. Use x/sync instead |
| `github.com/sirupsen/logrus` | Deprecated in favor of stdlib slog |
| `github.com/fatih/color` | Replaced by lipgloss v2 |
| `github.com/manifoldco/promptui` | Replaced by huh v2 |
| `github.com/gdamore/tcell` | Lower-level than bubbletea, more work for same result |
| `github.com/rivo/tview` | Built on tcell, less composable than bubbletea |

---

## Implementation Priority

**Phase 1 — Walking skeleton:**
```
bubbletea/v2, lipgloss/v2, bubbles/v2, glamour/v2
net/http, go-retryablehttp, go-sse
mvdan.cc/sh/v3, doublestar/v4
google/uuid
```

**Phase 2 — Tool system:**
```
chroma/v2, sergi/go-diff, fsnotify
santhosh-tekuri/jsonschema/v6
huh/v2 (permission prompts)
```

**Phase 3 — MCP & collaboration:**
```
mark3labs/mcp-go, coder/websocket
golang.org/x/sync (errgroup, semaphore)
hashicorp/golang-lru/v2
```

**Phase 4 — Git & config:**
```
go-git/v5, go-github/v84, oauth2
knadh/koanf/v2, goccy/go-yaml
cobra
```

**Phase 5 — Production hardening:**
```
go-keyring, opentelemetry-go
growthbook-golang
log/slog (stdlib, already available)
```
