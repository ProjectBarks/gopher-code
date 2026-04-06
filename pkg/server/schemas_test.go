package server

import "testing"

// ---------------------------------------------------------------------------
// T94: ConnectResponse validation tests
// ---------------------------------------------------------------------------

func TestValidateConnectResponse_Valid(t *testing.T) {
	r := ConnectResponse{
		SessionID: "sess-abc",
		WSURL:     "wss://api.anthropic.com/ws/sess-abc",
		WorkDir:   "/home/user",
	}
	if err := ValidateConnectResponse(r); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateConnectResponse_ValidWithoutWorkDir(t *testing.T) {
	r := ConnectResponse{
		SessionID: "sess-abc",
		WSURL:     "wss://api.anthropic.com/ws/sess-abc",
	}
	if err := ValidateConnectResponse(r); err != nil {
		t.Errorf("expected valid (work_dir optional), got error: %v", err)
	}
}

func TestValidateConnectResponse_MissingSessionID(t *testing.T) {
	r := ConnectResponse{
		WSURL: "wss://example.com",
	}
	err := ValidateConnectResponse(r)
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
	if got := err.Error(); got != "connect response: session_id is required" {
		t.Errorf("error = %q", got)
	}
}

func TestValidateConnectResponse_MissingWSURL(t *testing.T) {
	r := ConnectResponse{
		SessionID: "sess-1",
	}
	err := ValidateConnectResponse(r)
	if err == nil {
		t.Fatal("expected error for missing ws_url")
	}
	if got := err.Error(); got != "connect response: ws_url is required" {
		t.Errorf("error = %q", got)
	}
}
