// Package updater provides update notification and auto-update UI.
// Source: components/AutoUpdater.tsx, AutoUpdaterWrapper.tsx
//
// In TS, the auto-updater manages npm/native package updates. In Go,
// the binary is self-contained, so updates are simpler: check a version
// endpoint, notify the user, and optionally download.
package updater

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// UpdateStatus tracks the update check lifecycle.
type UpdateStatus string

const (
	StatusIdle       UpdateStatus = "idle"
	StatusChecking   UpdateStatus = "checking"
	StatusAvailable  UpdateStatus = "available"
	StatusDownloading UpdateStatus = "downloading"
	StatusInstalled  UpdateStatus = "installed"
	StatusFailed     UpdateStatus = "failed"
	StatusUpToDate   UpdateStatus = "up_to_date"
	StatusSkipped    UpdateStatus = "skipped"
)

// UpdateInfo holds version comparison data.
type UpdateInfo struct {
	CurrentVersion  string
	LatestVersion   string
	ReleaseNotesURL string
	IsRequired      bool // security or critical update
}

// NeedsUpdate returns true if there's a newer version.
func (u UpdateInfo) NeedsUpdate() bool {
	return u.LatestVersion != "" && u.LatestVersion != u.CurrentVersion
}

// RenderUpdateBanner renders a notification banner when an update is available.
// Source: components/AutoUpdater.tsx — update available message
func RenderUpdateBanner(info UpdateInfo) string {
	if !info.NeedsUpdate() {
		return ""
	}

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	titleStyle := lipgloss.NewStyle().Bold(true)

	var sb strings.Builder
	sb.WriteString(style.Render("⬆ "))
	sb.WriteString(titleStyle.Render("Update available: "))
	sb.WriteString(fmt.Sprintf("%s → %s", info.CurrentVersion, info.LatestVersion))

	if info.IsRequired {
		sb.WriteString(" ")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true).Render("[REQUIRED]"))
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Run /update to install"))

	return sb.String()
}

// RenderUpdateProgress renders the download/install progress.
func RenderUpdateProgress(status UpdateStatus, version string) string {
	switch status {
	case StatusChecking:
		return lipgloss.NewStyle().Faint(true).Render("⏺ Checking for updates...")
	case StatusDownloading:
		return lipgloss.NewStyle().Faint(true).Render("⏺ Downloading " + version + "...")
	case StatusInstalled:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(
			"✓ Updated to " + version + ". Restart to apply.")
	case StatusFailed:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(
			"✗ Update failed. Try again with /update.")
	case StatusUpToDate:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(
			"✓ Already on the latest version (" + version + ")")
	case StatusSkipped:
		return lipgloss.NewStyle().Faint(true).Render("Update skipped")
	default:
		return ""
	}
}

// RenderUpdateResult renders a compact one-line update result.
func RenderUpdateResult(status UpdateStatus, from, to string) string {
	switch status {
	case StatusInstalled:
		return fmt.Sprintf("Updated %s → %s", from, to)
	case StatusUpToDate:
		return "Up to date (" + from + ")"
	case StatusFailed:
		return "Update failed"
	default:
		return ""
	}
}
