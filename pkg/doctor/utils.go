package doctor

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Source: utils/doctorDiagnostic.ts — additional diagnostic utilities

// GlobWarning describes a Linux glob pattern misconfiguration.
type GlobWarning struct {
	Issue string
	Fix   string
}

// DetectLinuxGlobPatternWarnings checks for bash glob settings that can
// interfere with tool patterns like "*.go".
// Source: doctorDiagnostic.ts — detectLinuxGlobPatternWarnings
func DetectLinuxGlobPatternWarnings() []GlobWarning {
	if runtime.GOOS != "linux" {
		return nil
	}

	var warnings []GlobWarning

	// Check if nullglob is set (causes "*.xyz" to expand to empty)
	out, err := exec.Command("bash", "-c", "shopt nullglob").Output()
	if err == nil && strings.Contains(string(out), "on") {
		warnings = append(warnings, GlobWarning{
			Issue: "bash nullglob is enabled — glob patterns like *.go expand to empty when no matches exist",
			Fix:   "Add 'shopt -u nullglob' to your .bashrc or run it before launching claude",
		})
	}

	// Check if failglob is set (causes "*.xyz" to error when no matches)
	out, err = exec.Command("bash", "-c", "shopt failglob").Output()
	if err == nil && strings.Contains(string(out), "on") {
		warnings = append(warnings, GlobWarning{
			Issue: "bash failglob is enabled — glob patterns error when no matches exist",
			Fix:   "Add 'shopt -u failglob' to your .bashrc",
		})
	}

	return warnings
}

// RipgrepStatus describes the ripgrep installation state.
type RipgrepStatus struct {
	Working    bool
	Mode       string // "system", "builtin", "embedded"
	SystemPath string
}

// DetectRipgrep checks if ripgrep is available and how.
func DetectRipgrep() RipgrepStatus {
	// Try system rg
	path, err := exec.LookPath("rg")
	if err == nil {
		return RipgrepStatus{Working: true, Mode: "system", SystemPath: path}
	}
	// Ripgrep not found — tools will fall back to grep
	return RipgrepStatus{Working: false, Mode: "embedded"}
}

// Summary formats the diagnostic data as a human-readable report.
func (d *DiagnosticData) Summary() string {
	var sb strings.Builder

	sb.WriteString("Claude Code Doctor\n")
	sb.WriteString("==================\n\n")

	sb.WriteString(fmt.Sprintf("Version:      %s\n", d.Version))
	sb.WriteString(fmt.Sprintf("Installation: %s\n", d.InstallationType))
	sb.WriteString(fmt.Sprintf("Binary:       %s\n", d.InvokedBinary))
	sb.WriteString(fmt.Sprintf("Platform:     %s/%s\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("Auto-updates: %s\n", d.AutoUpdates))

	if d.Sandbox.Available {
		sb.WriteString(fmt.Sprintf("Sandbox:      %s (available)\n", d.Sandbox.Type))
	} else {
		sb.WriteString("Sandbox:      not available\n")
	}

	// Warnings
	warningCount := len(d.SettingsErrors) + len(d.KeybindingWarnings) + len(d.MCPWarnings) + len(d.EnvValidation)

	if warningCount > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠ %d warning(s) found\n", warningCount))
	} else {
		sb.WriteString("\n✓ No issues found\n")
	}

	return sb.String()
}
