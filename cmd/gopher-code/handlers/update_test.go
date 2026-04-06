package handlers

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Version comparison tests
// ---------------------------------------------------------------------------

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.9.9", "2.0.0", -1},
		{"0.2.0", "0.2.0", 0},
		{"0.2.0", "0.3.0", -1},
		{"v1.2.3", "1.2.3", 0},   // leading v tolerated
		{"1.2.3-beta", "1.2.3", 0}, // pre-release stripped
		{"1.2.3+build", "1.2.3", 0}, // build metadata stripped
		{"0.0.0", "0.0.1", -1},
		{"10.0.0", "9.9.9", 1},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.a, tt.b), func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.2.3", [3]int{1, 2, 3}},
		{"0.2.0", [3]int{0, 2, 0}},
		{"1.2.3-beta.1", [3]int{1, 2, 3}},
		{"1.2.3+sha.abc", [3]int{1, 2, 3}},
		{"10.20.30", [3]int{10, 20, 30}},
		{"1", [3]int{1, 0, 0}},
		{"1.2", [3]int{1, 2, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseSemver(tt.input)
			if got != tt.want {
				t.Errorf("parseSemver(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Installation type detection tests
// ---------------------------------------------------------------------------

func TestNormalizeInstallType(t *testing.T) {
	tests := []struct {
		input InstallationType
		want  InstallMethod
	}{
		{InstallNPMLocal, MethodLocal},
		{InstallNPMGlobal, MethodGlobal},
		{InstallNative, MethodNative},
		{InstallDevelopment, MethodUnknown},
		{InstallPackageManager, MethodUnknown},
		{InstallUnknown, MethodUnknown},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := NormalizeInstallType(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeInstallType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInstallationType_Constants(t *testing.T) {
	// Verify all installation type string values match TS source.
	if InstallNPMLocal != "npm-local" {
		t.Error("InstallNPMLocal mismatch")
	}
	if InstallNPMGlobal != "npm-global" {
		t.Error("InstallNPMGlobal mismatch")
	}
	if InstallNative != "native" {
		t.Error("InstallNative mismatch")
	}
	if InstallPackageManager != "package-manager" {
		t.Error("InstallPackageManager mismatch")
	}
	if InstallDevelopment != "development" {
		t.Error("InstallDevelopment mismatch")
	}
}

// ---------------------------------------------------------------------------
// Output string tests
// ---------------------------------------------------------------------------

func TestUpdate_CurrentVersionPrinted(t *testing.T) {
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &bytes.Buffer{},
		Version: "1.2.3",
		DetectInstallType: func() InstallationType { return InstallUnknown },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) {
			return "1.2.3", nil
		},
	})
	if !strings.Contains(buf.String(), "Current version: 1.2.3") {
		t.Errorf("expected current version string, got: %s", buf.String())
	}
}

func TestUpdate_CheckingUpdatesChannel(t *testing.T) {
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:             &buf,
		Stderr:             &bytes.Buffer{},
		Version:            "1.0.0",
		AutoUpdatesChannel: "beta",
		DetectInstallType:  func() InstallationType { return InstallUnknown },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) {
			return "1.0.0", nil
		},
	})
	if !strings.Contains(buf.String(), "Checking for updates to beta version...") {
		t.Errorf("expected channel in checking message, got: %s", buf.String())
	}
}

func TestUpdate_DefaultChannelIsLatest(t *testing.T) {
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &bytes.Buffer{},
		Version: "1.0.0",
		DetectInstallType: func() InstallationType { return InstallUnknown },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) {
			return "1.0.0", nil
		},
	})
	if !strings.Contains(buf.String(), "Checking for updates to latest version...") {
		t.Errorf("expected default 'latest' channel, got: %s", buf.String())
	}
}

func TestUpdate_UpToDate(t *testing.T) {
	var buf bytes.Buffer
	code := Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &bytes.Buffer{},
		Version: "2.0.0",
		DetectInstallType: func() InstallationType { return InstallUnknown },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) {
			return "2.0.0", nil
		},
	})
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}
	if !strings.Contains(buf.String(), "Claude is up to date!") {
		t.Errorf("expected up-to-date message, got: %s", buf.String())
	}
}

func TestUpdate_UpdateAvailableArrow(t *testing.T) {
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &bytes.Buffer{},
		Version: "1.0.0",
		DetectInstallType: func() InstallationType { return InstallUnknown },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) {
			return "2.0.0", nil
		},
	})
	out := buf.String()
	if !strings.Contains(out, "1.0.0 \u2192 2.0.0") {
		t.Errorf("expected arrow (U+2192) in update available message, got: %s", out)
	}
}

func TestUpdate_DevelopmentBuild(t *testing.T) {
	var buf bytes.Buffer
	code := Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &bytes.Buffer{},
		Version: "0.0.1",
		DetectInstallType: func() InstallationType { return InstallDevelopment },
	})
	if code != 1 {
		t.Errorf("expected exit 1 for dev build, got %d", code)
	}
	if !strings.Contains(buf.String(), "Warning: Cannot update development build") {
		t.Errorf("expected dev warning, got: %s", buf.String())
	}
}

func TestUpdate_FetchError(t *testing.T) {
	var buf, errBuf bytes.Buffer
	code := Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &errBuf,
		Version: "1.0.0",
		DetectInstallType: func() InstallationType { return InstallUnknown },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) {
			return "", fmt.Errorf("network error")
		},
	})
	if code != 1 {
		t.Errorf("expected exit 1 on fetch error, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "Failed to check for updates") {
		t.Errorf("expected failure message on stderr, got: %s", errBuf.String())
	}
}

