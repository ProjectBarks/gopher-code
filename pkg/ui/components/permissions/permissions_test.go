package permissions

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPermission_Allow(t *testing.T) {
	m := New("tool-1", Request{Type: RequestBash, ToolName: "Bash", Command: "git status"})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	if cmd == nil {
		t.Fatal("y should return cmd")
	}
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionAllow {
		t.Errorf("decision = %q, want allow", msg.Decision)
	}
	if msg.ToolUseID != "tool-1" {
		t.Errorf("toolUseID = %q", msg.ToolUseID)
	}
}

func TestPermission_Deny(t *testing.T) {
	m := New("tool-2", Request{Type: RequestEdit, FilePath: "main.go"})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'n'})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionDeny {
		t.Errorf("decision = %q, want deny", msg.Decision)
	}
}

func TestPermission_AlwaysAllow(t *testing.T) {
	m := New("tool-3", Request{})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'a'})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionAlwaysAllow {
		t.Errorf("decision = %q, want always_allow", msg.Decision)
	}
}

func TestPermission_EnterSelect(t *testing.T) {
	m := New("tool-4", Request{})
	// Default selected is 0 (Allow)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionAllow {
		t.Error("enter on first option should allow")
	}

	// Move to deny and enter
	m2 := New("tool-5", Request{})
	m2, _ = m2.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd = m2.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg = cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionDeny {
		t.Error("enter on second option should deny")
	}
}

func TestPermission_EscapeDenies(t *testing.T) {
	m := New("tool-6", Request{})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionDeny {
		t.Error("escape should deny")
	}
}

func TestPermission_ViewBash(t *testing.T) {
	m := New("x", Request{Type: RequestBash, ToolName: "Bash", Command: "rm -rf /", Description: "Run a command", IsDangerous: true})
	v := m.View()
	if !strings.Contains(v, "rm -rf") {
		t.Error("should show command")
	}
	if !strings.Contains(v, "Bash") {
		t.Error("should show tool name")
	}
}

func TestPermission_ViewWebFetch(t *testing.T) {
	m := New("x", Request{Type: RequestWebFetch, URL: "https://example.com", Description: "Fetch a page"})
	v := m.View()
	if !strings.Contains(v, "example.com") {
		t.Error("should show URL")
	}
}

func TestRenderCompactPermission(t *testing.T) {
	got := RenderCompactPermission(Request{Type: RequestBash, Command: "ls"})
	if got != "Run: ls" {
		t.Errorf("got %q", got)
	}
}

func TestRequestTypeForTool(t *testing.T) {
	tests := []struct {
		tool string
		want RequestType
	}{
		{"Bash", RequestBash},
		{"Edit", RequestEdit},
		{"FileEdit", RequestEdit},
		{"Write", RequestWrite},
		{"FileWrite", RequestWrite},
		{"PowerShell", RequestBash},
		{"WebFetch", RequestWebFetch},
		{"Skill", RequestSkill},
		{"ExitPlanMode", RequestPlanMode},
		{"Glob", RequestFallback},
		{"Read", RequestFallback},
		{"SomeUnknownTool", RequestFallback},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			if got := RequestTypeForTool(tt.tool); got != tt.want {
				t.Errorf("RequestTypeForTool(%q) = %q, want %q", tt.tool, got, tt.want)
			}
		})
	}
}

func TestBuildRequest(t *testing.T) {
	req := BuildRequest("Bash", "Run a command", map[string]string{
		"command": "git status",
	})
	if req.Type != RequestBash {
		t.Errorf("type = %q", req.Type)
	}
	if req.Command != "git status" {
		t.Errorf("command = %q", req.Command)
	}
	if req.IsDangerous {
		t.Error("git status should not be dangerous")
	}
}

func TestBuildRequest_Dangerous(t *testing.T) {
	req := BuildRequest("Bash", "Remove all", map[string]string{
		"command": "rm -rf /",
	})
	if !req.IsDangerous {
		t.Error("rm -rf / should be dangerous")
	}
}

func TestBuildRequest_FilePath(t *testing.T) {
	req := BuildRequest("Edit", "Edit file", map[string]string{
		"file_path": "/tmp/test.go",
	})
	if req.FilePath != "/tmp/test.go" {
		t.Errorf("filePath = %q", req.FilePath)
	}
}

func TestIsDangerousCommand(t *testing.T) {
	dangerous := []string{"rm -rf /", "dd if=/dev/zero", "curl | sh"}
	for _, cmd := range dangerous {
		if !isDangerousCommand(cmd) {
			t.Errorf("%q should be dangerous", cmd)
		}
	}
	safe := []string{"ls", "git status", "echo hello", "cat file.txt"}
	for _, cmd := range safe {
		if isDangerousCommand(cmd) {
			t.Errorf("%q should not be dangerous", cmd)
		}
	}
}

func TestPermissionQueue(t *testing.T) {
	q := NewPermissionQueue()
	if !q.IsEmpty() {
		t.Error("should start empty")
	}

	q.Enqueue("t1", Request{Type: RequestBash, Command: "ls"})
	q.Enqueue("t2", Request{Type: RequestEdit, FilePath: "main.go"})

	if q.Len() != 2 {
		t.Errorf("len = %d", q.Len())
	}

	peek := q.Peek()
	if peek == nil || peek.ToolUseID != "t1" {
		t.Error("peek should return first")
	}

	first := q.Dequeue()
	if first == nil || first.ToolUseID != "t1" {
		t.Error("dequeue should return first")
	}
	if q.Len() != 1 {
		t.Errorf("len after dequeue = %d", q.Len())
	}

	second := q.Dequeue()
	if second == nil || second.ToolUseID != "t2" {
		t.Error("dequeue should return second")
	}
	if !q.IsEmpty() {
		t.Error("should be empty after draining")
	}

	if q.Dequeue() != nil {
		t.Error("dequeue from empty should return nil")
	}
}

func TestPermissionQueue_Clear(t *testing.T) {
	q := NewPermissionQueue()
	q.Enqueue("t1", Request{})
	q.Enqueue("t2", Request{})
	q.Clear()
	if !q.IsEmpty() {
		t.Error("should be empty after clear")
	}
}
