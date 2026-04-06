package server

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// T90: SessionState enum contract tests
// ---------------------------------------------------------------------------

func TestSessionState_AllFiveStates(t *testing.T) {
	states := AllSessionStates()
	if len(states) != 5 {
		t.Fatalf("expected 5 states, got %d", len(states))
	}

	// Verify exact string values match TS literals.
	want := []string{"starting", "running", "detached", "stopping", "stopped"}
	for i, s := range states {
		if string(s) != want[i] {
			t.Errorf("state[%d] = %q, want %q", i, s, want[i])
		}
	}
}

func TestSessionState_Valid(t *testing.T) {
	for _, s := range AllSessionStates() {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if SessionState("unknown").Valid() {
		t.Error("unknown state should not be valid")
	}
}

func TestSessionState_IsTerminal(t *testing.T) {
	tests := []struct {
		state    SessionState
		terminal bool
	}{
		{SessionStarting, false},
		{SessionRunning, false},
		{SessionDetached, false},
		{SessionStopping, false},
		{SessionStopped, true},
	}
	for _, tt := range tests {
		if got := tt.state.IsTerminal(); got != tt.terminal {
			t.Errorf("%q.IsTerminal() = %v, want %v", tt.state, got, tt.terminal)
		}
	}
}

func TestSessionState_IsActive(t *testing.T) {
	tests := []struct {
		state  SessionState
		active bool
	}{
		{SessionStarting, true},
		{SessionRunning, true},
		{SessionDetached, true},
		{SessionStopping, false},
		{SessionStopped, false},
	}
	for _, tt := range tests {
		if got := tt.state.IsActive(); got != tt.active {
			t.Errorf("%q.IsActive() = %v, want %v", tt.state, got, tt.active)
		}
	}
}

// ---------------------------------------------------------------------------
// T89: ServerConfig contract tests
// ---------------------------------------------------------------------------

func TestServerConfig_JSONRoundTrip(t *testing.T) {
	cfg := ServerConfig{
		Port:          8080,
		Host:          "127.0.0.1",
		AuthToken:     "secret-token",
		Unix:          "/tmp/claude.sock",
		IdleTimeoutMs: 300000,
		MaxSessions:   10,
		Workspace:     "/home/user/projects",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ServerConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got != cfg {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", got, cfg)
	}
}

func TestServerConfig_JSONOmitsOptionalZeroValues(t *testing.T) {
	cfg := ServerConfig{
		Port:      3000,
		Host:      "0.0.0.0",
		AuthToken: "tok",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify optional fields with zero values are omitted.
	m := make(map[string]any)
	json.Unmarshal(data, &m)

	if _, ok := m["unix"]; ok {
		t.Error("unix should be omitted when empty")
	}
	if _, ok := m["idleTimeoutMs"]; ok {
		t.Error("idleTimeoutMs should be omitted when zero")
	}
	if _, ok := m["maxSessions"]; ok {
		t.Error("maxSessions should be omitted when zero")
	}
	if _, ok := m["workspace"]; ok {
		t.Error("workspace should be omitted when empty")
	}

	// Required fields must be present.
	if _, ok := m["port"]; !ok {
		t.Error("port should be present")
	}
	if _, ok := m["host"]; !ok {
		t.Error("host should be present")
	}
	if _, ok := m["authToken"]; !ok {
		t.Error("authToken should be present")
	}
}

func TestServerConfig_ZeroIdleTimeoutMeansNeverExpire(t *testing.T) {
	cfg := ServerConfig{
		Port:          8080,
		Host:          "localhost",
		AuthToken:     "tok",
		IdleTimeoutMs: 0,
	}
	// IdleTimeoutMs == 0 means never expire (no timeout).
	// Just verify it serializes correctly — the semantics are documented.
	data, _ := json.Marshal(cfg)
	var got ServerConfig
	json.Unmarshal(data, &got)
	if got.IdleTimeoutMs != 0 {
		t.Errorf("expected 0, got %d", got.IdleTimeoutMs)
	}
}

// ---------------------------------------------------------------------------
// ConnectResponse contract test
// ---------------------------------------------------------------------------

func TestConnectResponse_JSONShape(t *testing.T) {
	resp := ConnectResponse{
		SessionID: "sess-abc",
		WSURL:     "wss://api.anthropic.com/v1/sessions/ws/sess-abc/subscribe",
		WorkDir:   "/home/user/project",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify JSON keys match the TS schema exactly.
	m := make(map[string]any)
	json.Unmarshal(data, &m)

	if m["session_id"] != "sess-abc" {
		t.Errorf("session_id = %v", m["session_id"])
	}
	if m["ws_url"] != resp.WSURL {
		t.Errorf("ws_url = %v", m["ws_url"])
	}
	if m["work_dir"] != "/home/user/project" {
		t.Errorf("work_dir = %v", m["work_dir"])
	}
}

func TestConnectResponse_WorkDirOptional(t *testing.T) {
	resp := ConnectResponse{
		SessionID: "s1",
		WSURL:     "wss://example.com",
	}

	data, _ := json.Marshal(resp)
	m := make(map[string]any)
	json.Unmarshal(data, &m)

	if _, ok := m["work_dir"]; ok {
		t.Error("work_dir should be omitted when empty")
	}
}

// ---------------------------------------------------------------------------
// SessionIndexEntry / SessionIndex contract tests
// ---------------------------------------------------------------------------

func TestSessionIndexEntry_JSONRoundTrip(t *testing.T) {
	entry := SessionIndexEntry{
		SessionID:           "sess-1",
		TranscriptSessionID: "transcript-1",
		CWD:                 "/home/user",
		PermissionMode:      "auto",
		CreatedAt:           1700000000000,
		LastActiveAt:        1700000001000,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SessionIndexEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got != entry {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", got, entry)
	}
}

func TestSessionIndex_MapBehavior(t *testing.T) {
	idx := SessionIndex{
		"key-1": {
			SessionID:           "sess-1",
			TranscriptSessionID: "sess-1",
			CWD:                 "/tmp",
			CreatedAt:           1000,
			LastActiveAt:        2000,
		},
		"key-2": {
			SessionID:           "sess-2",
			TranscriptSessionID: "sess-2",
			CWD:                 "/home",
			PermissionMode:      "deny",
			CreatedAt:           3000,
			LastActiveAt:        4000,
		},
	}

	if len(idx) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(idx))
	}

	// Verify JSON round-trip of the full index.
	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SessionIndex
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 entries after round-trip, got %d", len(got))
	}

	e1 := got["key-1"]
	if e1.SessionID != "sess-1" || e1.CWD != "/tmp" {
		t.Errorf("key-1 mismatch: %+v", e1)
	}

	e2 := got["key-2"]
	if e2.PermissionMode != "deny" {
		t.Errorf("key-2 permissionMode = %q, want %q", e2.PermissionMode, "deny")
	}
}

// ---------------------------------------------------------------------------
// SessionInfo contract test
// ---------------------------------------------------------------------------

func TestSessionInfo_StatusUsesSessionState(t *testing.T) {
	info := SessionInfo{
		ID:        "sess-1",
		Status:    SessionRunning,
		CreatedAt: 1700000000000,
		WorkDir:   "/home/user",
	}

	if !info.Status.Valid() {
		t.Errorf("status %q should be valid", info.Status)
	}

	data, _ := json.Marshal(info)
	m := make(map[string]any)
	json.Unmarshal(data, &m)

	if m["status"] != "running" {
		t.Errorf("status JSON = %v, want %q", m["status"], "running")
	}
}
