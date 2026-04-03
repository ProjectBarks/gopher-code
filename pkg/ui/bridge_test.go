package ui

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/query"
)

func TestNewEventBridge(t *testing.T) {
	bridge := NewEventBridge()
	if bridge == nil {
		t.Fatal("EventBridge should not be nil")
	}
}

func TestEventBridgeCallbackType(t *testing.T) {
	bridge := NewEventBridge()
	cb := bridge.BridgeCallback()
	if cb == nil {
		t.Fatal("BridgeCallback should not be nil")
	}
}

func TestEventBridgeOnQueryEventWithoutProgram(t *testing.T) {
	bridge := NewEventBridge()
	// Should not panic without program set
	evt := query.QueryEvent{Type: query.QEventTextDelta, Text: "hello"}
	bridge.OnQueryEvent(evt)
}

func TestEventBridgeSetProgram(t *testing.T) {
	bridge := NewEventBridge()
	// SetProgram with nil should not panic
	bridge.SetProgram(nil)
}
