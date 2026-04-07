package main

import (
	"log/slog"
	"testing"
)

// TestSetupRemoteIO_NoEnv verifies that setupRemoteIO returns nil when
// CLAUDE_CODE_SESSION_STREAM_URL is not set (the common local-dev case).
func TestSetupRemoteIO_NoEnv(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_STREAM_URL", "")

	rio, err := setupRemoteIO(slog.Default())
	if err != nil {
		t.Fatalf("setupRemoteIO returned unexpected error: %v", err)
	}
	if rio != nil {
		rio.Close()
		t.Fatal("expected nil RemoteIO when CLAUDE_CODE_SESSION_STREAM_URL is empty")
	}
}

// TestSetupRemoteIO_WithStreamURL verifies that a RemoteIO is constructed
// when the stream URL env var is set, using the default noop transport.
func TestSetupRemoteIO_WithStreamURL(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_STREAM_URL", "wss://example.com/sessions/test-123")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess-integration-test")
	t.Setenv("CLAUDE_CODE_SESSION_INGRESS_TOKEN", "tok-test")
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_ENVIRONMENT_KIND", "")

	rio, err := setupRemoteIO(slog.Default())
	if err != nil {
		t.Fatalf("setupRemoteIO: %v", err)
	}
	if rio == nil {
		t.Fatal("expected non-nil RemoteIO when CLAUDE_CODE_SESSION_STREAM_URL is set")
	}
	defer rio.Close()

	// Verify the Reader is wired (non-nil).
	if rio.Reader() == nil {
		t.Error("RemoteIO.Reader() returned nil")
	}
}

// TestSetupRemoteIO_InvalidURL verifies that an invalid URL produces an error.
func TestSetupRemoteIO_InvalidURL(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_STREAM_URL", "://bad-url")
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_ENVIRONMENT_KIND", "")

	rio, err := setupRemoteIO(slog.Default())
	if err == nil {
		if rio != nil {
			rio.Close()
		}
		t.Fatal("expected error for invalid stream URL")
	}
}

// TestNoopTransport_Implements verifies that noopTransport satisfies the
// pkgcli.Transport interface at compile time and that its methods don't panic.
func TestNoopTransport_Implements(t *testing.T) {
	tr := &noopTransport{}

	if err := tr.Connect(); err != nil {
		t.Errorf("Connect: %v", err)
	}
	if err := tr.Write(map[string]string{"type": "test"}); err != nil {
		t.Errorf("Write: %v", err)
	}

	called := false
	tr.SetOnData(func(s string) { called = true })
	if tr.onData == nil {
		t.Error("SetOnData did not store callback")
	}
	tr.onData("hello")
	if !called {
		t.Error("onData callback was not invoked")
	}

	tr.SetOnClose(func() {})
	if tr.onClose == nil {
		t.Error("SetOnClose did not store callback")
	}

	// Close should not panic.
	tr.Close()
}
