package bridge

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// ValidateBridgeID
// ---------------------------------------------------------------------------

func TestValidateBridgeID_Valid(t *testing.T) {
	valid := []string{
		"abc", "ABC", "123", "a-b", "a_b", "aB1-_2",
		"env-abc-123", "work_item_42",
	}
	for _, id := range valid {
		got, err := ValidateBridgeID(id, "test")
		if err != nil {
			t.Errorf("ValidateBridgeID(%q) unexpected error: %v", id, err)
		}
		if got != id {
			t.Errorf("ValidateBridgeID(%q) = %q, want %q", id, got, id)
		}
	}
}

func TestValidateBridgeID_Invalid(t *testing.T) {
	invalid := []string{
		"", "../admin", "foo/bar", "foo.bar", "hello world",
		"id;drop", "a\x00b", "path/../traversal",
	}
	for _, id := range invalid {
		_, err := ValidateBridgeID(id, "myLabel")
		if err == nil {
			t.Errorf("ValidateBridgeID(%q) expected error, got nil", id)
			continue
		}
		if !strings.Contains(err.Error(), "Invalid myLabel") {
			t.Errorf("ValidateBridgeID(%q) error = %q, want it to contain 'Invalid myLabel'", id, err.Error())
		}
		if !strings.Contains(err.Error(), "contains unsafe characters") {
			t.Errorf("ValidateBridgeID(%q) error = %q, want 'contains unsafe characters'", id, err.Error())
		}
	}
}

// ---------------------------------------------------------------------------
// BridgeFatalError
// ---------------------------------------------------------------------------

func TestBridgeFatalError_Error(t *testing.T) {
	e := &BridgeFatalError{Msg: "boom", Status: 401, ErrorType: "auth_expired"}
	if e.Error() != "boom" {
		t.Errorf("Error() = %q, want %q", e.Error(), "boom")
	}
	if e.Status != 401 {
		t.Errorf("Status = %d, want 401", e.Status)
	}
	if e.ErrorType != "auth_expired" {
		t.Errorf("ErrorType = %q, want %q", e.ErrorType, "auth_expired")
	}
}

// ---------------------------------------------------------------------------
// IsExpiredErrorType
// ---------------------------------------------------------------------------

