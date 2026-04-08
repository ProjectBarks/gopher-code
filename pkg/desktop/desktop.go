// Package desktop provides deep link and desktop app utilities.
// Source: utils/desktop/ (deepLinks, launchDesktop)
package desktop

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
)

// DeepLinkScheme is the URL scheme for Claude desktop deep links.
const DeepLinkScheme = "claude"

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

// OpenURL opens a URL in the default browser.
func OpenURL(rawURL string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "linux":
		return exec.Command("xdg-open", rawURL).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", rawURL).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
