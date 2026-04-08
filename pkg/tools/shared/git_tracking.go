// Package shared provides tool utilities shared across multiple tool implementations.
// Source: tools/shared/gitOperationTracking.ts
package shared

import (
	"regexp"
	"strconv"
	"strings"
)

// CommitKind classifies a git commit operation.
type CommitKind string

const (
	CommitKindCommit      CommitKind = "committed"
	CommitKindAmended     CommitKind = "amended"
	CommitKindCherryPick  CommitKind = "cherry-picked"
)

// BranchAction classifies a branch integration.
type BranchAction string

const (
	BranchMerged  BranchAction = "merged"
	BranchRebased BranchAction = "rebased"
)

// PrAction classifies a PR operation.
type PrAction string

const (
	PrCreated   PrAction = "created"
	PrEdited    PrAction = "edited"
	PrMerged    PrAction = "merged"
	PrCommented PrAction = "commented"
	PrClosed    PrAction = "closed"
	PrReady     PrAction = "ready"
)

// GitOperation holds detected git operations from a command + output pair.
type GitOperation struct {
	Commit *CommitInfo
	Push   *PushInfo
	Branch *BranchInfo
	PR     *PRInfo
}

// CommitInfo describes a detected commit.
type CommitInfo struct {
	SHA  string
	Kind CommitKind
}

// PushInfo describes a detected push.
type PushInfo struct {
	Branch string
}

// BranchInfo describes a merge/rebase.
type BranchInfo struct {
	Ref    string
	Action BranchAction
}

// PRInfo describes a detected PR operation.
type PRInfo struct {
	Number int
	URL    string
	Action PrAction
}

// Regexes for git command detection.
// Source: gitOperationTracking.ts:23-34
var (
	gitCommitRe     = regexp.MustCompile(`\bgit(?:\s+-[cC]\s+\S+|\s+--\S+=\S+)*\s+commit\b`)
	gitPushRe       = regexp.MustCompile(`\bgit(?:\s+-[cC]\s+\S+|\s+--\S+=\S+)*\s+push\b`)
	gitCherryPickRe = regexp.MustCompile(`\bgit(?:\s+-[cC]\s+\S+|\s+--\S+=\S+)*\s+cherry-pick\b`)
	gitMergeRe      = regexp.MustCompile(`\bgit(?:\s+-[cC]\s+\S+|\s+--\S+=\S+)*\s+merge\b`)
	gitRebaseRe     = regexp.MustCompile(`\bgit(?:\s+-[cC]\s+\S+|\s+--\S+=\S+)*\s+rebase\b`)
	commitIDRe      = regexp.MustCompile(`\[[\w./-]+(?:\s+\(root-commit\))?\s+([0-9a-f]+)\]`)
	pushBranchRe    = regexp.MustCompile(`(?m)^\s*[+\-*!= ]?\s*(?:\[new branch\]|\S+\.\.+\S+)\s+\S+\s*->\s*(\S+)`)
	ghPrURLRe       = regexp.MustCompile(`https://github\.com/([^/\s]+/[^/\s]+)/pull/(\d+)`)
	ghPrNumRe       = regexp.MustCompile(`[Pp]ull request (?:\S+#)?#?(\d+)`)
)

type ghPrAction struct {
	re     *regexp.Regexp
	action PrAction
}

var ghPrActions = []ghPrAction{
	{regexp.MustCompile(`\bgh\s+pr\s+create\b`), PrCreated},
	{regexp.MustCompile(`\bgh\s+pr\s+edit\b`), PrEdited},
	{regexp.MustCompile(`\bgh\s+pr\s+merge\b`), PrMerged},
	{regexp.MustCompile(`\bgh\s+pr\s+comment\b`), PrCommented},
	{regexp.MustCompile(`\bgh\s+pr\s+close\b`), PrClosed},
	{regexp.MustCompile(`\bgh\s+pr\s+ready\b`), PrReady},
}

// DetectGitOperation scans a command string + output for git operations.
// Source: gitOperationTracking.ts:135-186
func DetectGitOperation(command, output string) GitOperation {
	var result GitOperation

	// Commit / cherry-pick
	isCherryPick := gitCherryPickRe.MatchString(command)
	if gitCommitRe.MatchString(command) || isCherryPick {
		if sha := ParseGitCommitID(output); sha != "" {
			kind := CommitKindCommit
			if isCherryPick {
				kind = CommitKindCherryPick
			} else if strings.Contains(command, "--amend") {
				kind = CommitKindAmended
			}
			result.Commit = &CommitInfo{SHA: sha[:min(6, len(sha))], Kind: kind}
		}
	}

	// Push
	if gitPushRe.MatchString(command) {
		if m := pushBranchRe.FindStringSubmatch(output); len(m) > 1 {
			result.Push = &PushInfo{Branch: m[1]}
		}
	}

	// Merge
	if gitMergeRe.MatchString(command) {
		if strings.Contains(output, "Fast-forward") || strings.Contains(output, "Merge made by") {
			if ref := parseRefFromCommand(command, "merge"); ref != "" {
				result.Branch = &BranchInfo{Ref: ref, Action: BranchMerged}
			}
		}
	}

	// Rebase
	if gitRebaseRe.MatchString(command) && strings.Contains(output, "Successfully rebased") {
		if ref := parseRefFromCommand(command, "rebase"); ref != "" {
			result.Branch = &BranchInfo{Ref: ref, Action: BranchRebased}
		}
	}

	// PR actions
	for _, a := range ghPrActions {
		if a.re.MatchString(command) {
			if m := ghPrURLRe.FindStringSubmatch(output); len(m) > 2 {
				num, _ := strconv.Atoi(m[2])
				result.PR = &PRInfo{Number: num, URL: m[0], Action: a.action}
			} else if m := ghPrNumRe.FindStringSubmatch(output); len(m) > 1 {
				num, _ := strconv.Atoi(m[1])
				result.PR = &PRInfo{Number: num, Action: a.action}
			}
			break
		}
	}

	return result
}

// ParseGitCommitID extracts a commit SHA from git commit output.
// Source: gitOperationTracking.ts:79-84
func ParseGitCommitID(stdout string) string {
	m := commitIDRe.FindStringSubmatch(stdout)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func parseRefFromCommand(command, verb string) string {
	re := regexp.MustCompile(`\bgit\b.*\b` + verb + `\b`)
	loc := re.FindStringIndex(command)
	if loc == nil {
		return ""
	}
	after := strings.TrimSpace(command[loc[1]:])
	for _, t := range strings.Fields(after) {
		if strings.HasPrefix(t, "&") || strings.HasPrefix(t, "|") || strings.HasPrefix(t, ";") {
			break
		}
		if strings.HasPrefix(t, "-") {
			continue
		}
		return t
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
