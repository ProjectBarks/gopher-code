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

	flag.Parse()

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

	// Determine system prompt
	sysPrompt := prompt.BuildSystemPrompt(*systemPrompt, *cwd, *model)

	// Determine permission mode
	permMode := permissions.AutoApprove
	if *skipPerms {
		permMode = permissions.AutoApprove // explicit: skip all checks
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Model: %s\n", *model)
		fmt.Fprintf(os.Stderr, "CWD: %s\n", *cwd)
		fmt.Fprintf(os.Stderr, "Max turns: %d\n", *maxTurns)
	}

	// Create provider, registry, session
	prov := provider.NewAnthropicProvider(apiKey, *model)
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	cfg := session.SessionConfig{
		Model:          *model,
		SystemPrompt:   sysPrompt,
		MaxTurns:       *maxTurns,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permMode,
	}
	sess := session.New(cfg, *cwd)

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
		err := query.Query(ctx, sess, prov, registry, orchestrator, cli.PrintEvent)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Log session management flags (placeholders until persistence is implemented)
	if *continueSession {
		if *verbose {
			fmt.Fprintln(os.Stderr, "Note: --continue requires session persistence (not yet implemented)")
		}
	}
	if *resume != "" {
		if *verbose {
			fmt.Fprintf(os.Stderr, "Note: --resume=%s requires session persistence (not yet implemented)\n", *resume)
		}
	}

	// Interactive REPL
	cli.RunREPL(ctx, sess, prov, registry)
}
