package permissions

import (
	"path"
	"path/filepath"
	"strings"
)

// Source: utils/permissions/filesystem.ts

// DangerousFiles lists files that should be protected from auto-editing.
// These files can be used for code execution or data exfiltration.
// Source: filesystem.ts:57-68
var DangerousFiles = []string{
	".gitconfig",
	".gitmodules",
	".bashrc",
	".bash_profile",
	".zshrc",
	".zprofile",
	".profile",
	".ripgreprc",
	".mcp.json",
	".claude.json",
}

// DangerousDirectories lists directories that should be protected from auto-editing.
// Source: filesystem.ts:74-79
var DangerousDirectories = []string{
	".git",
	".vscode",
	".idea",
	".claude",
}

// NormalizeCaseForComparison lowercases a path for case-insensitive comparison.
// Prevents bypassing security checks on case-insensitive filesystems (macOS/Windows).
// Source: filesystem.ts:90-92
func NormalizeCaseForComparison(p string) string {
	return strings.ToLower(p)
}

// IsDangerousFilePath checks if a path is dangerous to auto-edit.
// Checks both dangerous directories (segments) and dangerous filenames.
// Source: filesystem.ts:435-488
func IsDangerousFilePath(filePath string) bool {
	absPath := filepath.Clean(filePath)
	segments := splitPathSegments(absPath)
	fileName := ""
	if len(segments) > 0 {
		fileName = segments[len(segments)-1]
	}

	// Check for UNC paths (defense-in-depth)
	if strings.HasPrefix(filePath, `\\`) || strings.HasPrefix(filePath, "//") {
		return true
	}

	// Check if path is within dangerous directories (case-insensitive)
	for i, seg := range segments {
		normalizedSeg := NormalizeCaseForComparison(seg)
		for _, dir := range DangerousDirectories {
			if normalizedSeg != NormalizeCaseForComparison(dir) {
				continue
			}
			// Special case: .claude/worktrees/ is structural, not dangerous
			// Source: filesystem.ts:458-467
			if dir == ".claude" && i+1 < len(segments) {
				nextSeg := NormalizeCaseForComparison(segments[i+1])
				if nextSeg == "worktrees" {
					break // skip this .claude, continue checking
				}
			}
			return true
		}
	}

	// Check for dangerous configuration files (case-insensitive)
	if fileName != "" {
		normalizedName := NormalizeCaseForComparison(fileName)
		for _, df := range DangerousFiles {
			if NormalizeCaseForComparison(df) == normalizedName {
				return true
			}
		}
	}

	return false
}

// PathSafetyResult is the result of a path safety check.
type PathSafetyResult struct {
	Safe                 bool
	Message              string
	ClassifierApprovable bool
}

// CheckPathSafetyForAutoEdit checks if a path is safe for auto-editing.
// Returns information about why the path is unsafe, or Safe=true if all checks pass.
// Source: filesystem.ts:620-665
func CheckPathSafetyForAutoEdit(filePath string) PathSafetyResult {
	if IsDangerousFilePath(filePath) {
		return PathSafetyResult{
			Safe:                 false,
			Message:              "Claude requested permissions to edit " + filePath + " which is a sensitive file.",
			ClassifierApprovable: true,
		}
	}
	return PathSafetyResult{Safe: true}
}

// IsPathInWorkingDir checks if a path is contained within a working directory.
// Uses case-insensitive comparison and handles macOS /private symlinks.
// Source: filesystem.ts:709-744
func IsPathInWorkingDir(filePath string, workingDir string) bool {
	absPath := filepath.Clean(filePath)
	absWorkDir := filepath.Clean(workingDir)

	// Normalize macOS symlinks: /private/var → /var, /private/tmp → /tmp
	// Source: filesystem.ts:716-721
	absPath = normalizeMacOSPath(absPath)
	absWorkDir = normalizeMacOSPath(absWorkDir)

	// Case-insensitive comparison
	// Source: filesystem.ts:725-726
	normalizedPath := NormalizeCaseForComparison(absPath)
	normalizedWorkDir := NormalizeCaseForComparison(absWorkDir)

	// Compute relative path (POSIX-style)
	// Source: filesystem.ts:731-743
	rel, err := filepath.Rel(normalizedWorkDir, normalizedPath)
	if err != nil {
		return false
	}

	// Same path
	if rel == "." {
		return true
	}

	// Path traversal check — reject if relative path goes up
	if containsPathTraversal(rel) {
		return false
	}

	// Must be a relative (non-absolute) path that doesn't traverse up
	return !filepath.IsAbs(rel)
}

