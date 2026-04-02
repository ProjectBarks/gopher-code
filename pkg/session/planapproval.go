package session

import (
	"encoding/json"
	"time"
)

// Source: utils/teammateMailbox.ts:684-715

// PlanApprovalRequest is sent by a teammate to request approval for a plan.
// Source: utils/teammateMailbox.ts:684-693
type PlanApprovalRequest struct {
	Type         string `json:"type"` // "plan_approval_request"
	From         string `json:"from"`
	Timestamp    string `json:"timestamp"`
	PlanFilePath string `json:"planFilePath"`
	PlanContent  string `json:"planContent"`
	RequestID    string `json:"requestId"`
}

// PlanApprovalResponse is sent by the leader back to the requesting teammate.
// Source: utils/teammateMailbox.ts:702-711
type PlanApprovalResponse struct {
	Type           string `json:"type"` // "plan_approval_response"
	RequestID      string `json:"requestId"`
	Approved       bool   `json:"approved"`
	Feedback       string `json:"feedback,omitempty"`
	Timestamp      string `json:"timestamp"`
	PermissionMode string `json:"permissionMode,omitempty"`
}

// NewPlanApprovalRequest creates a plan approval request message.
func NewPlanApprovalRequest(from, planFilePath, planContent, requestID string) PlanApprovalRequest {
	return PlanApprovalRequest{
		Type:         "plan_approval_request",
		From:         from,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		PlanFilePath: planFilePath,
		PlanContent:  planContent,
		RequestID:    requestID,
	}
}

// NewPlanApprovalResponse creates a plan approval response.
func NewPlanApprovalResponse(requestID string, approved bool, feedback string) PlanApprovalResponse {
	return PlanApprovalResponse{
		Type:      "plan_approval_response",
		RequestID: requestID,
		Approved:  approved,
		Feedback:  feedback,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// IsPlanApprovalRequest checks if a message text is a plan approval request.
// Source: utils/teammateMailbox.ts:885-897
func IsPlanApprovalRequest(messageText string) *PlanApprovalRequest {
	var req PlanApprovalRequest
	if err := json.Unmarshal([]byte(messageText), &req); err != nil {
		return nil
	}
	if req.Type != "plan_approval_request" {
		return nil
	}
	if req.From == "" || req.RequestID == "" {
		return nil
	}
	return &req
}

// IsPlanApprovalResponse checks if a message text is a plan approval response.
// Source: utils/teammateMailbox.ts:936-948
func IsPlanApprovalResponse(messageText string) *PlanApprovalResponse {
	var resp PlanApprovalResponse
	if err := json.Unmarshal([]byte(messageText), &resp); err != nil {
		return nil
	}
	if resp.Type != "plan_approval_response" {
		return nil
	}
	if resp.RequestID == "" {
		return nil
	}
	return &resp
}

// SendPlanApprovalRequest sends a plan approval request to the team leader
// via the mailbox system.
func SendPlanApprovalRequest(mb *Mailbox, teamName, from, planPath, planContent, requestID string) error {
	req := NewPlanApprovalRequest(from, planPath, planContent, requestID)
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return mb.WriteToMailbox(TeamLeadName, teamName, from, string(data),
		WithSummary("Plan approval request"))
}

// SendPlanApprovalResponse sends a plan approval response back to the requester.
func SendPlanApprovalResponse(mb *Mailbox, teamName, recipient, requestID string, approved bool, feedback string) error {
	resp := NewPlanApprovalResponse(requestID, approved, feedback)
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return mb.WriteToMailbox(recipient, teamName, TeamLeadName, string(data),
		WithSummary("Plan approval response"))
}
