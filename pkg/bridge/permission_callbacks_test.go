package bridge

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/permissions"
)

// ---------------------------------------------------------------------------
// BridgePermissionResponse — parsing / validation
// ---------------------------------------------------------------------------

func TestParseBridgePermissionResponse_Allow(t *testing.T) {
	raw := `{"behavior":"allow","message":"ok"}`
	resp, err := ParseBridgePermissionResponse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Behavior != BehaviorAllow {
		t.Fatalf("behavior = %q, want %q", resp.Behavior, BehaviorAllow)
	}
	if resp.Message != "ok" {
		t.Fatalf("message = %q, want %q", resp.Message, "ok")
	}
}

func TestParseBridgePermissionResponse_Deny(t *testing.T) {
	raw := `{"behavior":"deny","message":"not allowed"}`
	resp, err := ParseBridgePermissionResponse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Behavior != BehaviorDeny {
		t.Fatalf("behavior = %q, want %q", resp.Behavior, BehaviorDeny)
	}
}

func TestParseBridgePermissionResponse_InvalidBehavior(t *testing.T) {
	raw := `{"behavior":"maybe"}`
	_, err := ParseBridgePermissionResponse([]byte(raw))
	if err == nil {
		t.Fatal("expected error for invalid behavior, got nil")
	}
}

func TestParseBridgePermissionResponse_MissingBehavior(t *testing.T) {
	raw := `{"message":"no behavior"}`
	_, err := ParseBridgePermissionResponse([]byte(raw))
	if err == nil {
		t.Fatal("expected error for missing behavior, got nil")
	}
}

func TestParseBridgePermissionResponse_NotObject(t *testing.T) {
	raw := `"just a string"`
	_, err := ParseBridgePermissionResponse([]byte(raw))
	if err == nil {
		t.Fatal("expected error for non-object, got nil")
	}
}

func TestParseBridgePermissionResponse_WithUpdatedInput(t *testing.T) {
	raw := `{"behavior":"allow","updatedInput":{"file":"/tmp/x"}}`
	resp, err := ParseBridgePermissionResponse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UpdatedInput["file"] != "/tmp/x" {
		t.Fatalf("updatedInput.file = %v, want /tmp/x", resp.UpdatedInput["file"])
	}
}

func TestParseBridgePermissionResponse_WithUpdatedPermissions(t *testing.T) {
	raw := `{"behavior":"allow","updatedPermissions":[{"type":"addRules","destination":"session","rules":["Write(**)"]}]}`
	resp, err := ParseBridgePermissionResponse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.UpdatedPermissions) != 1 {
		t.Fatalf("len(updatedPermissions) = %d, want 1", len(resp.UpdatedPermissions))
	}
	if resp.UpdatedPermissions[0].Type != permissions.UpdateAddRules {
		t.Fatalf("type = %q, want %q", resp.UpdatedPermissions[0].Type, permissions.UpdateAddRules)
	}
}

// ---------------------------------------------------------------------------
// Behavior enum constants
// ---------------------------------------------------------------------------

func TestBehaviorConstants(t *testing.T) {
	if BehaviorAllow != "allow" {
		t.Fatalf("BehaviorAllow = %q, want %q", BehaviorAllow, "allow")
	}
	if BehaviorDeny != "deny" {
		t.Fatalf("BehaviorDeny = %q, want %q", BehaviorDeny, "deny")
	}
}

// ---------------------------------------------------------------------------
// BridgePermissionRequest — JSON round-trip
// ---------------------------------------------------------------------------

