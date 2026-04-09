package messages

import (
	"strings"
	"testing"
)

func TestRenderCompactBoundary(t *testing.T) {
	got := RenderCompactBoundary()
	if !strings.Contains(got, "compacted") {
		t.Error("should mention compacted")
	}
	if !strings.Contains(got, "ctrl+o") {
		t.Error("should mention ctrl+o shortcut")
	}
}

func TestRenderAttachment_File(t *testing.T) {
	got := RenderAttachment("report.pdf", "application/pdf", 2048)
	if !strings.Contains(got, "report.pdf") {
		t.Error("should contain filename")
	}
	if !strings.Contains(got, "📎") {
		t.Error("should show file icon")
	}
}

func TestRenderAttachment_Image(t *testing.T) {
	got := RenderAttachment("photo.png", "image/png", 1024*500)
	if !strings.Contains(got, "🖼") {
		t.Error("should show image icon for image types")
	}
}

func TestRenderRateLimit(t *testing.T) {
	got := RenderRateLimit(30)
	if !strings.Contains(got, "30s") {
		t.Error("should show retry time")
	}

	got = RenderRateLimit(0)
	if !strings.Contains(got, "Waiting") {
		t.Error("zero retry should show waiting message")
	}
}

func TestRenderShutdown(t *testing.T) {
	got := RenderShutdown("")
	if !strings.Contains(got, "Session ended") {
		t.Error("should show session ended")
	}

	got = RenderShutdown("token expired")
	if !strings.Contains(got, "token expired") {
		t.Error("should include reason")
	}
}

func TestRenderHookProgress(t *testing.T) {
	running := RenderHookProgress("lint", true)
	if !strings.Contains(running, "Running") {
		t.Error("running hook should show Running")
	}

	done := RenderHookProgress("lint", false)
	if !strings.Contains(done, "completed") {
		t.Error("completed hook should show completed")
	}
}

func TestRenderPlanApproval(t *testing.T) {
	got := RenderPlanApproval("Refactor auth module")
	if !strings.Contains(got, "approval") {
		t.Error("should mention approval")
	}
	if !strings.Contains(got, "Refactor") {
		t.Error("should contain plan summary")
	}
}
