package session

import "testing"

// Source: utils/teammateMailbox.ts:684-715, 885-948

func TestPlanApprovalRequest(t *testing.T) {
	// Source: teammateMailbox.ts:684-693

	t.Run("create_and_parse", func(t *testing.T) {
		req := NewPlanApprovalRequest("researcher", "/tmp/plan.md", "1. Do X\n2. Do Y", "req-001")
		if req.Type != "plan_approval_request" {
			t.Errorf("type = %q", req.Type)
		}
		if req.From != "researcher" {
			t.Errorf("from = %q", req.From)
		}
	})

	t.Run("detect_request", func(t *testing.T) {
		// Source: teammateMailbox.ts:885-897
		text := `{"type":"plan_approval_request","from":"researcher","timestamp":"2026-01-01T00:00:00Z","planFilePath":"/tmp/plan.md","planContent":"do stuff","requestId":"req-001"}`
		req := IsPlanApprovalRequest(text)
		if req == nil {
			t.Fatal("should detect plan approval request")
		}
		if req.RequestID != "req-001" {
			t.Errorf("requestId = %q", req.RequestID)
		}
	})

	t.Run("reject_non_request", func(t *testing.T) {
		if IsPlanApprovalRequest("just a normal message") != nil {
			t.Error("should return nil for non-request")
		}
		if IsPlanApprovalRequest(`{"type":"other"}`) != nil {
			t.Error("should return nil for wrong type")
		}
	})
}

func TestPlanApprovalResponse(t *testing.T) {
	// Source: teammateMailbox.ts:702-711

	t.Run("create_approved", func(t *testing.T) {
		resp := NewPlanApprovalResponse("req-001", true, "")
		if resp.Type != "plan_approval_response" {
			t.Errorf("type = %q", resp.Type)
		}
		if !resp.Approved {
			t.Error("should be approved")
		}
	})

	t.Run("create_rejected_with_feedback", func(t *testing.T) {
		resp := NewPlanApprovalResponse("req-001", false, "Too risky")
		if resp.Approved {
			t.Error("should not be approved")
		}
		if resp.Feedback != "Too risky" {
			t.Errorf("feedback = %q", resp.Feedback)
		}
	})

	t.Run("detect_response", func(t *testing.T) {
		// Source: teammateMailbox.ts:936-948
		text := `{"type":"plan_approval_response","requestId":"req-001","approved":true,"timestamp":"2026-01-01T00:00:00Z"}`
		resp := IsPlanApprovalResponse(text)
		if resp == nil {
			t.Fatal("should detect plan approval response")
		}
		if !resp.Approved {
			t.Error("should be approved")
		}
	})

	t.Run("reject_non_response", func(t *testing.T) {
		if IsPlanApprovalResponse("not json") != nil {
			t.Error("should return nil for non-JSON")
		}
	})
}

func TestPlanApprovalFlow(t *testing.T) {
	// Full flow: agent -> leader -> response
	dir := t.TempDir()
	mb := NewMailbox(dir)

	// 1. Agent sends plan approval request to leader
	err := SendPlanApprovalRequest(mb, "team1", "researcher", "/plan.md", "Do X", "req-001")
	if err != nil {
		t.Fatalf("send request failed: %v", err)
	}

	// 2. Leader reads inbox
	messages, _ := mb.ReadMailbox(TeamLeadName, "team1")
	if len(messages) != 1 {
		t.Fatalf("expected 1 message in leader inbox, got %d", len(messages))
	}
	req := IsPlanApprovalRequest(messages[0].Text)
	if req == nil {
		t.Fatal("should be a plan approval request")
	}

	// 3. Leader sends approval back
	err = SendPlanApprovalResponse(mb, "team1", "researcher", req.RequestID, true, "Looks good")
	if err != nil {
		t.Fatalf("send response failed: %v", err)
	}

	// 4. Agent reads response
	agentMsgs, _ := mb.ReadMailbox("researcher", "team1")
	if len(agentMsgs) != 1 {
		t.Fatalf("expected 1 response, got %d", len(agentMsgs))
	}
	resp := IsPlanApprovalResponse(agentMsgs[0].Text)
	if resp == nil {
		t.Fatal("should be a plan approval response")
	}
	if !resp.Approved {
		t.Error("should be approved")
	}
	if resp.Feedback != "Looks good" {
		t.Errorf("feedback = %q", resp.Feedback)
	}
}
