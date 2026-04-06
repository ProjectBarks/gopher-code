// Package hooks provides TUI hook implementations ported from the TS
// hooks layer. Unlike React hooks these are plain structs with methods.
package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	ignore "github.com/sabhiram/go-gitignore"
)

// MaxSuggestions is the maximum number of autocomplete suggestions returned.
const MaxSuggestions = 15

// refreshThrottle controls how often the background cache may refresh.
const refreshThrottle = 5 * time.Second

// SuggestionItem represents a single autocomplete suggestion.
type SuggestionItem struct {
	ID          string
	DisplayText string
	Score       float64
}

// FileSuggester builds and queries a file index for @-mention and path
// autocomplete. It discovers files via git ls-files, respects .gitignore,
// and provides fuzzy matching with ranking by match quality and path depth.
type FileSuggester struct {
	mu sync.Mutex

	// cwd is the working directory for file discovery.
	cwd string

	// cached state
	files   []string // relative paths (tracked + untracked + config)
	dirs    []string // unique parent directories with trailing separator
	lastRef time.Time

	// gitignore filter (nil = no filtering)
	ig *ignore.GitIgnore

	// subscribers notified when an index build completes
	onComplete []func()
}

// NewFileSuggester creates a suggester rooted at cwd.
func NewFileSuggester(cwd string) *FileSuggester {
	return &FileSuggester{cwd: cwd}
}

// ClearCaches resets all cached state. Call on session resume.
func (fs *FileSuggester) ClearCaches() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.files = nil
	fs.dirs = nil
	fs.lastRef = time.Time{}
	fs.ig = nil
	fs.onComplete = nil
}

// OnIndexBuildComplete registers a callback fired after each index build.
func (fs *FileSuggester) OnIndexBuildComplete(fn func()) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.onComplete = append(fs.onComplete, fn)
}

// emitComplete fires build-complete callbacks outside the lock.
func (fs *FileSuggester) emitComplete() {
	fs.mu.Lock()
	cbs := make([]func(), len(fs.onComplete))
	copy(cbs, fs.onComplete)
	fs.mu.Unlock()
	for _, fn := range cbs {
		fn()
	}
}

// Refresh rebuilds the file index. It is throttled: repeated calls within
// refreshThrottle are no-ops unless force is true.
func (fs *FileSuggester) Refresh(force bool) {
	fs.mu.Lock()
	if !force && !fs.lastRef.IsZero() && time.Since(fs.lastRef) < refreshThrottle {
		fs.mu.Unlock()
		return
	}
	fs.mu.Unlock()

	tracked, untracked := fs.gitFiles()
	all := fs.filterIgnored(append(tracked, untracked...))
	dirs := collectDirs(all)

	fs.mu.Lock()
	fs.files = all
	fs.dirs = dirs
	fs.lastRef = time.Now()
	fs.mu.Unlock()

	fs.emitComplete()
}

// GenerateSuggestions returns up to MaxSuggestions matching the partial path.
// An empty partial returns top-level entries when showOnEmpty is true.
func (fs *FileSuggester) GenerateSuggestions(partial string, showOnEmpty bool) []SuggestionItem {
	if partial == "" && !showOnEmpty {
		return nil
	}

	// Lazy-init: first call triggers a synchronous build.
	fs.mu.Lock()
	needsBuild := fs.files == nil
	fs.mu.Unlock()
	if needsBuild {
		fs.Refresh(true)
	}

	// Empty / dot: return top-level directory listing.
	if partial == "" || partial == "." || partial == "./" {
		return fs.topLevel()
	}

	// Normalize ./ prefix and ~ expansion.
	norm := partial
	if strings.HasPrefix(norm, "./") {
		norm = norm[2:]
	}
	if strings.HasPrefix(norm, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			norm = filepath.Join(home, norm[1:])
		}
	}

	fs.mu.Lock()
	files := fs.files
	dirs := fs.dirs
	fs.mu.Unlock()

	candidates := make([]string, 0, len(files)+len(dirs))
	candidates = append(candidates, dirs...)
	candidates = append(candidates, files...)

	return fuzzyMatch(candidates, norm, MaxSuggestions)
}

