package commands

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// CommandResult is the message returned after executing a command.
type CommandResult struct {
	Command string
	Output  string
	Error   error
}

// ModelSwitchMsg requests switching to a different model.
type ModelSwitchMsg struct {
	Model string
}

// SessionSwitchMsg requests switching sessions.
type SessionSwitchMsg struct{}

// ClearConversationMsg requests clearing the conversation.
type ClearConversationMsg struct{}

// ShowHelpMsg requests showing help.
type ShowHelpMsg struct{}

// QuitMsg requests quitting.
type QuitMsg struct{}

// CompactMsg requests compacting the conversation.
type CompactMsg struct{}

// ThinkingToggleMsg requests toggling thinking mode.
type ThinkingToggleMsg struct{}

// ShowDoctorMsg requests showing the /doctor screen.
type ShowDoctorMsg struct{}

// ShowResumeMsg requests showing the /resume screen.
type ShowResumeMsg struct{}

// Handler is a function that processes a slash command.
type Handler func(args string) tea.Cmd

// Dispatcher routes slash commands to their handlers.
type Dispatcher struct {
	handlers map[string]Handler
}

// NewDispatcher creates a new command dispatcher with default handlers.
func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		handlers: make(map[string]Handler),
	}
	d.registerDefaults()
	return d
}

// Register adds a handler for a command name.
func (d *Dispatcher) Register(name string, handler Handler) {
	d.handlers[strings.ToLower(name)] = handler
}

// Dispatch routes a command string to its handler.
// Returns nil if the command is not recognized.
func (d *Dispatcher) Dispatch(input string) tea.Cmd {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return nil
	}

	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	handler, ok := d.handlers[cmd]
	if !ok {
		return func() tea.Msg {
			return CommandResult{
				Command: cmd,
				Error:   fmt.Errorf("unknown command: %s", cmd),
			}
		}
	}

	return handler(args)
}

// IsCommand returns true if the input looks like a slash command.
func IsCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}

// HasHandler returns true if a handler exists for the command.
func (d *Dispatcher) HasHandler(cmd string) bool {
	_, ok := d.handlers[strings.ToLower(cmd)]
	return ok
}

// Commands returns all registered command names.
func (d *Dispatcher) Commands() []string {
	cmds := make([]string, 0, len(d.handlers))
	for name := range d.handlers {
		cmds = append(cmds, name)
	}
	return cmds
}

func (d *Dispatcher) registerDefaults() {
	d.Register("/model", func(args string) tea.Cmd {
		if args == "" {
			return func() tea.Msg {
				return CommandResult{Command: "/model", Error: fmt.Errorf("usage: /model <name>")}
			}
		}
		return func() tea.Msg { return ModelSwitchMsg{Model: args} }
	})

	d.Register("/session", func(args string) tea.Cmd {
		return func() tea.Msg { return SessionSwitchMsg{} }
	})

	d.Register("/clear", func(args string) tea.Cmd {
		return func() tea.Msg { return ClearConversationMsg{} }
	})

	d.Register("/help", func(args string) tea.Cmd {
		return func() tea.Msg { return ShowHelpMsg{} }
	})

	d.Register("/quit", func(args string) tea.Cmd {
		return func() tea.Msg { return QuitMsg{} }
	})

	d.Register("/compact", func(args string) tea.Cmd {
		return func() tea.Msg { return CompactMsg{} }
	})

	d.Register("/thinking", func(args string) tea.Cmd {
		return func() tea.Msg { return ThinkingToggleMsg{} }
	})

	d.Register("/doctor", func(args string) tea.Cmd {
		return func() tea.Msg { return ShowDoctorMsg{} }
	})

	d.Register("/resume", func(args string) tea.Cmd {
		return func() tea.Msg { return ShowResumeMsg{} }
	})
}