// ---------------------------------------------------------------------------
// Package manager template tests
// ---------------------------------------------------------------------------

func TestUpdate_Homebrew(t *testing.T) {
	var buf bytes.Buffer
	code := Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &bytes.Buffer{},
		Version: "1.0.0",
		DetectInstallType:  func() InstallationType { return InstallPackageManager },
		DetectPackageMgr:   func() PackageManager { return PMHomebrew },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) { return "2.0.0", nil },
	})
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "Claude is managed by Homebrew.") {
		t.Errorf("expected Homebrew managed message, got: %s", out)
	}
	if !strings.Contains(out, "brew upgrade claude-code") {
		t.Errorf("expected brew upgrade command, got: %s", out)
	}
	if !strings.Contains(out, "1.0.0 \u2192 2.0.0") {
		t.Errorf("expected update available with arrow, got: %s", out)
	}
}

func TestUpdate_Homebrew_UpToDate(t *testing.T) {
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &bytes.Buffer{},
		Version: "2.0.0",
		DetectInstallType:  func() InstallationType { return InstallPackageManager },
		DetectPackageMgr:   func() PackageManager { return PMHomebrew },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) { return "2.0.0", nil },
	})
	out := buf.String()
	if !strings.Contains(out, "Claude is up to date!") {
		t.Errorf("expected up-to-date message, got: %s", out)
	}
	if strings.Contains(out, "brew upgrade") {
		t.Error("should not show upgrade command when up to date")
	}
}

func TestUpdate_Winget(t *testing.T) {
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &bytes.Buffer{},
		Version: "1.0.0",
		DetectInstallType:  func() InstallationType { return InstallPackageManager },
		DetectPackageMgr:   func() PackageManager { return PMWinget },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) { return "2.0.0", nil },
	})
	out := buf.String()
	if !strings.Contains(out, "Claude is managed by winget.") {
		t.Errorf("expected winget message, got: %s", out)
	}
	if !strings.Contains(out, "winget upgrade Anthropic.ClaudeCode") {
		t.Errorf("expected winget upgrade command, got: %s", out)
	}
}

func TestUpdate_Apk(t *testing.T) {
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &bytes.Buffer{},
		Version: "1.0.0",
		DetectInstallType:  func() InstallationType { return InstallPackageManager },
		DetectPackageMgr:   func() PackageManager { return PMApk },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) { return "2.0.0", nil },
	})
	out := buf.String()
	if !strings.Contains(out, "Claude is managed by apk.") {
		t.Errorf("expected apk message, got: %s", out)
	}
	if !strings.Contains(out, "apk upgrade claude-code") {
		t.Errorf("expected apk upgrade command, got: %s", out)
	}
}

func TestUpdate_GenericPackageManager(t *testing.T) {
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:  &buf,
		Stderr:  &bytes.Buffer{},
		Version: "1.0.0",
		DetectInstallType: func() InstallationType { return InstallPackageManager },
		DetectPackageMgr:  func() PackageManager { return PMOther },
	})
	out := buf.String()
	if !strings.Contains(out, "Claude is managed by a package manager.") {
		t.Errorf("expected generic PM message, got: %s", out)
	}
	if !strings.Contains(out, "Please use your package manager to update.") {
		t.Errorf("expected generic update instruction, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// Config mismatch tests
// ---------------------------------------------------------------------------

func TestUpdate_ConfigMismatch(t *testing.T) {
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:              &buf,
		Stderr:              &bytes.Buffer{},
		Version:             "1.0.0",
		ConfigInstallMethod: "native",
		DetectInstallType: func() InstallationType { return InstallNPMGlobal },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) {
			return "2.0.0", nil
		},
	})
	out := buf.String()
	if !strings.Contains(out, "Warning: Configuration mismatch") {
		t.Errorf("expected config mismatch warning, got: %s", out)
	}
	if !strings.Contains(out, "Config expects: native installation") {
		t.Errorf("expected config expects line, got: %s", out)
	}
}

func TestUpdate_NoConfigMismatchWhenMatching(t *testing.T) {
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:              &buf,
		Stderr:              &bytes.Buffer{},
		Version:             "1.0.0",
		ConfigInstallMethod: "global",
		DetectInstallType: func() InstallationType { return InstallNPMGlobal },
		FetchLatestVersion: func(_ context.Context, _ string) (string, error) {
			return "2.0.0", nil
		},
	})
	if strings.Contains(buf.String(), "Configuration mismatch") {
		t.Error("should not show mismatch when config matches install type")
	}
}

// ---------------------------------------------------------------------------
// Channel passthrough test
// ---------------------------------------------------------------------------

func TestUpdate_ChannelPassedToFetcher(t *testing.T) {
	var capturedChannel string
	var buf bytes.Buffer
	Update(UpdateOpts{
		Output:             &buf,
		Stderr:             &bytes.Buffer{},
		Version:            "1.0.0",
		AutoUpdatesChannel: "beta",
		DetectInstallType:  func() InstallationType { return InstallUnknown },
		FetchLatestVersion: func(_ context.Context, ch string) (string, error) {
			capturedChannel = ch
			return "1.0.0", nil
		},
	})
	if capturedChannel != "beta" {
		t.Errorf("expected channel 'beta' passed to fetcher, got %q", capturedChannel)
	}
}
