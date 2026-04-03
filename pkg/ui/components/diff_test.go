package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func getTestDiffViewer() *DiffViewer {
	return NewDiffViewer(theme.Current())
}

const testDiff = `--- a/file.go
+++ b/file.go
@@ -1,5 +1,5 @@
 package main

-func old() {}
+func new() {}

 func keep() {}`

func TestDiffViewerCreation(t *testing.T) {
	dv := getTestDiffViewer()
	if dv == nil {
		t.Fatal("DiffViewer should not be nil")
	}
	if dv.mode != DiffUnified {
		t.Error("Default mode should be DiffUnified")
	}
}

func TestDiffViewerSetDiff(t *testing.T) {
	dv := getTestDiffViewer()
	dv.SetDiff(testDiff)
	if len(dv.Lines()) == 0 {
		t.Error("Expected parsed diff lines")
	}
}

func TestDiffViewerParsing(t *testing.T) {
	dv := getTestDiffViewer()
	dv.SetDiff(testDiff)
	lines := dv.Lines()

	hasAdded := false
	hasRemoved := false
	hasHeader := false
	for _, l := range lines {
		switch l.Type {
		case DiffAdded:
			hasAdded = true
		case DiffRemoved:
			hasRemoved = true
		case DiffHeader:
			hasHeader = true
		}
	}
	if !hasAdded || !hasRemoved || !hasHeader {
		t.Error("Expected added, removed, and header lines")
	}
}

func TestDiffViewerView(t *testing.T) {
	dv := getTestDiffViewer()
	dv.SetDiff(testDiff)
	dv.SetSize(80, 20)
	view := dv.View()
	if view.Content == "" {
		t.Error("Expected non-empty view")
	}
}

func TestDiffViewerEmpty(t *testing.T) {
	dv := getTestDiffViewer()
	view := dv.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "No diff") {
		t.Error("Expected 'No diff' message for empty viewer")
	}
}

func TestDiffViewerToggleMode(t *testing.T) {
	dv := getTestDiffViewer()
	if dv.Mode() != DiffUnified {
		t.Error("Default should be unified")
	}
	dv.ToggleMode()
	if dv.Mode() != DiffSideBySide {
		t.Error("Should be side-by-side after toggle")
	}
	dv.ToggleMode()
	if dv.Mode() != DiffUnified {
		t.Error("Should be unified after second toggle")
	}
}

func TestDiffViewerScrolling(t *testing.T) {
	dv := getTestDiffViewer()
	dv.SetSize(80, 3)
	dv.SetDiff(testDiff)

	// Scroll down
	dv.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if dv.scrollPos != 1 {
		t.Errorf("Expected scrollPos 1, got %d", dv.scrollPos)
	}

	// Scroll up
	dv.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if dv.scrollPos != 0 {
		t.Errorf("Expected scrollPos 0, got %d", dv.scrollPos)
	}
}

func TestDiffViewerFileName(t *testing.T) {
	dv := getTestDiffViewer()
	dv.SetFileName("main.go")
	dv.SetDiff(testDiff)
	dv.SetSize(80, 20)
	view := dv.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "main.go") {
		t.Error("Expected file name in output")
	}
}

func TestDiffViewerFocus(t *testing.T) {
	dv := getTestDiffViewer()
	if dv.Focused() {
		t.Error("Should not be focused initially")
	}
	dv.Focus()
	if !dv.Focused() {
		t.Error("Should be focused after Focus()")
	}
	dv.Blur()
	if dv.Focused() {
		t.Error("Should not be focused after Blur()")
	}
}
