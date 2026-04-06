package tools

import (
	"regexp"
	"strings"
)

// PS destructive command patterns — informational warnings for permission dialog.
// Source: destructiveCommandWarning.ts:12-96
type psDestructivePattern struct {
	Pattern *regexp.Regexp
	Warning string
}

// psDestructivePatterns lists patterns that match dangerous/irreversible PS commands.
// Source: destructiveCommandWarning.ts:12-96 — DESTRUCTIVE_PATTERNS
var psDestructivePatterns = []psDestructivePattern{
	// Remove-Item with -Recurse and -Force (either order)
	{regexp.MustCompile(`(?i)(?:^|[|;&\n({])\s*(?:Remove-Item|rm|del|rd|rmdir|ri)\b[^|;&\n}]*-Recurse\b[^|;&\n}]*-Force\b`),
		"Note: may recursively force-remove files"},
	{regexp.MustCompile(`(?i)(?:^|[|;&\n({])\s*(?:Remove-Item|rm|del|rd|rmdir|ri)\b[^|;&\n}]*-Force\b[^|;&\n}]*-Recurse\b`),
		"Note: may recursively force-remove files"},
	// Remove-Item with -Recurse only
	{regexp.MustCompile(`(?i)(?:^|[|;&\n({])\s*(?:Remove-Item|rm|del|rd|rmdir|ri)\b[^|;&\n}]*-Recurse\b`),
		"Note: may recursively remove files"},
	// Remove-Item with -Force only
	{regexp.MustCompile(`(?i)(?:^|[|;&\n({])\s*(?:Remove-Item|rm|del|rd|rmdir|ri)\b[^|;&\n}]*-Force\b`),
		"Note: may force-remove files"},
	// Clear-Content on broad paths
	{regexp.MustCompile(`(?i)\bClear-Content\b[^|;&\n]*\*`),
		"Note: may clear content of multiple files"},
	// Format-Volume and Clear-Disk
	{regexp.MustCompile(`(?i)\bFormat-Volume\b`),
		"Note: may format a disk volume"},
	{regexp.MustCompile(`(?i)\bClear-Disk\b`),
		"Note: may clear a disk"},
	// Git destructive operations
	{regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`),
		"Note: may discard uncommitted changes"},
	{regexp.MustCompile(`(?i)\bgit\s+push\b[^|;&\n]*\s+(?:--force|--force-with-lease|-f)\b`),
		"Note: may overwrite remote history"},
	// git clean -f (without dry-run) — Go regexp lacks lookahead so handle separately
	{regexp.MustCompile(`(?i)\bgit\s+stash\s+(?:drop|clear)\b`),
		"Note: may permanently remove stashed changes"},
	// Database operations
	{regexp.MustCompile(`(?i)\b(?:DROP|TRUNCATE)\s+(?:TABLE|DATABASE|SCHEMA)\b`),
		"Note: may drop or truncate database objects"},
	// System operations
	{regexp.MustCompile(`(?i)\bStop-Computer\b`),
		"Note: will shut down the computer"},
	{regexp.MustCompile(`(?i)\bRestart-Computer\b`),
		"Note: will restart the computer"},
	{regexp.MustCompile(`(?i)\bClear-RecycleBin\b`),
		"Note: permanently deletes recycled files"},
}

// psGitCleanForceRe matches `git clean` with `-f` flag.
var psGitCleanForceRe = regexp.MustCompile(`(?i)\bgit\s+clean\b[^|;&\n]*-[a-zA-Z]*f`)

// psGitCleanDryRunRe matches `git clean` with dry-run flag.
var psGitCleanDryRunRe = regexp.MustCompile(`(?i)\bgit\s+clean\b[^|;&\n]*(?:-[a-zA-Z]*n|--dry-run)`)

// GetPSDestructiveCommandWarning checks if a PowerShell command matches known
// destructive patterns and returns a human-readable warning, or empty string if safe.
// Source: destructiveCommandWarning.ts:102-109 — getDestructiveCommandWarning()
func GetPSDestructiveCommandWarning(command string) string {
	// Custom check for git clean -f (needs negative lookahead equivalent)
	if psGitCleanForceRe.MatchString(command) && !psGitCleanDryRunRe.MatchString(command) {
		return "Note: may permanently delete untracked files"
	}

	for _, dp := range psDestructivePatterns {
		if dp.Pattern.MatchString(command) {
			return dp.Warning
		}
	}
	return ""
}

// PS search commands (grep equivalents) for collapsible display.
// Source: PowerShellTool.tsx:54-61
var psSearchCommands = map[string]bool{
	"select-string": true, "get-childitem": true,
	"findstr": true, "where.exe": true,
}

// PS read/view commands for collapsible display.
// Source: PowerShellTool.tsx:67-88
var psReadCommands = map[string]bool{
	"get-content": true, "get-item": true, "test-path": true,
	"resolve-path": true, "get-process": true, "get-service": true,
	"get-childitem": true, "get-location": true, "get-filehash": true,
	"get-acl": true, "format-hex": true,
}

// PS semantic-neutral commands that don't change search/read nature.
// Source: PowerShellTool.tsx:93-95
var psSemanticNeutralCommands = map[string]bool{
	"write-output": true, "write-host": true,
}

// IsSearchOrReadPSCommand checks if a command is a search or read operation.
// Source: PowerShellTool.tsx:99-118
func IsSearchOrReadPSCommand(command string) (isSearch, isRead bool) {
	canonical := ResolvePSToCanonical(strings.TrimSpace(command))
	lower := strings.ToLower(canonical)
	// Extract first word
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return false, false
	}
	first := fields[0]

	if psSearchCommands[first] {
		return true, false
	}
	if psReadCommands[first] {
		return false, true
	}
	return false, false
}

// PSCmdletAliases maps common PS aliases to canonical cmdlet names (lowercase).
// Source: readOnlyValidation.ts — resolveToCanonical
var PSCmdletAliases = map[string]string{
	"dir": "get-childitem", "ls": "get-childitem", "gci": "get-childitem",
	"cd": "set-location", "sl": "set-location", "chdir": "set-location",
	"pushd": "push-location", "popd": "pop-location",
	"cat": "get-content", "gc": "get-content", "type": "get-content",
	"rm": "remove-item", "del": "remove-item", "erase": "remove-item",
	"rd": "remove-item", "rmdir": "remove-item", "ri": "remove-item",
	"cp": "copy-item", "copy": "copy-item", "ci": "copy-item",
	"mv": "move-item", "move": "move-item", "mi": "move-item",
	"md": "new-item", "mkdir": "new-item", "ni": "new-item",
	"cls": "clear-host", "clear": "clear-host",
	"echo": "write-output", "write": "write-output",
	"pwd": "get-location", "gl": "get-location",
	"ps": "get-process", "gps": "get-process",
	"gsv": "get-service",
	"iwr": "invoke-webrequest", "wget": "invoke-webrequest", "curl": "invoke-webrequest",
	"iex": "invoke-expression",
	"sal": "set-alias", "nal": "new-alias",
	"sls": "select-string",
	"ft": "format-table", "fl": "format-list", "fw": "format-wide",
	"sort": "sort-object", "group": "group-object", "measure": "measure-object",
	"select": "select-object", "where": "where-object", "foreach": "foreach-object",
	"?": "where-object", "%": "foreach-object",
	"sc": "set-content",
	"ac": "add-content",
	"clc": "clear-content",
	"gi": "get-item",
	"ii": "invoke-item",
	"si": "set-item",
	"sp": "stop-process",
}

// ResolvePSToCanonical converts PS aliases to canonical cmdlet name (lowercase).
// Source: readOnlyValidation.ts — resolveToCanonical()
func ResolvePSToCanonical(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return lower
	}
	first := fields[0]
	if canonical, ok := PSCmdletAliases[first]; ok {
		fields[0] = canonical
		return strings.Join(fields, " ")
	}
	return lower
}

// psReadOnlyCmdlets are cmdlets that are always safe (read-only).
// Source: readOnlyValidation.ts — CMDLET_ALLOWLIST (read-only subset)
var psReadOnlyCmdlets = map[string]bool{
	"get-childitem": true, "get-content": true, "get-item": true,
	"get-itemproperty": true, "get-itempropertyvalue": true,
	"get-location": true, "get-command": true, "get-alias": true,
	"get-variable": true, "get-process": true, "get-service": true,
	"get-eventlog": true, "get-winevent": true, "get-date": true,
	"get-random": true, "get-unique": true, "get-member": true,
	"get-host": true, "get-culture": true, "get-uiculture": true,
	"get-help": true, "get-module": true, "get-psreadlinekeyhandler": true,
	"get-executionpolicy": true, "get-filehash": true, "get-acl": true,
	"get-computerinfo": true, "get-timezone": true,
	"test-path": true, "test-connection": true,
	"resolve-path": true, "split-path": true, "join-path": true,
	"convert-path": true,
	"select-object": true, "where-object": true, "foreach-object": true,
	"sort-object": true, "group-object": true, "measure-object": true,
	"compare-object": true,
	"format-table": true, "format-list": true, "format-wide": true,
	"format-custom": true, "format-hex": true,
	"convertto-json": true, "convertfrom-json": true,
	"convertto-csv": true, "convertfrom-csv": true,
	"convertto-xml": true, "convertto-html": true,
	"select-string": true,
	"write-output": true, "write-host": true, "write-verbose": true,
	"write-debug": true, "write-information": true, "write-warning": true,
	"out-string": true, "out-null": true,
	"measure-command": true,
}

// psReadOnlyExternalCommands are external commands (not PS cmdlets) that are read-only.
// Source: readOnlyValidation.ts — isExternalCommandSafe (git/gh/docker read subcommands)
var psReadOnlyExternalCommands = map[string]bool{
	"where.exe": true,
}

// IsPSReadOnlyCommand checks if a PowerShell command is provably read-only.
// Source: readOnlyValidation.ts — isReadOnlyCommand()
func IsPSReadOnlyCommand(command string) bool {
	canonical := ResolvePSToCanonical(strings.TrimSpace(command))
	lower := strings.ToLower(canonical)

	// Split on statement separators (;)
	statements := strings.Split(lower, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		fields := strings.Fields(stmt)
		if len(fields) == 0 {
			continue
		}
		first := fields[0]
		// Resolve alias again for the individual statement
		if alias, ok := PSCmdletAliases[first]; ok {
			first = alias
		}
		if !psReadOnlyCmdlets[first] && !psReadOnlyExternalCommands[first] && !isPSGitReadOnly(fields) {
			return false
		}
	}
	return len(statements) > 0
}

// isPSGitReadOnly checks if a git command (given as fields) is read-only.
func isPSGitReadOnly(fields []string) bool {
	if len(fields) < 2 || fields[0] != "git" {
		return false
	}
	return gitReadOnlySubcommands[fields[1]]
}

// gitReadOnlySubcommands lists git subcommands that are read-only.
// (reuses the same set from shellparse.go if available, otherwise defines locally)
var psGitReadOnlySubcommands = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true, "branch": true,
	"tag": true, "remote": true, "describe": true, "rev-parse": true,
	"rev-list": true, "ls-files": true, "ls-tree": true, "cat-file": true,
	"name-rev": true, "shortlog": true, "blame": true, "config": true,
	"reflog": true, "stash": true, "worktree": true,
}

// PS common parameters (available on all cmdlets via [CmdletBinding()]).
// Source: commonParameters.ts:12-30
var PSCommonSwitches = []string{"-verbose", "-debug"}

var PSCommonValueParams = []string{
	"-erroraction", "-warningaction", "-informationaction", "-progressaction",
	"-errorvariable", "-warningvariable", "-informationvariable",
	"-outvariable", "-outbuffer", "-pipelinevariable",
}

// PSCommonParameters is the union of common switches and value params.
// Source: commonParameters.ts:27-30
var PSCommonParameters = func() map[string]bool {
	m := make(map[string]bool, len(PSCommonSwitches)+len(PSCommonValueParams))
	for _, s := range PSCommonSwitches {
		m[s] = true
	}
	for _, p := range PSCommonValueParams {
		m[p] = true
	}
	return m
}()

// IsPSCommonParameter checks if a parameter name is a PS common parameter.
func IsPSCommonParameter(param string) bool {
	return PSCommonParameters[strings.ToLower(param)]
}

// PS command-specific exit code semantics.
// Source: commandSemantics.ts — COMMAND_SEMANTICS map

// InterpretPSCommandResult applies PS command-specific exit code semantics.
// Source: commandSemantics.ts:124-140 — interpretCommandResult()
func InterpretPSCommandResult(command string, exitCode int) CommandSemantic {
	base := extractPSLastBaseCommand(command)
	switch base {
	case "grep", "rg", "findstr", "select-string":
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "No matches found"}
		}
		return CommandSemantic{IsError: exitCode >= 2}
	case "robocopy":
		// robocopy: 0-7 = success/info, 8+ = error
		if exitCode >= 8 {
			return CommandSemantic{IsError: true, Message: "Robocopy error"}
		}
		if exitCode == 0 {
			return CommandSemantic{IsError: false, Message: "No files copied (already in sync)"}
		}
		if exitCode&1 != 0 {
			return CommandSemantic{IsError: false, Message: "Files copied successfully"}
		}
		return CommandSemantic{IsError: false, Message: "Robocopy completed (no errors)"}
	default:
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "Command failed with exit code"}
		}
		return CommandSemantic{IsError: false}
	}
}

// extractPSLastBaseCommand extracts the base command from the last pipeline segment.
// Source: commandSemantics.ts:112-119 — heuristicallyExtractBaseCommand()
func extractPSLastBaseCommand(command string) string {
	// Split on ; | to get last segment
	segments := splitPSOnSeparators(command)
	last := strings.TrimSpace(segments[len(segments)-1])

	// Strip `& .` call operator
	last = strings.TrimSpace(last)
	if strings.HasPrefix(last, "& ") {
		last = strings.TrimPrefix(last, "& ")
		last = strings.TrimSpace(last)
	}

	// Extract first word, strip path and .exe
	fields := strings.Fields(last)
	if len(fields) == 0 {
		return ""
	}
	name := fields[0]
	// Strip path
	if idx := strings.LastIndexAny(name, `/\`); idx >= 0 {
		name = name[idx+1:]
	}
	// Strip .exe suffix
	name = strings.TrimSuffix(strings.ToLower(name), ".exe")
	return name
}

// splitPSOnSeparators splits a PS command on ;, |.
func splitPSOnSeparators(command string) []string {
	var segments []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(command); i++ {
		ch := command[i]
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			current.WriteByte(ch)
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			current.WriteByte(ch)
			continue
		}
		if (ch == '|' || ch == ';') && !inSingle && !inDouble {
			segments = append(segments, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	segments = append(segments, current.String())
	return segments
}

// DisallowedPSAutoBackgroundCommands are commands that should NOT be auto-backgrounded.
// Source: PowerShellTool.tsx:220
var DisallowedPSAutoBackgroundCommands = []string{"start-sleep"}

// CommonPSBackgroundCommands are known long-running commands allowed for auto-backgrounding.
// Source: PowerShellTool.tsx:222-234
var CommonPSBackgroundCommands = []string{
	"npm", "yarn", "pnpm", "node", "python", "python3", "go", "cargo",
	"make", "docker", "terraform", "webpack", "vite", "jest", "pytest",
	"curl", "invoke-webrequest", "build", "test", "serve", "watch", "dev",
}

// ValidatePSCommand runs the full validation pipeline for a PS command.
// Returns nil if the command is allowed, or a ToolOutput error if rejected.
// Source: PowerShellTool.tsx — pre-execution validation
func ValidatePSCommand(command string, cwd string, projectDir string, planMode bool) *ToolOutput {
	// 1. Plan mode: reject write commands
	if planMode {
		if !IsPSReadOnlyCommand(command) {
			return ErrorOutput("cannot execute write commands in plan mode — only read-only commands are allowed")
		}
	}

	// 2. Validate working directory doesn't escape project
	if cwd != "" && projectDir != "" {
		if err := ValidateWorkingDirectory(cwd, projectDir); err != nil {
			return ErrorOutput("working directory rejected: " + err.Error())
		}
	}

	return nil // All checks passed
}
