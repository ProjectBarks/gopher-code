package cli

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
	"github.com/projectbarks/gopher-code/pkg/ui"
)

// RunTUIV2 starts the new Bubble Tea UI.
// It creates the AppModel, wires the EventBridge, and runs the tea.Program.
func RunTUIV2(
	ctx context.Context,
	sess *session.SessionState,
	prov provider.ModelProvider,
	registry *tools.ToolRegistry,
) error {
	// Create the event bridge for query → UI communication
	bridge := ui.NewEventBridge()

	// Create the top-level app model
	appModel := ui.NewAppModel(sess, bridge)

	// Wire the query function — this is what gets called when the user submits input.
	// It creates a ToolOrchestrator per-query (matching REPL behavior) and calls
	// query.Query with the bridge callback for streaming events into the UI.
	orchestrator := tools.NewOrchestrator(registry)
	appModel.SetQueryFunc(func(qctx context.Context, qsess *session.SessionState, onEvent query.EventCallback) error {
		return query.Query(qctx, qsess, prov, registry, orchestrator, onEvent)
	})

	// Create the Bubbletea program.
	// Alternate screen is enabled via View().AltScreen = true in AppModel.
	// Source: ink/ink.tsx — TS Ink uses alternate screen buffer.
	p := tea.NewProgram(appModel)

	// Wire the bridge to the program (must happen before any queries)
	bridge.SetProgram(p)

	// Handle parent context cancellation (e.g., signal from main)
	go func() {
		<-ctx.Done()
		p.Send(tea.QuitMsg{})
	}()

	// Run the program (blocks until exit)
	_, err := p.Run()
	return err
}

// UseNewUI returns true unless GOPHER_OLD_UI is explicitly set.
// The new Bubbletea TUI is the default. Set GOPHER_OLD_UI=1 for the legacy REPL.
func UseNewUI() bool {
	v := os.Getenv("GOPHER_OLD_UI")
	return v != "1" && v != "true"
}

func init() {
	if os.Getenv("GOPHER_DEBUG") != "" {
		if UseNewUI() {
			fmt.Fprintln(os.Stderr, "[debug] Using new TUI v2")
		}
	}
}