// IsPathInAnyWorkingDir checks if a path is in the primary cwd or any additional directory.
// Source: filesystem.ts:683-707
func IsPathInAnyWorkingDir(filePath string, cwd string, additionalDirs map[string]string) bool {
	if IsPathInWorkingDir(filePath, cwd) {
		return true
	}
	for dir := range additionalDirs {
		if IsPathInWorkingDir(filePath, dir) {
			return true
		}
	}
	return false
}

// normalizeMacOSPath strips the /private prefix from macOS symlinked paths.
func normalizeMacOSPath(p string) string {
	p = strings.Replace(p, "/private/var/", "/var/", 1)
	if strings.HasPrefix(p, "/private/tmp/") {
		p = "/tmp/" + p[len("/private/tmp/"):]
	} else if p == "/private/tmp" {
		p = "/tmp"
	}
	return p
}

// containsPathTraversal checks if a relative path contains ".." traversal.
func containsPathTraversal(rel string) bool {
	if rel == ".." {
		return true
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	// Also check POSIX separator for cross-platform safety
	if strings.HasPrefix(rel, "../") {
		return true
	}
	return false
}

// splitPathSegments splits a path into its segments.
func splitPathSegments(p string) []string {
	// Use forward-slash version for consistent splitting
	posixPath := filepath.ToSlash(p)
	parts := strings.Split(posixPath, "/")
	var segments []string
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

// IsClaudeSettingsPath checks if a path is a Claude settings file.
// Source: filesystem.ts:200-222
func IsClaudeSettingsPath(filePath string) bool {
	cleaned := filepath.Clean(filePath)
	normalized := NormalizeCaseForComparison(cleaned)

	sep := string(filepath.Separator)
	if strings.HasSuffix(normalized, sep+".claude"+sep+"settings.json") {
		return true
	}
	if strings.HasSuffix(normalized, sep+".claude"+sep+"settings.local.json") {
		return true
	}
	return false
}

// MatchingRuleForInput finds a rule that matches a tool+input combination using
// gitignore-style path patterns. Returns the matched rule or nil.
// Source: filesystem.ts step 1.6
func MatchingRuleForInput(rules []PermissionRule, toolName string, toolInput string) *PermissionRule {
	for i := range rules {
		if RuleMatchesToolCall(rules[i].RuleValue, toolName, toolInput) {
			return &rules[i]
		}
	}
	return nil
}

// CheckReadPermissionForTool returns a permission decision for read-type tools.
// Source: filesystem.ts:checkReadPermissionForTool
func CheckReadPermissionForTool(
	toolName string,
	filePath string,
	cwd string,
	additionalDirs map[string]string,
	denyRules []PermissionRule,
	allowRules []PermissionRule,
) PermissionDecision {
	// Step 1: Check deny rules
	if rule := MatchingRuleForInput(denyRules, toolName, filePath); rule != nil {
		return DenyDecision{Reason: "denied by rule from " + rule.Source}
	}

	// Step 2: In working directory — allow
	if IsPathInAnyWorkingDir(filePath, cwd, additionalDirs) {
		return AllowDecision{}
	}

	// Step 3: Check allow rules
	if rule := MatchingRuleForInput(allowRules, toolName, filePath); rule != nil {
		return AllowDecision{}
	}

	// Step 4: Outside working directory — ask
	return AskDecision{
		Message: path.Base(filePath) + " is outside the current working directory. Allow reading?",
	}
}

// CheckWritePermissionForTool returns a permission decision for write-type tools.
// Source: filesystem.ts:checkWritePermissionForTool
func CheckWritePermissionForTool(
	toolName string,
	filePath string,
	cwd string,
	additionalDirs map[string]string,
	denyRules []PermissionRule,
	allowRules []PermissionRule,
) PermissionDecision {
	// Step 1: Check deny rules
	if rule := MatchingRuleForInput(denyRules, toolName, filePath); rule != nil {
		return DenyDecision{Reason: "denied by rule from " + rule.Source}
	}

	// Step 2: Safety check for auto-edits
	safety := CheckPathSafetyForAutoEdit(filePath)
	if !safety.Safe {
		return AskDecision{Message: safety.Message}
	}

	// Step 3: Must be in working directory
	if !IsPathInAnyWorkingDir(filePath, cwd, additionalDirs) {
		return AskDecision{
			Message: path.Base(filePath) + " is outside the current working directory. Allow writing?",
		}
	}

	// Step 4: Check allow rules
	if rule := MatchingRuleForInput(allowRules, toolName, filePath); rule != nil {
		return AllowDecision{}
	}

	// Step 5: In working directory, no explicit rule — allow for file tools
	return AllowDecision{}
}
