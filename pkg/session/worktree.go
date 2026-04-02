package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Source: utils/swarm/teamHelpers.ts:488-551

// CreateWorktreeForTeammate creates an isolated git worktree for a teammate.
// The worktree is created as a new branch from the current HEAD.
// Returns the path to the new worktree directory.
func CreateWorktreeForTeammate(repoDir, teammateName string) (string, error) {
	safeName := sanitizePathComponent(teammateName)
	worktreePath := filepath.Join(repoDir, ".claude", "worktrees", safeName)

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return "", fmt.Errorf("create worktree parent: %w", err)
	}

	// Create git worktree with a new branch
	branchName := "claude-teammate-" + safeName
	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If branch already exists, try without -b
		cmd2 := exec.Command("git", "worktree", "add", worktreePath, branchName)
		cmd2.Dir = repoDir
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return "", fmt.Errorf("git worktree add failed: %s\n%s", err, string(append(out, out2...)))
		}
	}

	return worktreePath, nil
}

// DestroyWorktree removes a git worktree at the given path.
// First attempts `git worktree remove`, falls back to os.RemoveAll.
// Source: utils/swarm/teamHelpers.ts:488-551
func DestroyWorktree(worktreePath string) error {
	// Read .git file to find the main repo
	// Source: teamHelpers.ts:493-510
	mainRepoPath := findMainRepoFromWorktree(worktreePath)

	// Try git worktree remove first
	// Source: teamHelpers.ts:513-538
	if mainRepoPath != "" {
		cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
		cmd.Dir = mainRepoPath
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// Fallback: manually remove the directory
	// Source: teamHelpers.ts:540-550
	return os.RemoveAll(worktreePath)
}

// findMainRepoFromWorktree reads the .git file in a worktree to find the main repo.
// Source: teamHelpers.ts:493-510
func findMainRepoFromWorktree(worktreePath string) string {
	gitFilePath := filepath.Join(worktreePath, ".git")
	data, err := os.ReadFile(gitFilePath)
	if err != nil {
		return ""
	}

	content := strings.TrimSpace(string(data))
	// Format: "gitdir: /path/to/repo/.git/worktrees/name"
	if !strings.HasPrefix(content, "gitdir: ") {
		return ""
	}

	worktreeGitDir := strings.TrimPrefix(content, "gitdir: ")
	// Go up 2 levels from .git/worktrees/name to .git, then parent for repo root
	// Source: teamHelpers.ts:503-506
	mainGitDir := filepath.Join(worktreeGitDir, "..", "..")
	return filepath.Join(mainGitDir, "..")
}

// ListWorktrees returns all worktree paths for the current repository.
func ListWorktrees(repoDir string) ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}
	return paths, nil
}
