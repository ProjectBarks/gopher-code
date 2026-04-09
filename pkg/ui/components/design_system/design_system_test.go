package design_system

import (
	"strings"
	"testing"
)

func TestRenderPane(t *testing.T) {
	got := RenderPane("content here", "12", 40)
	if !strings.Contains(got, "─") {
		t.Error("should have border line")
	}
	if !strings.Contains(got, "content here") {
		t.Error("should contain content")
	}
}

func TestRenderDialog(t *testing.T) {
	got := RenderDialog("Title", "Body text", 50)
	if got == "" {
		t.Error("should produce output")
	}
}

func TestRenderDivider(t *testing.T) {
	plain := RenderDivider(40, "")
	if len(plain) < 10 {
		t.Error("should produce a line")
	}

	labeled := RenderDivider(40, "Section")
	if !strings.Contains(labeled, "Section") {
		t.Error("labeled divider should contain label")
	}
}

func TestRenderBadge(t *testing.T) {
	got := RenderBadge("NEW", "10")
	if !strings.Contains(got, "NEW") {
		t.Error("should contain badge text")
	}
}

func TestRenderStatusIcon(t *testing.T) {
	tests := map[StatusType]string{
		StatusSuccess: "✓",
		StatusError:   "✗",
		StatusWarning: "⚠",
		StatusInfo:    "ℹ",
		StatusPending: "⏺",
	}
	for status, icon := range tests {
		got := RenderStatusIcon(status)
		if !strings.Contains(got, icon) {
			t.Errorf("RenderStatusIcon(%q) should contain %q, got %q", status, icon, got)
		}
	}
}

func TestRenderProgressBar(t *testing.T) {
	got := RenderProgressBar(0.5, 20)
	if !strings.Contains(got, "50%") {
		t.Errorf("50%% progress, got %q", got)
	}

	full := RenderProgressBar(1.0, 10)
	if !strings.Contains(full, "100%") {
		t.Error("should show 100%")
	}

	zero := RenderProgressBar(0, 10)
	if !strings.Contains(zero, "0%") {
		t.Error("should show 0%")
	}
}

func TestRenderShortcutHint(t *testing.T) {
	got := RenderShortcutHint("Escape", "to cancel")
	if got == "" {
		t.Error("should produce output")
	}
}

func TestRenderTabs(t *testing.T) {
	got := RenderTabs([]string{"General", "Commands", "Usage"}, 1)
	if got == "" {
		t.Error("should produce output")
	}
}

func TestRenderListItem(t *testing.T) {
	got := RenderListItem("Item text", "✓", 1)
	if !strings.Contains(got, "Item text") {
		t.Error("should contain text")
	}
	if !strings.Contains(got, "✓") {
		t.Error("should contain icon")
	}
}

func TestRenderLoadingState(t *testing.T) {
	got := RenderLoadingState("Loading plugins")
	if !strings.Contains(got, "Loading plugins") {
		t.Error("should contain message")
	}
}
