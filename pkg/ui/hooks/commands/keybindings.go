// Package commands provides a bubbletea Model that resolves key presses to
// slash-command actions via the keybinding system, and a priority queue that
// drains commands one at a time.
//
// TS sources: src/hooks/useCommandKeybindings.tsx,
//             src/hooks/useCommandQueue.ts,
//             src/hooks/useQueueProcessor.ts,
//             src/utils/messageQueueManager.ts,
//             src/utils/queueProcessor.ts
package commands

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/keybindings"
)

// commandPrefix is the action prefix for command keybindings.
// Bindings whose action starts with "command:" map to slash commands.
const commandPrefix = "command:"

// ExecuteCommandMsg is dispatched when a keybinding resolves to a slash
// command. Receivers should route the Command string (e.g. "/commit")
// through the command dispatcher.
type ExecuteCommandMsg struct {
	Command        string // e.g. "/commit"
	FromKeybinding bool   // always true when sent from CommandKeybindings
}

// Resolver maps a tea.KeyPressMsg string to an action in a given context.
// The default implementation scans the BindingMap, but callers may substitute
// a stub for testing.
type Resolver interface {
	Resolve(keyStr string, ctx keybindings.Context) (keybindings.Action, bool)
}

// bindingMapResolver is the production Resolver backed by a BindingMap.
type bindingMapResolver struct {
	bindings keybindings.BindingMap
}

func (r *bindingMapResolver) Resolve(keyStr string, ctx keybindings.Context) (keybindings.Action, bool) {
	if b, ok := r.bindings[ctx]; ok {
		if action, ok := b[keyStr]; ok {
			return action, true
		}
	}
	return "", false
}

// NewResolver creates a Resolver from a BindingMap.
func NewResolver(bindings keybindings.BindingMap) Resolver {
	return &bindingMapResolver{bindings: bindings}
}

// CommandKeybindings is a bubbletea Model that intercepts KeyPressMsg events,
// resolves them via the keybinding system, and dispatches ExecuteCommandMsg
// for any "command:*" actions. It also checks the Global context for actions
// that map to well-known slash commands (e.g. app:toggleTodos -> /tasks).
type CommandKeybindings struct {
	resolver Resolver
	active   bool
	// contexts lists the keybinding contexts to check, in priority order.
	contexts []keybindings.Context
	// actionCommands maps well-known non-command: actions to slash commands.
	actionCommands map[keybindings.Action]string
}

// NewCommandKeybindings creates a CommandKeybindings model. The resolver is
// used to look up key-to-action mappings. By default it checks the Chat and
// Global contexts.
func NewCommandKeybindings(resolver Resolver) *CommandKeybindings {
	return &CommandKeybindings{
		resolver: resolver,
		active:   true,
		contexts: []keybindings.Context{
			keybindings.ContextChat,
			keybindings.ContextGlobal,
		},
		actionCommands: map[keybindings.Action]string{
			keybindings.ActionAppToggleTodos:      "/tasks",
			keybindings.ActionAppToggleTranscript:  "/transcript",
			keybindings.ActionAppToggleBrief:       "/brief",
		},
	}
}

// SetActive enables or disables keybinding processing (e.g. when a modal
// dialog is open).
func (m *CommandKeybindings) SetActive(active bool) {
	m.active = active
}

// Init implements tea.Model.
func (m *CommandKeybindings) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It intercepts KeyPressMsg, resolves the key
// through the keybinding resolver, and returns an ExecuteCommandMsg command
// for "command:*" actions.
func (m *CommandKeybindings) Update(msg tea.Msg) tea.Cmd {
	if !m.active {
		return nil
	}

	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	keyStr := press.String()

	// Check each context in priority order.
	for _, ctx := range m.contexts {
		action, found := m.resolver.Resolve(keyStr, ctx)
		if !found {
			continue
		}

		// "command:xyz" -> "/xyz"
		if strings.HasPrefix(string(action), commandPrefix) {
			cmdName := string(action)[len(commandPrefix):]
			return executeCmd(cmdName)
		}

		// Well-known action -> slash command mapping.
		if cmd, ok := m.actionCommands[action]; ok {
			return func() tea.Msg {
				return ExecuteCommandMsg{
					Command:        cmd,
					FromKeybinding: true,
				}
			}
		}
	}

	return nil
}

// View implements tea.Model. CommandKeybindings produces no visual output.
func (m *CommandKeybindings) View() string {
	return ""
}

func executeCmd(name string) tea.Cmd {
	return func() tea.Msg {
		return ExecuteCommandMsg{
			Command:        "/" + name,
			FromKeybinding: true,
		}
	}
}
