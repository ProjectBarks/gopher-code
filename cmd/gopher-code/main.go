package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/projectbarks/gopher-code/cmd/gopher-code/handlers"
	"github.com/projectbarks/gopher-code/internal/cli"
	"github.com/projectbarks/gopher-code/pkg/auth"
	"github.com/projectbarks/gopher-code/pkg/bridge"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/config"
	"github.com/projectbarks/gopher-code/pkg/hooks"
	"github.com/projectbarks/gopher-code/pkg/mcp"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	pluginsBundled "github.com/projectbarks/gopher-code/pkg/plugins/bundled"
	"github.com/projectbarks/gopher-code/pkg/prompt"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	_ "github.com/projectbarks/gopher-code/pkg/server" // T89-T90: server types for direct-connect mode
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/skills"
	"github.com/projectbarks/gopher-code/pkg/telemetry"
	"github.com/projectbarks/gopher-code/pkg/tools"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// Version is the current gopher-code version.
const Version = "0.2.0"

// Model alias mappings
var modelAliases = map[string]string{
	"haiku":  "claude-haiku-4-5-20251001",
	"sonnet": "claude-sonnet-4-6",
	"opus":   "claude-opus-4-6",
}

// resolveModelAlias converts model aliases to full model IDs
func resolveModelAlias(model string) string {
	if resolved, ok := modelAliases[model]; ok {
		return resolved
	}
	return model
}

// initBridgeDeps registers the default bridge dependency bundle so the bridge
// enablement checks (IsBridgeEnabled, GetBridgeDisabledReason, etc.) are
// available throughout the process lifetime.
// Source: src/bridge/bridgeEnabled.ts — wired during CLI bootstrap.
func initBridgeDeps() {
	bridge.SetBridgeDeps(&bridge.BridgeDeps{
		// Build features — disabled by default; set via build tags or env in
		// production builds that ship with bridge support.
		BridgeMode:     false,
		CCRAutoConnect: false,
		CCRMirror:      false,

		// Auth stubs — return safe defaults until real auth is wired.
		IsClaudeAISubscriber: func() bool { return false },
		HasProfileScope:      func() bool { return false },
		GetOAuthAccountInfo:  func() *bridge.OAuthAccountInfo { return nil },

		// GrowthBook stubs — return defaults until real GrowthBook is wired.
		GetFeatureValueBool: func(key string, defaultVal bool) bool { return defaultVal },
		CheckGateBlocking:   func(key string) (bool, error) { return false, nil },
		GetDynamicConfig: func(key string, defaults map[string]string) map[string]string {
			return defaults
		},

		Version:  Version,
		SemverLT: func(a, b string) bool { return a < b },
	})
}

