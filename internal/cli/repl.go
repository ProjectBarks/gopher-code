package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// RunREPL starts an interactive read-eval-print loop.
func RunREPL(ctx context.Context, sess *session.SessionState, prov provider.ModelProvider, registry *tools.ToolRegistry) {
	scanner := bufio.NewScanner(os.Stdin)
	orchestrator := tools.NewOrchestrator(registry)

	fmt.Println("gopher-code v0.1.0")
	fmt.Printf("Model: %s | CWD: %s\n", sess.Config.Model, sess.CWD)
	fmt.Println("Type your message. Press Ctrl+D to exit.")
	fmt.Println()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Slash commands
		switch {
		case input == "/exit" || input == "/quit":
			return
		case input == "/help":
			printHelp()
			continue
		case input == "/clear":
			sess.Messages = sess.Messages[:0]
			sess.TurnCount = 0
			fmt.Println("Conversation cleared.")
			continue
		case input == "/cost":
			fmt.Printf("Input: %d tokens, Output: %d tokens\n",
				sess.TotalInputTokens, sess.TotalOutputTokens)
			continue
		case input == "/compact":
			query.CompactSession(sess)
			fmt.Println("Session compacted.")
			continue
		case strings.HasPrefix(input, "/model"):
			parts := strings.Fields(input)
			if len(parts) > 1 {
				newModel := parts[1]
				sess.Config.Model = newModel
				fmt.Printf("Model set to: %s\n", newModel)
			} else {
				fmt.Printf("Current model: %s\n", sess.Config.Model)
			}
			continue
		case input == "/status":
			fmt.Printf("Session: %s\n", sess.ID)
			fmt.Printf("Model: %s\n", sess.Config.Model)
			fmt.Printf("CWD: %s\n", sess.CWD)
			fmt.Printf("Turns: %d / %d\n", sess.TurnCount, sess.Config.MaxTurns)
			fmt.Printf("Messages: %d\n", len(sess.Messages))
			fmt.Printf("Input tokens: %d\n", sess.TotalInputTokens)
			fmt.Printf("Output tokens: %d\n", sess.TotalOutputTokens)
			continue
		case input == "/context":
			budget := sess.Config.TokenBudget
			used := sess.TotalInputTokens
			total := budget.InputBudget()
			pct := 0.0
			if total > 0 {
				pct = float64(used) / float64(total) * 100
			}
			fmt.Printf("Context window: %d tokens\n", budget.ContextWindow)
			fmt.Printf("Input budget: %d tokens\n", total)
			fmt.Printf("Used: ~%d tokens (%.1f%%)\n", used, pct)
			fmt.Printf("Output limit: %d tokens\n", budget.MaxOutputTokens)
			if budget.ShouldCompact(used) {
				fmt.Println("Warning: approaching compact threshold")
			}
			continue
		case input == "/diff":
			cmd := exec.Command("git", "diff", "--stat")
			cmd.Dir = sess.CWD
			out, err := cmd.Output()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else if len(out) == 0 {
				fmt.Println("No changes")
			} else {
				fmt.Print(string(out))
			}
			continue
		case input == "/resume":
			fmt.Println("Session persistence not yet implemented.")
			continue
		case input == "/save":
			fmt.Println("Session persistence not yet implemented.")
			continue
		case strings.HasPrefix(input, "/permissions"):
			parts := strings.Fields(input)
			if len(parts) > 1 {
				switch parts[1] {
				case "auto":
					sess.Config.PermissionMode = permissions.AutoApprove
					fmt.Println("Permission mode: auto-approve")
				case "deny":
					sess.Config.PermissionMode = permissions.Deny
					fmt.Println("Permission mode: deny all")
				case "interactive":
					sess.Config.PermissionMode = permissions.Interactive
					fmt.Println("Permission mode: interactive")
				default:
					fmt.Printf("Unknown mode: %s (use auto, deny, interactive)\n", parts[1])
				}
			} else {
				modes := map[permissions.PermissionMode]string{
					permissions.AutoApprove: "auto-approve",
					permissions.Deny:       "deny",
					permissions.Interactive: "interactive",
				}
				fmt.Printf("Permission mode: %s\n", modes[sess.Config.PermissionMode])
			}
			continue
		case input == "/doctor":
			fmt.Println("gopher-code doctor")
			fmt.Println()
			// Check API key
			if os.Getenv("ANTHROPIC_API_KEY") != "" {
				fmt.Println("  [ok] ANTHROPIC_API_KEY is set")
			} else {
				fmt.Println("  [missing] ANTHROPIC_API_KEY not set")
			}
			// Check ripgrep
			if _, err := exec.LookPath("rg"); err == nil {
				fmt.Println("  [ok] ripgrep (rg) found")
			} else {
				fmt.Println("  [warn] ripgrep (rg) not found (grep will use fallback)")
			}
			// Check git
			if _, err := exec.LookPath("git"); err == nil {
				fmt.Println("  [ok] git found")
			} else {
				fmt.Println("  [missing] git not found")
			}
			fmt.Printf("  [ok] Go %s\n", runtime.Version())
			fmt.Printf("  [ok] Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			continue
		}

		sess.PushMessage(message.UserMessage(input))

		// Create per-query context (Ctrl+C cancels query, not REPL)
		queryCtx, queryCancel := context.WithCancel(ctx)
		err := query.Query(queryCtx, sess, prov, registry, orchestrator, PrintEvent)
		queryCancel()

		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		}
		fmt.Println()
	}
}

func printHelp() {
	fmt.Println("Slash commands:")
	fmt.Println("  /help          Show this help")
	fmt.Println("  /clear         Clear conversation history")
	fmt.Println("  /compact       Compact conversation to save context")
	fmt.Println("  /cost          Show token usage")
	fmt.Println("  /context       Show context window usage")
	fmt.Println("  /status        Show session info")
	fmt.Println("  /model [name]  Show or set model")
	fmt.Println("  /permissions   Show or set permission mode")
	fmt.Println("  /diff          Show git diff in CWD")
	fmt.Println("  /save          Save session")
	fmt.Println("  /resume        List saved sessions")
	fmt.Println("  /doctor        Check system health")
	fmt.Println("  /exit          Exit")
}
