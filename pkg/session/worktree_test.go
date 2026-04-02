package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Source: utils/swarm/teamHelpers.ts:488-551

func TestCreateAndDestroyWorktree(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create a temporary git repo
	repoDir := t.TempDir()
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}

	runGit("init")
	os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Test"), 0644)
	runGit("add", ".")
	runGit("commit", "-m", "initial")

	t.Run("create_worktree", func(t *testing.T) {
		path, err := CreateWorktreeForTeammate(repoDir, "researcher")
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if path == "" {
			t.Fatal("expected non-empty path")
		}

		// Verify worktree exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("worktree directory should exist")
		}

		// Verify .git file exists (worktree marker)
		gitFile := filepath.Join(path, ".git")
		if _, err := os.Stat(gitFile); os.IsNotExist(err) {
			t.Error(".git file should exist in worktree")
		}

		// Clean up
		DestroyWorktree(path)
	})

	t.Run("destroy_worktree", func(t *testing.T) {
		path, err := CreateWorktreeForTeammate(repoDir, "analyzer")
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}

		err = DestroyWorktree(path)
		if err != nil {
			t.Fatalf("destroy failed: %v", err)
		}

		// Should be removed
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("worktree should be removed after destroy")
		}
	})

	t.Run("destroy_nonexistent_safe", func(t *testing.T) {
		// Source: teamHelpers.ts:491 — safe to call on non-existent paths
		err := DestroyWorktree(filepath.Join(t.TempDir(), "nonexistent"))
		if err != nil {
			t.Errorf("destroy nonexistent should not error, got: %v", err)
		}
	})
}

func TestFindMainRepoFromWorktree(t *testing.T) {
	t.Run("not_a_worktree", func(t *testing.T) {
		result := findMainRepoFromWorktree(t.TempDir())
		if result != "" {
			t.Errorf("expected empty for non-worktree, got %q", result)
		}
	})
}
