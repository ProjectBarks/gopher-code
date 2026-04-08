package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/config"
	pkgcontext "github.com/projectbarks/gopher-code/pkg/context"
	"github.com/projectbarks/gopher-code/pkg/hooks"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// T568: Config utilities
func TestConfigUtilities_WiredIntoBinary(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Load(dir)
	if cfg == nil {
		t.Fatal("config.Load should return non-nil")
	}
}

// T569: Context utilities
func TestContextUtilities_WiredIntoBinary(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("hello world this is a test message"),
		message.AssistantMessage("here is a response"),
	}
	stats := pkgcontext.AnalyzeContext(msgs)
	if stats == nil {
		t.Fatal("AnalyzeContext should return non-nil")
	}

	pcts := pkgcontext.CalculateContextPercentages(&pkgcontext.TokenUsage{
		InputTokens: 1000,
	}, 200000)
	if pcts.Used == nil {
		t.Error("Used should not be nil")
	}
}

// T570: Message utilities
func TestMessageUtilities_WiredIntoBinary(t *testing.T) {
	user := message.UserMessage("hello")
	if user.Role != message.RoleUser {
		t.Errorf("UserMessage role = %q, want user", user.Role)
	}

	asst := message.AssistantMessage("world")
	if asst.Role != message.RoleAssistant {
		t.Errorf("AssistantMessage role = %q, want assistant", asst.Role)
	}

	if message.NoContentMessage == "" {
		t.Error("NoContentMessage should not be empty")
	}

	wrapped := message.WrapTag("test", "content")
	if wrapped == "" {
		t.Error("WrapTag should produce non-empty output")
	}
}

// T571: Permission utilities
func TestPermissionUtilities_WiredIntoBinary(t *testing.T) {
	rules := permissions.ParsePermissionRuleValue("Allow: Bash(*)")
	_ = rules

	safe, stripped := permissions.StripDangerousPermissions(nil)
	_ = safe
	_ = stripped
}

// T572: Hook utilities
func TestHookUtilities_WiredIntoBinary(t *testing.T) {
	runner := hooks.NewHookRunner(nil)
	if runner == nil {
		t.Fatal("NewHookRunner should return non-nil")
	}

	registry := hooks.NewHookRegistry()
	if registry == nil {
		t.Fatal("NewHookRegistry should return non-nil")
	}

	emitter := hooks.NewHookEventEmitter()
	if emitter == nil {
		t.Fatal("NewHookEventEmitter should return non-nil")
	}
}

// T573: Session utilities
func TestSessionUtilities_WiredIntoBinary(t *testing.T) {
	dir := t.TempDir()
	sess := session.New(session.DefaultConfig(), dir)
	if sess == nil {
		t.Fatal("session.New should return non-nil")
	}
	// CWD may be resolved (symlinks), so just check it's non-empty.
	if sess.CWD == "" {
		t.Error("CWD should not be empty")
	}
	if sess.Config.Model == "" {
		t.Error("Config.Model should have a default value")
	}
}
