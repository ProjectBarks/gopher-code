package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// withExitCapture replaces the package-level exitFunc, stderr, and stdout
// with test doubles, runs fn, then restores the originals.
// It returns the captured exit code and the stderr/stdout output.
func withExitCapture(fn func()) (exitCode int, stderrOut, stdoutOut string) {
	origExit := exitFunc
	origStderr := stderr
	origStdout := stdout
	defer func() {
		exitFunc = origExit
		stderr = origStderr
		stdout = origStdout
	}()

	exitCode = -1
	var errBuf, outBuf bytes.Buffer
	stderr = &errBuf
	stdout = &outBuf
	exitFunc = func(code int) { exitCode = code }

	fn()

	return exitCode, errBuf.String(), outBuf.String()
}

func TestCLIError_WritesToStderr(t *testing.T) {
	code, stderrOut, stdoutOut := withExitCapture(func() {
		cliError("something went wrong")
	})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stderrOut != "something went wrong\n" {
		t.Fatalf("expected stderr %q, got %q", "something went wrong\n", stderrOut)
	}
	if stdoutOut != "" {
		t.Fatalf("expected empty stdout, got %q", stdoutOut)
	}
}

func TestCLIError_EmptyMsg(t *testing.T) {
	code, stderrOut, _ := withExitCapture(func() {
		cliError("")
	})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stderrOut != "" {
		t.Fatalf("expected empty stderr for empty msg, got %q", stderrOut)
	}
}

func TestCLIOk_WritesToStdout(t *testing.T) {
	code, stderrOut, stdoutOut := withExitCapture(func() {
		cliOk("all good")
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stdoutOut != "all good\n" {
		t.Fatalf("expected stdout %q, got %q", "all good\n", stdoutOut)
	}
	if stderrOut != "" {
		t.Fatalf("expected empty stderr, got %q", stderrOut)
	}
}

func TestCLIOk_EmptyMsg(t *testing.T) {
	code, _, stdoutOut := withExitCapture(func() {
		cliOk("")
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stdoutOut != "" {
		t.Fatalf("expected empty stdout for empty msg, got %q", stdoutOut)
	}
}

func TestCLIErrorf(t *testing.T) {
	code, stderrOut, _ := withExitCapture(func() {
		cliErrorf("bad value: %d", 42)
	})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stderrOut != "bad value: 42\n" {
		t.Fatalf("expected stderr %q, got %q", "bad value: 42\n", stderrOut)
	}
}

// TestMainGo_NoDirectOsExit scans all non-test Go files for raw os.Exit calls.
// All exit paths must use cliError / cliErrorf / cliOk instead.
// The only allowed file for os.Exit is exit.go (which defines exitFunc).
func TestMainGo_NoDirectOsExit(t *testing.T) {
	// Locate the package directory from this test file.
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(thisFile)

	// Pattern matches os.Exit( but not inside comments or the exitFunc var declaration.
	osExitRe := regexp.MustCompile(`\bos\.Exit\(`)

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("reading package dir: %v", err)
	}

	for _, e := range entries {
		name := e.Name()
		// Skip test files, exit.go (defines exitFunc = os.Exit), and non-Go files.
		if strings.HasSuffix(name, "_test.go") || name == "exit.go" || !strings.HasSuffix(name, ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(pkgDir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			// Skip comment-only lines.
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if osExitRe.MatchString(line) {
				t.Errorf("%s:%d: found direct os.Exit call — use cliError/cliErrorf/cliOk instead:\n  %s", name, i+1, trimmed)
			}
		}
	}
}

// TestNoDirectExitFunc scans all non-test Go files (except exit.go) for bare
// exitFunc( calls. All exit paths must go through cliError/cliErrorf/cliOk.
func TestNoDirectExitFunc(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(thisFile)

	exitFuncRe := regexp.MustCompile(`\bexitFunc\(`)

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("reading package dir: %v", err)
	}

	for _, e := range entries {
		name := e.Name()
		// Skip test files, exit.go (defines exitFunc and the helpers), and non-Go files.
		if strings.HasSuffix(name, "_test.go") || name == "exit.go" || !strings.HasSuffix(name, ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(pkgDir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if exitFuncRe.MatchString(line) {
				t.Errorf("%s:%d: found direct exitFunc call — use cliError/cliErrorf/cliOk instead:\n  %s", name, i+1, trimmed)
			}
		}
	}
}
