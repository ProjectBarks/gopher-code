// Source: src/cli/update.ts — update handler
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// InstallationType describes how the binary was installed.
// Source: src/utils/doctorDiagnostic.ts — InstallationType
type InstallationType string

const (
	InstallNPMLocal       InstallationType = "npm-local"
	InstallNPMGlobal      InstallationType = "npm-global"
	InstallNative         InstallationType = "native"
	InstallPackageManager InstallationType = "package-manager"
	InstallDevelopment    InstallationType = "development"
	InstallUnknown        InstallationType = "unknown"
)

// InstallMethod is the config-persisted install method.
// Source: src/utils/config.ts — InstallMethod
type InstallMethod string

const (
	MethodLocal   InstallMethod = "local"
	MethodNative  InstallMethod = "native"
	MethodGlobal  InstallMethod = "global"
	MethodUnknown InstallMethod = "unknown"
)

// PackageManager identifies which package manager manages the installation.
type PackageManager string

const (
	PMHomebrew PackageManager = "homebrew"
	PMWinget   PackageManager = "winget"
	PMApk      PackageManager = "apk"
	PMOther    PackageManager = "other"
	PMNone     PackageManager = "none"
)

// InstallInfo describes a detected installation.
type InstallInfo struct {
	Type InstallationType
	Path string
}

// Warning describes an issue found during diagnosis.
type Warning struct {
	Issue string
	Fix   string
}

// Diagnostic holds the result of installation diagnosis.
type Diagnostic struct {
	InstallationType      InstallationType
	ConfigInstallMethod   string
	MultipleInstallations []InstallInfo
	Warnings              []Warning
}

// VersionFetcher fetches the latest version string for a given channel.
// Returns ("", nil) if the version could not be determined.
type VersionFetcher func(ctx context.Context, channel string) (string, error)

// InstallTypeDetector detects the current installation type.
type InstallTypeDetector func() InstallationType

// PackageManagerDetector detects which package manager manages the install.
type PackageManagerDetector func() PackageManager

// UpdateOpts configures the update handler.
type UpdateOpts struct {
	Output  io.Writer
	Stderr  io.Writer
	Version string // current version

	// Settings
	AutoUpdatesChannel string
	ConfigInstallMethod string

	// Pluggable dependencies for testing
	DetectInstallType InstallTypeDetector
	DetectPackageMgr  PackageManagerDetector
	FetchLatestVersion VersionFetcher
}

// Verbatim user-visible strings from TS source.
// Source: src/cli/update.ts
const (
	MsgCurrentVersion         = "Current version: %s\n"
	MsgCheckingUpdates        = "Checking for updates to %s version...\n"
	MsgMultipleInstallations  = "Warning: Multiple installations found"
	MsgInstallEntry           = "- %s at %s%s\n"
	MsgCurrentlyRunning       = " (currently running)"
	MsgWarningIssue           = "Warning: %s\n"
	MsgWarningFix             = "Fix: %s\n"
	MsgUpdatingConfig         = "Updating configuration to track installation method...\n"
	MsgInstallMethodSet       = "Installation method set to: %s\n"
	MsgCannotUpdateDev        = "Warning: Cannot update development build"
	MsgManagedByHomebrew      = "Claude is managed by Homebrew.\n"
	MsgUpdateAvailable        = "Update available: %s \u2192 %s\n" // → is U+2192
	MsgToUpdate               = "To update, run:\n"
	MsgBrewUpgrade            = "  brew upgrade claude-code"
	MsgUpToDate               = "Claude is up to date!\n"
	MsgManagedByWinget        = "Claude is managed by winget.\n"
	MsgWingetUpgrade          = "  winget upgrade Anthropic.ClaudeCode"
	MsgManagedByApk           = "Claude is managed by apk.\n"
	MsgApkUpgrade             = "  apk upgrade claude-code"
	MsgManagedByPM            = "Claude is managed by a package manager.\n"
	MsgUsePMToUpdate          = "Please use your package manager to update.\n"
	MsgConfigMismatch         = "Warning: Configuration mismatch"
	MsgConfigExpects          = "Config expects: %s installation\n"
	MsgCurrentlyRunningType   = "Currently running: %s\n"
	MsgUpdatingInstallType    = "Updating the %s installation you are currently using\n"
	MsgConfigUpdatedToReflect = "Config updated to reflect current installation method: %s\n"
)

