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

	// Handle "remote-control" subcommand before flag.Parse()
	// Source: src/cli.ts — `claude remote-control` CLI subcommand dispatches
	// to the bridge REPL initializer (pkg/bridge/init_repl.go).
	if len(os.Args) > 1 && os.Args[1] == "remote-control" {
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
		_, bridgeAuthed := bridge.BridgeAccessToken(bridgeDeps)

		fmt.Fprintf(os.Stderr, "Starting remote control session")
		if rcName != "" {
			fmt.Fprintf(os.Stderr, " %q", rcName)
		}
		fmt.Fprintf(os.Stderr, " (bridge: %s, authed: %v)\n", bridgeURL, bridgeAuthed)

		// Build initial bridge config from CLI context.
		// This wires the bridge types into the binary dependency tree (T169).
		rcCwd, _ := os.Getwd()
		rcCfg := bridge.NewRemoteControlConfig(rcCwd, rcName)
		slog.Debug("bridge: remote-control config",
			"dir", rcCfg.Dir,
			"spawn_mode", rcCfg.SpawnMode,
			"max_sessions", rcCfg.MaxSessions,
			"worker_type", rcCfg.WorkerType,
		)
		// TODO(T195+): Wire full bridge REPL init once bridge core is implemented.
		// For now, exit cleanly after printing the config.
		_ = rcCfg
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
		loaded, err := session.Load(*resume)
		if err != nil {
			cliErrorf("Cannot resume session %s: %v", *resume, err)
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

	// Apply --session-id and --name overrides
	if *sessionID != "" {
		sess.ID = *sessionID
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
