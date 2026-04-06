package hooks

import (
	"fmt"
	"strconv"
	"strings"
)

// UpdateNotification tracks version-based update notifications and deduplicates
// them so the user is not re-notified for the same semver.
// Source: hooks/useUpdateNotification.ts

// UpdateNotification holds the state for version notification deduplication.
type UpdateNotification struct {
	lastNotifiedSemver string
}

// NewUpdateNotification creates a tracker seeded with the current app version.
// The initial version is normalized to major.minor.patch so that pre-release
// suffixes are ignored for notification purposes.
func NewUpdateNotification(currentVersion string) *UpdateNotification {
	return &UpdateNotification{
		lastNotifiedSemver: getSemverPart(currentVersion),
	}
}

// Check returns the new normalized semver string if updatedVersion differs
// from the last notified version, or "" if no notification is needed.
// When a new version is detected the internal state is updated so subsequent
// calls with the same version return "".
func (u *UpdateNotification) Check(updatedVersion string) string {
	if updatedVersion == "" {
		return ""
	}
	semver := getSemverPart(updatedVersion)
	if semver == "" {
		return ""
	}
	if semver != u.lastNotifiedSemver {
		u.lastNotifiedSemver = semver
		return semver
	}
	return ""
}

// getSemverPart extracts the "major.minor.patch" portion of a version string,
// stripping any leading "v" and ignoring pre-release/build metadata.
// Uses loose parsing: tolerates missing minor/patch (defaults to 0).
func getSemverPart(version string) string {
	v := strings.TrimPrefix(version, "v")

	// Strip pre-release (-...) and build metadata (+...).
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}

	parts := strings.SplitN(v, ".", 3)
	nums := make([]int, 3)
	for i := 0; i < 3; i++ {
		if i < len(parts) {
			n, err := strconv.Atoi(parts[i])
			if err != nil {
				return ""
			}
			nums[i] = n
		}
	}
	return fmt.Sprintf("%d.%d.%d", nums[0], nums[1], nums[2])
}
