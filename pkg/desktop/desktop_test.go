package desktop

import (
	"strings"
	"testing"
)

func TestBuildDeepLink(t *testing.T) {
	link := BuildDeepLink("resume", map[string]string{
		"session": "abc123",
		"cwd":     "/home/user/project",
	})
	if !strings.HasPrefix(link, "claude://resume") {
		t.Errorf("should start with claude://resume, got %q", link)
	}
	if !strings.Contains(link, "session=abc123") {
		t.Error("should contain session param")
	}
	if !strings.Contains(link, "cwd=") {
		t.Error("should contain cwd param")
	}
}

func TestBuildResumeLink(t *testing.T) {
	link := BuildResumeLink("sess-1", "/project")
	if !strings.HasPrefix(link, "claude://resume") {
		t.Errorf("wrong prefix: %q", link)
	}
	if !strings.Contains(link, "session=sess-1") {
		t.Error("should contain session ID")
	}
}

func TestBuildDeepLink_EmptyParams(t *testing.T) {
	link := BuildDeepLink("action", nil)
	if link != "claude://action" {
		t.Errorf("got %q", link)
	}
}

func TestGetDownloadURL(t *testing.T) {
	url := GetDownloadURL()
	if !strings.Contains(url, "claude.ai/api/desktop") {
		t.Errorf("unexpected download URL: %q", url)
	}
}

func TestInstallStatus_Constants(t *testing.T) {
	if StatusNotInstalled != "not-installed" {
		t.Error("wrong")
	}
	if StatusVersionOld != "version-too-old" {
		t.Error("wrong")
	}
	if StatusReady != "ready" {
		t.Error("wrong")
	}
}

func TestIsVersionAtLeast(t *testing.T) {
	tests := []struct {
		version, min string
		want         bool
	}{
		{"1.1.2396", "1.1.2396", true},  // equal
		{"1.2.0", "1.1.2396", true},     // minor higher
		{"2.0.0", "1.1.2396", true},     // major higher
		{"1.1.2395", "1.1.2396", false}, // one below
		{"1.0.0", "1.1.2396", false},    // minor lower
		{"0.9.0", "1.1.2396", false},    // major lower
		{"1.1.2397", "1.1.2396", true},  // one above
		{"1.1", "1.1.2396", false},      // fewer parts
		{"1.1.2396.1", "1.1.2396", true}, // more parts (equal prefix)
	}
	for _, tt := range tests {
		t.Run(tt.version+"_vs_"+tt.min, func(t *testing.T) {
			got := isVersionAtLeast(tt.version, tt.min)
			if got != tt.want {
				t.Errorf("isVersionAtLeast(%q, %q) = %v, want %v", tt.version, tt.min, got, tt.want)
			}
		})
	}
}

func TestSplitVersion(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1.2.3", []int{1, 2, 3}},
		{"1.1.2396", []int{1, 1, 2396}},
		{"2.0.0", []int{2, 0, 0}},
		{"1.0.0-beta", []int{1, 0, 0}}, // stops at non-digit
		{"", []int{0}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitVersion(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("part[%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetInstallStatus(t *testing.T) {
	// This test exercises the real code path — it should not panic
	status := GetInstallStatus()
	// On CI/test machines, Desktop is typically not installed
	if status.Status != StatusNotInstalled && status.Status != StatusReady && status.Status != StatusVersionOld {
		t.Errorf("unexpected status: %q", status.Status)
	}
}

func TestMinDesktopVersion(t *testing.T) {
	if MinDesktopVersion != "1.1.2396" {
		t.Errorf("MinDesktopVersion = %q", MinDesktopVersion)
	}
}

func TestDesktopDocsURL(t *testing.T) {
	if DesktopDocsURL != "https://clau.de/desktop" {
		t.Errorf("DesktopDocsURL = %q", DesktopDocsURL)
	}
}
