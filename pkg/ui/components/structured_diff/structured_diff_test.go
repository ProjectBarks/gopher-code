package structured_diff

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/components/diff"
)

func TestRenderStructuredDiff_SingleHunk(t *testing.T) {
	fd := diff.FileDiff{
		Path: "main.go",
		Hunks: []diff.Hunk{{
			OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 4,
			Lines: []diff.Line{
				{Type: diff.LineContext, Content: "package main", OldNum: 1, NewNum: 1},
				{Type: diff.LineRemoved, Content: "old", OldNum: 2},
				{Type: diff.LineAdded, Content: "new1", NewNum: 2},
				{Type: diff.LineAdded, Content: "new2", NewNum: 3},
			},
		}},
	}

	got := RenderStructuredDiff(fd, 80, false)
	if got == "" {
		t.Error("should produce output")
	}
	// Content has ANSI codes — verify it's substantial
	if len(got) < 20 {
		t.Error("output too short")
	}
}

func TestRenderStructuredDiff_MultipleHunks(t *testing.T) {
	fd := diff.FileDiff{
		Path: "file.go",
		Hunks: []diff.Hunk{
			{OldStart: 1, OldCount: 2, NewStart: 1, NewCount: 2, Lines: []diff.Line{
				{Type: diff.LineRemoved, Content: "a", OldNum: 1},
				{Type: diff.LineAdded, Content: "b", NewNum: 1},
			}},
			{OldStart: 50, OldCount: 2, NewStart: 50, NewCount: 2, Lines: []diff.Line{
				{Type: diff.LineRemoved, Content: "x", OldNum: 50},
				{Type: diff.LineAdded, Content: "y", NewNum: 50},
			}},
		},
	}

	got := RenderStructuredDiff(fd, 80, false)
	if !strings.Contains(got, "...") {
		t.Error("multiple hunks should have ellipsis separator")
	}
}

func TestRenderStructuredDiff_Dim(t *testing.T) {
	fd := diff.FileDiff{
		Path: "file.go",
		Hunks: []diff.Hunk{{
			OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1,
			Lines: []diff.Line{
				{Type: diff.LineAdded, Content: "added", NewNum: 1},
			},
		}},
	}
	// Should not panic with dim=true
	got := RenderStructuredDiff(fd, 80, true)
	if got == "" {
		t.Error("should produce output")
	}
}

func TestRenderDiffSummary(t *testing.T) {
	fd := diff.FileDiff{
		Hunks: []diff.Hunk{{
			Lines: []diff.Line{
				{Type: diff.LineAdded},
				{Type: diff.LineAdded},
				{Type: diff.LineRemoved},
			},
		}},
	}
	got := RenderDiffSummary(fd)
	if !strings.Contains(got, "+2") {
		t.Errorf("should show +2, got %q", got)
	}
	if !strings.Contains(got, "-1") {
		t.Errorf("should show -1, got %q", got)
	}
}

func TestRenderDiffSummary_NoChanges(t *testing.T) {
	fd := diff.FileDiff{}
	got := RenderDiffSummary(fd)
	if got != "no changes" {
		t.Errorf("got %q, want 'no changes'", got)
	}
}
