package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"encoding/json"

	"github.com/projectbarks/gopher-code/pkg/config"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// RunREPL starts an interactive read-eval-print loop.
func RunREPL(ctx context.Context, sess *session.SessionState, prov provider.ModelProvider, registry *tools.ToolRegistry, verbose bool) {
	scanner := bufio.NewScanner(os.Stdin)
	orchestrator := tools.NewOrchestrator(registry)

	fmt.Println("gopher-code v0.1.0")
	fmt.Printf("Model: %s | CWD: %s\n", sess.Config.Model, sess.CWD)
	fmt.Printf("Session: %s\n", sess.ID[:8])
	fmt.Println("Type your message. Ctrl+D to exit. /help for commands.")
	fmt.Println()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()

		// Multi-line: if line ends with \, continue reading
		for strings.HasSuffix(strings.TrimSpace(input), "\\") {
			input = strings.TrimSuffix(strings.TrimSpace(input), "\\")
			fmt.Print("... ")
			if !scanner.Scan() {
				break
			}
			input += "\n" + scanner.Text()
		}
		input = strings.TrimSpace(input)
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
			metas, err := session.ListSessions()
			if err != nil || len(metas) == 0 {
				fmt.Println("No saved sessions found.")
			} else {
				fmt.Println("Saved sessions (most recent first):")
				limit := len(metas)
				if limit > 10 {
					limit = 10
				}
				for i := 0; i < limit; i++ {
					m := metas[i]
					fmt.Printf("  %s  %s  turns=%d  %s\n",
						m.ID[:8], m.CWD, m.TurnCount,
						m.UpdatedAt.Format("2006-01-02 15:04"))
				}
				fmt.Println("Use --resume <id> to resume a session.")
			}
			continue
		case input == "/save":
			if err := sess.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Save failed: %v\n", err)
			} else {
				fmt.Printf("Session %s saved.\n", sess.ID[:8])
			}
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
		case strings.HasPrefix(input, "!"):
			cmd := strings.TrimSpace(strings.TrimPrefix(input, "!"))
			if cmd == "" {
				continue
			}
			shellCmd := exec.Command("sh", "-c", cmd)
			shellCmd.Dir = sess.CWD
			shellCmd.Stdout = os.Stdout
			shellCmd.Stderr = os.Stderr
			shellCmd.Stdin = os.Stdin
			shellCmd.Run()
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
		case input == "/config":
			cfg := config.Load(sess.CWD)
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
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

		// Auto-save after each exchange
		if saveErr := sess.Save(); saveErr != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Auto-save failed: %v\n", saveErr)
			}
		}

		fmt.Println()
	}
}

func printHelp() {
	fmt.Println("Commands:")
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
	fmt.Println("  /config        Show loaded settings")
	fmt.Println("  /exit          Exit")
	fmt.Println()
	fmt.Println("  ! <command>    Run a shell command")
	fmt.Println("  line ending \\  Continue input on next line")
}
