package doctor

import (
	"fmt"
	"runtime"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// SandboxStatus describes sandbox availability.
// Source: Doctor.tsx — SandboxDoctorSection component
type SandboxStatus struct {
	Available bool
	Type      string // e.g. "docker", "macos-sandbox", "none"
	Message   string // optional detail or reason for unavailability
}

// DetectSandboxStatus returns the current sandbox availability.
// Source: Doctor.tsx — sandbox detection logic
func DetectSandboxStatus() SandboxStatus {
	switch runtime.GOOS {
	case "darwin":
		return SandboxStatus{
			Available: true,
			Type:      "macos-sandbox",
			Message:   "macOS sandbox available (sandbox-exec)",
		}
	case "linux":
		return SandboxStatus{
			Available: true,
			Type:      "docker",
			Message:   "Linux sandbox available",
		}
	default:
		return SandboxStatus{
			Available: false,
			Type:      "none",
			Message:   fmt.Sprintf("No sandbox support on %s", runtime.GOOS),
		}
	}
}

// RenderSandbox renders the sandbox diagnostic section.
// Source: Doctor.tsx — SandboxDoctorSection component
func RenderSandbox(status SandboxStatus) string {
	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)

	var lines []string
	lines = append(lines, bold.Render("Sandbox"))

	if status.Available {
		lines = append(lines, fmt.Sprintf("└ Status: %s",
			t.TextSuccess().Render("available")))
		lines = append(lines, fmt.Sprintf("└ Type: %s", status.Type))
	} else {
		lines = append(lines, fmt.Sprintf("└ Status: %s",
			t.TextWarning().Render("unavailable")))
	}

	if status.Message != "" {
		lines = append(lines, dim.Render(fmt.Sprintf("└ %s", status.Message)))
	}

	return strings.Join(lines, "\n")
}
