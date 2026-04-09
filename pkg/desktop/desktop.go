// Package desktop provides deep link and desktop app utilities.
// Source: utils/desktopDeepLink.ts, components/DesktopHandoff.tsx
package desktop

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// DeepLinkScheme is the URL scheme for Claude desktop deep links.
const DeepLinkScheme = "claude"

// MinDesktopVersion is the minimum desktop app version for session resume.
const MinDesktopVersion = "1.1.2396"

// DesktopDocsURL is the documentation URL for Claude Desktop.
const DesktopDocsURL = "https://clau.de/desktop"

// BuildDeepLink creates a claude:// deep link URL.
func BuildDeepLink(action string, params map[string]string) string {
	u := &url.URL{
		Scheme: DeepLinkScheme,
		Host:   action,
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// BuildResumeLink creates a deep link to resume a session in Claude Desktop.
// Format: claude://resume?session={sessionId}&cwd={cwd}
// Source: desktopDeepLink.ts:35-41
func BuildResumeLink(sessionID, cwd string) string {
	return BuildDeepLink("resume", map[string]string{
		"session": sessionID,
		"cwd":     cwd,
	})
}

// OpenURL opens a URL in the default browser/app handler.
func OpenURL(rawURL string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "linux":
		return exec.Command("xdg-open", rawURL).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", "", rawURL).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// InstallStatus describes the desktop app installation state.
// Source: desktopDeepLink.ts:129-133
type InstallStatus struct {
	Status  string // "not-installed", "version-too-old", "ready"
	Version string // installed version, or "" if not installed
}

const (
	StatusNotInstalled = "not-installed"
	StatusVersionOld   = "version-too-old"
	StatusReady        = "ready"
)

// GetInstallStatus checks if Claude Desktop is installed and its version.
// Source: desktopDeepLink.ts:137-162
func GetInstallStatus() InstallStatus {
	if !IsInstalled() {
		return InstallStatus{Status: StatusNotInstalled}
	}

	version := GetVersion()
	if version == "" {
		return InstallStatus{Status: StatusReady, Version: "unknown"}
	}

	if !isVersionAtLeast(version, MinDesktopVersion) {
		return InstallStatus{Status: StatusVersionOld, Version: version}
	}

	return InstallStatus{Status: StatusReady, Version: version}
}

// IsInstalled checks if Claude Desktop is installed on this platform.
// Source: desktopDeepLink.ts:50-81
func IsInstalled() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := os.Stat("/Applications/Claude.app")
		return err == nil
	case "linux":
		out, err := exec.Command("xdg-mime", "query", "default", "x-scheme-handler/claude").Output()
		return err == nil && len(strings.TrimSpace(string(out))) > 0
	case "windows":
		err := exec.Command("reg", "query", `HKEY_CLASSES_ROOT\claude`, "/ve").Run()
		return err == nil
	default:
		return false
	}
}

// GetVersion returns the installed Claude Desktop version, or "" if unknown.
// Source: desktopDeepLink.ts:89-127
func GetVersion() string {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("defaults", "read",
			"/Applications/Claude.app/Contents/Info.plist",
			"CFBundleShortVersionString").Output()
		if err != nil {
			return ""
		}
		v := strings.TrimSpace(string(out))
		if v == "" {
			return ""
		}
		return v
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return ""
		}
		installDir := filepath.Join(localAppData, "AnthropicClaude")
		entries, err := os.ReadDir(installDir)
		if err != nil {
			return ""
		}
		var versions []string
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "app-") {
				versions = append(versions, e.Name()[4:])
			}
		}
		if len(versions) == 0 {
			return ""
		}
		sort.Strings(versions)
		return versions[len(versions)-1]
	default:
		return ""
	}
}

// GetDownloadURL returns the download URL for Claude Desktop on this platform.
// Source: components/DesktopHandoff.tsx:13-20
func GetDownloadURL() string {
	if runtime.GOOS == "windows" {
		return "https://claude.ai/api/desktop/win32/x64/exe/latest/redirect"
	}
	return "https://claude.ai/api/desktop/darwin/universal/dmg/latest/redirect"
}

// OpenCurrentSession opens the current session in Claude Desktop via deep link.
// Source: desktopDeepLink.ts:206-236
func OpenCurrentSession(sessionID, cwd string) (deepLinkURL string, err error) {
	if !IsInstalled() {
		return "", fmt.Errorf("Claude Desktop is not installed. Install it from https://claude.ai/download")
	}

	deepLinkURL = BuildResumeLink(sessionID, cwd)
	if err := OpenURL(deepLinkURL); err != nil {
		return deepLinkURL, fmt.Errorf("failed to open Claude Desktop: %w", err)
	}

	return deepLinkURL, nil
}

// isVersionAtLeast returns true if version >= minVersion (simple string compare).
// Works for dot-separated numeric versions like "1.2.3".
func isVersionAtLeast(version, minVersion string) bool {
	vParts := splitVersion(version)
	mParts := splitVersion(minVersion)

	for i := 0; i < len(mParts); i++ {
		if i >= len(vParts) {
			return false // version has fewer parts
		}
		if vParts[i] > mParts[i] {
			return true
		}
		if vParts[i] < mParts[i] {
			return false
		}
	}
	return true // equal or version has more parts
}

// splitVersion splits a version string into numeric parts.
func splitVersion(v string) []int {
	parts := strings.Split(v, ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n := 0
		for _, c := range p {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			} else {
				break // stop at first non-digit
			}
		}
		nums[i] = n
	}
	return nums
}
