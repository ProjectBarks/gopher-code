package thinkback

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNew(t *testing.T) {
	m := New()
	if m.step != StepChecking {
		t.Errorf("step = %q", m.step)
	}
}

func TestModel_PluginCheck_NotInstalled(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: false})
	if m.step != StepInstalling {
		t.Errorf("step = %q, want installing", m.step)
	}
}

func TestModel_PluginCheck_HasAnimation(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: true, HasAnimation: true})
	if m.step != StepMenu {
		t.Errorf("step = %q, want menu", m.step)
	}
	if !m.hasAnimation {
		t.Error("should have animation")
	}
}

func TestModel_PluginCheck_NoAnimation(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: true, HasAnimation: false})
	if m.step != StepMenu {
		t.Errorf("step = %q", m.step)
	}
	if m.hasAnimation {
		t.Error("should not have animation")
	}
}

func TestModel_PluginCheck_Error(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Error: "network error"})
	if m.step != StepError {
		t.Errorf("step = %q", m.step)
	}
	if m.message != "network error" {
		t.Errorf("message = %q", m.message)
	}
}

func TestModel_Menu_NoAnimation(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: true, HasAnimation: false})

	// Only one option: "Let's go!" (regenerate)
	opts := m.menuOptions()
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
	if opts[0].Action != ActionRegenerate {
		t.Errorf("action = %q", opts[0].Action)
	}
}

func TestModel_Menu_HasAnimation(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: true, HasAnimation: true})

	opts := m.menuOptions()
	if len(opts) != 4 {
		t.Fatalf("expected 4 options, got %d", len(opts))
	}
	if opts[0].Action != ActionPlay {
		t.Error("first should be play")
	}
}

func TestModel_Menu_SelectPlay(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: true, HasAnimation: true})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(PlayAnimationMsg); !ok {
		t.Fatalf("expected PlayAnimationMsg, got %T", msg)
	}
}

func TestModel_Menu_SelectRegenerate(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: true, HasAnimation: false})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	skill, ok := msg.(SkillInvokeMsg)
	if !ok {
		t.Fatalf("expected SkillInvokeMsg, got %T", msg)
	}
	if !strings.Contains(skill.Prompt, "regenerate") {
		t.Errorf("prompt should contain regenerate: %q", skill.Prompt)
	}
}

func TestModel_Menu_Navigation(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: true, HasAnimation: true})

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor = %d", m.cursor)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor = %d", m.cursor)
	}
}

func TestModel_Menu_Cancel(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: true, HasAnimation: true})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should close")
	}
	if _, ok := cmd().(DoneMsg); !ok {
		t.Error("expected DoneMsg")
	}
}

func TestModel_View_Checking(t *testing.T) {
	m := New()
	v := m.View()
	if !strings.Contains(v, "Checking") {
		t.Error("should show checking status")
	}
}

func TestModel_View_Menu_NoAnimation(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: true, HasAnimation: false})
	v := m.View()
	if !strings.Contains(v, "Relive your year") {
		t.Error("should show intro text")
	}
	if !strings.Contains(v, "Let's go!") {
		t.Error("should show generate option")
	}
}

func TestModel_View_Menu_HasAnimation(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Installed: true, HasAnimation: true})
	v := m.View()
	if !strings.Contains(v, "Play animation") {
		t.Error("should show play option")
	}
	if !strings.Contains(v, "Edit content") {
		t.Error("should show edit option")
	}
}

func TestModel_View_Error(t *testing.T) {
	m := New()
	m, _ = m.Update(PluginCheckMsg{Error: "plugin failed"})
	v := m.View()
	if !strings.Contains(v, "Error") {
		t.Error("should show error")
	}
	if !strings.Contains(v, "plugin failed") {
		t.Error("should show error message")
	}
}

func TestPromptConstants(t *testing.T) {
	if !strings.Contains(EditPrompt, "edit") {
		t.Error("edit prompt should contain edit")
	}
	if !strings.Contains(FixPrompt, "fix") {
		t.Error("fix prompt should contain fix")
	}
	if !strings.Contains(RegeneratePrompt, "regenerate") {
		t.Error("regenerate prompt should contain regenerate")
	}
}
