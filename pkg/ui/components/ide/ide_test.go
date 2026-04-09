package ide

import (
	"strings"
	"testing"

	pkgide "github.com/projectbarks/gopher-code/pkg/ide"
)

func TestDisplayName(t *testing.T) {
	tests := []struct {
		ide  pkgide.IdeType
		want string
	}{
		{pkgide.IdeVSCode, "VS Code"},
		{pkgide.IdeCursor, "Cursor"},
		{pkgide.IdeWindsurf, "Windsurf"},
		{pkgide.IdeJetBrains, "JetBrains"},
		{pkgide.IdeNone, "your IDE"},
	}
	for _, tt := range tests {
		t.Run(string(tt.ide), func(t *testing.T) {
			got := DisplayName(tt.ide)
			if got != tt.want {
				t.Errorf("DisplayName(%q) = %q, want %q", tt.ide, got, tt.want)
			}
		})
	}
}

func TestOnboardingContent_VSCode(t *testing.T) {
	content := OnboardingContent(pkgide.IdeVSCode, "1.2.3")

	if !strings.Contains(content, "VS Code") {
		t.Error("should mention VS Code")
	}
	if !strings.Contains(content, "v1.2.3") {
		t.Error("should contain version")
	}
	if !strings.Contains(content, "open files") {
		t.Error("should mention open files feature")
	}
	if !strings.Contains(content, "selected lines") {
		t.Error("should mention selected lines feature")
	}
	if !strings.Contains(content, "Press Enter to continue") {
		t.Error("should have continue prompt")
	}
}

func TestOnboardingContent_JetBrains(t *testing.T) {
	content := OnboardingContent(pkgide.IdeJetBrains, "")

	if !strings.Contains(content, "JetBrains") {
		t.Error("should mention JetBrains")
	}
	// No version → no "installed plugin v" line
	if strings.Contains(content, "installed") {
		t.Error("should not show installed line without version")
	}
}

func TestOnboardingContent_NoVersion(t *testing.T) {
	content := OnboardingContent(pkgide.IdeVSCode, "")
	if strings.Contains(content, "installed extension") {
		t.Error("no version = no installed line")
	}
}

func TestStatusIndicator_Connected_Selection(t *testing.T) {
	sel := &Selection{
		Text:      "some selected code",
		LineCount: 5,
	}
	result := StatusIndicator(true, sel)
	if !strings.Contains(result, "5 lines selected") {
		t.Errorf("should show line count: %q", result)
	}
}

func TestStatusIndicator_Connected_SingleLine(t *testing.T) {
	sel := &Selection{
		Text:      "one line",
		LineCount: 1,
	}
	result := StatusIndicator(true, sel)
	if !strings.Contains(result, "1 line selected") {
		t.Errorf("should show singular 'line': %q", result)
	}
}

func TestStatusIndicator_Connected_FilePath(t *testing.T) {
	sel := &Selection{
		FilePath: "/home/user/project/main.go",
	}
	result := StatusIndicator(true, sel)
	if !strings.Contains(result, "In main.go") {
		t.Errorf("should show basename: %q", result)
	}
}

func TestStatusIndicator_NotConnected(t *testing.T) {
	sel := &Selection{FilePath: "/test.go"}
	result := StatusIndicator(false, sel)
	if result != "" {
		t.Errorf("should be empty when not connected: %q", result)
	}
}

func TestStatusIndicator_NilSelection(t *testing.T) {
	result := StatusIndicator(true, nil)
	if result != "" {
		t.Errorf("should be empty for nil selection: %q", result)
	}
}

func TestStatusIndicator_EmptySelection(t *testing.T) {
	sel := &Selection{} // no file, no text
	result := StatusIndicator(true, sel)
	if result != "" {
		t.Errorf("should be empty for empty selection: %q", result)
	}
}

func TestStatusIndicator_PreferTextOverFilePath(t *testing.T) {
	sel := &Selection{
		FilePath:  "/test.go",
		Text:      "some code",
		LineCount: 3,
	}
	result := StatusIndicator(true, sel)
	// Should prefer showing "3 lines selected" over "In test.go"
	if !strings.Contains(result, "3 lines selected") {
		t.Errorf("should prefer text over filepath: %q", result)
	}
}