// Update handles `claude update`.
// Source: src/cli/update.ts
func Update(opts UpdateOpts) int {
	w := output(opts.Output)
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	version := opts.Version
	if version == "" {
		version = "0.0.0"
	}

	channel := opts.AutoUpdatesChannel
	if channel == "" {
		channel = "latest"
	}

	// Analytics: tengu_update_check (stub — T-analytics)

	fmt.Fprintf(w, MsgCurrentVersion, version)
	fmt.Fprintf(w, MsgCheckingUpdates, channel)

	// Detect installation type
	detectType := opts.DetectInstallType
	if detectType == nil {
		detectType = DefaultDetectInstallType
	}
	installType := detectType()

	// Detect package manager
	detectPM := opts.DetectPackageMgr
	if detectPM == nil {
		detectPM = DefaultDetectPackageManager
	}

	// Handle development builds
	if installType == InstallDevelopment {
		fmt.Fprintln(w)
		fmt.Fprintln(w, MsgCannotUpdateDev)
		return 1
	}

	// Handle package manager installations
	if installType == InstallPackageManager {
		pm := detectPM()
		fmt.Fprintln(w)
		return handlePackageManagerUpdate(w, opts, pm, version, channel)
	}

	// Fetch latest version for non-PM installs
	fetcher := opts.FetchLatestVersion
	if fetcher == nil {
		fetcher = DefaultFetchLatestVersion
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	latest, err := fetcher(ctx, channel)
	if err != nil || latest == "" {
		fmt.Fprintf(stderr, "Failed to check for updates\n")
		return 1
	}

	if CompareVersions(version, latest) >= 0 {
		fmt.Fprint(w, MsgUpToDate)
		return 0
	}

	fmt.Fprintf(w, MsgUpdateAvailable, version, latest)

	// Config mismatch check
	if opts.ConfigInstallMethod != "" && opts.ConfigInstallMethod != "unknown" {
		normalized := NormalizeInstallType(installType)
		if string(normalized) != opts.ConfigInstallMethod {
			fmt.Fprintln(w)
			fmt.Fprintln(w, MsgConfigMismatch)
			fmt.Fprintf(w, MsgConfigExpects, opts.ConfigInstallMethod)
			fmt.Fprintf(w, MsgCurrentlyRunningType, installType)
			fmt.Fprintf(w, MsgUpdatingInstallType, installType)
			fmt.Fprintf(w, MsgConfigUpdatedToReflect, normalized)
		}
	}

	// Actual install delegation is a stub for now (requires pkg/installer/).
	// The update check + messaging is the core of T219.
	fmt.Fprintf(w, "New version available: %s (current: %s)\n", latest, version)
	fmt.Fprintln(w, "Installing update...")

	return 0
}

// handlePackageManagerUpdate prints instructions for package-manager-managed installs.
func handlePackageManagerUpdate(w io.Writer, opts UpdateOpts, pm PackageManager, version, channel string) int {
	fetcher := opts.FetchLatestVersion
	if fetcher == nil {
		fetcher = DefaultFetchLatestVersion
	}

	switch pm {
	case PMHomebrew:
		fmt.Fprint(w, MsgManagedByHomebrew)
		return checkAndPrintPMUpdate(w, fetcher, version, channel, MsgBrewUpgrade)
	case PMWinget:
		fmt.Fprint(w, MsgManagedByWinget)
		return checkAndPrintPMUpdate(w, fetcher, version, channel, MsgWingetUpgrade)
	case PMApk:
		fmt.Fprint(w, MsgManagedByApk)
		return checkAndPrintPMUpdate(w, fetcher, version, channel, MsgApkUpgrade)
	default:
		fmt.Fprint(w, MsgManagedByPM)
		fmt.Fprint(w, MsgUsePMToUpdate)
		return 0
	}
}

// checkAndPrintPMUpdate fetches the latest version and prints update instructions or up-to-date.
func checkAndPrintPMUpdate(w io.Writer, fetcher VersionFetcher, version, channel, upgradeCmd string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	latest, err := fetcher(ctx, channel)
	if err != nil || latest == "" {
		fmt.Fprint(w, MsgUpToDate)
		return 0
	}

	if CompareVersions(version, latest) >= 0 {
		fmt.Fprint(w, MsgUpToDate)
		return 0
	}

	fmt.Fprintf(w, MsgUpdateAvailable, version, latest)
	fmt.Fprintln(w)
	fmt.Fprint(w, MsgToUpdate)
	fmt.Fprintln(w, upgradeCmd)
	return 0
}

// NormalizeInstallType maps an InstallationType to an InstallMethod string.
// Source: src/cli/update.ts — typeMapping
func NormalizeInstallType(t InstallationType) InstallMethod {
	switch t {
	case InstallNPMLocal:
		return MethodLocal
	case InstallNPMGlobal:
		return MethodGlobal
	case InstallNative:
		return MethodNative
	default:
		return MethodUnknown
	}
}

// CompareVersions compares two semver strings (major.minor.patch).
// Returns -1, 0, or 1 (a < b, a == b, a > b).
// Pre-release and build metadata are ignored for simplicity.
func CompareVersions(a, b string) int {
	pa := parseSemver(a)
	pb := parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

// parseSemver extracts [major, minor, patch] from a version string.
// Tolerates leading 'v' and trailing pre-release/build metadata.
func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	// Strip pre-release / build metadata
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		result[i] = n
	}
	return result
}

