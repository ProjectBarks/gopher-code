package bridge

import (
	"testing"
)

// ---------------------------------------------------------------------------
// GetTransportForUrl — transport factory tests
// Source: src/cli/transports/transportUtils.ts
// ---------------------------------------------------------------------------

func TestGetTransportForUrl_CCRV2_WSS_SelectsSSE(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "1")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "")

	sel, err := GetTransportForUrl(
		"wss://example.com/sessions/abc123/",
		map[string]string{"Authorization": "Bearer tok"},
		"sess-1",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Kind != TransportKindSSE {
		t.Errorf("expected TransportKindSSE, got %d", sel.Kind)
	}
	// Scheme must be swapped from wss→https.
	if sel.URL.Scheme != "https" {
		t.Errorf("expected scheme https, got %s", sel.URL.Scheme)
	}
	// Path must have trailing slash stripped and /worker/events/stream appended.
	wantPath := "/sessions/abc123/worker/events/stream"
	if sel.URL.Path != wantPath {
		t.Errorf("expected path %q, got %q", wantPath, sel.URL.Path)
	}
	if sel.SessionID != "sess-1" {
		t.Errorf("expected sessionID sess-1, got %s", sel.SessionID)
	}
}

func TestGetTransportForUrl_CCRV2_WS_SelectsSSE(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "true")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "")

	sel, err := GetTransportForUrl(
		"ws://localhost:8080/sessions/def456",
		nil,
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Kind != TransportKindSSE {
		t.Errorf("expected TransportKindSSE, got %d", sel.Kind)
	}
	if sel.URL.Scheme != "http" {
		t.Errorf("expected scheme http, got %s", sel.URL.Scheme)
	}
	wantPath := "/sessions/def456/worker/events/stream"
	if sel.URL.Path != wantPath {
		t.Errorf("expected path %q, got %q", wantPath, sel.URL.Path)
	}
}

func TestGetTransportForUrl_PostIngress_SelectsHybrid(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "1")

	sel, err := GetTransportForUrl(
		"wss://example.com/sessions/abc",
		map[string]string{"X-Custom": "val"},
		"sess-2",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Kind != TransportKindHybrid {
		t.Errorf("expected TransportKindHybrid, got %d", sel.Kind)
	}
	if sel.URL.Scheme != "wss" {
		t.Errorf("expected scheme wss, got %s", sel.URL.Scheme)
	}
}

func TestGetTransportForUrl_WS_DefaultSelectsWebSocket(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "")

	sel, err := GetTransportForUrl(
		"ws://localhost:9090/sessions/xyz",
		nil,
		"sess-3",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Kind != TransportKindWebSocket {
		t.Errorf("expected TransportKindWebSocket, got %d", sel.Kind)
	}
	if sel.URL.Scheme != "ws" {
		t.Errorf("expected scheme ws, got %s", sel.URL.Scheme)
	}
}

func TestGetTransportForUrl_WSS_DefaultSelectsWebSocket(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "")

	sel, err := GetTransportForUrl(
		"wss://example.com/sessions/xyz",
		nil,
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Kind != TransportKindWebSocket {
		t.Errorf("expected TransportKindWebSocket, got %d", sel.Kind)
	}
}

func TestGetTransportForUrl_UnsupportedProtocol(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "")

	_, err := GetTransportForUrl(
		"https://example.com/sessions/abc",
		nil,
		"",
	)
	if err == nil {
		t.Fatal("expected error for unsupported protocol, got nil")
	}
	want := "Unsupported protocol: https:"
	if err.Error() != want {
		t.Errorf("expected error %q, got %q", want, err.Error())
	}
}

func TestGetTransportForUrl_CCRV2_Priority(t *testing.T) {
	// When both env vars are set, CCR_V2 takes priority → SSE.
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "1")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "1")

	sel, err := GetTransportForUrl(
		"wss://example.com/sessions/abc",
		nil,
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Kind != TransportKindSSE {
		t.Errorf("expected TransportKindSSE (v2 takes priority), got %d", sel.Kind)
	}
}

func TestGetTransportForUrl_CCRV2_HTTPScheme_Passthrough(t *testing.T) {
	// When CCR_V2 is set, non-WS schemes pass through without error.
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "1")

	sel, err := GetTransportForUrl(
		"https://example.com/sessions/abc",
		nil,
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Kind != TransportKindSSE {
		t.Errorf("expected TransportKindSSE, got %d", sel.Kind)
	}
	// https stays https (no swap needed).
	if sel.URL.Scheme != "https" {
		t.Errorf("expected scheme https, got %s", sel.URL.Scheme)
	}
}

func TestGetTransportForUrl_NilHeaders(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "")

	sel, err := GetTransportForUrl("wss://example.com/s", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Headers == nil {
		t.Error("expected non-nil headers map")
	}
}
