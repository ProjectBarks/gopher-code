package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// T101: DirectConnectError
// ---------------------------------------------------------------------------

func TestDirectConnectError_ImplementsError(t *testing.T) {
	var err error = &DirectConnectError{Msg: "test error"}
	if err.Error() != "test error" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test error")
	}
}

func TestIsDirectConnectError(t *testing.T) {
	if !IsDirectConnectError(&DirectConnectError{Msg: "x"}) {
		t.Error("expected true for *DirectConnectError")
	}
	if IsDirectConnectError(http.ErrAbortHandler) {
		t.Error("expected false for non-DirectConnectError")
	}
}

// ---------------------------------------------------------------------------
// T100 + T102: CreateDirectConnectSession
// ---------------------------------------------------------------------------

func TestCreateDirectConnectSession_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request shape (T102)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/sessions" {
			t.Errorf("path = %s, want /sessions", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer tok-123" {
			t.Errorf("Authorization = %q, want Bearer tok-123", auth)
		}

		// Verify body
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["cwd"] != "/home/user" {
			t.Errorf("cwd = %v", body["cwd"])
		}
		if body["dangerously_skip_permissions"] != true {
			t.Errorf("dangerously_skip_permissions = %v", body["dangerously_skip_permissions"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"session_id": "sess-abc",
			"ws_url":     "wss://example.com/ws/sess-abc",
			"work_dir":   "/work",
		})
	}))
	defer srv.Close()

	result, err := CreateDirectConnectSession(nil, CreateDirectConnectSessionOpts{
		ServerURL:                  srv.URL,
		AuthToken:                  "tok-123",
		CWD:                        "/home/user",
		DangerouslySkipPermissions: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Config.SessionID != "sess-abc" {
		t.Errorf("SessionID = %q", result.Config.SessionID)
	}
	if result.Config.WSURL != "wss://example.com/ws/sess-abc" {
		t.Errorf("WSURL = %q", result.Config.WSURL)
	}
	if result.Config.ServerURL != srv.URL {
		t.Errorf("ServerURL = %q", result.Config.ServerURL)
	}
	if result.Config.AuthToken != "tok-123" {
		t.Errorf("AuthToken = %q", result.Config.AuthToken)
	}
	if result.WorkDir != "/work" {
		t.Errorf("WorkDir = %q", result.WorkDir)
	}
}

func TestCreateDirectConnectSession_NoAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("expected no Authorization header, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"session_id": "sess-1",
			"ws_url":     "wss://example.com/ws",
		})
	}))
	defer srv.Close()

	result, err := CreateDirectConnectSession(nil, CreateDirectConnectSessionOpts{
		ServerURL: srv.URL,
		CWD:       "/tmp",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Config.AuthToken != "" {
		t.Errorf("expected empty AuthToken, got %q", result.Config.AuthToken)
	}
}

func TestCreateDirectConnectSession_NoDangerouslySkipPermissions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["dangerously_skip_permissions"]; ok {
			t.Error("dangerously_skip_permissions should not be present when false")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"session_id": "s1",
			"ws_url":     "wss://x",
		})
	}))
	defer srv.Close()

	_, err := CreateDirectConnectSession(nil, CreateDirectConnectSessionOpts{
		ServerURL: srv.URL,
		CWD:       "/tmp",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateDirectConnectSession_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := CreateDirectConnectSession(nil, CreateDirectConnectSessionOpts{
		ServerURL: srv.URL,
		CWD:       "/tmp",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsDirectConnectError(err) {
		t.Errorf("expected DirectConnectError, got %T", err)
	}
	dce := err.(*DirectConnectError)
	if dce.Msg == "" {
		t.Error("expected non-empty error message")
	}
	// TS: "Failed to create session: {status} {statusText}"
	if got := dce.Msg; got == "" {
		t.Error("expected error message about failed session creation")
	}
}

func TestCreateDirectConnectSession_NetworkError(t *testing.T) {
	_, err := CreateDirectConnectSession(nil, CreateDirectConnectSessionOpts{
		ServerURL: "http://localhost:1", // connection refused
		CWD:       "/tmp",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsDirectConnectError(err) {
		t.Errorf("expected DirectConnectError, got %T", err)
	}
	dce := err.(*DirectConnectError)
	// TS: "Failed to connect to server at {serverUrl}: {errorMessage}"
	if dce.Msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestCreateDirectConnectSession_InvalidResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Missing session_id
		json.NewEncoder(w).Encode(map[string]string{
			"ws_url": "wss://x",
		})
	}))
	defer srv.Close()

	_, err := CreateDirectConnectSession(nil, CreateDirectConnectSessionOpts{
		ServerURL: srv.URL,
		CWD:       "/tmp",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsDirectConnectError(err) {
		t.Errorf("expected DirectConnectError, got %T", err)
	}
	dce := err.(*DirectConnectError)
	// TS: "Invalid session response: {message}"
	if dce.Msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestCreateDirectConnectSession_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := CreateDirectConnectSession(nil, CreateDirectConnectSessionOpts{
		ServerURL: srv.URL,
		CWD:       "/tmp",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsDirectConnectError(err) {
		t.Errorf("expected DirectConnectError, got %T", err)
	}
}
