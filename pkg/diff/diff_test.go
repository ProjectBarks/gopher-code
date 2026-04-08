package diff

import (
	"strings"
	"testing"
)

func TestAdjustHunkLineNumbers(t *testing.T) {
	hunks := []Hunk{{OldStart: 1, NewStart: 1}}
	adjusted := AdjustHunkLineNumbers(hunks, 10)
	if adjusted[0].OldStart != 11 || adjusted[0].NewStart != 11 {
		t.Errorf("expected 11,11 got %d,%d", adjusted[0].OldStart, adjusted[0].NewStart)
	}

	// Zero offset returns same slice
	same := AdjustHunkLineNumbers(hunks, 0)
	if &same[0] != &hunks[0] {
		t.Error("zero offset should return same slice")
	}
}

func TestCountLinesChanged(t *testing.T) {
	hunks := []Hunk{{
		Lines: []string{" context", "+added1", "+added2", "-removed", " context2"},
	}}
	counts := CountLinesChanged(hunks, "")
	if counts.Added != 2 {
		t.Errorf("Added = %d, want 2", counts.Added)
	}
	if counts.Removed != 1 {
		t.Errorf("Removed = %d, want 1", counts.Removed)
	}
}

func TestCountLinesChanged_NewFile(t *testing.T) {
	counts := CountLinesChanged(nil, "line1\nline2\nline3")
	if counts.Added != 3 {
		t.Errorf("Added = %d, want 3", counts.Added)
	}
}

func TestFormatUnifiedDiff(t *testing.T) {
	hunks := []Hunk{{
		OldStart: 1, OldLines: 3, NewStart: 1, NewLines: 4,
		Lines: []string{" context", "-old", "+new1", "+new2", " end"},
	}}
	out := FormatUnifiedDiff("file.go", hunks)
	if !strings.Contains(out, "--- a/file.go") {
		t.Error("should contain old file header")
	}
	if !strings.Contains(out, "+++ b/file.go") {
		t.Error("should contain new file header")
	}
	if !strings.Contains(out, "@@ -1,3 +1,4 @@") {
		t.Error("should contain hunk header")
	}
	if !strings.Contains(out, "+new1") {
		t.Error("should contain added line")
	}
}
