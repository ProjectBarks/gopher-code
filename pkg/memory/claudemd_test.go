package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFiles_ProjectMemory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project rules"), 0644)

	files := DiscoverFiles(dir)

	found := false
	for _, f := range files {
		if f.Type == TypeProject && f.Path == filepath.Join(dir, "CLAUDE.md") {
			found = true
			if f.Content != "# Project rules" {
				t.Errorf("content = %q, want '# Project rules'", f.Content)
			}
		}
	}
	if !found {
		t.Error("should discover CLAUDE.md in project dir")
	}
}

func TestDiscoverFiles_RulesDir(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".claude", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "style.md"), []byte("Use tabs"), 0644)
	os.WriteFile(filepath.Join(rulesDir, "testing.md"), []byte("Always test"), 0644)
	os.WriteFile(filepath.Join(rulesDir, "readme.txt"), []byte("Not md"), 0644)

	files := DiscoverFiles(dir)

	mdCount := 0
	for _, f := range files {
		if f.Type == TypeProject && filepath.Dir(f.Path) == rulesDir {
			mdCount++
		}
	}
	if mdCount != 2 {
		t.Errorf("expected 2 .md rule files, got %d", mdCount)
	}
}

func TestDiscoverFiles_LocalMemory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.local.md"), []byte("local override"), 0644)

	files := DiscoverFiles(dir)

	found := false
	for _, f := range files {
		if f.Type == TypeLocal {
			found = true
			if f.Content != "local override" {
				t.Errorf("content = %q", f.Content)
			}
		}
	}
	if !found {
		t.Error("should discover CLAUDE.local.md")
	}
}

func TestDiscoverFiles_LargeFileDetection(t *testing.T) {
	dir := t.TempDir()
	// Create a file larger than MaxCharacterCount
	large := make([]byte, MaxCharacterCount+100)
	for i := range large {
		large[i] = 'x'
	}
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), large, 0644)

	files := DiscoverFiles(dir)
	for _, f := range files {
		if f.Type == TypeProject && f.Path == filepath.Join(dir, "CLAUDE.md") {
			if !f.IsLarge {
				t.Error("file should be marked as large")
			}
		}
	}
}

func TestGetLargeFiles(t *testing.T) {
	files := []FileInfo{
		{Path: "small.md", CharCount: 100, IsLarge: false},
		{Path: "big.md", CharCount: MaxCharacterCount + 1, IsLarge: true},
	}
	large := GetLargeFiles(files)
	if len(large) != 1 || large[0].Path != "big.md" {
		t.Errorf("expected 1 large file, got %d", len(large))
	}
}

func TestIsMemoryFilePath(t *testing.T) {
	tests := map[string]bool{
		"CLAUDE.md":                          true,
		"claude.md":                          true,
		"CLAUDE.local.md":                    true,
		".claude/rules/style.md":             true,
		"src/main.go":                        false,
		"README.md":                          false,
	}
	for path, want := range tests {
		if got := IsMemoryFilePath(path); got != want {
			t.Errorf("IsMemoryFilePath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestStripHTMLComments(t *testing.T) {
	input := "before <!-- hidden --> after"
	got := StripHTMLComments(input)
	if got != "before  after" {
		t.Errorf("StripHTMLComments = %q, want 'before  after'", got)
	}
}
