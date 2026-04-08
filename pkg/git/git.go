// Package git provides git operations and repository detection utilities.
// Source: utils/git.ts — findGitRoot, getBranch, getDefaultBranch, getDiff, etc.
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindRoot walks up from startPath looking for a .git directory or file.
// Returns the directory containing .git, or "" if not found.
// Source: utils/git.ts — findGitRoot
func FindRoot(startPath string) string {
	current, err := filepath.Abs(startPath)
	if err != nil {
		return ""
	}

	for {
		gitPath := filepath.Join(current, ".git")
		info, err := os.Stat(gitPath)
		if err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return current
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}

// IsRepo returns true if the directory is inside a git repository.
func IsRepo(dir string) bool {
	return FindRoot(dir) != ""
}

// IsWorktree returns true if dir is a git worktree (not the main repo).
// Worktrees have a .git file (not directory) pointing to the main repo.
func IsWorktree(dir string) bool {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() // file = worktree, dir = main repo
}

// Run executes a git command in the given directory and returns stdout.
// Returns ("", err) on failure.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Branch returns the current branch name, or "" if detached.
// Source: utils/git.ts — getBranch
func Branch(dir string) string {
	out, err := Run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	if out == "HEAD" {
		return "" // detached HEAD
	}
	return out
}

// DefaultBranch returns the default branch (main/master).
// Checks remote HEAD first, falls back to checking if main or master exists.
// Source: utils/git.ts — getDefaultBranch
func DefaultBranch(dir string) string {
	// Try remote HEAD
	out, err := Run(dir, "symbolic-ref", "refs/remotes/origin/HEAD", "--short")
	if err == nil && out != "" {
		parts := strings.SplitN(out, "/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return out
	}

	// Fall back to checking branches
	for _, name := range []string{"main", "master"} {
		if _, err := Run(dir, "rev-parse", "--verify", name); err == nil {
			return name
		}
	}
	return "main" // default assumption
}

// Head returns the current HEAD commit hash (short).
func Head(dir string) string {
	out, err := Run(dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return out
}

// IsClean returns true if the working tree has no uncommitted changes.
func IsClean(dir string) bool {
	out, err := Run(dir, "status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == ""
}

// ChangedFiles returns the list of files with uncommitted changes.
func ChangedFiles(dir string) []string {
	out, err := Run(dir, "status", "--porcelain", "-z")
	if err != nil {
		return nil
	}
	if out == "" {
		return nil
	}
	var files []string
	for _, entry := range strings.Split(out, "\x00") {
		if len(entry) > 3 {
			files = append(files, entry[3:])
		}
	}
	return files
}

// Diff returns the unified diff of unstaged changes.
// If staged is true, shows staged changes instead.
func Diff(dir string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	return Run(dir, args...)
}

// DiffFromBase returns the diff between the current branch and a base ref.
func DiffFromBase(dir, baseRef string) (string, error) {
	return Run(dir, "diff", baseRef+"...HEAD")
}

// RemoteURL returns the origin remote URL, or "" if none.
func RemoteURL(dir string) string {
	out, err := Run(dir, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return out
}