func TestIsExpiredErrorType(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"unknown", false},
		{"expired", true},
		{"environment_expired", true},
		{"session_lifetime_exceeded", true},
		{"lifetime", true},
	}
	for _, c := range cases {
		got := IsExpiredErrorType(c.input)
		if got != c.want {
			t.Errorf("IsExpiredErrorType(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// IsSuppressible403
// ---------------------------------------------------------------------------

func TestIsSuppressible403(t *testing.T) {
	cases := []struct {
		status int
		msg    string
		want   bool
	}{
		{403, "missing scope external_poll_sessions", true},
		{403, "requires environments:manage permission", true},
		{403, "generic forbidden", false},
		{401, "external_poll_sessions", false}, // wrong status
	}
	for _, c := range cases {
		err := &BridgeFatalError{Status: c.status, Msg: c.msg}
		got := IsSuppressible403(err)
		if got != c.want {
			t.Errorf("IsSuppressible403(status=%d, msg=%q) = %v, want %v", c.status, c.msg, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// handleErrorStatus
// ---------------------------------------------------------------------------

func TestHandleErrorStatus_Success(t *testing.T) {
	for _, code := range []int{200, 204} {
		if err := handleErrorStatus(code, nil, "Test"); err != nil {
			t.Errorf("handleErrorStatus(%d) = %v, want nil", code, err)
		}
	}
}

func TestHandleErrorStatus_401(t *testing.T) {
	data, _ := json.Marshal(map[string]any{"error": map[string]any{"type": "auth_error", "message": "bad token"}})
	err := handleErrorStatus(401, data, "Ctx")
	var fatal *BridgeFatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("expected BridgeFatalError, got %T: %v", err, err)
	}
	if fatal.Status != 401 {
		t.Errorf("status = %d, want 401", fatal.Status)
	}
	if !strings.Contains(fatal.Msg, "Authentication failed (401)") {
		t.Errorf("msg = %q, missing 'Authentication failed (401)'", fatal.Msg)
	}
	if !strings.Contains(fatal.Msg, BridgeLoginInstruction) {
		t.Errorf("msg missing login instruction")
	}
}

func TestHandleErrorStatus_403_Expired(t *testing.T) {
	data, _ := json.Marshal(map[string]any{"error": map[string]any{"type": "session_expired"}})
	err := handleErrorStatus(403, data, "Ctx")
	var fatal *BridgeFatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("expected BridgeFatalError, got %T", err)
	}
	if !strings.Contains(fatal.Msg, "session has expired") {
		t.Errorf("expected expired message, got %q", fatal.Msg)
	}
}

func TestHandleErrorStatus_410(t *testing.T) {
	err := handleErrorStatus(410, nil, "Ctx")
	var fatal *BridgeFatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("expected BridgeFatalError, got %T", err)
	}
	if fatal.Status != 410 {
		t.Errorf("status = %d, want 410", fatal.Status)
	}
	if fatal.ErrorType != "environment_expired" {
		t.Errorf("errorType = %q, want environment_expired", fatal.ErrorType)
	}
}

func TestHandleErrorStatus_429(t *testing.T) {
	err := handleErrorStatus(429, nil, "Poll")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Rate limited (429)") {
		t.Errorf("error = %q, missing rate limit text", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Endpoint URL construction via httptest
// ---------------------------------------------------------------------------

func TestEndpointURLs(t *testing.T) {
	var mu sync.Mutex
	var captured []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		captured = append(captured, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		// Return valid JSON for all endpoints.
		switch {
		case strings.HasSuffix(r.URL.Path, "/bridge") && r.Method == "POST":
			json.NewEncoder(w).Encode(map[string]string{"environment_id": "env1", "environment_secret": "sec1"})
		case strings.HasSuffix(r.URL.Path, "/heartbeat"):
			json.NewEncoder(w).Encode(map[string]any{"lease_extended": true, "state": "active"})
		case strings.HasSuffix(r.URL.Path, "/poll"):
			w.Write([]byte("null"))
		default:
			w.WriteHeader(200)
			w.Write([]byte("{}"))
		}
	}))
	defer srv.Close()

	client := NewBridgeAPIClient(BridgeAPIClientConfig{
		BaseURL:        srv.URL,
		RunnerVersion:  "1.0.0",
		GetAccessToken: func() string { return "tok" },
	})

	// Call every endpoint.
	client.RegisterBridgeEnvironment(BridgeConfig{
		MachineName: "m", Dir: "/tmp", Branch: "main", MaxSessions: 1, WorkerType: "claude_code",
	})
	client.PollForWork("env1", "sec1", nil)
	client.AcknowledgeWork("env1", "work1", "stok")
	client.StopWork("env1", "work1", false)
	client.DeregisterEnvironment("env1")
	client.ArchiveSession("sess1")
	client.ReconnectSession("env1", "sess1")
	client.HeartbeatWork("env1", "work1", "stok")
	client.SendPermissionResponseEvent("sess1", PermissionResponseEvent{Type: "control_response"}, "stok")

	mu.Lock()
	defer mu.Unlock()

	expected := []string{
		"POST /v1/environments/bridge",
		"GET /v1/environments/env1/work/poll",
		"POST /v1/environments/env1/work/work1/ack",
		"POST /v1/environments/env1/work/work1/stop",
		"DELETE /v1/environments/bridge/env1",
		"POST /v1/sessions/sess1/archive",
		"POST /v1/environments/env1/bridge/reconnect",
		"POST /v1/environments/env1/work/work1/heartbeat",
		"POST /v1/sessions/sess1/events",
	}

	if len(captured) != len(expected) {
		t.Fatalf("captured %d requests, want %d: %v", len(captured), len(expected), captured)
	}
	for i, want := range expected {
		if captured[i] != want {
			t.Errorf("request[%d] = %q, want %q", i, captured[i], want)
		}
	}
}

// ---------------------------------------------------------------------------
// Header injection via httptest
// ---------------------------------------------------------------------------

func TestHeaderInjection(t *testing.T) {
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"environment_id": "e", "environment_secret": "s"})
	}))
	defer srv.Close()

	client := NewBridgeAPIClient(BridgeAPIClientConfig{
		BaseURL:               srv.URL,
		RunnerVersion:         "2.5.0",
		GetAccessToken:        func() string { return "my-access-token" },
		GetTrustedDeviceToken: func() string { return "device-tok-123" },
	})

	client.RegisterBridgeEnvironment(BridgeConfig{
		MachineName: "m", Dir: "/tmp", Branch: "main", MaxSessions: 1, WorkerType: "claude_code",
	})

	// Verify required headers.
	checks := map[string]string{
		"Authorization":                "Bearer my-access-token",
		"Content-Type":                 "application/json",
		"Anthropic-Version":            AnthropicVersion,
		"Anthropic-Beta":               BetaHeader,
		"X-Environment-Runner-Version": "2.5.0",
		"X-Trusted-Device-Token":       "device-tok-123",
	}

	for key, want := range checks {
		got := capturedHeaders.Get(key)
		if got != want {
			t.Errorf("header %q = %q, want %q", key, got, want)
		}
	}
}

