package logo

import (
	"strings"
	"testing"
)

func TestRenderWelcome(t *testing.T) {
	got := RenderWelcome("claude-opus-4-6", "/projects/my-app")
	if !strings.Contains(got, "Welcome") {
		t.Error("should contain Welcome")
	}
	if !strings.Contains(got, Version) {
		t.Error("should contain version")
	}
	if !strings.Contains(got, "/help") {
		t.Error("should mention /help")
	}
}

func TestRenderWelcome_NoModel(t *testing.T) {
	got := RenderWelcome("", "")
	if got == "" {
		t.Error("should produce output even without model/cwd")
	}
}

func TestRenderCondensedLogo(t *testing.T) {
	got := RenderCondensedLogo("opus-4-6")
	if !strings.Contains(got, "Claude Code") {
		t.Error("should contain Claude Code")
	}
}

func TestRenderCondensedLogo_NoModel(t *testing.T) {
	got := RenderCondensedLogo("")
	if got == "" {
		t.Error("should produce output without model")
	}
}

func TestRenderSpinnerLogo(t *testing.T) {
	got := RenderSpinnerLogo("opus", "Thinking")
	if !strings.Contains(got, "Claude") {
		t.Error("should contain Claude")
	}
}

func TestRenderEffortSuffix(t *testing.T) {
	if RenderEffortSuffix("") != "" {
		t.Error("empty should return empty")
	}
	if RenderEffortSuffix("high") != " with high effort" {
		t.Error("should format effort suffix")
	}
}
