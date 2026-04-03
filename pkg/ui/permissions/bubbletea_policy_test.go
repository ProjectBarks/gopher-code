package permissions

import (
	"context"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/ui/components"
)

func TestBubbleTeaPolicyCreation(t *testing.T) {
	policy := NewBubbleTeaPolicy(5 * time.Second)
	if policy == nil {
		t.Fatal("BubbleTeaPolicy should not be nil")
	}
}

func TestBubbleTeaPolicyDefaultTimeout(t *testing.T) {
	policy := NewBubbleTeaPolicy(0)
	if policy.timeout != 30*time.Second {
		t.Errorf("Expected 30s default timeout, got %v", policy.timeout)
	}
}

func TestBubbleTeaPolicyCheckApproval(t *testing.T) {
	policy := NewBubbleTeaPolicy(5 * time.Second)

	// Simulate UI handling in a goroutine
	go func() {
		req := <-policy.Requests()
		if req.ToolName != "bash" {
			t.Errorf("Expected tool name 'bash', got %s", req.ToolName)
		}
		req.ResponseCh <- permissions.AllowDecision{}
	}()

	ctx := context.Background()
	decision := policy.Check(ctx, "bash", "t1")

	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Errorf("Expected AllowDecision, got %T", decision)
	}
}

func TestBubbleTeaPolicyCheckDenial(t *testing.T) {
	policy := NewBubbleTeaPolicy(5 * time.Second)

	go func() {
		req := <-policy.Requests()
		req.ResponseCh <- permissions.DenyDecision{Reason: "denied by user"}
	}()

	ctx := context.Background()
	decision := policy.Check(ctx, "bash", "t1")

	deny, ok := decision.(permissions.DenyDecision)
	if !ok {
		t.Fatalf("Expected DenyDecision, got %T", decision)
	}
	if deny.Reason != "denied by user" {
		t.Errorf("Expected 'denied by user', got %q", deny.Reason)
	}
}

func TestBubbleTeaPolicyCheckTimeout(t *testing.T) {
	policy := NewBubbleTeaPolicy(50 * time.Millisecond)

	// Don't respond — should timeout
	go func() {
		<-policy.Requests() // consume the request but don't respond
	}()

	ctx := context.Background()
	decision := policy.Check(ctx, "bash", "t1")

	deny, ok := decision.(permissions.DenyDecision)
	if !ok {
		t.Fatalf("Expected DenyDecision on timeout, got %T", decision)
	}
	if deny.Reason != "permission timeout" {
		t.Errorf("Expected 'permission timeout', got %q", deny.Reason)
	}
}

func TestBubbleTeaPolicyCheckContextCancelled(t *testing.T) {
	policy := NewBubbleTeaPolicy(5 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	decision := policy.Check(ctx, "bash", "t1")

	deny, ok := decision.(permissions.DenyDecision)
	if !ok {
		t.Fatalf("Expected DenyDecision on cancel, got %T", decision)
	}
	if deny.Reason != "context cancelled" {
		t.Errorf("Expected 'context cancelled', got %q", deny.Reason)
	}
}

func TestHandleApproval(t *testing.T) {
	responseCh := make(chan permissions.PermissionDecision, 1)
	req := PermissionRequest{
		ToolName:   "bash",
		ToolID:     "t1",
		ResponseCh: responseCh,
	}

	HandleApproval(req, components.ApprovalApproved)

	select {
	case decision := <-responseCh:
		if _, ok := decision.(permissions.AllowDecision); !ok {
			t.Errorf("Expected AllowDecision, got %T", decision)
		}
	default:
		t.Error("Expected decision on channel")
	}
}

func TestHandleApprovalDeny(t *testing.T) {
	responseCh := make(chan permissions.PermissionDecision, 1)
	req := PermissionRequest{
		ToolName:   "bash",
		ToolID:     "t1",
		ResponseCh: responseCh,
	}

	HandleApproval(req, components.ApprovalRejected)

	select {
	case decision := <-responseCh:
		if _, ok := decision.(permissions.DenyDecision); !ok {
			t.Errorf("Expected DenyDecision, got %T", decision)
		}
	default:
		t.Error("Expected decision on channel")
	}
}
