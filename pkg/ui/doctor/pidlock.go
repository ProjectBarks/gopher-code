package doctor

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// LockInfo describes a single PID-based version lock.
// Source: Doctor.tsx — LockInfo type from pidLock.ts
type LockInfo struct {
	Version          string
	PID              int
	IsProcessRunning bool
}

// VersionLockInfo aggregates PID lock state for /doctor display.
// Source: Doctor.tsx — VersionLockInfo type
type VersionLockInfo struct {
	Enabled            bool
	Locks              []LockInfo
	LocksDir           string
	StaleLocksCleaned  int
}

// RenderPIDLocks renders the Version Locks section.
// Source: Doctor.tsx — "Version Locks" section
func RenderPIDLocks(info *VersionLockInfo) string {
	if info == nil || !info.Enabled {
		return ""
	}

	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)
	warn := t.TextWarning()

	var lines []string
	lines = append(lines, bold.Render("Version Locks"))

	if info.StaleLocksCleaned > 0 {
		lines = append(lines, dim.Render(
			fmt.Sprintf("└ Cleaned %d stale lock(s)", info.StaleLocksCleaned),
		))
	}

	if len(info.Locks) == 0 {
		lines = append(lines, dim.Render("└ No active version locks"))
	} else {
		for _, lock := range info.Locks {
			status := "(running)"
			if !lock.IsProcessRunning {
				status = warn.Render("(stale)")
			}
			lines = append(lines, fmt.Sprintf("└ %s: PID %d %s", lock.Version, lock.PID, status))
		}
	}

	return strings.Join(lines, "\n")
}
