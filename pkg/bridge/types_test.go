package bridge

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestDefaultSessionTimeoutMS(t *testing.T) {
	if DefaultSessionTimeoutMS != 86_400_000 {
		t.Fatalf("DefaultSessionTimeoutMS = %d, want 86400000", DefaultSessionTimeoutMS)
	}
}

func TestBridgeLoginInstruction(t *testing.T) {
	want := "Remote Control is only available with claude.ai subscriptions. Please use `/login` to sign in with your claude.ai account."
	if BridgeLoginInstruction != want {
		t.Fatalf("BridgeLoginInstruction mismatch:\n got: %s\nwant: %s", BridgeLoginInstruction, want)
	}
}

func TestBridgeLoginError(t *testing.T) {
	want := "Error: You must be logged in to use Remote Control.\n\n" +
		"Remote Control is only available with claude.ai subscriptions. Please use `/login` to sign in with your claude.ai account."
	if BridgeLoginError != want {
		t.Fatalf("BridgeLoginError mismatch:\n got: %q\nwant: %q", BridgeLoginError, want)
	}
}

func TestRemoteControlDisconnectedMsg(t *testing.T) {
	if RemoteControlDisconnectedMsg != "Remote Control disconnected." {
		t.Fatalf("RemoteControlDisconnectedMsg = %q", RemoteControlDisconnectedMsg)
	}
}

// ---------------------------------------------------------------------------
// Enum string values
// ---------------------------------------------------------------------------

func TestSessionDoneStatusValues(t *testing.T) {
	cases := []struct {
		val  SessionDoneStatus
		want string
	}{
		{SessionDoneCompleted, "completed"},
		{SessionDoneFailed, "failed"},
		{SessionDoneInterrupted, "interrupted"},
	}
	for _, c := range cases {
		if string(c.val) != c.want {
			t.Errorf("SessionDoneStatus %q != %q", c.val, c.want)
		}
	}
}

func TestSessionActivityTypeValues(t *testing.T) {
	cases := []struct {
		val  SessionActivityType
		want string
	}{
		{ActivityToolStart, "tool_start"},
		{ActivityText, "text"},
		{ActivityResult, "result"},
		{ActivityError, "error"},
	}
	for _, c := range cases {
		if string(c.val) != c.want {
			t.Errorf("SessionActivityType %q != %q", c.val, c.want)
		}
	}
}

func TestSpawnModeValues(t *testing.T) {
	cases := []struct {
		val  SpawnMode
		want string
	}{
		{SpawnModeSingleSession, "single-session"},
		{SpawnModeWorktree, "worktree"},
		{SpawnModeSameDir, "same-dir"},
	}
	for _, c := range cases {
		if string(c.val) != c.want {
			t.Errorf("SpawnMode %q != %q", c.val, c.want)
		}
	}
}

func TestBridgeWorkerTypeValues(t *testing.T) {
	cases := []struct {
		val  BridgeWorkerType
		want string
	}{
		{WorkerTypeClaudeCode, "claude_code"},
		{WorkerTypeClaudeCodeAssistant, "claude_code_assistant"},
	}
	for _, c := range cases {
		if string(c.val) != c.want {
			t.Errorf("BridgeWorkerType %q != %q", c.val, c.want)
		}
	}
}