func TestHeaderInjection_NoDeviceToken(t *testing.T) {
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"environment_id": "e", "environment_secret": "s"})
	}))
	defer srv.Close()

	client := NewBridgeAPIClient(BridgeAPIClientConfig{
		BaseURL:        srv.URL,
		RunnerVersion:  "1.0.0",
		GetAccessToken: func() string { return "tok" },
		// No GetTrustedDeviceToken — header should be absent.
	})

	client.RegisterBridgeEnvironment(BridgeConfig{
		MachineName: "m", Dir: "/tmp", Branch: "main", MaxSessions: 1, WorkerType: "claude_code",
	})

	if got := capturedHeaders.Get("X-Trusted-Device-Token"); got != "" {
		t.Errorf("X-Trusted-Device-Token should be absent, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// OAuth 401 retry
// ---------------------------------------------------------------------------

func TestOAuth401Retry(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":{"type":"auth_error","message":"expired"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"environment_id": "e", "environment_secret": "s"})
	}))
	defer srv.Close()

	tokenNum := 0
	client := NewBridgeAPIClient(BridgeAPIClientConfig{
		BaseURL:       srv.URL,
		RunnerVersion: "1.0.0",
		GetAccessToken: func() string {
			tokenNum++
			if tokenNum <= 1 {
				return "old-tok"
			}
			return "new-tok"
		},
		OnAuth401: func(stale string) (bool, error) {
			if stale != "old-tok" {
				t.Errorf("OnAuth401 got stale=%q, want old-tok", stale)
			}
			return true, nil
		},
	})

	resp, err := client.RegisterBridgeEnvironment(BridgeConfig{
		MachineName: "m", Dir: "/tmp", Branch: "main", MaxSessions: 1, WorkerType: "claude_code",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.EnvironmentID != "e" {
		t.Errorf("environment_id = %q, want %q", resp.EnvironmentID, "e")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

// ---------------------------------------------------------------------------
// PollForWork — empty poll counter
// ---------------------------------------------------------------------------

func TestPollForWork_EmptyPollCounter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("null"))
	}))
	defer srv.Close()

	var debugMsgs []string
	client := NewBridgeAPIClient(BridgeAPIClientConfig{
		BaseURL:        srv.URL,
		RunnerVersion:  "1.0.0",
		GetAccessToken: func() string { return "tok" },
		OnDebug:        func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	// First poll should log (consecutiveEmptyPolls == 1).
	resp, err := client.PollForWork("env1", "sec1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response for empty poll")
	}
	if len(debugMsgs) == 0 || !strings.Contains(debugMsgs[len(debugMsgs)-1], "1 consecutive empty polls") {
		t.Errorf("expected first empty poll debug log, got %v", debugMsgs)
	}
}

// ---------------------------------------------------------------------------
// ArchiveSession — 409 idempotent
// ---------------------------------------------------------------------------

