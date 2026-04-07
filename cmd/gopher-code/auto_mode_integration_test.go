package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/cmd/gopher-code/handlers"
)

// buildBinary builds the gopher-code binary into a temp directory and returns
// its path. The caller is responsible for cleaning up the directory.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "gopher-code")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	// Use runtime.Caller to locate this source file's directory so the build
	// runs in the correct package regardless of the test runner's cwd.
	_, thisFile, _, _ := runtime.Caller(0)
	cmd.Dir = filepath.Dir(thisFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

// TestAutoModeSubcommand_Defaults verifies that `gopher-code auto-mode defaults`
// exits 0 and produces valid JSON with the expected keys.
func TestAutoModeSubcommand_Defaults(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "auto-mode", "defaults")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v", err)
	}

	var rules handlers.AutoModeRules
	if err := json.Unmarshal(out, &rules); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(rules.Allow) == 0 {
		t.Error("expected non-empty allow rules")
	}
	if len(rules.SoftDeny) == 0 {
		t.Error("expected non-empty soft_deny rules")
	}
	if len(rules.Environment) == 0 {
		t.Error("expected non-empty environment rules")
	}
}

// TestAutoModeSubcommand_Config verifies that `gopher-code auto-mode` and
// `gopher-code auto-mode config` both produce valid JSON output.
func TestAutoModeSubcommand_Config(t *testing.T) {
	bin := buildBinary(t)

	for _, args := range [][]string{
		{"auto-mode"},
		{"auto-mode", "config"},
	} {
		cmd := exec.Command(bin, args...)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("args=%v: expected exit 0, got error: %v", args, err)
		}

		var rules handlers.AutoModeRules
		if err := json.Unmarshal(out, &rules); err != nil {
			t.Fatalf("args=%v: output is not valid JSON: %v\noutput: %s", args, err, out)
		}
	}
}

// TestAutoModeSubcommand_Unknown verifies that an unknown auto-mode subcommand
// exits with a non-zero code and an informative error message.
func TestAutoModeSubcommand_Unknown(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "auto-mode", "bogus")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown subcommand")
	}

	if !strings.Contains(string(out), "bogus") {
		t.Fatalf("expected error to mention unknown subcommand, got: %s", out)
	}
}
