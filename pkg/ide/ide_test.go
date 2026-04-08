package ide

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsVSCodeIDE(t *testing.T) {
	if !IsVSCodeIDE(IdeVSCode) {
		t.Error("vscode should be VS Code family")
	}
	if !IsVSCodeIDE(IdeCursor) {
		t.Error("cursor should be VS Code family")
	}
	if !IsVSCodeIDE(IdeWindsurf) {
		t.Error("windsurf should be VS Code family")
	}
	if IsVSCodeIDE(IdeJetBrains) {
		t.Error("jetbrains should NOT be VS Code family")
	}
	if IsVSCodeIDE(IdeNone) {
		t.Error("none should NOT be VS Code family")
	}
}

func TestGetTerminalIdeType_VSCode(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "vscode")
	t.Setenv("TERMINAL_EMULATOR", "")
	if got := GetTerminalIdeType(); got != IdeVSCode {
		t.Errorf("GetTerminalIdeType = %q, want vscode", got)
	}
}

func TestGetTerminalIdeType_JetBrains(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERMINAL_EMULATOR", "JetBrains-JediTerm")
	if got := GetTerminalIdeType(); got != IdeJetBrains {
		t.Errorf("GetTerminalIdeType = %q, want jetbrains", got)
	}
}

func TestGetTerminalIdeType_None(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	t.Setenv("TERMINAL_EMULATOR", "")
	got := GetTerminalIdeType()
	// May detect from parent process — just verify it doesn't panic
	_ = got
}

func TestIsSupportedTerminal(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "vscode")
	t.Setenv("TERMINAL_EMULATOR", "")
	if !IsSupportedTerminal() {
		t.Error("should be supported in vscode")
	}
}

func TestGetIdeLockfiles(t *testing.T) {
	dir := t.TempDir()
	lockDir := filepath.Join(dir, ".claude", "ide")
	os.MkdirAll(lockDir, 0755)
	os.WriteFile(filepath.Join(lockDir, "8080.lock"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(lockDir, "9090.lock"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(lockDir, "readme.md"), []byte("not a lock"), 0644)

	// Temporarily override home
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	files, err := GetIdeLockfiles()
	if err != nil {
		t.Fatalf("GetIdeLockfiles error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 lockfiles, got %d", len(files))
	}
}

func TestLockfilePath(t *testing.T) {
	path := LockfilePath()
	if path == "" {
		t.Error("LockfilePath should not be empty")
	}
}
