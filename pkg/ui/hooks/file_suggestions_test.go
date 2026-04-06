package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// helper: create a temp directory tree with the given files.
func setupTree(t *testing.T, files []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// ----- Tests -----

func TestFuzzyMatch_PartialFilename(t *testing.T) {
	candidates := []string{
		"src/main.go",
		"src/utils/helpers.go",
		"pkg/config/settings.go",
		"README.md",
		"Makefile",
	}

	tests := []struct {
		query   string
		wantHas string // a path that must appear (empty = expect no results)
	}{
		{"main", "src/main.go"},
		{"help", "src/utils/helpers.go"},
		{"sett", "pkg/config/settings.go"},
		{"READ", "README.md"},          // case-insensitive
		{"mg", "src/main.go"},           // subsequence m..g
		{"suh", "src/utils/helpers.go"}, // subsequence s..u..h
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			items := fuzzyMatch(candidates, tt.query, MaxSuggestions)
			found := false
			for _, it := range items {
				if it.DisplayText == tt.wantHas {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("query %q: expected %q in results, got %v", tt.query, tt.wantHas, displayTexts(items))
			}
		})
	}
}

func TestFuzzyMatch_NoMatch(t *testing.T) {
	candidates := []string{"src/main.go", "README.md"}
	items := fuzzyMatch(candidates, "zzzzzzz", MaxSuggestions)
	if len(items) != 0 {
		t.Errorf("expected no matches, got %v", displayTexts(items))
	}
}

func TestFuzzyMatch_Ranking(t *testing.T) {
	candidates := []string{
		"pkg/deep/nested/main.go",
		"main.go",
		"src/main.go",
		"cmd/main_test.go",
	}

	items := fuzzyMatch(candidates, "main", MaxSuggestions)
	if len(items) == 0 {
		t.Fatal("expected matches")
	}

	// "main.go" is a prefix match at depth 0 -- should rank first.
	if items[0].DisplayText != "main.go" {
		t.Errorf("expected main.go first, got %q (score %f)", items[0].DisplayText, items[0].Score)
	}

	// Verify shallower paths rank higher for same match type.
	for i := 1; i < len(items); i++ {
		if items[i].Score > items[0].Score {
			t.Errorf("item %d (%q score=%f) ranked higher than item 0 (%q score=%f)",
				i, items[i].DisplayText, items[i].Score, items[0].DisplayText, items[0].Score)
		}
	}
}

func TestFuzzyMatch_BasenameMatch(t *testing.T) {
	candidates := []string{
		"very/deep/path/to/config.yaml",
	}
	items := fuzzyMatch(candidates, "config", MaxSuggestions)
	if len(items) == 0 {
		t.Fatal("expected basename match for 'config'")
	}
	if items[0].DisplayText != "very/deep/path/to/config.yaml" {
		t.Errorf("unexpected match: %q", items[0].DisplayText)
	}
}

func TestGitignoreExclusion(t *testing.T) {
	files := []string{
		"src/main.go",
		"src/utils.go",
		"build/output.bin",
		"build/cache/tmp.dat",
		"node_modules/pkg/index.js",
		".env",
	}
	dir := setupTree(t, files)

	// Write a .gitignore that excludes build/ and node_modules/ and .env
	gitignore := "build/\nnode_modules/\n.env\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSuggester(dir)
	ig := fs.loadIgnore()
	if ig == nil {
		t.Fatal("expected ignore patterns to load")
	}

	// These should be excluded.
	for _, p := range []string{"build/output.bin", "build/cache/tmp.dat", "node_modules/pkg/index.js", ".env"} {
		if !ig.MatchesPath(p) {
			t.Errorf("expected %q to be ignored", p)
		}
	}

	// These should NOT be excluded.
	for _, p := range []string{"src/main.go", "src/utils.go"} {
		if ig.MatchesPath(p) {
			t.Errorf("expected %q to NOT be ignored", p)
		}
	}
}

