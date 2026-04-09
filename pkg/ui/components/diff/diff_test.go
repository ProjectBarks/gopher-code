package diff

import (
	"strings"
	"testing"
)

func TestParseUnifiedDiff(t *testing.T) {
	diffText := `diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
@@ -1,5 +1,6 @@
 package main

-import "fmt"
+import (
+	"fmt"
+)

 func main() {`

	diffs := ParseUnifiedDiff(diffText)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 file diff, got %d", len(diffs))
	}

	fd := diffs[0]
	if fd.Path != "file.go" {
		t.Errorf("path = %q", fd.Path)
	}
	if len(fd.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(fd.Hunks))
	}

	hunk := fd.Hunks[0]
	if hunk.OldStart != 1 || hunk.NewStart != 1 {
		t.Errorf("hunk starts: old=%d new=%d", hunk.OldStart, hunk.NewStart)
	}

	// Count line types
	added, removed, context := 0, 0, 0
	for _, l := range hunk.Lines {
		switch l.Type {
		case LineAdded:
			added++
		case LineRemoved:
			removed++
		case LineContext:
			context++
		}
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if added != 3 {
		t.Errorf("added = %d, want 3", added)
	}
}

func TestRenderFileDiff(t *testing.T) {
	fd := FileDiff{
		Path: "main.go",
		Hunks: []Hunk{{
			OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 4,
			Lines: []Line{
				{Type: LineContext, Content: "package main", OldNum: 1, NewNum: 1},
				{Type: LineRemoved, Content: "old line", OldNum: 2},
				{Type: LineAdded, Content: "new line 1", NewNum: 2},
				{Type: LineAdded, Content: "new line 2", NewNum: 3},
				{Type: LineContext, Content: "end", OldNum: 3, NewNum: 4},
			},
		}},
	}

	styles := DefaultStyles()
	got := RenderFileDiff(fd, styles, 80)

	if !strings.Contains(got, "main.go") {
		t.Error("should contain file path")
	}
	if !strings.Contains(got, "@@") {
		t.Error("should contain hunk header")
	}
}

func TestRenderFileDiff_Rename(t *testing.T) {
	fd := FileDiff{Path: "new.go", OldPath: "old.go"}
	styles := DefaultStyles()
	got := RenderFileDiff(fd, styles, 80)
	if !strings.Contains(got, "→") {
		t.Error("rename should show arrow")
	}
}

func TestDefaultStyles(t *testing.T) {
	s := DefaultStyles()
	// Just verify they render without panics
	s.Added.Render("+added")
	s.Removed.Render("-removed")
	s.Context.Render(" context")
	s.HunkHeader.Render("@@ -1,3 +1,4 @@")
}

func TestParseUnifiedDiff_MultipleFiles(t *testing.T) {
	diffText := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,2 +1,2 @@
-old
+new
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,2 +1,2 @@
-old2
+new2`

	diffs := ParseUnifiedDiff(diffText)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 file diffs, got %d", len(diffs))
	}
	if diffs[0].Path != "a.go" {
		t.Errorf("first file = %q", diffs[0].Path)
	}
	if diffs[1].Path != "b.go" {
		t.Errorf("second file = %q", diffs[1].Path)
	}
}