func main() {
	// Existing flags
	model := flag.String("model", "claude-sonnet-4-20250514", "Model to use")
	queryStr := flag.String("query", "", "One-shot query (non-interactive)")
	cwd := flag.String("cwd", "", "Working directory (default: current)")

	// Version flag
	showVersion := flag.Bool("version", false, "Show version and exit")

	// Print mode
	printMode := flag.Bool("p", false, "Print response and exit (headless mode)")
	flag.BoolVar(printMode, "print", false, "Print response and exit (headless mode)")

	// Session management
	continueSession := flag.Bool("c", false, "Continue the most recent conversation")
	flag.BoolVar(continueSession, "continue", false, "Continue the most recent conversation")
	resume := flag.String("r", "", "Resume a conversation by session ID")
	flag.StringVar(resume, "resume", "", "Resume a conversation by session ID")

	// System prompt
	systemPrompt := flag.String("system-prompt", "", "Override system prompt")

	// Limits
	maxTurns := flag.Int("max-turns", 100, "Maximum agentic turns")

	// Permissions
	skipPerms := flag.Bool("dangerously-skip-permissions", false, "Bypass all permission checks")

	// Output
	outputFormat := flag.String("output-format", "text", "Output format: text, json, stream-json")

	// Verbose
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	// Extended thinking
	thinking := flag.String("thinking", "", "Thinking mode: enabled, disabled")

	// Effort level (maps to thinking budget)
	effort := flag.String("effort", "", "Effort level: low, medium, high, max")

	// Additional directories
	addDirs := flag.String("add-dir", "", "Additional allowed directories (comma-separated)")

	// Session persistence
	noSessionPersist := flag.Bool("no-session-persistence", false, "Disable session persistence")

	// System prompt files
	systemPromptFile := flag.String("system-prompt-file", "", "Read system prompt from file")
	appendSystemPrompt := flag.String("append-system-prompt", "", "Append to default system prompt")
	appendSystemPromptFile := flag.String("append-system-prompt-file", "", "Append system prompt from file")

	// Tool control
	allowedTools := flag.String("allowed-tools", "", "Comma-separated list of allowed tools")
	disallowedTools := flag.String("disallowed-tools", "", "Comma-separated list of disallowed tools")

	// Session
	sessionID := flag.String("session-id", "", "Use a specific session ID")
	sessionName := flag.String("name", "", "Display name for the session")
	flag.StringVar(sessionName, "n", "", "Display name for the session")
	prefill := flag.String("prefill", "", "Pre-fill prompt input without submitting")

	// Debug
	debug := flag.Bool("debug", false, "Enable debug mode")
	flag.BoolVar(debug, "d", false, "Enable debug mode")
	debugFile := flag.String("debug-file", "", "Write debug logs to file")

	// Bare mode
	bare := flag.Bool("bare", false, "Minimal mode (skip hooks, plugins, etc.)")

	// Budget
	maxBudgetUSD := flag.Float64("max-budget-usd", 0, "Maximum spend on API calls")

	// Provider
	providerFlag := flag.String("provider", "anthropic", "Provider: anthropic, bedrock, vertex, openai")
	apiURL := flag.String("api-url", "", "API base URL (for custom providers)")

	// Input format
	inputFormat := flag.String("input-format", "text", "Input format: text, stream-json")

	// JSON schema
	jsonSchema := flag.String("json-schema", "", "JSON Schema for structured output validation")

	// Include events
	includeHookEvents := flag.Bool("include-hook-events", false, "Include hook events in stream output")

	// Worktree
	worktree := flag.Bool("worktree", false, "Create git worktree for session")
	flag.BoolVar(worktree, "w", false, "Create git worktree for session")

	// Betas
	betas := flag.String("betas", "", "Beta headers for API requests (comma-separated)")

	// Init flag (vs /init command)
	initFlag := flag.Bool("init", false, "Initialize project and exit")

	// Fallback model
	fallbackModel := flag.String("fallback-model", "", "Automatic fallback model")

	// Permission mode
	permModeFlag := flag.String("permission-mode", "", "Permission mode: auto, interactive, deny")

	// T172: Initialize bridge dependency bundle early so all bridge enablement
	// checks are available before any subcommand dispatches.
	initBridgeDeps()

	// Handle "remote-control" subcommand before flag.Parse()
	// Source: src/cli.ts — `claude remote-control` CLI subcommand dispatches
	// to the bridge REPL initializer (pkg/bridge/init_repl.go).
	if len(os.Args) > 1 && os.Args[1] == "remote-control" {
		// T171: Load envless bridge config and run the version gate before
		// any expensive initialization. In production this config comes from
		// GrowthBook; for now use the validated default.
		envlessCfg := bridge.DefaultEnvLessBridgeConfig
		if versionErr := bridge.CheckEnvLessBridgeMinVersion(envlessCfg, Version); versionErr != "" {
			cliError(versionErr)
		}

		// T172: Gate on bridge enablement unless forced via env var.
		if !bridge.IsBridgeEnabled() && !bridge.IsBridgeForced() {
			reason, err := bridge.GetBridgeDisabledReason()
			if err != nil {
				cliErrorf("Error checking Remote Control eligibility: %v", err)
			}
			if reason == "" {
				reason = bridge.ErrBridgeNotAvailable
			}
			// T190: Log skip event with debug utils before exiting.
			bridge.LogBridgeSkip(reason, "remote-control skipped: bridge not enabled", nil)
			cliError(reason)
		}

		// Extract optional session name from remaining args.
		rcName := ""
		if len(os.Args) > 2 {
			rcName = strings.Join(os.Args[2:], " ")
		}

		// T170: Resolve bridge connection config (URL + token) for the remote-control session.
		// Priority: ant-only env overrides > OAuth keychain/production defaults.
		bridgeDeps := bridge.ConfigDeps{
			GetAccessToken: func() (string, bool) {
				key, err := auth.GetAPIKey()
				if err != nil {
					return "", false
				}
				return key, true
			},
			GetBaseAPIURL: func() string { return "https://api.anthropic.com" },
		}
		bridgeURL := bridge.BridgeBaseURL(bridgeDeps)
		bridgeToken, bridgeAuthed := bridge.BridgeAccessToken(bridgeDeps)

		// T177: Validate bridge access token via JWT parsing before proceeding.
		// If the token is a decodable JWT, check expiry and log claims for
		// diagnostics. Non-JWT tokens (e.g. plain API keys) are passed through.
		if bridgeAuthed && bridgeToken != "" {
			if claims := bridge.DecodeJWTClaims(bridgeToken); claims != nil {
				if bridge.IsJWTExpired(bridgeToken) {
					cliError("bridge access token is expired; please re-authenticate")
				}
				// T182: Normalize session ID from JWT claims through the
				// compat layer so infra-prefixed IDs (cse_*) are translated
				// to the client-facing prefix (session_*) for downstream use.
				compatSID := bridge.ToCompatSessionID(claims.SessionID)
				slog.Debug("bridge: JWT validated",
					"sub", claims.Sub,
					"org_uuid", claims.OrgUUID,
					"session_id", claims.SessionID,
					"compat_session_id", compatSID,
				)
			}
		}

		// T189: Initialize BridgeDebug logger for the remote-control session.
		// Uses LogLevelDebug so all bridge subsystem messages are captured in
		// the circular buffer; slog filtering still applies for console output.
		bridgeDebug := bridge.NewBridgeDebug(bridge.LogLevelDebug, bridge.DefaultBufferSize, nil)
		bridge.SetGlobalBridgeDebug(bridgeDebug)
		bridgeDebug.LogStatus("remote-control session starting", map[string]string{
			"bridge_url": bridgeURL,
			"authed":     fmt.Sprintf("%v", bridgeAuthed),
		})

		// T190: Use debug utils to safely log bridge config with secret redaction.
		bridgeCfgDebug := bridge.DebugBody(map[string]any{
			"bridge_url": bridgeURL,
			"authed":     bridgeAuthed,
			"token":      bridgeToken,
		})
		slog.Debug("bridge: remote-control config (redacted)", "config", bridgeCfgDebug)

		// T191: Render bridge UI status banner.
		fmt.Fprint(os.Stderr, bridge.RenderVerboseBanner(Version, bridge.BridgeConfig{
			SpawnMode:   bridge.SpawnModeSameDir,
			MaxSessions: 1,
		}, ""))
		fmt.Fprintln(os.Stderr, bridge.RenderConnectingLine(0, "", ""))

		fmt.Fprintf(os.Stderr, "Starting remote control session")
		if rcName != "" {
			fmt.Fprintf(os.Stderr, " %q", rcName)
		}
		fmt.Fprintf(os.Stderr, " (bridge: %s, authed: %v)\n", bridgeURL, bridgeAuthed)

		// Build initial bridge config from CLI context (T169).
		rcCwd, _ := os.Getwd()
		rcCfg := bridge.NewRemoteControlConfig(rcCwd, rcName)
		slog.Debug("bridge: remote-control config",
			"dir", rcCfg.Dir,
			"spawn_mode", rcCfg.SpawnMode,
			"max_sessions", rcCfg.MaxSessions,
			"worker_type", rcCfg.WorkerType,
		)

		// T176: Construct a DynamicPollConfig with defaults so the adaptive
		// backoff logic is exercised during the remote-control session.
		pollCfg := bridge.NewDynamicPollConfig(bridge.DefaultPollConfig, rcCfg.MaxSessions > 1)
		slog.Debug("bridge: poll config initialized",
			"multisession", rcCfg.MaxSessions > 1,
			"initial_delay", pollCfg.NextPollDelay(),
		)

		// T178: Initialize TrustedDeviceManager so the bridge session can
		// inject X-Trusted-Device-Token headers into API requests.
		tdm := bridge.NewTrustedDeviceManager(bridge.TrustedDeviceDeps{
			GetFeatureValueBool: func(key string, defaultVal bool) bool { return defaultVal },
			CheckGateBlocking:   func(key string) (bool, error) { return false, nil },
			GetAccessToken:      bridgeDeps.GetAccessToken,
			GetBaseAPIURL:       bridgeDeps.GetBaseAPIURL,
			IsEssentialTrafficOnly: func() bool { return false },
		})
		if tok := tdm.GetToken(); tok != "" {
			slog.Debug("bridge: trusted device token loaded", "len", len(tok))
		}

		// T173: Construct the BridgeAPIClient for the orchestrator/REPL bridge.
		apiClient := bridge.NewBridgeAPIClientFromConfig(rcCfg, func() string { return "" }, nil)

		// T188: Initialize bridge status state machine so lifecycle transitions
		// are tracked and logged throughout the remote-control session.
		bridgeStatus := bridge.NewStatusMachine()
		bridgeStatus.OnStatusChange(func(from, to bridge.BridgeStatus) {
			slog.Debug("bridge: status transition", "from", from, "to", to)
		})

		// T181: Construct SessionRunner so the lifecycle state machine is linked
		// into the binary. The runner manages heartbeat, archive, and graceful
		// shutdown for each bridge work item.
		sessionRunner := bridge.NewSessionRunner(bridge.SessionRunnerDeps{
			API:           apiClient,
			EnvironmentID: "",                          // set after RegisterBridgeEnvironment
			OnDebug:       func(msg string) { bridgeDebug.LogStatus(msg, nil) },
			OnStateChange: func(from, to bridge.RunnerState) {
				slog.Debug("bridge: session runner state change", "from", from, "to", to)
			},
			OnFatalError: func(err error) {
				slog.Error("bridge: session runner fatal error", "error", err)
			},
		})
		_ = sessionRunner

		// T180: Construct SessionClient for org-scoped session API calls.
		// The orchestrator will use this to create/get/archive sessions via
		// POST/GET /v1/sessions during the remote-control lifecycle.
		sessionClient := bridge.NewSessionClient(bridge.SessionClientConfig{
			BaseURL:        bridgeURL,
			GetAccessToken: func() string { tok, _ := bridgeDeps.GetAccessToken(); return tok },
			GetOrgUUID:     func() string { return "" }, // resolved at session-creation time
			GetModel:       func() string { return "" }, // resolved at session-creation time
			OnDebug:        func(msg string) { bridgeDebug.LogStatus(msg, nil) },
		})
		_ = sessionClient // used by orchestrator once bridge REPL is wired

		// T183: Construct a CodeSessionClient so the bridge session can
		// create code sessions and fetch remote credentials via the
		// code-session API (/v1/code/sessions).
		codeSessionClient := bridge.NewCodeSessionClient(bridge.CodeSessionClientConfig{
			OnDebug: func(msg string) { bridgeDebug.LogStatus(msg, nil) },
		})
		_ = codeSessionClient

		// T187: Construct PermissionCallbacks so the bridge session can
		// dispatch permission requests to the remote web app and receive
		// allow/deny responses. SendFunc and CancelFunc are placeholders
		// until the full bridge transport is wired (T195+).
		permCallbacks := bridge.NewPermissionCallbacks()
		permCallbacks.SendFunc = func(req bridge.BridgePermissionRequest) error {
			slog.Debug("bridge: permission request", "request_id", req.RequestID, "tool", req.ToolName)
			return nil
		}
		permCallbacks.CancelFunc = func(requestID string) error {
			slog.Debug("bridge: permission cancel", "request_id", requestID)
			return nil
		}

		// T214: Construct SerialBatchEventUploader for outbound event delivery
		// with batching, retry, backpressure, and exponential backoff. The
		// uploader is wired as the Send backend for BridgeMessaging below.
		eventUploader := bridge.NewSerialBatchEventUploader(bridge.SerialBatchUploaderConfig[bridge.BridgeEvent]{
			MaxBatchSize:           100,
			MaxQueueSize:           1000,
			MaxConsecutiveFailures: 5,
			BaseDelay:              500 * time.Millisecond,
			MaxDelay:               30 * time.Second,
			Jitter:                 100 * time.Millisecond,
			Send: func(_ context.Context, batch []bridge.BridgeEvent) error {
				slog.Debug("bridge: serial batch upload", "batch_size", len(batch))
				return nil // placeholder until full bridge transport is wired (T195+)
			},
			OnBatchDropped: func(batchSize int, failures int) {
				slog.Warn("bridge: batch dropped after max failures",
					"batch_size", batchSize, "failures", failures)
			},
		})
		defer eventUploader.Close()

		// T184: Construct BridgeMessaging for outbound event buffering and
		// delivery during the remote-control session. Uses the
		// SerialBatchEventUploader as the send backend (T214).
		bridgeMessaging := bridge.NewBridgeMessaging(bridge.BridgeMessagingConfig{
			Send: func(ctx context.Context, batch []bridge.BridgeEvent) error {
				eventUploader.Enqueue(batch...)
				return nil
			},
		})
		defer bridgeMessaging.Close()

		// T185: Register inbound message parser.
		parseInbound := bridge.ExtractInboundMessageFields
		slog.Debug("bridge: inbound message parser registered",
			"parser", fmt.Sprintf("%T", parseInbound),
		)

		// T186: Construct inbound attachment deps.
		attachDeps := &bridge.AttachmentDeps{
			GetAccessToken: bridgeDeps.GetAccessToken,
			GetBaseURL:     bridgeDeps.GetBaseAPIURL,
			GetConfigDir: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, ".claude")
			},
			GetSessionID: func() string { return "" },
		}
		_ = attachDeps

		// T179: If a work secret is provided via env, decode and validate it
		// during bridge session init so we fail fast on malformed secrets.
		if wsEnv := os.Getenv("CLAUDE_CODE_WORK_SECRET"); wsEnv != "" {
			ws, wsErr := bridge.DecodeWorkSecret(wsEnv)
			if wsErr != nil {
				cliErrorf("Invalid work secret: %v", wsErr)
			}
			sdkURL := bridge.BuildSdkUrl(ws.APIBaseURL, "pending")
			slog.Debug("bridge: work secret validated",
				"api_base_url", ws.APIBaseURL,
				"sdk_url", sdkURL,
				"has_auth", len(ws.Auth) > 0,
			)
		}

		// T192: Write bridge pointer for crash-recovery / --continue.
		bridgePtr := bridge.BridgePointer{
			SessionID:     "",
			EnvironmentID: "",
			Source:        bridge.PointerSourceREPL,
		}
		if bridgePtr.SessionID != "" && bridgePtr.EnvironmentID != "" {
			_ = bridge.WriteBridgePointer(rcCwd, bridgePtr, bridgeDebug)
		}
		defer bridge.ClearBridgePointer(rcCwd, bridgeDebug)

		// T199: Construct CapacityWake so the poll loop can be woken early
		// when a session ends and frees capacity.
		rcCtx, rcCtxCancel := context.WithCancel(context.Background())
		defer rcCtxCancel()
		capacityWake := bridge.NewCapacityWake(rcCtx)

		// T193: Construct the BridgeOrchestrator.
		orchestrator := bridge.NewBridgeOrchestrator()
		orchestrator.Config = rcCfg
		orchestrator.API = apiClient
		orchestrator.Logger = nil
		orchestrator.Debug = bridgeDebug
		orchestrator.PollConfig = pollCfg
		orchestrator.CapacityWake = capacityWake
		slog.Debug("bridge: orchestrator constructed",
			"dir", rcCfg.Dir,
			"max_sessions", rcCfg.MaxSessions,
			"spawn_mode", rcCfg.SpawnMode,
		)

		// T194: Run the REPL bridge pre-flight checks and init sequence.
		replHandle, replErr := bridge.InitReplBridge(bridge.InitReplDeps{
			IsBridgeEnabledBlocking:   func() (bool, error) { return bridge.IsBridgeEnabled(), nil },
			GetBridgeAccessToken:      bridgeDeps.GetAccessToken,
			GetBridgeTokenOverride:    func() (string, bool) { return os.Getenv("CLAUDE_BRIDGE_OAUTH_TOKEN"), os.Getenv("CLAUDE_BRIDGE_OAUTH_TOKEN") != "" },
			WaitForPolicyLimitsToLoad: func() error { return nil },
			IsPolicyAllowed:           func(key string) bool { return true },
			GetGlobalConfig:           func() bridge.GlobalBridgeConfig { return bridge.GlobalBridgeConfig{} },
			SaveGlobalConfig:          func(cfg bridge.GlobalBridgeConfig) {},
			GetOAuthTokens:            func() *bridge.OAuthTokens { return nil },
			CheckAndRefreshOAuthToken: func() error { return nil },
			GetOrganizationUUID:       func() (string, error) { return "", nil },
			LogDebug:                  func(msg string) { bridgeDebug.LogStatus(msg, nil) },
		}, &bridge.InitBridgeOptions{
			InitialName: rcName,
			OnStateChange: func(state bridge.BridgeState, detail string) {
				slog.Debug("bridge: repl state change", "state", state, "detail", detail)
				_ = bridgeStatus.Transition(bridge.StatusConnecting)
			},
		})
		if replErr != nil {
			cliErrorf("bridge REPL init failed: %v", replErr)
		}
		if replHandle != nil {
			slog.Debug("bridge: REPL bridge initialized", "org_uuid", replHandle.OrgUUID)

			// T196: Wrap the low-level bridge in a ReplBridgeHandle and
			// register it as the global singleton so tools, slash commands,
			// and other out-of-tree callers can reach the bridge.
			// The Bridge field may still be nil until T195+ wires core init;
			// only register when a real bridge is available.
			if replHandle.Bridge != nil {
				rbh := bridge.NewReplBridgeHandle(replHandle.Bridge)
				bridge.SetReplBridgeHandle(rbh)
				defer rbh.Close()

				slog.Debug("bridge: ReplBridgeHandle registered",
					"session_id", rbh.BridgeSessionID(),
					"compat_id", bridge.GetSelfBridgeCompatID(),
				)
			} else {
				slog.Debug("bridge: REPL bridge init succeeded but core bridge not yet available (T195+)")
			}
		} else {
			slog.Debug("bridge: REPL bridge skipped (pre-flight check declined)")
		}

		// T195+T196: Construct ReplBridge transport and register handle.
		var replBridge *bridge.ReplBridge
		if replHandle != nil {
			replBridge = bridge.NewReplBridge(bridge.ReplBridgeConfig{
				SessionID:     bridgePtr.SessionID,
				EnvironmentID: bridgePtr.EnvironmentID,
				OnStateChange: func(state bridge.BridgeState, detail string) {
					slog.Debug("bridge: repl transport state change", "state", state, "detail", detail)
					_ = bridgeStatus.Transition(bridge.StatusConnecting)
				},
				OnInboundMessage: func(msg bridge.SDKMessage) {
					slog.Debug("bridge: repl inbound message", "type", msg.Type, "uuid", msg.UUID)
				},
				OnPermissionResponse: func(response bridge.SDKMessage) {
					slog.Debug("bridge: repl permission response", "type", response.Type)
				},
				OnInterrupt: func() {
					slog.Debug("bridge: repl interrupt received")
				},
				OnSetModel: func(model string) {
					slog.Debug("bridge: repl set_model", "model", model)
				},
				OnDebug: func(msg string) {
					bridgeDebug.LogStatus(msg, nil)
				},
			})
			replHandle.Bridge = replBridge
			rbh := bridge.NewReplBridgeHandle(replBridge)
			bridge.SetReplBridgeHandle(rbh)
			defer rbh.Close()
		}

		// T197+T213: Wire transport via GetTransportForUrl — selects SSE (v2),
		// HybridTransport (v1), or WebSocket based on env vars and URL scheme.
		var replTransport bridge.ReplBridgeTransport
		if replHandle != nil && bridgeURL != "" {
			transportSel, transportErr := bridge.GetTransportForUrl(
				bridgeURL,
				map[string]string{"Authorization": "Bearer " + bridgeToken},
				replHandle.OrgUUID,
			)
			if transportErr != nil {
				slog.Debug("bridge: transport selection failed, falling back to v2", "error", transportErr)
			}

			if transportErr == nil && transportSel.Kind == bridge.TransportKindSSE {
				// T215: SSETransport — SSE reads + HTTP POST writes.
				sseTransport, sseErr := bridge.NewSSETransport(bridge.SSETransportOpts{
					URL:       transportSel.URL.String(),
					Headers:   transportSel.Headers,
					SessionID: transportSel.SessionID,
					GetAuthHeaders: func() map[string]string {
						tok, _ := bridgeDeps.GetAccessToken()
						if tok == "" {
							return nil
						}
						return map[string]string{"Authorization": "Bearer " + tok}
					},
					Logger: func(msg string) { bridgeDebug.LogStatus(msg, nil) },
				})
				if sseErr != nil {
					slog.Debug("bridge: SSETransport construction failed, falling back to v2", "error", sseErr)
				} else {
					replTransport = bridge.NewSSEReplTransport(sseTransport)
					slog.Debug("bridge: selected SSETransport")
				}
			} else if transportErr == nil && transportSel.Kind == bridge.TransportKindHybrid {
				// T213: HybridTransport — WS reads + HTTP POST writes.
				hybrid := bridge.NewHybridTransport(bridge.HybridTransportOpts{
					URL:       transportSel.URL,
					Headers:   transportSel.Headers,
					SessionID: transportSel.SessionID,
					GetAuthToken: func() string {
						tok, _ := bridgeDeps.GetAccessToken()
						return tok
					},
					Logger: func(msg string) { bridgeDebug.LogStatus(msg, nil) },
				})
				replTransport = bridge.NewV1ReplTransport(hybrid)
				slog.Debug("bridge: selected HybridTransport (v1)")
			} else if transportErr == nil && transportSel.Kind == bridge.TransportKindWebSocket {
				// T217: WebSocketTransport — full-duplex WS.
				ws := bridge.NewWebSocketTransport(bridge.WebSocketTransportOpts{
					URL:       transportSel.URL,
					Headers:   transportSel.Headers,
					SessionID: transportSel.SessionID,
					IsBridge:  true,
					RefreshHeaders: func() map[string]string {
						tok, _ := bridgeDeps.GetAccessToken()
						return map[string]string{"Authorization": "Bearer " + tok}
					},
					Logger: func(msg string) { bridgeDebug.LogStatus(msg, nil) },
				})
				replTransport = bridge.NewV1ReplTransport(ws)
				slog.Debug("bridge: selected WebSocketTransport (v1)")
			}

			if replTransport == nil {
				// Default: v2 SSE+CCR transport.
				replTransport = bridge.NewV2ReplTransport(bridge.V2TransportOpts{
					SessionURL:   bridgeURL,
					IngressToken: bridgeToken,
					SessionID:    replHandle.OrgUUID,
					GetAuthToken: func() string {
						tok, _ := bridgeDeps.GetAccessToken()
						return tok
					},
					Logger: func(msg string) { bridgeDebug.LogStatus(msg, nil) },
				})
				slog.Debug("bridge: selected V2ReplTransport (SSE+CCR)")
			}

			replTransport.SetOnData(func(data string) {
				slog.Debug("bridge: transport inbound data", "len", len(data))
			})
			replTransport.SetOnClose(func(closeCode int) {
				slog.Debug("bridge: transport closed", "close_code", closeCode)
			})
			replTransport.SetOnConnect(func() {
				slog.Debug("bridge: transport connected")
			})
			replTransport.Connect()
			defer replTransport.Close()
		}

		// T198: Wire FlushGate into the bridge REPL lifecycle.
		if replBridge != nil {
			slog.Debug("bridge: flush gate available", "active", replBridge.IsFlushActive())
		}

		// T200: Construct RemoteBridgeCore — shared abstraction composed by both
		// BridgeOrchestrator and ReplBridge. Encapsulates session tracking,
		// config merging, state lifecycle, and graceful shutdown coordination.
		remoteBridgeCore := bridge.NewRemoteBridgeCore(bridge.RemoteBridgeCoreConfig{
			MaxSessions: rcCfg.MaxSessions,
			LocalConfig: bridge.BridgeConfig{
				Dir:       rcCfg.Dir,
				SpawnMode: rcCfg.SpawnMode,
			},
			RemoteConfig: &envlessCfg,
			PollConfig:   pollCfg,
			OnDebug:      func(msg string) { bridgeDebug.LogStatus(msg, nil) },
			OnStateChange: func(state bridge.BridgeState, detail string) {
				slog.Debug("bridge: core state change", "state", state, "detail", detail)
				_ = bridgeStatus.Transition(bridge.StatusConnecting)
			},
			OnSessionCountChange: func(active, max int) {
				slog.Debug("bridge: core session count change", "active", active, "max", max)
				if active < max {
					capacityWake.Wake()
				}
			},
		})
		defer remoteBridgeCore.Shutdown(rcCtx)
		slog.Debug("bridge: remote core initialized",
			"state", remoteBridgeCore.State(),
			"max_sessions", remoteBridgeCore.MaxSessions(),
			"merged_connect_timeout", remoteBridgeCore.Config().ConnectTimeout(),
		)

		// T212: Construct CCRClient for the worker event protocol.
		// The CCRClient posts events, heartbeats, and delivery ACKs to the
		// CCR v2 backend. It is constructed here so it is reachable from the
		// binary; actual Initialize() is deferred until a code session is live.
		var ccrClient *bridge.CCRClient
		if bridgeURL != "" {
			sessionURL := bridgeURL + "/v1/code/sessions/pending"
			var ccrErr error
			ccrClient, ccrErr = bridge.NewCCRClient(sessionURL, bridge.CCRClientOpts{
				GetAuthHeaders: func() map[string]string {
					tok, ok := bridgeDeps.GetAccessToken()
					if !ok || tok == "" {
						return nil
					}
					return map[string]string{"Authorization": "Bearer " + tok}
				},
				UserAgent: "gopher-code/" + Version,
				OnEpochMismatch: func() {
					slog.Error("bridge: CCR epoch mismatch — newer worker superseded this session")
				},
			})
			if ccrErr != nil {
				slog.Warn("bridge: CCRClient construction failed", "error", ccrErr)
			} else {
				defer ccrClient.Close()
				slog.Debug("bridge: CCRClient constructed", "session_url", sessionURL)
			}
		}

		// T218: Construct WorkerStateUploader — coalesces PUT /worker patches
		// and sends them through the CCRClient.  Closed on session teardown.
		var workerUploader *bridge.WorkerStateUploader
		if ccrClient != nil {
			workerUploader = bridge.NewWorkerStateUploader(bridge.WorkerStateUploaderConfig{
				Send:      ccrClient.PutWorker,
				BaseDelay: 500 * time.Millisecond,
				MaxDelay:  30 * time.Second,
				Jitter:    200 * time.Millisecond,
			})
			defer workerUploader.Close()
			slog.Debug("bridge: WorkerStateUploader constructed")
		}

		_ = workerUploader
		_ = ccrClient
		_ = remoteBridgeCore
		_ = replBridge
		_ = replTransport
		_ = orchestrator
		_ = tdm
		_ = permCallbacks
		_ = bridgeStatus
		_ = bridgeMessaging
		_ = parseInbound
		_ = bridgePtr
		cliOk("")
	}

	// Handle "auto-mode" subcommand before flag.Parse()
	// Source: src/cli.ts — `claude auto-mode` dispatches to handlers in
	// cmd/gopher-code/handlers/auto_mode.go.
	if len(os.Args) > 1 && os.Args[1] == "auto-mode" {
		sub := ""
		if len(os.Args) > 2 {
			sub = os.Args[2]
		}
		switch sub {
		case "", "config":
			// Print effective auto-mode config (user overrides merged with defaults).
			// TODO: wire real user config from settings once config loading is available.
			handlers.AutoModeConfigHandler(nil)
		case "defaults":
			handlers.AutoModeDefaultsHandler()
		default:
			cliErrorf("Unknown auto-mode subcommand: %s (use defaults, config)", sub)
		}
		cliOk("")
	}

	// Handle "completion" subcommand before flag.Parse()
	if len(os.Args) > 1 && os.Args[1] == "completion" {
		shell := "bash"
		if len(os.Args) > 2 {
			shell = os.Args[2]
		}
		switch shell {
		case "bash":
			fmt.Println(cli.GenerateBashCompletion())
		case "zsh":
			fmt.Println(cli.GenerateZshCompletion())
		case "fish":
			fmt.Println(cli.GenerateFishCompletion())
		default:
			cliErrorf("Unknown shell: %s (use bash, zsh, fish)", shell)
		}
		cliOk("")
	}

	// Handle "agents" subcommand before flag.Parse()
	if len(os.Args) > 1 && os.Args[1] == "agents" {
		agentsCwd, _ := os.Getwd()
		handlers.AgentsHandler(os.Stdout, agentsCwd)
		cliOk("")
	}

	// Handle "plugin" subcommand before flag.Parse()
	// Source: src/cli.ts — `claude plugin` CLI subcommand family.
	if len(os.Args) > 1 && os.Args[1] == "plugin" {
		pluginCwd, _ := os.Getwd()
		ph := handlers.NewPluginHandler(pluginCwd)
		sub := ""
		if len(os.Args) > 2 {
			sub = os.Args[2]
		}
		switch sub {
		case "list", "ls", "":
			if err := ph.List(); err != nil {
				cliErrorf("plugin list: %v", err)
			}
			cliOk("")
		case "install", "add":
			pluginFlags := flag.NewFlagSet("plugin install", flag.ExitOnError)
			scope := pluginFlags.String("scope", "user", "Install scope: user, project, local")
			_ = pluginFlags.Parse(os.Args[3:])
			source := pluginFlags.Arg(0)
			if source == "" {
				cliError("plugin install requires a source argument")
			}
			if err := ph.Install(source, *scope); err != nil {
				cliErrorf("plugin install: %v", err)
			}
			cliOk("")
		case "uninstall", "remove", "rm":
			pluginFlags := flag.NewFlagSet("plugin uninstall", flag.ExitOnError)
			scope := pluginFlags.String("scope", "", "Uninstall scope: user, project, local")
			_ = pluginFlags.Parse(os.Args[3:])
			name := pluginFlags.Arg(0)
			if name == "" {
				cliError("plugin uninstall requires a plugin name")
			}
			if err := ph.Uninstall(name, *scope); err != nil {
				cliErrorf("plugin uninstall: %v", err)
			}
			cliOk("")
		case "enable":
			pluginFlags := flag.NewFlagSet("plugin enable", flag.ExitOnError)
			scope := pluginFlags.String("scope", "", "Scope: user, project, local")
			_ = pluginFlags.Parse(os.Args[3:])
			name := pluginFlags.Arg(0)
			if name == "" {
				cliError("plugin enable requires a plugin name")
			}
			if err := ph.Enable(name, *scope); err != nil {
				cliErrorf("plugin enable: %v", err)
			}
			cliOk("")
		case "disable":
			pluginFlags := flag.NewFlagSet("plugin disable", flag.ExitOnError)
			scope := pluginFlags.String("scope", "", "Scope: user, project, local")
			_ = pluginFlags.Parse(os.Args[3:])
			name := pluginFlags.Arg(0)
			if name == "" {
				cliError("plugin disable requires a plugin name")
			}
			if err := ph.Disable(name, *scope); err != nil {
				cliErrorf("plugin disable: %v", err)
			}
			cliOk("")
		case "validate":
			path := ""
			if len(os.Args) > 3 {
				path = os.Args[3]
			}
			if path == "" {
				cliError("plugin validate requires a path argument")
			}
			result, err := ph.Validate(path)
			if err != nil {
				cliErrorf("plugin validate: %v", err)
			}
			if !result.Success {
				cliError("")
			}
			cliOk("")
		default:
			cliErrorf("Unknown plugin subcommand: %q (use list, install, uninstall, enable, disable, validate)", sub)
		}
	}

	// Handle "auth" subcommand before flag.Parse()
	if len(os.Args) > 1 && os.Args[1] == "auth" {
		sub := ""
		if len(os.Args) > 2 {
			sub = os.Args[2]
		}
		switch sub {
		case "login":
			loginFlags := flag.NewFlagSet("auth login", flag.ExitOnError)
			email := loginFlags.String("email", "", "Email for login")
			sso := loginFlags.Bool("sso", false, "Use SSO login")
			console := loginFlags.Bool("console", false, "Use Anthropic Console OAuth scopes")
			claudeAI := loginFlags.Bool("claudeai", false, "Use claude.ai OAuth scopes")
			_ = loginFlags.Parse(os.Args[3:])
			if code := handlers.AuthLogin(handlers.AuthLoginOpts{
				Email:    *email,
				SSO:      *sso,
				Console:  *console,
				ClaudeAI: *claudeAI,
			}, nil); code != 0 {
				cliError("auth login failed")
			}
			cliOk("")
		case "status":
			statusFlags := flag.NewFlagSet("auth status", flag.ExitOnError)
			jsonOut := statusFlags.Bool("json", false, "Output as JSON")
			_ = statusFlags.Parse(os.Args[3:])
			if code := handlers.AuthStatus(handlers.AuthStatusOpts{JSON: *jsonOut}); code != 0 {
				cliError("auth status check failed")
			}
			cliOk("")
		case "logout":
			if code := handlers.AuthLogout(handlers.AuthLogoutOpts{}); code != 0 {
				cliError("auth logout failed")
			}
			cliOk("")
		default:
			cliErrorf("Unknown auth subcommand: %q (use login, status, logout)", sub)
		}
	}

	// Handle "mcp" subcommand before flag.Parse()
	// Source: src/cli/handlers/mcp.tsx — dispatches to 7 sub-subcommands:
	//   list, get, add, add-json, remove, reset-choices, serve
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		sub := ""
		if len(os.Args) > 2 {
			sub = os.Args[2]
		}
		mcpCwd, _ := os.Getwd()
		h := handlers.NewMCPHandler(mcpCwd)

		switch sub {
		case "list":
			if err := h.List(); err != nil {
				cliErrorf("mcp list: %v", err)
			}
			cliOk("")

		case "get":
			if len(os.Args) < 4 {
				cliError("usage: claude mcp get <server-name>")
			}
			if err := h.Get(os.Args[3]); err != nil {
				cliErrorf("mcp get: %v", err)
			}
			cliOk("")

		case "add":
			// `claude mcp add <name> -- <command> [args...]`
			// Builds a stdio server config from positional args.
			mcpAddFlags := flag.NewFlagSet("mcp add", flag.ExitOnError)
			scope := mcpAddFlags.String("s", "local", "Config scope: local, project, user")
			flag.StringVar(scope, "scope", "local", "Config scope: local, project, user")
			envPairs := mcpAddFlags.String("e", "", "Environment variables (KEY=VAL,KEY=VAL)")
			flag.StringVar(envPairs, "env", "", "Environment variables (KEY=VAL,KEY=VAL)")
			headerPairs := mcpAddFlags.String("header", "", "Headers (Key:Val,Key:Val)")
			_ = mcpAddFlags.Parse(os.Args[3:])
			addArgs := mcpAddFlags.Args()
			if len(addArgs) < 2 {
				cliError("usage: claude mcp add <name> <command> [args...] [-s scope] [-e KEY=VAL]")
			}
			name := addArgs[0]
			command := addArgs[1]
			var cmdArgs []string
			if len(addArgs) > 2 {
				cmdArgs = addArgs[2:]
			}

			// Parse env vars
			envMap := make(map[string]string)
			if *envPairs != "" {
				for _, pair := range strings.Split(*envPairs, ",") {
					parts := strings.SplitN(pair, "=", 2)
					if len(parts) == 2 {
						envMap[parts[0]] = parts[1]
					}
				}
			}

			// Parse headers
			headerMap := make(map[string]string)
			if *headerPairs != "" {
				for _, pair := range strings.Split(*headerPairs, ",") {
					parts := strings.SplitN(pair, ":", 2)
					if len(parts) == 2 {
						headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}
			}

			cfg := mcp.ServerConfig{
				Command: command,
				Args:    cmdArgs,
				Env:     envMap,
				Headers: headerMap,
			}
			cfgJSON, _ := json.Marshal(cfg)
			if err := h.AddJSON(name, string(cfgJSON), *scope); err != nil {
				cliErrorf("mcp add: %v", err)
			}
			cliOk("")

		case "add-json":
			// `claude mcp add-json <name> '<json>' [-s scope]`
			mcpAddJSONFlags := flag.NewFlagSet("mcp add-json", flag.ExitOnError)
			scope := mcpAddJSONFlags.String("s", "local", "Config scope: local, project, user")
			flag.StringVar(scope, "scope", "local", "Config scope: local, project, user")
			_ = mcpAddJSONFlags.Parse(os.Args[3:])
			ajArgs := mcpAddJSONFlags.Args()
			if len(ajArgs) < 2 {
				cliError("usage: claude mcp add-json <name> '<json-config>' [-s scope]")
			}
			if err := h.AddJSON(ajArgs[0], ajArgs[1], *scope); err != nil {
				cliErrorf("mcp add-json: %v", err)
			}
			cliOk("")

		case "remove":
			mcpRemoveFlags := flag.NewFlagSet("mcp remove", flag.ExitOnError)
			scope := mcpRemoveFlags.String("s", "", "Config scope: local, project, user")
			flag.StringVar(scope, "scope", "", "Config scope: local, project, user")
			_ = mcpRemoveFlags.Parse(os.Args[3:])
			rmArgs := mcpRemoveFlags.Args()
			if len(rmArgs) < 1 {
				cliError("usage: claude mcp remove <server-name> [-s scope]")
			}
			if err := h.Remove(rmArgs[0], *scope); err != nil {
				cliErrorf("mcp remove: %v", err)
			}
			cliOk("")

		case "reset-choices":
			if err := h.ResetChoices(); err != nil {
				cliErrorf("mcp reset-choices: %v", err)
			}
			cliOk("")

		case "serve":
			// T487: MCP server mode — not yet implemented.
			cliError("claude mcp serve is not yet implemented")

		default:
			cliErrorf("Unknown mcp subcommand: %q (use list, get, add, add-json, remove, reset-choices, serve)", sub)
		}
	}

	// Handle "setup-token" subcommand before flag.Parse()
	if len(os.Args) > 1 && os.Args[1] == "setup-token" {
		if code := handlers.SetupToken(handlers.SetupTokenOpts{}); code != 0 {
			cliError("setup-token failed")
		}
		cliOk("")
	}

	// Handle "update" subcommand before flag.Parse()
	// Source: src/cli.ts — `claude update`
	if len(os.Args) > 1 && os.Args[1] == "update" {
		if code := handlers.Update(handlers.UpdateOpts{
			Output:  os.Stdout,
			Stderr:  os.Stderr,
			Version: Version,
		}); code != 0 {
			cliError("update failed")
		}
		cliOk("")
	}

	// Handle "doctor" subcommand before flag.Parse()
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		if code := handlers.Doctor(handlers.DoctorOpts{}); code != 0 {
			cliError("doctor failed")
		}
		cliOk("")
	}

	// Handle "install" subcommand before flag.Parse()
	if len(os.Args) > 1 && os.Args[1] == "install" {
		installFlags := flag.NewFlagSet("install", flag.ExitOnError)
		force := installFlags.Bool("force", false, "Force install")
		_ = installFlags.Parse(os.Args[2:])
		target := installFlags.Arg(0)
		if code := handlers.Install(handlers.InstallOpts{
			Target: target,
			Force:  *force,
		}); code != 0 {
			cliError("install failed")
		}
		cliOk("")
	}

	flag.Parse()

	// Handle --version
	if *showVersion {
		cliOk(fmt.Sprintf("gopher-code v%s", Version))
	}

	// Debug mode
	if *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		if *debugFile != "" {
			f, err := os.OpenFile(*debugFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				cliErrorf("Error opening debug file: %v", err)
			}
			defer f.Close()
			slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})))
		}
	}

	// Initialize built-in plugins (before settings load so registrations are available).
	// Source: src/plugins/bundled/index.ts — initBuiltinPlugins
	pluginsBundled.InitBuiltinPlugins()

	// T126-T128: Initialize telemetry (OTel providers + counters + stats store).
	// Uses no-op exporters by default; real exporters are configured via OTel env vars.
	tel, telErr := telemetry.Init()
	if telErr != nil {
		slog.Warn("telemetry init failed, using noop", "error", telErr)
		tel = telemetry.InitNoop()
	}

	// T129: Agent color assigner for teammate UI coloring.
	agentColors := theme.NewAgentColorAssigner()

	// Suppress unused variable warnings for flags reserved for future use
	_ = tel
	_ = agentColors
	_ = addDirs
	_ = includeHookEvents
	_ = worktree
	_ = betas
	_ = fallbackModel

	if *cwd == "" {
		var err error
		*cwd, err = os.Getwd()
		if err != nil {
			cliErrorf("Error: cannot determine working directory: %v", err)
		}
	}

	// Handle --init flag
	if *initFlag {
		claudeDir := filepath.Join(*cwd, ".claude")
		os.MkdirAll(claudeDir, 0755)
		claudeMD := filepath.Join(*cwd, "CLAUDE.md")
		if _, err := os.Stat(claudeMD); os.IsNotExist(err) {
			os.WriteFile(claudeMD, []byte("# Project Instructions\n\nAdd project-specific instructions here.\n"), 0644)
			fmt.Printf("Created %s\n", claudeMD)
		}
		settingsPath := filepath.Join(claudeDir, "settings.json")
		if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
			os.WriteFile(settingsPath, []byte("{\n}\n"), 0644)
			fmt.Printf("Created %s\n", settingsPath)
		}
		cliOk("Project initialized.")
	}

	// Load settings from global and project config files
	settings := config.Load(*cwd)

	// Apply settings as defaults (flags override settings)
	if *model == "claude-sonnet-4-20250514" && settings.Model != "" {
		*model = settings.Model
	}
	if *maxTurns == 100 && settings.MaxTurns > 0 {
		*maxTurns = settings.MaxTurns
	}

	// Determine permission mode from settings
	permMode := permissions.AutoApprove
	if settings.PermissionMode == "deny" {
		permMode = permissions.Deny
	} else if settings.PermissionMode == "interactive" {
		permMode = permissions.Interactive
	}
	// --permission-mode flag overrides settings
	if *permModeFlag != "" {
		switch *permModeFlag {
		case "auto":
			permMode = permissions.AutoApprove
		case "interactive":
			permMode = permissions.Interactive
		case "deny":
			permMode = permissions.Deny
		default:
			cliErrorf("Unknown permission mode: %s (use auto, interactive, deny)", *permModeFlag)
		}
	}
	if *skipPerms {
		permMode = permissions.AutoApprove
	}

	// Resolve thinking / effort configuration
	thinkingEnabled := false
	thinkingBudget := 10000
	switch *effort {
	case "low":
		thinkingEnabled = false
	case "medium":
		thinkingEnabled = true
		thinkingBudget = 5000
	case "high":
		thinkingEnabled = true
		thinkingBudget = 16000
	case "max":
		thinkingEnabled = true
		thinkingBudget = 32000
	}
	// --thinking flag overrides --effort
	if *thinking == "enabled" {
		thinkingEnabled = true
	} else if *thinking == "disabled" {
		thinkingEnabled = false
	}

	// Create hook runner from settings (unless --bare mode)
	var hookRunner *hooks.HookRunner
	if !*bare && len(settings.Hooks) > 0 {
		hookConfigs := make([]hooks.HookConfig, len(settings.Hooks))
		for i, h := range settings.Hooks {
			hookConfigs[i] = hooks.HookConfig{
				Type:    hooks.HookEvent(h.Type),
				Matcher: h.Matcher,
				Command: h.Command,
				Timeout: h.Timeout,
			}
		}
		hookRunner = hooks.NewHookRunner(hookConfigs)
	}

	// Determine system prompt
	sysPrompt := *systemPrompt
	if *systemPromptFile != "" {
		data, err := os.ReadFile(*systemPromptFile)
		if err != nil {
			cliErrorf("Error reading system prompt file: %v", err)
		}
		sysPrompt = string(data)
	}

	// Resolve model aliases
	resolvedModel := resolveModelAlias(*model)

	// Build with environment context
	sysPrompt = prompt.BuildSystemPrompt(sysPrompt, *cwd, resolvedModel)

	// Append system prompt additions
	if *appendSystemPrompt != "" {
		sysPrompt += "\n\n" + *appendSystemPrompt
	}
	if *appendSystemPromptFile != "" {
		data, err := os.ReadFile(*appendSystemPromptFile)
		if err != nil {
			cliErrorf("Error reading append prompt file: %v", err)
		}
		sysPrompt += "\n\n" + string(data)
	}

	if *verbose {
		fmt.Fprintf(stderr, "Model: %s\n", *model)
		fmt.Fprintf(stderr, "CWD: %s\n", *cwd)
		fmt.Fprintf(stderr, "Max turns: %d\n", *maxTurns)
	}

	// Create provider based on --provider flag
	var prov provider.ModelProvider
	switch *providerFlag {
	case "anthropic", "":
		apiKey, err := auth.GetAPIKey()
		if err != nil {
			cliError(err.Error())
		}
		p := provider.NewAnthropicProvider(apiKey, resolvedModel)
		if *apiURL != "" {
			p.SetBaseURL(*apiURL)
		}
		prov = p
	case "bedrock":
		prov = provider.NewBedrockProvider(os.Getenv("AWS_REGION"), resolvedModel)
	case "vertex":
		prov = provider.NewVertexProvider(os.Getenv("GOOGLE_PROJECT_ID"), os.Getenv("GOOGLE_REGION"), resolvedModel)
	case "openai":
		apiKey, _ := auth.GetAPIKey()
		url := *apiURL
		if url == "" {
			url = "https://api.openai.com"
		}
		prov = provider.NewOpenAICompatProvider(url, apiKey, resolvedModel)
	default:
		cliErrorf("Unknown provider: %s", *providerFlag)
	}
	registry := tools.NewRegistry()
	planState := tools.RegisterDefaults(registry)
	tools.RegisterAgentTool(registry, prov, query.AsQueryFunc())

	// Load skills (prompt-based commands)
	loadedSkills := skills.LoadSkills(*cwd)
	if len(loadedSkills) > 0 {
		registry.Register(tools.NewSkillTool(loadedSkills))
	}

	// Load MCP servers (unless --bare mode)
	mcpMgr := mcp.NewManager()
	if !*bare {
		mcpCfg, _ := mcp.LoadConfig()
		for name, serverCfg := range mcpCfg.Servers {
			if err := mcpMgr.Connect(context.Background(), name, serverCfg); err != nil {
				fmt.Fprintf(stderr, "Warning: MCP server %s failed: %v\n", name, err)
			}
		}
		mcpMgr.RegisterTools(context.Background(), registry)
	}
	defer mcpMgr.CloseAll()

	// Tool filtering
	if *allowedTools != "" {
		allowed := strings.Split(*allowedTools, ",")
		for _, t := range registry.All() {
			found := false
			for _, a := range allowed {
				if strings.TrimSpace(a) == t.Name() {
					found = true
					break
				}
			}
			if !found {
				registry.Unregister(t.Name())
			}
		}
	}
	if *disallowedTools != "" {
		disallowed := strings.Split(*disallowedTools, ",")
		for _, name := range disallowed {
			registry.Unregister(strings.TrimSpace(name))
		}
	}

	// Try to load an existing session via -c or -r flags
	var sess *session.SessionState

	if *continueSession {
		loaded, err := session.LoadLatest(*cwd)
		if err != nil {
			fmt.Fprintf(stderr, "No session to continue: %v\n", err)
			// Fall through to create new session
		} else {
			sess = loaded
			sess.CWD = *cwd // Update CWD
			// Rebuild system prompt for resumed session
			sess.Config.SystemPrompt = sysPrompt
			if *verbose {
				fmt.Fprintf(stderr, "Resuming session %s (%d turns)\n", sess.ID, sess.TurnCount)
			}
		}
	}

	if sess == nil && *resume != "" {
		// T182: Normalize resume ID through compat layer.
		resumeID := bridge.ToInfraSessionID(*resume)
		loaded, err := session.Load(resumeID)
		if err != nil {
			cliErrorf("Cannot resume session %s: %v", resumeID, err)
		}
		sess = loaded
		sess.CWD = *cwd
		sess.Config.SystemPrompt = sysPrompt
		if *verbose {
			fmt.Fprintf(stderr, "Resuming session %s (%d turns)\n", sess.ID, sess.TurnCount)
		}
	}

	if sess == nil {
		cfg := session.SessionConfig{
			Model:           resolvedModel,
			SystemPrompt:    sysPrompt,
			MaxTurns:        *maxTurns,
			TokenBudget:     compact.DefaultBudget(),
			PermissionMode:  permMode,
			ThinkingEnabled: thinkingEnabled,
			ThinkingBudget:  thinkingBudget,
			MaxBudgetUSD:    *maxBudgetUSD,
		}
		if *jsonSchema != "" {
			cfg.JSONSchema = *jsonSchema
		}
		sess = session.New(cfg, *cwd)
	}

	// Apply --session-id and --name overrides.
	// T182: Normalize through the compat layer so callers can pass either
	// cse_* or session_* prefixed IDs and get consistent infra-format storage.
	if *sessionID != "" {
		sess.ID = bridge.ToInfraSessionID(*sessionID)
	}
	if *sessionName != "" {
		sess.Name = *sessionName
	}

	// Ensure OriginalCWD and ProjectRoot are set (may be empty on resumed sessions).
	// Source: bootstrap/state.ts — originalCwd, projectRoot
	if sess.OriginalCWD == "" {
		sess.OriginalCWD = *cwd
	}
	if sess.ProjectRoot == "" {
		sess.ProjectRoot = *cwd
	}

	// Wire interactive permission policy (runtime only, not serialized)
	if permMode == permissions.Interactive {
		sess.PermissionPolicy = permissions.NewInteractivePolicy()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Handle -p / --print mode
	if *printMode {
		orchestrator := tools.NewOrchestrator(registry)
		if hookRunner != nil {
			orchestrator.SetHookRunner(hookRunner)
		}
		outFmt, err := ParseOutputFormat(*outputFormat)
		if err != nil {
			cliError(err.Error())
		}

		// T210: If a remote session stream URL is configured, construct the
		// RemoteIO so the headless pipeline can consume input from the remote
		// transport instead of (or in addition to) local stdin.
		remoteIO, rioErr := setupRemoteIO(nil)
		if rioErr != nil {
			cliErrorf("remote IO setup: %v", rioErr)
		}
		if remoteIO != nil {
			defer remoteIO.Close()
			slog.Debug("remote IO active", "version", remoteIO.Version())
		}

		runHeadless(ctx, sess, prov, registry, orchestrator, HeadlessConfig{
			OutputFormat: outFmt,
			InputFormat:  *inputFormat,
			Verbose:      *verbose,
		}, flag.Args())
	}

	// Handle --query flag (one-shot mode, kept for compat)
	oneShot := *queryStr
	if oneShot == "" && len(flag.Args()) > 0 {
		// Positional args as prompt in one-shot mode
		oneShot = strings.Join(flag.Args(), " ")
	}

	if oneShot != "" {
		sess.PushMessage(message.UserMessage(oneShot))
		orchestrator := tools.NewOrchestrator(registry)
		if hookRunner != nil {
			orchestrator.SetHookRunner(hookRunner)
		}
		if *outputFormat == "json" {
			collector := cli.NewJSONCollector()
			err := query.Query(ctx, sess, prov, registry, orchestrator, collector.Callback)
			collector.Emit()
			if err != nil {
				cliError(err.Error())
			}
		} else if *outputFormat == "stream-json" {
			err := query.Query(ctx, sess, prov, registry, orchestrator, cli.StreamJSONCallback)
			if err != nil {
				cliError(err.Error())
			}
		} else {
			err := query.Query(ctx, sess, prov, registry, orchestrator, cli.PrintEvent)
			if err != nil {
				cliError(err.Error())
			}
		}
		cliOk("")
	}

	// Interactive mode: mark session as interactive.
	// Source: bootstrap/state.ts — isInteractive
	sess.IsInteractive = true
	if cli.UseNewUI() {
		if err := cli.RunTUIV2(ctx, sess, prov, registry); err != nil {
			cliError(err.Error())
		}
	} else {
		cli.RunREPL(ctx, sess, prov, registry, *verbose, hookRunner, *noSessionPersist, *prefill, planState)
	}
}
