package bridge

import (
	"sync"
	"testing"
	"time"
)

// TestPermissionCallbackDispatch exercises the full request/response lifecycle:
// register a handler via OnResponse, send a response, verify the handler fires.
func TestPermissionCallbackDispatch(t *testing.T) {
	pc := NewPermissionCallbacks()

	var got BridgePermissionResponse
	var mu sync.Mutex
	done := make(chan struct{})

	unsub := pc.OnResponse("req-1", func(resp BridgePermissionResponse) {
		mu.Lock()
		got = resp
		mu.Unlock()
		close(done)
	})
	defer unsub()

	// Simulate a response arriving from the web app.
	pc.SendResponse("req-1", BridgePermissionResponse{
		Behavior: BehaviorAllow,
		Message:  "approved",
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for handler")
	}

	mu.Lock()
	defer mu.Unlock()
	if got.Behavior != BehaviorAllow {
		t.Errorf("behavior = %q, want %q", got.Behavior, BehaviorAllow)
	}
	if got.Message != "approved" {
		t.Errorf("message = %q, want %q", got.Message, "approved")
	}
}

// TestPermissionCallbackSendRequest verifies SendFunc is called.
func TestPermissionCallbackSendRequest(t *testing.T) {
	pc := NewPermissionCallbacks()

	var called bool
	pc.SendFunc = func(req BridgePermissionRequest) error {
		called = true
		if req.ToolName != "Bash" {
			t.Errorf("tool = %q, want %q", req.ToolName, "Bash")
		}
		return nil
	}

	err := pc.SendRequest(BridgePermissionRequest{
		RequestID: "req-2",
		ToolName:  "Bash",
		Input:     map[string]any{"command": "ls"},
	})
	if err != nil {
		t.Fatalf("SendRequest error: %v", err)
	}
	if !called {
		t.Error("SendFunc was not called")
	}
}

// TestPermissionCallbackCancelRequest verifies cancellation cleans up handlers.
func TestPermissionCallbackCancelRequest(t *testing.T) {
	pc := NewPermissionCallbacks()

	handlerCalled := false
	pc.OnResponse("req-3", func(_ BridgePermissionResponse) {
		handlerCalled = true
	})

	if err := pc.CancelRequest("req-3"); err != nil {
		t.Fatalf("CancelRequest error: %v", err)
	}

	// After cancel, sending a response should not invoke the handler.
	pc.SendResponse("req-3", BridgePermissionResponse{Behavior: BehaviorDeny})
	if handlerCalled {
		t.Error("handler was called after cancel")
	}
}

// TestPermissionCallbackWaitForResponse verifies the blocking helper.
func TestPermissionCallbackWaitForResponse(t *testing.T) {
	pc := NewPermissionCallbacks()

	go func() {
		time.Sleep(50 * time.Millisecond)
		pc.SendResponse("req-4", BridgePermissionResponse{
			Behavior: BehaviorDeny,
			Message:  "denied by user",
		})
	}()

	resp := pc.WaitForResponse("req-4", 2*time.Second)
	if resp == nil {
		t.Fatal("WaitForResponse returned nil (timeout)")
	}
	if resp.Behavior != BehaviorDeny {
		t.Errorf("behavior = %q, want %q", resp.Behavior, BehaviorDeny)
	}
}

// TestPermissionCallbackWaitTimeout verifies timeout returns nil.
func TestPermissionCallbackWaitTimeout(t *testing.T) {
	pc := NewPermissionCallbacks()

	resp := pc.WaitForResponse("req-never", 50*time.Millisecond)
	if resp != nil {
		t.Errorf("expected nil on timeout, got %+v", resp)
	}
}

// TestParsePermissionResponse exercises the JSON parser.
func TestParsePermissionResponse(t *testing.T) {
	valid := `{"behavior":"allow","message":"ok"}`
	resp, err := ParseBridgePermissionResponse([]byte(valid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Behavior != BehaviorAllow {
		t.Errorf("behavior = %q, want %q", resp.Behavior, BehaviorAllow)
	}

	// Invalid behavior
	_, err = ParseBridgePermissionResponse([]byte(`{"behavior":"maybe"}`))
	if err == nil {
		t.Error("expected error for invalid behavior")
	}

	// Malformed JSON
	_, err = ParseBridgePermissionResponse([]byte(`{broken`))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}
