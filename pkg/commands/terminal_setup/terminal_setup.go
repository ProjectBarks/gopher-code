// Package terminal_setup provides terminal detection and configuration.
//
// Source: commands/terminalSetup/terminalSetup.tsx
//
// Detects the current terminal emulator, determines what keyboard/display
// setup is needed, and provides instructions or auto-configuration.
package terminal_setup

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// Terminal identifies the detected terminal emulator.
type Terminal string

const (
	TerminalApple     Terminal = "Apple_Terminal"
	TerminalVSCode    Terminal = "vscode"
	TerminalCursor    Terminal = "cursor"
	TerminalWindsurf  Terminal = "windsurf"
	TerminalAlacritty Terminal = "alacritty"
	TerminalZed       Terminal = "zed"
	TerminalITerm2    Terminal = "iTerm.app"
	TerminalKitty     Terminal = "kitty"
	TerminalGhostty   Terminal = "ghostty"
	TerminalWezTerm   Terminal = "WezTerm"
	TerminalWarp      Terminal = "WarpTerminal"
	TerminalUnknown   Terminal = ""
)

// NativeCSIuTerminals are terminals that natively support CSI u / Kitty keyboard protocol.
var NativeCSIuTerminals = map[Terminal]string{
	TerminalGhostty: "Ghostty",
	TerminalKitty:   "Kitty",
	TerminalITerm2:  "iTerm2",
	TerminalWezTerm: "WezTerm",
	TerminalWarp:    "Warp",
}

// DetectTerminal identifies the current terminal emulator.
func DetectTerminal() Terminal {
	termProgram := os.Getenv("TERM_PROGRAM")
	switch termProgram {
	case "Apple_Terminal":
		return TerminalApple
	case "vscode":
		return TerminalVSCode
	case "cursor":
		return TerminalCursor
	case "windsurf":
		return TerminalWindsurf
	case "iTerm.app":
		return TerminalITerm2
	case "WezTerm":
		return TerminalWezTerm
	case "WarpTerminal":
		return TerminalWarp
	}

	if os.Getenv("TERM") == "xterm-ghostty" {
		return TerminalGhostty
	}
	if os.Getenv("TERM") == "xterm-kitty" {
		return TerminalKitty
	}
	if os.Getenv("ALACRITTY_WINDOW_ID") != "" {
		return TerminalAlacritty
	}
	if os.Getenv("ZED_TERM") != "" {
		return TerminalZed
	}

	return TerminalUnknown
}

// DisplayName returns the user-facing name for a terminal.
func DisplayName(t Terminal) string {
	switch t {
	case TerminalApple:
		return "Apple Terminal"
	case TerminalVSCode:
		return "VS Code"
	case TerminalCursor:
		return "Cursor"
	case TerminalWindsurf:
		return "Windsurf"
	case TerminalAlacritty:
		return "Alacritty"
	case TerminalZed:
		return "Zed"
	case TerminalITerm2:
		return "iTerm2"
	case TerminalKitty:
		return "Kitty"
	case TerminalGhostty:
		return "Ghostty"
	case TerminalWezTerm:
		return "WezTerm"
	case TerminalWarp:
		return "Warp"
	default:
		return "Unknown terminal"
	}
}

// NeedsSetup returns true if the terminal needs configuration for optimal use.
func NeedsSetup(t Terminal) bool {
	if runtime.GOOS == "darwin" && t == TerminalApple {
		return true
	}
	switch t {
	case TerminalVSCode, TerminalCursor, TerminalWindsurf, TerminalAlacritty, TerminalZed:
		return true
	}
	return false
}

// HasNativeCSIu returns true if the terminal natively supports CSI u keyboard protocol.
func HasNativeCSIu(t Terminal) bool {
	_, ok := NativeCSIuTerminals[t]
	return ok
}

// IsVSCodeRemoteSSH detects if we're in a VS Code Remote SSH session.
func IsVSCodeRemoteSSH() bool {
	askpass := os.Getenv("VSCODE_GIT_ASKPASS_MAIN")
	path := os.Getenv("PATH")
	return strings.Contains(askpass, ".vscode-server") ||
		strings.Contains(askpass, ".cursor-server") ||
		strings.Contains(askpass, ".windsurf-server") ||
		strings.Contains(path, ".vscode-server") ||
		strings.Contains(path, ".cursor-server") ||
		strings.Contains(path, ".windsurf-server")
}

// SetupResult describes the outcome of terminal setup.
type SetupResult struct {
	Terminal    Terminal
	Message     string
	NeedsSetup bool
	IsNative    bool
}

// Check analyzes the current terminal and returns setup information.
func Check() SetupResult {
	term := DetectTerminal()
	name := DisplayName(term)

	if HasNativeCSIu(term) {
		return SetupResult{
			Terminal: term,
			Message:  fmt.Sprintf("%s natively supports enhanced keyboard input. No setup needed.", name),
			IsNative: true,
		}
	}

	if !NeedsSetup(term) {
		return SetupResult{
			Terminal: term,
			Message:  fmt.Sprintf("Terminal: %s. No additional setup required.", name),
		}
	}

	return SetupResult{
		Terminal:    term,
		NeedsSetup: true,
		Message:     setupInstructions(term),
	}
}

// Render returns a styled string for the terminal setup status.
func Render(result SetupResult) string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Terminal Setup"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  Terminal: %s\n", DisplayName(result.Terminal)))

	if result.IsNative {
		b.WriteString("  Status:   " + successStyle.Render("✓ Native keyboard support") + "\n")
	} else if result.NeedsSetup {
		b.WriteString("  Status:   " + warnStyle.Render("⚠ Setup recommended") + "\n")
	} else {
		b.WriteString("  Status:   " + successStyle.Render("✓ Ready") + "\n")
	}

	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render(result.Message) + "\n")

	return b.String()
}

// setupInstructions returns terminal-specific setup instructions.
func setupInstructions(t Terminal) string {
	switch t {
	case TerminalApple:
		return "Apple Terminal needs Option key configured as Meta.\n" +
			"  Terminal → Settings → Profiles → Keyboard → \"Use Option as Meta Key\""

	case TerminalVSCode:
		return "VS Code terminal needs Shift+Enter keybinding.\n" +
			"  Run /terminal-setup to auto-install the keybinding in keybindings.json"

	case TerminalCursor:
		return "Cursor terminal needs Shift+Enter keybinding.\n" +
			"  Run /terminal-setup to auto-install the keybinding"

	case TerminalWindsurf:
		return "Windsurf terminal needs Shift+Enter keybinding.\n" +
			"  Run /terminal-setup to auto-install the keybinding"

	case TerminalAlacritty:
		return "Alacritty needs Shift+Enter configured in alacritty.toml.\n" +
			"  Run /terminal-setup to auto-install the binding"

	case TerminalZed:
		return "Zed needs Shift+Enter keybinding configured.\n" +
			"  Run /terminal-setup to auto-install the keybinding"

	default:
		return "No specific setup instructions available for this terminal."
	}
}
