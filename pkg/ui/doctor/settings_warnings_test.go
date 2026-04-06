package doctor

import (
	"strings"
	"testing"
)

func TestRenderSettingsErrors_Empty(t *testing.T) {
	if out := RenderSettingsErrors(nil); out != "" {
		t.Error("nil should produce empty output")
	}
	if out := RenderSettingsErrors([]SettingsError{}); out != "" {
		t.Error("empty slice should produce empty output")
	}
}

func TestRenderSettingsErrors_WithErrors(t *testing.T) {
	errs := []SettingsError{
		{Path: "model", Message: "unknown model name"},
		{Path: "", Message: "general config error"},
	}
	out := RenderSettingsErrors(errs)
	if !strings.Contains(out, "Settings Errors") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "model: unknown model name") {
		t.Error("expected path-prefixed error")
	}
	if !strings.Contains(out, "general config error") {
		t.Error("expected path-less error")
	}
}

func TestRenderKeybindingWarnings_Empty(t *testing.T) {
	if out := RenderKeybindingWarnings(nil); out != "" {
		t.Error("nil should produce empty output")
	}
}

func TestRenderKeybindingWarnings_WithWarnings(t *testing.T) {
	warnings := []KeybindingWarning{
		{Key: "ctrl+shift+x", Message: "unrecognized modifier"},
	}
	out := RenderKeybindingWarnings(warnings)
	if !strings.Contains(out, "Keybinding Warnings") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "ctrl+shift+x") {
		t.Error("expected key")
	}
	if !strings.Contains(out, "unrecognized modifier") {
		t.Error("expected message")
	}
}

func TestRenderMCPWarnings_Empty(t *testing.T) {
	if out := RenderMCPWarnings(nil); out != "" {
		t.Error("nil should produce empty output")
	}
}

func TestRenderMCPWarnings_WithWarnings(t *testing.T) {
	warnings := []MCPParsingWarning{
		{Server: "my-server", Message: "invalid command field"},
	}
	out := RenderMCPWarnings(warnings)
	if !strings.Contains(out, "MCP Config Warnings") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "my-server") {
		t.Error("expected server name")
	}
	if !strings.Contains(out, "invalid command field") {
		t.Error("expected message")
	}
}
