package commands

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/keybindings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubResolver lets tests define exactly which keys map to which actions.
type stubResolver struct {
	m map[keybindings.Context]map[string]keybindings.Action
}

func (r *stubResolver) Resolve(keyStr string, ctx keybindings.Context) (keybindings.Action, bool) {
	if b, ok := r.m[ctx]; ok {
		if a, ok := b[keyStr]; ok {
			return a, true
		}
	}
	return "", false
}

func newStub(pairs map[keybindings.Context]map[string]keybindings.Action) *stubResolver {
	return &stubResolver{m: pairs}
}

// keyPress builds a KeyPressMsg whose String() returns the given value.
// bubbletea v2 KeyPressMsg.String() returns the canonical form, so we
// construct a Key with the right Code and Mod fields.
func keyPress(s string) tea.KeyPressMsg {
	parsed := keybindings.ParseKeystroke(s)
	var mod tea.KeyMod
	if parsed.Ctrl {
		mod |= tea.ModCtrl
	}
	if parsed.Alt || parsed.Meta {
		mod |= tea.ModAlt
	}
	if parsed.Shift {
		mod |= tea.ModShift
	}
	if parsed.Super {
		mod |= tea.ModSuper
	}

	code := rune(0)
	if len(parsed.Key) == 1 {
		code = rune(parsed.Key[0])
	} else {
		// Map well-known names to bubbletea key codes.
		switch parsed.Key {
		case "escape":
			code = tea.KeyEscape
		case "enter":
			code = tea.KeyEnter
		case "tab":
			code = tea.KeyTab
		case "up":
			code = tea.KeyUp
		case "down":
			code = tea.KeyDown
		}
	}

	return tea.KeyPressMsg{Code: code, Mod: mod}
}

func TestCommandKeybindings_CommandPrefix(t *testing.T) {
	// "command:commit" should dispatch ExecuteCommandMsg{Command: "/commit"}.
	resolver := newStub(map[keybindings.Context]map[string]keybindings.Action{
		keybindings.ContextChat: {
			"ctrl+k": "command:commit",
		},
	})

	m := NewCommandKeybindings(resolver)
	cmd := m.Update(keyPress("ctrl+k"))
	require.NotNil(t, cmd, "expected a command for ctrl+k")

	msg := cmd()
	ecm, ok := msg.(ExecuteCommandMsg)
	require.True(t, ok, "expected ExecuteCommandMsg, got %T", msg)
	assert.Equal(t, "/commit", ecm.Command)
	assert.True(t, ecm.FromKeybinding)
}

func TestCommandKeybindings_WellKnownAction(t *testing.T) {
	// ctrl+t -> app:toggleTodos -> "/tasks"
	resolver := newStub(map[keybindings.Context]map[string]keybindings.Action{
		keybindings.ContextGlobal: {
			"ctrl+t": keybindings.ActionAppToggleTodos,
		},
	})

	m := NewCommandKeybindings(resolver)
	cmd := m.Update(keyPress("ctrl+t"))
	require.NotNil(t, cmd)

	msg := cmd()
	ecm, ok := msg.(ExecuteCommandMsg)
	require.True(t, ok)
	assert.Equal(t, "/tasks", ecm.Command)
}

func TestCommandKeybindings_TranscriptAction(t *testing.T) {
	// ctrl+o -> app:toggleTranscript -> "/transcript"
	resolver := newStub(map[keybindings.Context]map[string]keybindings.Action{
		keybindings.ContextGlobal: {
			"ctrl+o": keybindings.ActionAppToggleTranscript,
		},
	})

	m := NewCommandKeybindings(resolver)
	cmd := m.Update(keyPress("ctrl+o"))
	require.NotNil(t, cmd)

	msg := cmd()
	ecm := msg.(ExecuteCommandMsg)
	assert.Equal(t, "/transcript", ecm.Command)
}

func TestCommandKeybindings_UnmappedKey(t *testing.T) {
	resolver := newStub(map[keybindings.Context]map[string]keybindings.Action{})
	m := NewCommandKeybindings(resolver)

	cmd := m.Update(keyPress("ctrl+z"))
	assert.Nil(t, cmd, "unmapped key should produce no command")
}

func TestCommandKeybindings_InactiveIgnoresKeys(t *testing.T) {
	resolver := newStub(map[keybindings.Context]map[string]keybindings.Action{
		keybindings.ContextChat: {
			"ctrl+k": "command:commit",
		},
	})

	m := NewCommandKeybindings(resolver)
	m.SetActive(false)

	cmd := m.Update(keyPress("ctrl+k"))
	assert.Nil(t, cmd, "inactive model should not dispatch commands")
}

func TestCommandKeybindings_ContextPriority(t *testing.T) {
	// Chat context should be checked before Global.
	resolver := newStub(map[keybindings.Context]map[string]keybindings.Action{
		keybindings.ContextChat: {
			"ctrl+t": "command:chat-specific",
		},
		keybindings.ContextGlobal: {
			"ctrl+t": keybindings.ActionAppToggleTodos,
		},
	})

	m := NewCommandKeybindings(resolver)
	cmd := m.Update(keyPress("ctrl+t"))
	require.NotNil(t, cmd)

	msg := cmd()
	ecm := msg.(ExecuteCommandMsg)
	// Chat context wins: "command:chat-specific" -> "/chat-specific"
	assert.Equal(t, "/chat-specific", ecm.Command)
}

func TestCommandKeybindings_NonCommandAction(t *testing.T) {
	// An action that is neither "command:*" nor in actionCommands produces
	// no output.
	resolver := newStub(map[keybindings.Context]map[string]keybindings.Action{
		keybindings.ContextChat: {
			"enter": keybindings.ActionChatSubmit,
		},
	})

	m := NewCommandKeybindings(resolver)
	cmd := m.Update(keyPress("enter"))
	assert.Nil(t, cmd, "non-command action should not dispatch")
}

func TestCommandKeybindings_NonKeyMsg(t *testing.T) {
	resolver := newStub(map[keybindings.Context]map[string]keybindings.Action{})
	m := NewCommandKeybindings(resolver)

	// A non-key message should be ignored.
	cmd := m.Update(ProcessQueueMsg{})
	assert.Nil(t, cmd)
}

func TestNewResolver_IntegrationWithDefaultBindings(t *testing.T) {
	bm := keybindings.DefaultBindingMap()
	r := NewResolver(bm)

	// ctrl+t in Global context should resolve to app:toggleTodos.
	action, ok := r.Resolve("ctrl+t", keybindings.ContextGlobal)
	require.True(t, ok)
	assert.Equal(t, keybindings.ActionAppToggleTodos, action)

	// Unknown key returns false.
	_, ok = r.Resolve("ctrl+z", keybindings.ContextGlobal)
	assert.False(t, ok)
}
