package updater

import (
	"strings"
	"testing"
)

func TestUpdateInfo_NeedsUpdate(t *testing.T) {
	yes := UpdateInfo{CurrentVersion: "0.1.0", LatestVersion: "0.2.0"}
	if !yes.NeedsUpdate() {
		t.Error("should need update")
	}

	no := UpdateInfo{CurrentVersion: "0.2.0", LatestVersion: "0.2.0"}
	if no.NeedsUpdate() {
		t.Error("same version should not need update")
	}

	empty := UpdateInfo{CurrentVersion: "0.1.0"}
	if empty.NeedsUpdate() {
		t.Error("empty latest should not need update")
	}
}

func TestRenderUpdateBanner(t *testing.T) {
	info := UpdateInfo{CurrentVersion: "0.1.0", LatestVersion: "0.2.0"}
	got := RenderUpdateBanner(info)
	if !strings.Contains(got, "0.2.0") {
		t.Error("should contain new version")
	}
	if !strings.Contains(got, "/update") {
		t.Error("should mention /update command")
	}
}

func TestRenderUpdateBanner_Required(t *testing.T) {
	info := UpdateInfo{CurrentVersion: "0.1.0", LatestVersion: "0.2.0", IsRequired: true}
	got := RenderUpdateBanner(info)
	if !strings.Contains(got, "REQUIRED") {
		t.Error("required update should show REQUIRED badge")
	}
}

func TestRenderUpdateBanner_NoUpdate(t *testing.T) {
	info := UpdateInfo{CurrentVersion: "0.2.0", LatestVersion: "0.2.0"}
	got := RenderUpdateBanner(info)
	if got != "" {
		t.Error("no update should return empty")
	}
}

func TestRenderUpdateProgress(t *testing.T) {
	tests := []struct {
		status UpdateStatus
		expect string
	}{
		{StatusChecking, "Checking"},
		{StatusDownloading, "Downloading"},
		{StatusInstalled, "Updated"},
		{StatusFailed, "failed"},
		{StatusUpToDate, "latest"},
	}
	for _, tt := range tests {
		got := RenderUpdateProgress(tt.status, "0.2.0")
		if !strings.Contains(got, tt.expect) {
			t.Errorf("status %q should contain %q, got %q", tt.status, tt.expect, got)
		}
	}
}

func TestRenderUpdateResult(t *testing.T) {
	got := RenderUpdateResult(StatusInstalled, "0.1.0", "0.2.0")
	if !strings.Contains(got, "0.1.0") || !strings.Contains(got, "0.2.0") {
		t.Errorf("should contain both versions, got %q", got)
	}
}
