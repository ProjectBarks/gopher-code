package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/projectbarks/gopher-code/internal/cli"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/config"
	"github.com/projectbarks/gopher-code/pkg/hooks"
	"github.com/projectbarks/gopher-code/pkg/mcp"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/prompt"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

func main() {
	// Existing flags
	model := flag.String("model", "claude-sonnet-4-20250514", "Model to use")
	queryStr := flag.String("query", "", "One-shot query (non-interactive)")
	cwd := flag.String("cwd", "", "Working directory (default: current)")

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

	flag.Parse()

	// Suppress unused variable warning for addDirs (reserved for future use)
	_ = addDirs

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: ANTHROPIC_API_KEY environment variable not set")
		os.Exit(1)
	}

	if *cwd == "" {
		var err error
		*cwd, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot determine working directory: %v\n", err)
			os.Exit(1)
		}
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

	// Create hook runner from settings
	var hookRunner *hooks.HookRunner
	if len(settings.Hooks) > 0 {
		hookConfigs := make([]hooks.HookConfig, len(settings.Hooks))
		for i, h := range settings.Hooks {
			hookConfigs[i] = hooks.HookConfig{
				Type:    hooks.HookType(h.Type),
				Matcher: h.Matcher,
				Command: h.Command,
				Timeout: h.Timeout,
			}
		}
		hookRunner = hooks.NewHookRunner(hookConfigs)
	}

	// Determine system prompt
	sysPrompt := prompt.BuildSystemPrompt(*systemPrompt, *cwd, *model)

	if *verbose {
		fmt.Fprintf(os.Stderr, "Model: %s\n", *model)
		fmt.Fprintf(os.Stderr, "CWD: %s\n", *cwd)
		fmt.Fprintf(os.Stderr, "Max turns: %d\n", *maxTurns)
	}

	// Create provider, registry, session
	prov := provider.NewAnthropicProvider(apiKey, *model)
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)
	tools.RegisterAgentTool(registry, prov, query.AsQueryFunc())

	// Load MCP servers
	mcpMgr := mcp.NewManager()
	mcpCfg, _ := mcp.LoadConfig()
	for name, serverCfg := range mcpCfg.Servers {
		if err := mcpMgr.Connect(context.Background(), name, serverCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: MCP server %s failed: %v\n", name, err)
		}
	}
	mcpMgr.RegisterTools(context.Background(), registry)
	defer mcpMgr.CloseAll()

	// Try to load an existing session via -c or -r flags
	var sess *session.SessionState

	if *continueSession {
		loaded, err := session.LoadLatest(*cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "No session to continue: %v\n", err)
			// Fall through to create new session
		} else {
			sess = loaded
			sess.CWD = *cwd // Update CWD
			// Rebuild system prompt for resumed session
			sess.Config.SystemPrompt = sysPrompt
			if *verbose {
				fmt.Fprintf(os.Stderr, "Resuming session %s (%d turns)\n", sess.ID, sess.TurnCount)
			}
		}
	}

	if sess == nil && *resume != "" {
		loaded, err := session.Load(*resume)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot resume session %s: %v\n", *resume, err)
			os.Exit(1)
		}
		sess = loaded
		sess.CWD = *cwd
		sess.Config.SystemPrompt = sysPrompt
		if *verbose {
			fmt.Fprintf(os.Stderr, "Resuming session %s (%d turns)\n", sess.ID, sess.TurnCount)
		}
	}

	if sess == nil {
		cfg := session.SessionConfig{
			Model:           *model,
			SystemPrompt:    sysPrompt,
			MaxTurns:        *maxTurns,
			TokenBudget:     compact.DefaultBudget(),
			PermissionMode:  permMode,
			ThinkingEnabled: thinkingEnabled,
			ThinkingBudget:  thinkingBudget,
		}
		sess = session.New(cfg, *cwd)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Handle -p / --print mode
	if *printMode {
		prompt := strings.Join(flag.Args(), " ")
		if prompt == "" {
			// Read from stdin
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				os.Exit(1)
			}
			prompt = strings.TrimSpace(string(data))
		}
		if prompt == "" {
			fmt.Fprintln(os.Stderr, "Error: no prompt provided")
			os.Exit(1)
		}

		sess.PushMessage(message.UserMessage(prompt))
		orchestrator := tools.NewOrchestrator(registry)
		if hookRunner != nil {
			orchestrator.SetHookRunner(hookRunner)
		}

		// Select callback based on output format
		var callback query.EventCallback
		switch *outputFormat {
		case "json":
			callback = cli.JSONCallback
		case "stream-json":
			callback = cli.StreamJSONCallback
		default:
			callback = cli.PlainTextCallback
		}

		err := query.Query(ctx, sess, prov, registry, orchestrator, callback)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println() // Final newline
		os.Exit(0)
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
		err := query.Query(ctx, sess, prov, registry, orchestrator, cli.PrintEvent)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Interactive REPL
	cli.RunREPL(ctx, sess, prov, registry, *verbose, hookRunner)
}
