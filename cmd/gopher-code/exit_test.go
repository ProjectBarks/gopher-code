package main

import (
	"bytes"
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
