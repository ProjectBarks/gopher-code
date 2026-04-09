package tool

import (
	"strings"
	"testing"
)

func TestClassifyResult(t *testing.T) {
	tests := []struct {
		content string
		isErr   bool
		want    ResultType
	}{
		{CancelMessage, false, ResultCanceled},
		{RejectMessage, false, ResultRejected},
		{InterruptMessage, false, ResultRejected},
		{"some error", true, ResultError},
		{"file content here", false, ResultSuccess},
	}
	for _, tt := range tests {
		got := ClassifyResult(tt.content, tt.isErr)
		if got != tt.want {
			t.Errorf("ClassifyResult(%q, %v) = %d, want %d", tt.content, tt.isErr, got, tt.want)
		}
	}
}

func TestRender_Success(t *testing.T) {
	r := Result{ToolName: "Read", Content: "file contents\nline 2", Type: ResultSuccess}
	got := Render(r)
	if !strings.Contains(got, "Read") {
		t.Error("should contain tool name")
	}
	if !strings.Contains(got, "file contents") {
		t.Error("should contain content")
	}
}

func TestRender_Error(t *testing.T) {
	r := Result{Content: "file not found", Type: ResultError}
	got := Render(r)
	if !strings.Contains(got, "Error") {
		t.Error("should contain 'Error'")
	}
	if !strings.Contains(got, "file not found") {
		t.Error("should contain error message")
	}
}

func TestRender_Canceled(t *testing.T) {
	r := Result{Type: ResultCanceled}
	got := Render(r)
	if !strings.Contains(got, "canceled") {
		t.Error("should indicate cancellation")
	}
}

func TestRender_Rejected(t *testing.T) {
	r := Result{Content: RejectMessage + ": not allowed", Type: ResultRejected}
	got := Render(r)
	if !strings.Contains(got, "denied") {
		t.Error("should indicate permission denied")
	}
}

func TestRender_Success_Truncation(t *testing.T) {
	// Generate long content
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line content"
	}
	r := Result{ToolName: "Grep", Content: strings.Join(lines, "\n"), Type: ResultSuccess}
	got := Render(r)
	if !strings.Contains(got, "more lines") {
		t.Error("long content should be truncated")
	}
}

func TestRenderCompact(t *testing.T) {
	if RenderCompact(Result{Type: ResultCanceled}) != "canceled" {
		t.Error("canceled compact")
	}
	if RenderCompact(Result{Type: ResultRejected}) != "rejected" {
		t.Error("rejected compact")
	}
	got := RenderCompact(Result{Content: "short result", Type: ResultSuccess})
	if got != "short result" {
		t.Errorf("success compact = %q", got)
	}
}
