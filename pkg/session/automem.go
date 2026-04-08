package session

import (
	"os"
	"path/filepath"
	"strings"
)

// Source: utils/memdir/autoMemoryPaths.ts

// AutoMemoryDir returns the path to the auto-memory directory for a project.
// This is ~/.claude/projects/{sanitized-path}/memory/
func AutoMemoryDir(projectDir string) string {
	home, _ := os.UserHomeDir()
	sanitized := sanitizeProjectPath(projectDir)
	return filepath.Join(home, ".claude", "projects", sanitized, "memory")
}

// EnsureAutoMemoryDir creates the auto-memory directory if it doesn't exist.
func EnsureAutoMemoryDir(projectDir string) (string, error) {
	dir := AutoMemoryDir(projectDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// ValidateMemoryFileName checks if a filename is valid for a memory file.
// Must be a .md file with a safe name (no path traversal, no dots except extension).
func ValidateMemoryFileName(name string) bool {
	if name == "" {
		return false
	}
	if filepath.Ext(name) != ".md" {
		return false
	}
	base := strings.TrimSuffix(name, ".md")
	if base == "" || base == "." || base == ".." {
		return false
	}
	if strings.ContainsAny(base, "/\\:") {
		return false
	}
	return true
}

// sanitizeProjectPath converts a directory path to a safe directory name.
// Replaces path separators with dashes and removes leading slashes.
func sanitizeProjectPath(dir string) string {
	// Remove leading slash, replace separators.
	dir = strings.TrimPrefix(dir, "/")
	dir = strings.ReplaceAll(dir, "/", "-")
	dir = strings.ReplaceAll(dir, "\\", "-")
	dir = strings.ReplaceAll(dir, ":", "-")
	return dir
}
