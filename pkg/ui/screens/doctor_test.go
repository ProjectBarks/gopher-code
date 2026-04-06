package screens

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/doctor"
)

func TestDoctorModel_Init(t *testing.T) {
	m := NewDoctorModel(DoctorConfig{})
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil cmd")
	}
}

func TestDoctorModel_RendersDiagnostics(t *testing.T) {
	cfg := DoctorConfig{
		Diagnostic: &DoctorDiagnostic{
			Version:            "0.2.0",
			InstallationType:   "go-binary",
			InstallationPath:   "/usr/local/bin/gopher-code",
			InvokedBinary:      "gopher-code",
			ConfigInstallMethod: "direct",
			AutoUpdates:        "enabled",
		},
		DistTags:      &doctor.DistTags{Stable: "1.0.0", Latest: "1.1.0"},
		AutoUpdates:   "enabled",
		UpdateChannel: "latest",
	}

	m := NewDoctorModel(cfg)
	// Send resize so View produces content
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})

	v := m.View()
	content := v.Content

	if !strings.Contains(content, "Diagnostics") {
		t.Error("expected Diagnostics header")
	}
	if !strings.Contains(content, "go-binary (0.2.0)") {
		t.Error("expected version info")
	}
	if !strings.Contains(content, "/usr/local/bin/gopher-code") {
		t.Error("expected installation path")
	}
	if !strings.Contains(content, "Updates") {
		t.Error("expected Updates section")
	}
	if !strings.Contains(content, "Stable version: 1.0.0") {
		t.Error("expected stable version")
	}
	if !strings.Contains(content, "Press Enter to continue") {
		t.Error("expected footer")
	}
}

func TestDoctorModel_EnterDismisses(t *testing.T) {
	m := NewDoctorModel(DoctorConfig{})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a cmd")
	}
	msg := cmd()
	if _, ok := msg.(DoctorDoneMsg); !ok {
		t.Errorf("expected DoctorDoneMsg, got %T", msg)
	}
}

func TestDoctorModel_EscapeDismisses(t *testing.T) {
	m := NewDoctorModel(DoctorConfig{})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Escape should produce a cmd")
	}
	msg := cmd()
	if _, ok := msg.(DoctorDoneMsg); !ok {
		t.Errorf("expected DoctorDoneMsg, got %T", msg)
	}
}

func TestDoctorModel_CtrlCDismisses(t *testing.T) {
	m := NewDoctorModel(DoctorConfig{})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+C should produce a cmd")
	}
	msg := cmd()
	if _, ok := msg.(DoctorDoneMsg); !ok {
		t.Errorf("expected DoctorDoneMsg, got %T", msg)
	}
}

func TestDoctorModel_ScrollUpDown(t *testing.T) {
	cfg := DoctorConfig{
		Diagnostic: &DoctorDiagnostic{
			Version:          "0.2.0",
			InstallationType: "go-binary",
			InstallationPath: "/usr/local/bin/gopher-code",
			InvokedBinary:    "gopher-code",
		},
	}
	m := NewDoctorModel(cfg)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 5}) // small window to force scrolling

	// Scroll down
	m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.scroll < 0 {
		t.Error("scroll should not be negative after down")
	}

	// Scroll up
	m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.scroll < 0 {
		t.Error("scroll should not go negative")
	}
}

func TestDoctorModel_WithAllSections(t *testing.T) {
	cfg := DoctorConfig{
		Diagnostic: &DoctorDiagnostic{
			Version:          "0.2.0",
			InstallationType: "go-binary",
			InstallationPath: "/bin/gopher",
			InvokedBinary:    "gopher",
			Warnings: []DoctorWarning{
				{Issue: "old version", Fix: "upgrade"},
			},
		},
		DistTags:      &doctor.DistTags{Latest: "2.0.0"},
		AutoUpdates:   "enabled",
		UpdateChannel: "latest",
		VersionLocks: &doctor.VersionLockInfo{
			Enabled: true,
			Locks: []doctor.LockInfo{
				{Version: "1.0.0", PID: 42, IsProcessRunning: true},
			},
		},
		AgentInfo: &doctor.AgentInfo{
			ActiveAgents:     []doctor.AgentEntry{{AgentType: "custom", Source: "user"}},
			UserAgentsDir:    "/home/.claude/agents",
			ProjectAgentsDir: "/proj/.claude/agents",
			UserDirExists:    true,
		},
		ContextWarnings: &doctor.ContextWarnings{
			ClaudeMDWarning: &doctor.ContextWarning{
				Message: "Large CLAUDE.md",
				Details: []string{"./CLAUDE.md"},
			},
		},
	}

	m := NewDoctorModel(cfg)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	v := m.View()
	content := v.Content

	for _, expected := range []string{
		"Diagnostics",
		"Updates",
		"Version Locks",
		"Agents",
		"Context Usage Warnings",
		"old version",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("expected %q in output", expected)
		}
	}
}

func TestDoctorModel_ZeroSize(t *testing.T) {
	m := NewDoctorModel(DoctorConfig{})
	v := m.View()
	if !strings.Contains(v.Content, "Checking installation status") {
		t.Error("zero-size should show loading message")
	}
}
