// Package ide provides IDE detection and extension management utilities.
// Source: utils/ide.ts — IDE lockfile detection, extension install, terminal detection
package ide

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// IdeType identifies which IDE family the terminal belongs to.
// Source: utils/ide.ts — IdeType
type IdeType string

const (
	IdeVSCode    IdeType = "vscode"
	IdeCursor    IdeType = "cursor"
	IdeWindsurf  IdeType = "windsurf"
	IdeJetBrains IdeType = "jetbrains"
	IdeNone      IdeType = ""
)

// DetectedIDEInfo describes a detected IDE instance.
// Source: utils/ide.ts — DetectedIDEInfo
type DetectedIDEInfo struct {
	Name             string
	Port             int
	WorkspaceFolders []string
	URL              string
	IsValid          bool
	AuthToken        string
}

// IsVSCodeIDE returns true for VS Code-family IDEs (VS Code, Cursor, Windsurf).
// Source: utils/ide.ts:isVSCodeIde
func IsVSCodeIDE(ide IdeType) bool {
	return ide == IdeVSCode || ide == IdeCursor || ide == IdeWindsurf
}

// IsJetBrainsIDE returns true for JetBrains IDEs.
func IsJetBrainsIDE(ide IdeType) bool {
	return ide == IdeJetBrains
}

// GetTerminalIdeType detects which IDE terminal we're running in.
// Checks environment variables set by IDE terminal integrations.
// Source: utils/ide.ts:getTerminalIdeType
func GetTerminalIdeType() IdeType {
	termProgram := os.Getenv("TERM_PROGRAM")
	switch termProgram {
	case "vscode":
		return IdeVSCode
	case "cursor":
		return IdeCursor
	case "windsurf":
		return IdeWindsurf
	}

	// JetBrains sets TERMINAL_EMULATOR
	if strings.Contains(os.Getenv("TERMINAL_EMULATOR"), "JetBrains") {
		return IdeJetBrains
	}

	// Check parent process (fallback for terminals that don't set env vars)
	if ide := detectFromParentProcess(); ide != IdeNone {
		return ide
	}

	return IdeNone
}

// IsSupportedTerminal returns true if we're running inside a supported IDE terminal.
func IsSupportedTerminal() bool {
	return GetTerminalIdeType() != IdeNone
}

// detectFromParentProcess checks parent process names for IDE indicators.
func detectFromParentProcess() IdeType {
	ppid := os.Getppid()
	if ppid <= 1 {
		return IdeNone
	}

	name := parentProcessName(ppid)
	lower := strings.ToLower(name)

	switch {
	case strings.Contains(lower, "code"):
		return IdeVSCode
	case strings.Contains(lower, "cursor"):
		return IdeCursor
	case strings.Contains(lower, "windsurf"):
		return IdeWindsurf
	case strings.Contains(lower, "idea") ||
		strings.Contains(lower, "webstorm") ||
		strings.Contains(lower, "pycharm") ||
		strings.Contains(lower, "goland") ||
		strings.Contains(lower, "rustrover"):
		return IdeJetBrains
	}
	return IdeNone
}

func parentProcessName(pid int) string {
	switch runtime.GOOS {
	case "darwin", "linux":
		out, err := exec.Command("ps", "-p", itoa(pid), "-o", "comm=").Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	default:
		return ""
	}
}

// IsVSCodeInstalled checks if VS Code CLI is available.
func IsVSCodeInstalled() bool {
	_, err := exec.LookPath("code")
	return err == nil
}

// IsCursorInstalled checks if Cursor CLI is available.
func IsCursorInstalled() bool {
	_, err := exec.LookPath("cursor")
	return err == nil
}

// LockfilePath returns the path to IDE lockfiles directory.
// Source: utils/ide.ts — uses ~/.claude/ide/
func LockfilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "ide")
}

// GetIdeLockfiles returns paths to all IDE lockfiles.
func GetIdeLockfiles() ([]string, error) {
	dir := LockfilePath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".lock") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	return paths, nil
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