func TestArchiveSession_409Idempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewBridgeAPIClient(BridgeAPIClientConfig{
		BaseURL:        srv.URL,
		RunnerVersion:  "1.0.0",
		GetAccessToken: func() string { return "tok" },
	})

	err := client.ArchiveSession("sess1")
	if err != nil {
		t.Fatalf("409 should not be an error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate IDs are checked before requests
// ---------------------------------------------------------------------------

func TestEndpoints_RejectUnsafeIDs(t *testing.T) {
	client := NewBridgeAPIClient(BridgeAPIClientConfig{
		BaseURL:        "http://unused",
		RunnerVersion:  "1.0.0",
		GetAccessToken: func() string { return "tok" },
	})

	badID := "../evil"

	if _, err := client.PollForWork(badID, "sec", nil); err == nil {
		t.Error("PollForWork should reject unsafe environmentId")
	}
	if err := client.AcknowledgeWork(badID, "w", "t"); err == nil {
		t.Error("AcknowledgeWork should reject unsafe environmentId")
	}
	if err := client.AcknowledgeWork("env", badID, "t"); err == nil {
		t.Error("AcknowledgeWork should reject unsafe workId")
	}
	if err := client.StopWork(badID, "w", false); err == nil {
		t.Error("StopWork should reject unsafe environmentId")
	}
	if err := client.DeregisterEnvironment(badID); err == nil {
		t.Error("DeregisterEnvironment should reject unsafe environmentId")
	}
	if err := client.ArchiveSession(badID); err == nil {
		t.Error("ArchiveSession should reject unsafe sessionId")
	}
	if err := client.ReconnectSession(badID, "s"); err == nil {
		t.Error("ReconnectSession should reject unsafe environmentId")
	}
	if err := client.ReconnectSession("env", badID); err == nil {
		t.Error("ReconnectSession should reject unsafe sessionId")
	}
	if _, err := client.HeartbeatWork(badID, "w", "t"); err == nil {
		t.Error("HeartbeatWork should reject unsafe environmentId")
	}
	if err := client.SendPermissionResponseEvent(badID, PermissionResponseEvent{}, "t"); err == nil {
		t.Error("SendPermissionResponseEvent should reject unsafe sessionId")
	}
}

// ---------------------------------------------------------------------------
// NewBridgeAPIClientFromConfig integration test
// ---------------------------------------------------------------------------

func TestNewBridgeAPIClientFromConfig_Integration(t *testing.T) {
	// Verify the convenience constructor produces a working client that is
	// reachable through the same code path used by cmd/gopher-code/main.go.
	cfg := BridgeConfig{
		Dir:           "/tmp/test",
		APIBaseURL:    "https://api.anthropic.com",
		MaxSessions:   SpawnSessionsDefault,
		SpawnMode:     SpawnModeSingleSession,
		WorkerType:    string(WorkerTypeClaudeCode),
		MachineName:   "test-machine",
	}

	var debugMsgs []string
	client := NewBridgeAPIClientFromConfig(cfg, func() string {
		return "test-token"
	}, func(msg string) {
		debugMsgs = append(debugMsgs, msg)
	})

	if client == nil {
		t.Fatal("NewBridgeAPIClientFromConfig returned nil")
	}

	// The client must satisfy the BridgeAPIClient interface — invoke a method
	// that performs local validation only (no network call). ValidateBridgeID
	// rejects the empty string, so PollForWork("", ...) must return an error.
	_, err := client.PollForWork("", "secret", nil)
	if err == nil {
		t.Fatal("expected error for empty environmentId, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid") {
		t.Errorf("expected validation error, got: %s", err)
	}

	// Also exercise via the orchestrator path to prove the types compose.
	orch := NewBridgeOrchestrator()
	orch.API = client
	orch.Config = cfg
	if orch.API == nil {
		t.Fatal("orchestrator API field is nil after assignment")
	}

	// Verify the client works against a real HTTP server.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"environment_id":     "env-123",
			"environment_secret": "sec-456",
		})
	}))
	defer ts.Close()

	serverCfg := BridgeConfig{
		Dir:        "/tmp/test",
		APIBaseURL: ts.URL,
	}
	serverClient := NewBridgeAPIClientFromConfig(serverCfg, func() string {
		return "tok"
	}, nil)

	resp, err := serverClient.RegisterBridgeEnvironment(serverCfg)
	if err != nil {
		t.Fatalf("RegisterBridgeEnvironment failed: %v", err)
	}
	if resp.EnvironmentID != "env-123" {
		t.Errorf("EnvironmentID = %q, want %q", resp.EnvironmentID, "env-123")
	}
}
