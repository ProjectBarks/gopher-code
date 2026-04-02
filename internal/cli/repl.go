package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/message"
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
	fmt.Println("Commands:")
	fmt.Println("  /help   - Show this help")
	fmt.Println("  /clear  - Clear conversation history")
	fmt.Println("  /cost   - Show token usage")
	fmt.Println("  /exit   - Exit")
}