func TestBridgePermissionRequest_JSON(t *testing.T) {
	req := BridgePermissionRequest{
		RequestID:   "req-1",
		ToolName:    "Write",
		Input:       map[string]any{"file": "/tmp/x"},
		ToolUseID:   "tu-1",
		Description: "Write to /tmp/x",
		BlockedPath: "/tmp/x",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got BridgePermissionRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.RequestID != req.RequestID {
		t.Fatalf("requestID = %q, want %q", got.RequestID, req.RequestID)
	}
	if got.BlockedPath != req.BlockedPath {
		t.Fatalf("blockedPath = %q, want %q", got.BlockedPath, req.BlockedPath)
	}
}

// ---------------------------------------------------------------------------
// PermissionCallbacks — send request and receive response
// ---------------------------------------------------------------------------

func TestPermissionCallbacks_SendAndReceiveResponse(t *testing.T) {
	pc := NewPermissionCallbacks()

	var sent BridgePermissionRequest
	pc.SendFunc = func(req BridgePermissionRequest) error {
		sent = req
		return nil
	}

	req := BridgePermissionRequest{
		RequestID:   "req-42",
		ToolName:    "Bash",
		Input:       map[string]any{"command": "rm -rf /"},
		ToolUseID:   "tu-42",
		Description: "dangerous command",
	}

	// Register handler before sending
	var received BridgePermissionResponse
	done := make(chan struct{})
	pc.OnResponse("req-42", func(resp BridgePermissionResponse) {
		received = resp
		close(done)
	})

	// Send the request
	if err := pc.SendRequest(req); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if sent.RequestID != "req-42" {
		t.Fatalf("SendFunc not called with correct request")
	}

	// Simulate the web app responding
	pc.SendResponse("req-42", BridgePermissionResponse{
		Behavior: BehaviorAllow,
		Message:  "approved",
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response handler")
	}

	if received.Behavior != BehaviorAllow {
		t.Fatalf("behavior = %q, want %q", received.Behavior, BehaviorAllow)
	}
	if received.Message != "approved" {
		t.Fatalf("message = %q, want %q", received.Message, "approved")
	}
}

func TestPermissionCallbacks_DenyResponse(t *testing.T) {
	pc := NewPermissionCallbacks()

	resp := pc.WaitForResponse("req-deny", 5*time.Second)

	// Deliver deny from another goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		pc.SendResponse("req-deny", BridgePermissionResponse{
			Behavior: BehaviorDeny,
			Message:  "rejected by admin",
		})
	}()

	// Re-register since WaitForResponse was called before goroutine
	// Actually, WaitForResponse blocks so we need to call it in a goroutine
	ch := make(chan *BridgePermissionResponse, 1)
	go func() {
		r := pc.WaitForResponse("req-deny-2", 5*time.Second)
		ch <- r
	}()
	time.Sleep(10 * time.Millisecond)
	pc.SendResponse("req-deny-2", BridgePermissionResponse{
		Behavior: BehaviorDeny,
		Message:  "rejected by admin",
	})

	got := <-ch
	if got == nil {
		t.Fatal("expected deny response, got nil (timeout)")
	}
	if got.Behavior != BehaviorDeny {
		t.Fatalf("behavior = %q, want %q", got.Behavior, BehaviorDeny)
	}
	if got.Message != "rejected by admin" {
		t.Fatalf("message = %q, want %q", got.Message, "rejected by admin")
	}
	// Clean up the first unused response
	_ = resp
}

// ---------------------------------------------------------------------------
// PermissionCallbacks — timeout
// ---------------------------------------------------------------------------

func TestPermissionCallbacks_Timeout(t *testing.T) {
	pc := NewPermissionCallbacks()

	start := time.Now()
	resp := pc.WaitForResponse("req-timeout", 50*time.Millisecond)
	elapsed := time.Since(start)

	if resp != nil {
		t.Fatalf("expected nil on timeout, got %+v", resp)
	}
	if elapsed < 40*time.Millisecond {
		t.Fatalf("returned too quickly: %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// PermissionCallbacks — cancel request
// ---------------------------------------------------------------------------

func TestPermissionCallbacks_CancelRequest(t *testing.T) {
	pc := NewPermissionCallbacks()

	var cancelledID string
	pc.CancelFunc = func(id string) error {
		cancelledID = id
		return nil
	}

	// Register a handler
	called := false
	pc.OnResponse("req-cancel", func(_ BridgePermissionResponse) {
		called = true
	})

	// Cancel it
	if err := pc.CancelRequest("req-cancel"); err != nil {
		t.Fatalf("CancelRequest: %v", err)
	}
	if cancelledID != "req-cancel" {
		t.Fatalf("CancelFunc not called with correct ID")
	}

	// Sending a response after cancel should be a no-op
	pc.SendResponse("req-cancel", BridgePermissionResponse{Behavior: BehaviorAllow})
	if called {
		t.Fatal("handler should not be called after cancel")
	}
}

// ---------------------------------------------------------------------------
// PermissionCallbacks — unsubscribe
// ---------------------------------------------------------------------------

func TestPermissionCallbacks_Unsubscribe(t *testing.T) {
	pc := NewPermissionCallbacks()

	called := false
	unsub := pc.OnResponse("req-unsub", func(_ BridgePermissionResponse) {
		called = true
	})

	// Unsubscribe
	unsub()

	// Response should be a no-op
	pc.SendResponse("req-unsub", BridgePermissionResponse{Behavior: BehaviorAllow})
	if called {
		t.Fatal("handler should not be called after unsubscribe")
	}
}

// ---------------------------------------------------------------------------
// PermissionCallbacks — response to unknown requestID is no-op
// ---------------------------------------------------------------------------

func TestPermissionCallbacks_UnknownRequestID(t *testing.T) {
	pc := NewPermissionCallbacks()
	// Should not panic
	pc.SendResponse("nonexistent", BridgePermissionResponse{Behavior: BehaviorAllow})
}