func TestGitignoreExclusion_FilterIgnored(t *testing.T) {
	files := []string{
		"src/main.go",
		"dist/bundle.js",
		"tmp/scratch.txt",
	}
	dir := setupTree(t, files)

	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("dist/\ntmp/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSuggester(dir)
	paths := []string{"src/main.go", "dist/bundle.js", "tmp/scratch.txt"}
	filtered := fs.filterIgnored(paths)

	if len(filtered) != 1 {
		t.Fatalf("expected 1 file after filtering, got %d: %v", len(filtered), filtered)
	}
	if filtered[0] != "src/main.go" {
		t.Errorf("expected src/main.go, got %q", filtered[0])
	}
}

func TestCollectDirs(t *testing.T) {
	files := []string{
		"src/index.js",
		"src/utils/helpers.js",
		"pkg/config/settings.go",
	}
	dirs := collectDirs(files)

	expected := map[string]bool{
		"src/":        true,
		"src/utils/":  true,
		"pkg/":        true,
		"pkg/config/": true,
	}
	for _, d := range dirs {
		if !expected[d] {
			t.Errorf("unexpected directory: %q", d)
		}
		delete(expected, d)
	}
	for d := range expected {
		t.Errorf("missing directory: %q", d)
	}
}

func TestFindLongestCommonPrefix(t *testing.T) {
	tests := []struct {
		items []SuggestionItem
		want  string
	}{
		{nil, ""},
		{
			[]SuggestionItem{{DisplayText: "src/main.go"}, {DisplayText: "src/utils.go"}},
			"src/",
		},
		{
			[]SuggestionItem{{DisplayText: "abc"}, {DisplayText: "xyz"}},
			"",
		},
		{
			[]SuggestionItem{{DisplayText: "config.yaml"}},
			"config.yaml",
		},
	}
	for _, tt := range tests {
		got := FindLongestCommonPrefix(tt.items)
		if got != tt.want {
			t.Errorf("FindLongestCommonPrefix(%v) = %q, want %q", tt.items, got, tt.want)
		}
	}
}

func TestApplySuggestion(t *testing.T) {
	input := "look at @src/ma and tell me"
	partial := "src/ma"
	startPos := 9 // position after "@"

	newInput, cursor := ApplySuggestion("src/main.go", input, partial, startPos)
	want := "look at @src/main.go and tell me"
	if newInput != want {
		t.Errorf("ApplySuggestion: got %q, want %q", newInput, want)
	}
	if cursor != 20 { // 9 + len("src/main.go")
		t.Errorf("cursor: got %d, want 20", cursor)
	}
}

func TestPathListSignature(t *testing.T) {
	paths := []string{"a/b.go", "c/d.go", "e/f.go"}
	sig1 := PathListSignature(paths)
	sig2 := PathListSignature(paths)
	if sig1 != sig2 {
		t.Errorf("same input should produce same signature: %q vs %q", sig1, sig2)
	}

	// Different input should (very likely) produce different signature.
	paths2 := []string{"x/y.go", "z/w.go"}
	sig3 := PathListSignature(paths2)
	if sig1 == sig3 {
		t.Errorf("different inputs produced same signature: %q", sig1)
	}

	// Empty list.
	sig4 := PathListSignature(nil)
	if sig4 != "0:811c9dc5" {
		t.Errorf("empty signature: got %q", sig4)
	}
}

func TestSubsequenceMatch(t *testing.T) {
	tests := []struct {
		haystack, needle string
		want             bool
	}{
		{"abcdef", "ace", true},
		{"abcdef", "adf", true},
		{"abcdef", "xyz", false},
		{"abcdef", "abcdef", true},
		{"abc", "abcd", false},
		{"", "", true},
		{"abc", "", true},
	}
	for _, tt := range tests {
		got := subsequenceMatch(tt.haystack, tt.needle)
		if got != tt.want {
			t.Errorf("subsequenceMatch(%q, %q) = %v, want %v", tt.haystack, tt.needle, got, tt.want)
		}
	}
}

func TestGenerateSuggestions_EmptyPartial(t *testing.T) {
	dir := setupTree(t, []string{"a.txt", "b.txt"})
	fs := NewFileSuggester(dir)

	// Without showOnEmpty, empty partial returns nil.
	items := fs.GenerateSuggestions("", false)
	if items != nil {
		t.Errorf("expected nil for empty partial without showOnEmpty, got %v", items)
	}

	// With showOnEmpty, returns top-level listing.
	items = fs.GenerateSuggestions("", true)
	if len(items) < 2 {
		t.Errorf("expected at least 2 top-level entries, got %d", len(items))
	}
}

func TestGenerateSuggestions_WithFiles(t *testing.T) {
	files := []string{
		"src/main.go",
		"src/config.go",
		"pkg/utils/helper.go",
		"README.md",
	}
	dir := setupTree(t, files)

	fs := NewFileSuggester(dir)
	items := fs.GenerateSuggestions("main", false)

	found := false
	for _, it := range items {
		if it.DisplayText == "src/main.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected src/main.go in suggestions for 'main', got %v", displayTexts(items))
	}
}

func TestGenerateSuggestions_GitRepo(t *testing.T) {
	// Skip if git is not available.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	files := []string{
		"src/server.go",
		"src/handler.go",
		"docs/readme.md",
	}
	dir := setupTree(t, files)

	// Init git repo.
	for _, args := range [][]string{
		{"git", "-C", dir, "init", "-q"},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "add", "-A"},
		{"git", "-C", dir, "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s\n%s", args, err, out)
		}
	}

	fs := NewFileSuggester(dir)
	items := fs.GenerateSuggestions("server", false)

	found := false
	for _, it := range items {
		if it.DisplayText == "src/server.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected src/server.go in git-backed suggestions, got %v", displayTexts(items))
	}
}

func TestClearCaches(t *testing.T) {
	dir := setupTree(t, []string{"a.txt"})
	fs := NewFileSuggester(dir)
	fs.Refresh(true)

	fs.mu.Lock()
	if fs.files == nil {
		fs.mu.Unlock()
		t.Fatal("files should be populated after Refresh")
	}
	fs.mu.Unlock()

	fs.ClearCaches()

	fs.mu.Lock()
	if fs.files != nil {
		fs.mu.Unlock()
		t.Fatal("files should be nil after ClearCaches")
	}
	fs.mu.Unlock()
}

func TestOnIndexBuildComplete(t *testing.T) {
	dir := setupTree(t, []string{"a.txt"})
	fs := NewFileSuggester(dir)

	called := false
	fs.OnIndexBuildComplete(func() { called = true })
	fs.Refresh(true)

	if !called {
		t.Error("OnIndexBuildComplete callback was not invoked")
	}
}

// displayTexts extracts DisplayText values for error messages.
func displayTexts(items []SuggestionItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.DisplayText
	}
	return out
}
