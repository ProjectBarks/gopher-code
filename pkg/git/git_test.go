package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644)
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

func TestFindRoot(t *testing.T) {
	dir := initTestRepo(t)
	sub := filepath.Join(dir, "subdir", "deep")
	os.MkdirAll(sub, 0755)

	root := FindRoot(sub)
	if root == "" {
		t.Fatal("should find git root")
	}
	// Resolve symlinks for macOS /private/var vs /var
	expected, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(root)
	if got != expected {
		t.Errorf("FindRoot = %q, want %q", got, expected)
	}
}

func TestFindRoot_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	if root := FindRoot(dir); root != "" {
		t.Errorf("FindRoot should return empty for non-repo, got %q", root)
	}
}

func TestIsRepo(t *testing.T) {
	dir := initTestRepo(t)
	if !IsRepo(dir) {
		t.Error("should be a repo")
	}
	if IsRepo(t.TempDir()) {
		t.Error("temp dir should not be a repo")
	}
}

func TestBranch(t *testing.T) {
	dir := initTestRepo(t)
	b := Branch(dir)
	if b != "main" {
		t.Errorf("Branch = %q, want main", b)
	}
}

func TestHead(t *testing.T) {
	dir := initTestRepo(t)
	h := Head(dir)
	if h == "" {
		t.Error("Head should return a short hash")
	}
	if len(h) < 7 {
		t.Errorf("Head hash too short: %q", h)
	}
}

func TestIsClean(t *testing.T) {
	dir := initTestRepo(t)
	if !IsClean(dir) {
		t.Error("should be clean after init")
	}
	os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty"), 0644)
	if IsClean(dir) {
		t.Error("should not be clean with untracked file")
	}
}

func TestChangedFiles(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644)
	files := ChangedFiles(dir)
	if len(files) == 0 {
		t.Error("should have changed files")
	}
}

func TestDiff(t *testing.T) {
	dir := initTestRepo(t)
	// Modify tracked file
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# modified"), 0644)

	d, err := Diff(dir, false)
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	if d == "" {
		t.Error("diff should not be empty for modified file")
	}
}
