package commands

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDispatcherCreation(t *testing.T) {
	d := NewDispatcher()
	if d == nil {
		t.Fatal("Dispatcher should not be nil")
	}
}

func TestDispatcherHasDefaultCommands(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/model") {
		t.Error("Should have /model handler")
	}
	if !d.HasHandler("/clear") {
		t.Error("Should have /clear handler")
	}
	if !d.HasHandler("/help") {
		t.Error("Should have /help handler")
	}
	if !d.HasHandler("/session") {
		t.Error("Should have /session handler")
	}
	if !d.HasHandler("/quit") {
		t.Error("Should have /quit handler")
	}
}

func TestDispatcherModelCommand(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/model opus")
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	msg := cmd()
	if switchMsg, ok := msg.(ModelSwitchMsg); !ok {
		t.Errorf("Expected ModelSwitchMsg, got %T", msg)
	} else if switchMsg.Model != "opus" {
		t.Errorf("Expected model 'opus', got %q", switchMsg.Model)
	}
}

func TestDispatcherClearCommand(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/clear")
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	msg := cmd()
	if _, ok := msg.(ClearConversationMsg); !ok {
		t.Errorf("Expected ClearConversationMsg, got %T", msg)
	}
}

func TestDispatcherHelpCommand(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/help")
	msg := cmd()
	if _, ok := msg.(ShowHelpMsg); !ok {
		t.Errorf("Expected ShowHelpMsg, got %T", msg)
	}
}

func TestDispatcherUnknownCommand(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/unknown")
	if cmd == nil {
		t.Fatal("Should return error command for unknown")
	}
	msg := cmd()
	result, ok := msg.(CommandResult)
	if !ok {
		t.Fatalf("Expected CommandResult, got %T", msg)
	}
	if result.Error == nil {
		t.Error("Expected error for unknown command")
	}
}

func TestDispatcherNotACommand(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("not a command")
	if cmd != nil {
		t.Error("Non-command input should return nil")
	}
}

func TestDispatcherModelWithoutArgs(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/model")
	msg := cmd()
	result, ok := msg.(CommandResult)
	if !ok {
		t.Fatalf("Expected CommandResult, got %T", msg)
	}
	if result.Error == nil {
		t.Error("Expected error for /model without args")
	}
}

func TestIsCommand(t *testing.T) {
	if !IsCommand("/help") {
		t.Error("/help should be a command")
	}
	if !IsCommand("  /help") {
		t.Error("  /help should be a command")
	}
	if IsCommand("not a command") {
		t.Error("Regular text should not be a command")
	}
}

func TestDispatcherCustomHandler(t *testing.T) {
	d := NewDispatcher()
	d.Register("/custom", func(args string) tea.Cmd {
		return func() tea.Msg {
			return CommandResult{Command: "/custom", Output: "custom: " + args}
		}
	})
	if !d.HasHandler("/custom") {
		t.Error("Should have custom handler")
	}
}

func TestDispatcherCommands(t *testing.T) {
	d := NewDispatcher()
	cmds := d.Commands()
	if len(cmds) == 0 {
		t.Error("Should have registered commands")
	}
}
