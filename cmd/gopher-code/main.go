package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

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
			OnDebug:       func(msg string) { slog.Debug(msg) },
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
			OnDebug:        func(msg string) { slog.Debug(msg) },
		})
		_ = sessionClient // used by orchestrator once bridge REPL is wired

		// T183: Construct a CodeSessionClient so the bridge session can
		// create code sessions and fetch remote credentials via the
		// code-session API (/v1/code/sessions).
		codeSessionClient := bridge.NewCodeSessionClient(bridge.CodeSessionClientConfig{
			OnDebug: func(msg string) { slog.Debug(msg) },
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

		// T184: Construct BridgeMessaging for outbound event buffering and
		// delivery during the remote-control session. The SendFunc is a
		// placeholder until the full bridge transport is wired (T195+).
		bridgeMessaging := bridge.NewBridgeMessaging(bridge.BridgeMessagingConfig{
			Send: func(_ context.Context, batch []bridge.BridgeEvent) error {
				slog.Debug("bridge: messaging flush", "batch_size", len(batch))
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

		// TODO(T195+): Wire full bridge REPL init once bridge core is implemented.
		_ = rcCfg
		_ = pollCfg
		_ = tdm
		_ = permCallbacks
		_ = bridgeStatus
		_ = bridgeMessaging
		_ = parseInbound
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