// ApplySuggestion splices the selected suggestion into input at startPos,
// replacing the partial path, and returns the new input and cursor position.
func ApplySuggestion(suggestion string, input string, partial string, startPos int) (newInput string, cursorPos int) {
	newInput = input[:startPos] + suggestion + input[startPos+len(partial):]
	cursorPos = startPos + len(suggestion)
	return
}

// FindLongestCommonPrefix returns the longest common prefix of display texts.
func FindLongestCommonPrefix(items []SuggestionItem) string {
	if len(items) == 0 {
		return ""
	}
	prefix := items[0].DisplayText
	for _, it := range items[1:] {
		prefix = commonPrefix(prefix, it.DisplayText)
		if prefix == "" {
			return ""
		}
	}
	return prefix
}

// ---------- internal ----------

// gitFiles returns tracked and untracked files relative to cwd via git.
func (fs *FileSuggester) gitFiles() (tracked, untracked []string) {
	repoRoot := findGitRoot(fs.cwd)
	if repoRoot == "" {
		// Not a git repo: fall back to walking the directory.
		return fs.walkFiles(), nil
	}

	// Tracked files.
	out, err := exec.Command("git", "-C", repoRoot,
		"-c", "core.quotepath=false",
		"ls-files", "--recurse-submodules").Output()
	if err == nil {
		tracked = splitLines(string(out))
		tracked = normalizePaths(tracked, repoRoot, fs.cwd)
	}

	// Untracked files (exclude standard gitignored).
	out, err = exec.Command("git", "-C", repoRoot,
		"-c", "core.quotepath=false",
		"ls-files", "--others", "--exclude-standard").Output()
	if err == nil {
		untracked = splitLines(string(out))
		untracked = normalizePaths(untracked, repoRoot, fs.cwd)
	}

	return
}

// walkFiles is a fallback for non-git directories.
func (fs *FileSuggester) walkFiles() []string {
	var files []string
	_ = filepath.WalkDir(fs.cwd, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() && (name == ".git" || name == "node_modules" || name == ".svn" || name == ".hg") {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(fs.cwd, p)
			files = append(files, rel)
		}
		return nil
	})
	return files
}

// filterIgnored removes paths that match .gitignore patterns.
func (fs *FileSuggester) filterIgnored(paths []string) []string {
	ig := fs.loadIgnore()
	if ig == nil {
		return paths
	}
	fs.mu.Lock()
	fs.ig = ig
	fs.mu.Unlock()

	filtered := make([]string, 0, len(paths))
	for _, p := range paths {
		if !ig.MatchesPath(p) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// loadIgnore loads .gitignore, .ignore, and .rgignore from cwd and repo root.
func (fs *FileSuggester) loadIgnore() *ignore.GitIgnore {
	var patterns []string
	root := findGitRoot(fs.cwd)
	dirs := []string{fs.cwd}
	if root != "" && root != fs.cwd {
		dirs = append(dirs, root)
	}
	for _, dir := range dirs {
		for _, name := range []string{".gitignore", ".ignore", ".rgignore"} {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					patterns = append(patterns, line)
				}
			}
		}
	}
	if len(patterns) == 0 {
		return nil
	}
	return ignore.CompileIgnoreLines(patterns...)
}

// topLevel returns entries from the working directory root.
func (fs *FileSuggester) topLevel() []SuggestionItem {
	entries, err := os.ReadDir(fs.cwd)
	if err != nil {
		return nil
	}
	var items []SuggestionItem
	for i, e := range entries {
		if i >= MaxSuggestions {
			break
		}
		name := e.Name()
		if e.IsDir() {
			name += string(filepath.Separator)
		}
		items = append(items, SuggestionItem{
			ID:          "file-" + name,
			DisplayText: name,
		})
	}
	return items
}

