package doctor

import (
	"strings"
	"testing"

	uidoctor "github.com/projectbarks/gopher-code/pkg/ui/doctor"
)

func TestDetectRipgrep(t *testing.T) {
	status := DetectRipgrep()
	// rg may or may not be installed; just verify the struct is populated
	if status.Mode == "" {
		t.Error("Mode should not be empty")
	}
	if status.Working && status.SystemPath == "" {
		t.Error("SystemPath should be set when Working is true")
	}
}

func TestDiagnosticData_Summary(t *testing.T) {
	data := &DiagnosticData{
		Version:          "0.2.0",
		InstallationType: "go-binary",
		InvokedBinary:    "/usr/local/bin/claude",
		AutoUpdates:      "enabled",
		Sandbox:          uidoctor.SandboxStatus{Available: true, Type: "seatbelt"},
	}

	summary := data.Summary()
	if !strings.Contains(summary, "0.2.0") {
		t.Error("summary should contain version")
	}
	if !strings.Contains(summary, "go-binary") {
		t.Error("summary should contain installation type")
	}
	if !strings.Contains(summary, "seatbelt") {
		t.Error("summary should contain sandbox info")
	}
	if !strings.Contains(summary, "No issues found") {
		t.Error("summary should show no issues when clean")
	}
}

func TestDiagnosticData_SummaryWithWarnings(t *testing.T) {
	data := &DiagnosticData{
		Version:          "0.2.0",
		InstallationType: "go-binary",
		InvokedBinary:    "/usr/local/bin/claude",
		AutoUpdates:      "enabled",
		SettingsErrors:   []uidoctor.SettingsError{{Message: "bad setting"}},
		Sandbox:          uidoctor.SandboxStatus{},
	}

	summary := data.Summary()
	if !strings.Contains(summary, "1 warning") {
		t.Errorf("summary should show 1 warning, got:\n%s", summary)
	}
}

func TestDetectLinuxGlobPatternWarnings(t *testing.T) {
	// Just verify it doesn't panic — results depend on platform/shell config
	warnings := DetectLinuxGlobPatternWarnings()
	_ = warnings
}