func TestWorkDataTypeValues(t *testing.T) {
	cases := []struct {
		val  WorkDataType
		want string
	}{
		{WorkDataTypeSession, "session"},
		{WorkDataTypeHealthcheck, "healthcheck"},
	}
	for _, c := range cases {
		if string(c.val) != c.want {
			t.Errorf("WorkDataType %q != %q", c.val, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// JSON round-trip tests
// ---------------------------------------------------------------------------

func TestWorkResponseJSON(t *testing.T) {
	orig := WorkResponse{
		ID:            "work-1",
		Type:          "work",
		EnvironmentID: "env-abc",
		State:         "pending",
		Data: WorkData{
			Type: WorkDataTypeSession,
			ID:   "sess-1",
		},
		Secret:    "base64payload",
		CreatedAt: "2025-01-01T00:00:00Z",
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	// Verify specific JSON keys match TS wire format.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "type", "environment_id", "state", "data", "secret", "created_at"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}

	var decoded WorkResponse
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != orig {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", decoded, orig)
	}
}

func TestWorkSecretJSON(t *testing.T) {
	useCS := true
	orig := WorkSecret{
		Version:             2,
		SessionIngressToken: "tok-abc",
		APIBaseURL:          "https://api.example.com",
		Sources: []WorkSecretSource{
			{Type: "git", GitInfo: &GitInfo{Type: "github", Repo: "owner/repo", Ref: "main"}},
		},
		Auth: []WorkSecretAuth{
			{Type: "bearer", Token: "secret-token"},
		},
		ClaudeCodeArgs:       map[string]string{"--model": "opus"},
		EnvironmentVariables: map[string]string{"FOO": "bar"},
		UseCodeSessions:      &useCS,
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	// Verify wire-format keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "session_ingress_token", "api_base_url", "sources", "auth", "claude_code_args", "environment_variables", "use_code_sessions"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}

	var decoded WorkSecret
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version != orig.Version ||
		decoded.SessionIngressToken != orig.SessionIngressToken ||
		decoded.APIBaseURL != orig.APIBaseURL ||
		*decoded.UseCodeSessions != *orig.UseCodeSessions {
		t.Errorf("round-trip scalar mismatch")
	}
}

func TestWorkSecretOmitsOptionalFields(t *testing.T) {
	orig := WorkSecret{
		Version:             1,
		SessionIngressToken: "tok",
		APIBaseURL:          "https://api.example.com",
		Sources:             []WorkSecretSource{},
		Auth:                []WorkSecretAuth{},
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"claude_code_args", "mcp_config", "environment_variables", "use_code_sessions"} {
		if _, ok := raw[key]; ok {
			t.Errorf("expected omitted JSON key %q to be absent, but it was present", key)
		}
	}
}

func TestBridgeConfigJSON(t *testing.T) {
	repoURL := "https://github.com/example/repo"
	timeout := 60000
	orig := BridgeConfig{
		Dir:                "/tmp/work",
		MachineName:        "my-machine",
		Branch:             "main",
		GitRepoURL:         &repoURL,
		MaxSessions:        5,
		SpawnMode:          SpawnModeWorktree,
		Verbose:            true,
		Sandbox:            false,
		BridgeID:           "bridge-uuid",
		WorkerType:         "claude_code",
		EnvironmentID:      "env-uuid",
		ReuseEnvironmentID: "env-backend-id",
		APIBaseURL:         "https://api.example.com",
		SessionIngressURL:  "https://ingress.example.com",
		DebugFile:          "/tmp/debug.log",
		SessionTimeoutMS:   &timeout,
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dir", "machine_name", "branch", "git_repo_url", "max_sessions", "spawn_mode", "verbose", "sandbox", "bridge_id", "worker_type", "environment_id", "reuse_environment_id", "api_base_url", "session_ingress_url", "debug_file", "session_timeout_ms"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}

	var decoded BridgeConfig
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Dir != orig.Dir || decoded.MachineName != orig.MachineName ||
		decoded.SpawnMode != orig.SpawnMode || decoded.MaxSessions != orig.MaxSessions ||
		*decoded.GitRepoURL != *orig.GitRepoURL || *decoded.SessionTimeoutMS != *orig.SessionTimeoutMS {
		t.Errorf("round-trip mismatch")
	}
}

func TestBridgeConfigOmitsOptionalFields(t *testing.T) {
	orig := BridgeConfig{
		Dir:           "/tmp",
		MachineName:   "m",
		Branch:        "main",
		MaxSessions:   1,
		SpawnMode:     SpawnModeSingleSession,
		BridgeID:      "b",
		WorkerType:    "claude_code",
		EnvironmentID: "e",
		APIBaseURL:    "https://api.example.com",
		SessionIngressURL: "https://ingress.example.com",
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"reuse_environment_id", "debug_file", "session_timeout_ms"} {
		if _, ok := raw[key]; ok {
			t.Errorf("expected omitted JSON key %q to be absent, but it was present", key)
		}
	}
}

func TestPermissionResponseEventJSON(t *testing.T) {
	orig := PermissionResponseEvent{
		Type: "control_response",
		Response: PermissionResponse{
			Subtype:   "success",
			RequestID: "req-123",
			Response:  map[string]any{"behavior": "allow"},
		},
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded PermissionResponseEvent
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != "control_response" {
		t.Errorf("type = %q, want control_response", decoded.Type)
	}
	if decoded.Response.Subtype != "success" {
		t.Errorf("subtype = %q, want success", decoded.Response.Subtype)
	}
	if decoded.Response.RequestID != "req-123" {
		t.Errorf("request_id = %q, want req-123", decoded.Response.RequestID)
	}

	// Verify wire keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["type"]; !ok {
		t.Error("missing JSON key \"type\"")
	}
	if _, ok := raw["response"]; !ok {
		t.Error("missing JSON key \"response\"")
	}
}

func TestHeartbeatResponseJSON(t *testing.T) {
	orig := HeartbeatResponse{LeaseExtended: true, State: "active"}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"lease_extended", "state"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}

	var decoded HeartbeatResponse
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != orig {
		t.Errorf("round-trip mismatch")
	}
}

func TestSessionActivityJSON(t *testing.T) {
	orig := SessionActivity{
		Type:      ActivityToolStart,
		Summary:   "Editing src/foo.ts",
		Timestamp: 1700000000000,
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"type", "summary", "timestamp"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}

	var decoded SessionActivity
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != orig {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, orig)
	}
}

func TestSessionSpawnOptsJSON(t *testing.T) {
	epoch := 42
	orig := SessionSpawnOpts{
		SessionID:   "sess-1",
		SDKURL:      "https://sdk.example.com",
		AccessToken: "tok",
		UseCcrV2:    true,
		WorkerEpoch: &epoch,
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"session_id", "sdk_url", "access_token", "use_ccr_v2", "worker_epoch"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}
}

func TestRegisterEnvironmentResponseJSON(t *testing.T) {
	orig := RegisterEnvironmentResponse{
		EnvironmentID:     "env-1",
		EnvironmentSecret: "secret-1",
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"environment_id", "environment_secret"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}

	var decoded RegisterEnvironmentResponse
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != orig {
		t.Errorf("round-trip mismatch")
	}
}
