// Package doctor provides diagnostic section renderers for the /doctor screen.
package doctor

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// DistTags holds version information fetched from a registry (npm or GCS).
type DistTags struct {
	Stable string // stable channel version (may be empty)
	Latest string // latest channel version
}

// FetchDistTagsFunc is the signature for a function that fetches dist tags.
// Callers inject this so tests can avoid network calls.
type FetchDistTagsFunc func() (*DistTags, error)

// RenderDistTags renders the Updates section showing dist-tag information.
// Source: Doctor.tsx — "Updates" section with auto-updates, channel, dist tags.
func RenderDistTags(tags *DistTags, fetchErr error, autoUpdates string, channel string) string {
	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)

	var lines []string
	lines = append(lines, bold.Render("Updates"))
	lines = append(lines, fmt.Sprintf("└ Auto-updates: %s", autoUpdates))
	lines = append(lines, fmt.Sprintf("└ Auto-update channel: %s", channel))

	if fetchErr != nil || tags == nil {
		lines = append(lines, dim.Render("└ Failed to fetch versions"))
	} else {
		if tags.Stable != "" {
			lines = append(lines, fmt.Sprintf("└ Stable version: %s", tags.Stable))
		}
		if tags.Latest != "" {
			lines = append(lines, fmt.Sprintf("└ Latest version: %s", tags.Latest))
		}
	}

	return strings.Join(lines, "\n")
}