// fuzzyMatch scores candidates against query and returns top-n results.
// Ranking: exact prefix > contains > subsequence. Ties broken by path
// depth (shallower first) then lexicographic order.
func fuzzyMatch(candidates []string, query string, n int) []SuggestionItem {
	if query == "" {
		return nil
	}
	lowerQ := strings.ToLower(query)

	type scored struct {
		path  string
		score float64
	}
	var matches []scored

	for _, c := range candidates {
		lowerC := strings.ToLower(c)
		var s float64
		switch {
		case strings.HasPrefix(lowerC, lowerQ):
			s = 1.0
		case strings.Contains(lowerC, lowerQ):
			s = 0.7
		case subsequenceMatch(lowerC, lowerQ):
			s = 0.4
		default:
			// Also try matching just the basename.
			base := strings.ToLower(filepath.Base(c))
			switch {
			case strings.HasPrefix(base, lowerQ):
				s = 0.9
			case strings.Contains(base, lowerQ):
				s = 0.6
			case subsequenceMatch(base, lowerQ):
				s = 0.35
			default:
				continue
			}
		}
		// Prefer shallower paths: penalise depth.
		depth := float64(strings.Count(c, string(filepath.Separator)))
		s -= depth * 0.01
		matches = append(matches, scored{c, s})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].path < matches[j].path
	})

	if len(matches) > n {
		matches = matches[:n]
	}

	items := make([]SuggestionItem, len(matches))
	for i, m := range matches {
		items[i] = SuggestionItem{
			ID:          "file-" + m.path,
			DisplayText: m.path,
			Score:       m.score,
		}
	}
	return items
}

// subsequenceMatch returns true if all chars of needle appear in haystack in order.
func subsequenceMatch(haystack, needle string) bool {
	hi := 0
	for ni := 0; ni < len(needle); ni++ {
		found := false
		for hi < len(haystack) {
			if haystack[hi] == needle[ni] {
				hi++
				found = true
				break
			}
			hi++
		}
		if !found {
			return false
		}
	}
	return true
}

// collectDirs extracts unique parent directory paths with trailing separator.
func collectDirs(files []string) []string {
	set := make(map[string]struct{})
	for _, f := range files {
		dir := filepath.Dir(f)
		for dir != "." {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			set[dir] = struct{}{}
			dir = parent
		}
	}
	dirs := make([]string, 0, len(set))
	for d := range set {
		dirs = append(dirs, d+string(filepath.Separator))
	}
	sort.Strings(dirs)
	return dirs
}

// findGitRoot walks up from dir looking for a .git directory.
func findGitRoot(dir string) string {
	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// normalizePaths converts repo-root-relative paths to cwd-relative.
func normalizePaths(files []string, repoRoot, cwd string) []string {
	if cwd == repoRoot {
		return files
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		abs := filepath.Join(repoRoot, f)
		rel, err := filepath.Rel(cwd, abs)
		if err != nil {
			continue
		}
		out = append(out, rel)
	}
	return out
}

// splitLines splits output on newlines, discarding empty strings.
func splitLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// commonPrefix returns the longest shared prefix of a and b.
func commonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}

// PathListSignature produces a cheap content-hash of a path list.
// Uses FNV-1a over a stride-sampled subset so it runs in <1ms on large lists.
func PathListSignature(paths []string) string {
	n := len(paths)
	stride := n / 500
	if stride < 1 {
		stride = 1
	}
	h := uint32(0x811c9dc5)
	for i := 0; i < n; i += stride {
		for j := 0; j < len(paths[i]); j++ {
			h ^= uint32(paths[i][j])
			h *= 0x01000193
		}
		h *= 0x01000193
	}
	if n > 0 {
		last := paths[n-1]
		for j := 0; j < len(last); j++ {
			h ^= uint32(last[j])
			h *= 0x01000193
		}
	}
	return fmt.Sprintf("%d:%x", n, h)
}