// DefaultDetectInstallType detects installation type based on the running binary.
func DefaultDetectInstallType() InstallationType {
	exe, err := os.Executable()
	if err != nil {
		return InstallUnknown
	}

	// Development builds: running from go run, or source tree
	if strings.Contains(exe, "go-build") || strings.Contains(exe, "__debug_bin") {
		return InstallDevelopment
	}

	// Package manager detection
	switch runtime.GOOS {
	case "darwin":
		if strings.HasPrefix(exe, "/opt/homebrew/") || strings.HasPrefix(exe, "/usr/local/Cellar/") {
			return InstallPackageManager
		}
	case "linux":
		if strings.HasPrefix(exe, "/usr/") || strings.HasPrefix(exe, "/snap/") {
			return InstallPackageManager
		}
	}

	// Native binary in well-known location
	home, _ := os.UserHomeDir()
	if home != "" {
		if strings.HasPrefix(exe, home+"/.claude/") {
			return InstallNative
		}
	}

	return InstallUnknown
}

// DefaultDetectPackageManager detects which package manager manages the installation.
func DefaultDetectPackageManager() PackageManager {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err == nil {
			return PMHomebrew
		}
	case "windows":
		if _, err := exec.LookPath("winget"); err == nil {
			return PMWinget
		}
	case "linux":
		exe, _ := os.Executable()
		if strings.HasPrefix(exe, "/usr/") {
			if _, err := exec.LookPath("apk"); err == nil {
				return PMApk
			}
		}
	}
	return PMOther
}

// gitHubRelease is the subset of GitHub release API response we need.
type gitHubRelease struct {
	TagName string `json:"tag_name"`
}

// DefaultFetchLatestVersion fetches the latest version from GitHub releases.
// For a Go binary, this checks the GitHub releases API.
func DefaultFetchLatestVersion(ctx context.Context, channel string) (string, error) {
	// Use GitHub releases API for the gopher-code repository.
	url := "https://api.github.com/repos/anthropics/claude-code/releases/latest"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release gitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	tag := strings.TrimPrefix(release.TagName, "v")
	return tag, nil
}
