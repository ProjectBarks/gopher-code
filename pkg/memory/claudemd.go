// Package memory provides CLAUDE.md file discovery, loading, and processing.
// Source: utils/claudemd.ts
//
// CLAUDE.md files are loaded in priority order (lowest → highest):
//  1. Managed (/etc/claude-code/CLAUDE.md)
//  2. User (~/.claude/CLAUDE.md)
//  3. Project (CLAUDE.md, .claude/CLAUDE.md, .claude/rules/*.md)
//  4. Local (CLAUDE.local.md)
//
// Files closer to cwd have higher priority (loaded later).
package memory

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MaxCharacterCount is the recommended max size for a single memory file.
// Source: claudemd.ts:92
const MaxCharacterCount = 40000

// MemoryInstructionPrompt is prepended to memory content in the system prompt.
const MemoryInstructionPrompt = "Codebase and user instructions are shown below. Be sure to adhere to these instructions. IMPORTANT: These instructions OVERRIDE any default behavior and you MUST follow them exactly as written."

// MemoryType classifies the source of a memory file.
type MemoryType string

const (
	TypeManaged MemoryType = "managed" // /etc/claude-code/CLAUDE.md
	TypeUser    MemoryType = "user"    // ~/.claude/CLAUDE.md
	TypeProject MemoryType = "project" // CLAUDE.md in project dirs
	TypeLocal   MemoryType = "local"   // CLAUDE.local.md
)

// FileInfo describes a discovered CLAUDE.md file.
// Source: claudemd.ts — MemoryFileInfo
type FileInfo struct {
	Path       string
	Content    string
	Type       MemoryType
	CharCount  int
	IsLarge    bool // > MaxCharacterCount
	Source     string // "managed", "user", "project", "local"
}

// DiscoverFiles finds all CLAUDE.md files from cwd up to root.
// Returns files in priority order (lowest first → highest last).
// Source: claudemd.ts — getMemoryFiles
func DiscoverFiles(cwd string) []FileInfo {
	var files []FileInfo

	// 1. Managed memory
	if f := readIfExists("/etc/claude-code/CLAUDE.md", TypeManaged); f != nil {
		files = append(files, *f)
	}

	// 2. User memory (~/.claude/CLAUDE.md, ~/.claude/rules/*.md)
	if home, err := os.UserHomeDir(); err == nil {
		if f := readIfExists(filepath.Join(home, ".claude", "CLAUDE.md"), TypeUser); f != nil {
			files = append(files, *f)
		}
		files = append(files, discoverRulesDir(filepath.Join(home, ".claude", "rules"), TypeUser)...)
	}

	// 3. Project memory — walk from root down to cwd (farther = lower priority)
	dirs := ancestorDirs(cwd)
	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		// CLAUDE.md
		if f := readIfExists(filepath.Join(dir, "CLAUDE.md"), TypeProject); f != nil {
			files = append(files, *f)
		}
		// .claude/CLAUDE.md
		if f := readIfExists(filepath.Join(dir, ".claude", "CLAUDE.md"), TypeProject); f != nil {
			files = append(files, *f)
		}
		// .claude/rules/*.md
		files = append(files, discoverRulesDir(filepath.Join(dir, ".claude", "rules"), TypeProject)...)
	}

	// 4. Local memory
	if f := readIfExists(filepath.Join(cwd, "CLAUDE.local.md"), TypeLocal); f != nil {
		files = append(files, *f)
	}

	return files
}

// GetLargeFiles returns files exceeding MaxCharacterCount.
// Source: claudemd.ts — getLargeMemoryFiles
func GetLargeFiles(files []FileInfo) []FileInfo {
	var large []FileInfo
	for _, f := range files {
		if f.IsLarge {
			large = append(large, f)
		}
	}
	return large
}

// IsMemoryFilePath returns true if the path looks like a CLAUDE.md / memory file.
// Source: claudemd.ts — isMemoryFilePath
func IsMemoryFilePath(path string) bool {
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	return lower == "claude.md" || lower == "claude.local.md" ||
		strings.HasSuffix(lower, ".md") && strings.Contains(path, ".claude/rules/")
}

// StripHTMLComments removes HTML comments from markdown content.
// Source: claudemd.ts — stripHtmlComments
func StripHTMLComments(content string) string {
	re := regexp.MustCompile(`<!--[\s\S]*?-->`)
	return re.ReplaceAllString(content, "")
}

// ClearCaches resets all memoized memory file state.
// Source: claudemd.ts — clearMemoryFileCaches
func ClearCaches() {
	// In Go, discovery is not memoized (called on demand), so this is a no-op.
	// Added for API parity with TS.
}

// readIfExists reads a file and returns a FileInfo, or nil if it doesn't exist.
func readIfExists(path string, memType MemoryType) *FileInfo {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	return &FileInfo{
		Path:      path,
		Content:   content,
		Type:      memType,
		CharCount: len(content),
		IsLarge:   len(content) > MaxCharacterCount,
		Source:    string(memType),
	}
}

// discoverRulesDir finds all .md files in a .claude/rules/ directory.
func discoverRulesDir(dir string, memType MemoryType) []FileInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []FileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if f := readIfExists(path, memType); f != nil {
			files = append(files, *f)
		}
	}
	return files
}

// ancestorDirs returns directories from cwd up to root.
func ancestorDirs(cwd string) []string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return []string{cwd}
	}
	var dirs []string
	current := abs
	for {
		dirs = append(dirs, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return dirs
}
