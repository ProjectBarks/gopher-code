package cli

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/provider"
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

	// Create the Bubbletea program
	p := tea.NewProgram(appModel)

	// Wire the bridge to the program (must happen before any queries)
	bridge.SetProgram(p)

	// Handle Ctrl+C cleanup via context cancellation
	go func() {
		<-ctx.Done()
		p.Send(tea.QuitMsg{})
	}()

	// Run the program (blocks until exit)
	_, err := p.Run()
	return err
}

// UseNewUI returns true if the GOPHER_NEW_UI env var is set.
func UseNewUI() bool {
	v := os.Getenv("GOPHER_NEW_UI")
	return v == "1" || v == "true"
}

func init() {
	// Print info about new UI availability (only if verbose)
	if os.Getenv("GOPHER_DEBUG") != "" {
		if UseNewUI() {
			fmt.Fprintln(os.Stderr, "[debug] Using new TUI v2")
		}
	}
}
