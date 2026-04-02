package cli

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Source: screens/REPL.tsx — Bubble Tea main loop

// AppState represents the TUI application state.
type AppState int

const (
	StateIdle    AppState = iota // Waiting for user input
	StateRunning                // Query is executing
	StateExiting                // User requested exit
)

// Model is the Bubble Tea model for the gopher-code TUI.
type Model struct {
	State      AppState
	Input      string
	Output     []string
	StatusText string
	Width      int
	Height     int
	Err        error
}

// InitialModel creates the initial TUI model.
func InitialModel() Model {
	return Model{
		State:      StateIdle,
		StatusText: "gopher-code ready",
		Width:      80,
		Height:     24,
	}
}

// Init returns the initial command for the Bubble Tea program.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			m.State = StateExiting
			return m, tea.Quit
		case "enter":
			if m.State == StateIdle && strings.TrimSpace(m.Input) != "" {
				input := m.Input
				m.Input = ""
				m.Output = append(m.Output, fmt.Sprintf("> %s", input))
				m.State = StateRunning
				m.StatusText = "thinking..."
				// In a real implementation, this would dispatch the query
				return m, nil
			}
		default:
			if m.State == StateIdle {
				m.Input += msg.String()
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

	case QueryCompleteMsg:
		m.State = StateIdle
		m.StatusText = "ready"
		m.Output = append(m.Output, msg.Response)
	}

	return m, nil
}

// View renders the TUI.
func (m Model) View() tea.View {
	var sb strings.Builder

	// Output area
	for _, line := range m.Output {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Input area
	if m.State == StateIdle {
		sb.WriteString(fmt.Sprintf("\n> %s█", m.Input))
	} else if m.State == StateRunning {
		sb.WriteString(fmt.Sprintf("\n⟳ %s", m.StatusText))
	}

	// Status bar
	sb.WriteString(fmt.Sprintf("\n\n\033[90m%s\033[0m", m.StatusText))

	return tea.NewView(sb.String())
}

// QueryCompleteMsg is sent when a query finishes.
type QueryCompleteMsg struct {
	Response string
}

// RunTUI starts the Bubble Tea program.
// This is the entry point for the rich TUI mode.
func RunTUI() error {
	p := tea.NewProgram(InitialModel())
	_, err := p.Run()
	return err
}
