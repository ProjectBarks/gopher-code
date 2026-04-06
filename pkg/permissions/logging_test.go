package permissions

import (
	"testing"
)

func TestApprovalSourceToString(t *testing.T) {
	tests := []struct {
		name   string
		source ApprovalSource
		want   string
	}{
		{"config", ApprovalSource{Type: ApprovalConfig}, "config"},
		{"classifier", ApprovalSource{Type: ApprovalClassifier}, "classifier"},
		{"hook", ApprovalSource{Type: ApprovalHook}, "hook"},
		{"user_permanent", ApprovalSource{Type: ApprovalUser, Permanent: true}, "user_permanent"},
		{"user_temporary", ApprovalSource{Type: ApprovalUser, Permanent: false}, "user_temporary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApprovalSourceToString(tt.source)
			if got != tt.want {
				t.Errorf("ApprovalSourceToString(%v) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestRejectionSourceToString(t *testing.T) {
	tests := []struct {
		name   string
		source RejectionSource
		want   string
	}{
		{"config", RejectionSource{Type: RejectionConfig}, "config"},
		{"hook", RejectionSource{Type: RejectionHook}, "hook"},
		{"user_abort", RejectionSource{Type: RejectionUserAbort}, "user_abort"},
		{"user_reject", RejectionSource{Type: RejectionUserReject}, "user_reject"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RejectionSourceToString(tt.source)
			if got != tt.want {
				t.Errorf("RejectionSourceToString(%v) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestApprovalEventName(t *testing.T) {
	// Source: permissionLogging.ts — 7 analytics event names
	tests := []struct {
		name   string
		source ApprovalSource
		want   string
	}{
		{"config", ApprovalSource{Type: ApprovalConfig}, "tengu_tool_use_granted_in_config"},
		{"classifier", ApprovalSource{Type: ApprovalClassifier}, "tengu_tool_use_granted_by_classifier"},
		{"user_permanent", ApprovalSource{Type: ApprovalUser, Permanent: true}, "tengu_tool_use_granted_in_prompt_permanent"},
		{"user_temporary", ApprovalSource{Type: ApprovalUser, Permanent: false}, "tengu_tool_use_granted_in_prompt_temporary"},
		{"hook", ApprovalSource{Type: ApprovalHook}, "tengu_tool_use_granted_by_permission_hook"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApprovalEventName(tt.source)
			if got != tt.want {
				t.Errorf("ApprovalEventName(%v) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestRejectionEventName(t *testing.T) {
	tests := []struct {
		name   string
		source RejectionSource
		want   string
	}{
		{"config", RejectionSource{Type: RejectionConfig}, "tengu_tool_use_denied_in_config"},
		{"hook", RejectionSource{Type: RejectionHook}, "tengu_tool_use_rejected_in_prompt"},
		{"user_abort", RejectionSource{Type: RejectionUserAbort}, "tengu_tool_use_rejected_in_prompt"},
		{"user_reject", RejectionSource{Type: RejectionUserReject, HasFeedback: true}, "tengu_tool_use_rejected_in_prompt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RejectionEventName(tt.source)
			if got != tt.want {
				t.Errorf("RejectionEventName(%v) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestIsCodeEditingTool(t *testing.T) {
	// Source: permissionLogging.ts — CODE_EDITING_TOOLS
	for _, name := range []string{"Edit", "Write", "NotebookEdit"} {
		if !IsCodeEditingTool(name) {
			t.Errorf("expected %q to be a code editing tool", name)
		}
	}
	for _, name := range []string{"Bash", "Read", "Glob", "Grep"} {
		if IsCodeEditingTool(name) {
			t.Errorf("expected %q to NOT be a code editing tool", name)
		}
	}
}

func TestCollectingLogger(t *testing.T) {
	logger := &CollectingLogger{}

	logger.LogDecision(DecisionRecord{
		ToolName: "Bash", ToolUseID: "tu-1", Decision: "accept", Source: "config",
	})
	logger.LogCancelled("Bash", "tu-2", "msg-1")

	if len(logger.Decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(logger.Decisions))
	}
	if len(logger.Cancelled) != 1 {
		t.Fatalf("expected 1 cancelled, got %d", len(logger.Cancelled))
	}
	if logger.Cancelled[0] != "tu-2" {
		t.Errorf("expected cancelled tu-2, got %q", logger.Cancelled[0])
	}
}
