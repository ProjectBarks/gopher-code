package shared

import "testing"

func TestDetectGitOperation_Commit(t *testing.T) {
	op := DetectGitOperation(
		"git commit -m 'fix bug'",
		"[main abc1234] fix bug\n 1 file changed, 2 insertions(+)",
	)
	if op.Commit == nil {
		t.Fatal("should detect commit")
	}
	if op.Commit.SHA != "abc123" {
		t.Errorf("SHA = %q, want abc123", op.Commit.SHA)
	}
	if op.Commit.Kind != CommitKindCommit {
		t.Errorf("Kind = %q, want committed", op.Commit.Kind)
	}
}

func TestDetectGitOperation_Amend(t *testing.T) {
	op := DetectGitOperation(
		"git commit --amend -m 'updated'",
		"[main def5678] updated\n 1 file changed",
	)
	if op.Commit == nil {
		t.Fatal("should detect amend")
	}
	if op.Commit.Kind != CommitKindAmended {
		t.Errorf("Kind = %q, want amended", op.Commit.Kind)
	}
}

func TestDetectGitOperation_CherryPick(t *testing.T) {
	op := DetectGitOperation(
		"git cherry-pick abc123",
		"[feature aaa1111] cherry picked commit\n 1 file changed",
	)
	if op.Commit == nil {
		t.Fatal("should detect cherry-pick")
	}
	if op.Commit.Kind != CommitKindCherryPick {
		t.Errorf("Kind = %q, want cherry-picked", op.Commit.Kind)
	}
}

func TestDetectGitOperation_Push(t *testing.T) {
	op := DetectGitOperation(
		"git push origin main",
		"   abc1234..def5678  main -> main",
	)
	if op.Push == nil {
		t.Fatal("should detect push")
	}
	if op.Push.Branch != "main" {
		t.Errorf("Branch = %q, want main", op.Push.Branch)
	}
}

func TestDetectGitOperation_Merge(t *testing.T) {
	op := DetectGitOperation(
		"git merge feature-branch",
		"Merge made by the 'ort' strategy.\n 1 file changed",
	)
	if op.Branch == nil {
		t.Fatal("should detect merge")
	}
	if op.Branch.Ref != "feature-branch" {
		t.Errorf("Ref = %q, want feature-branch", op.Branch.Ref)
	}
	if op.Branch.Action != BranchMerged {
		t.Errorf("Action = %q, want merged", op.Branch.Action)
	}
}

func TestDetectGitOperation_PR(t *testing.T) {
	op := DetectGitOperation(
		"gh pr create --title 'Fix' --body 'desc'",
		"https://github.com/owner/repo/pull/42\n",
	)
	if op.PR == nil {
		t.Fatal("should detect PR creation")
	}
	if op.PR.Number != 42 {
		t.Errorf("Number = %d, want 42", op.PR.Number)
	}
	if op.PR.Action != PrCreated {
		t.Errorf("Action = %q, want created", op.PR.Action)
	}
	if op.PR.URL == "" {
		t.Error("URL should be set")
	}
}

func TestDetectGitOperation_PRFromText(t *testing.T) {
	op := DetectGitOperation(
		"gh pr close 99",
		"✓ Closed pull request #99\n",
	)
	if op.PR == nil {
		t.Fatal("should detect PR close from text")
	}
	if op.PR.Number != 99 {
		t.Errorf("Number = %d, want 99", op.PR.Number)
	}
	if op.PR.Action != PrClosed {
		t.Errorf("Action = %q, want closed", op.PR.Action)
	}
}

func TestDetectGitOperation_NoMatch(t *testing.T) {
	op := DetectGitOperation("ls -la", "total 32\ndrwxr-xr-x")
	if op.Commit != nil || op.Push != nil || op.Branch != nil || op.PR != nil {
		t.Error("ls should not detect any git operations")
	}
}

func TestParseGitCommitID(t *testing.T) {
	tests := map[string]string{
		"[main abc1234] fix bug":                "abc1234",
		"[feature (root-commit) def5678] init":  "def5678",
		"no commit output here":                 "",
	}
	for input, want := range tests {
		got := ParseGitCommitID(input)
		if got != want {
			t.Errorf("ParseGitCommitID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDetectGitOperation_WithGitFlags(t *testing.T) {
	// git with -c option before subcommand
	op := DetectGitOperation(
		"git -c commit.gpgsign=false commit -m 'no sign'",
		"[main fff0001] no sign\n 1 file changed",
	)
	if op.Commit == nil {
		t.Fatal("should detect commit with -c flag")
	}
}
