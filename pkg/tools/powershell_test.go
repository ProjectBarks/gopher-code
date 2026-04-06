package tools_test

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestPowerShellTool(t *testing.T) {
	tool := &tools.PowerShellTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "PowerShell" {
			t.Errorf("expected 'PowerShell', got %q", tool.Name())
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if tool.IsReadOnly() {
			t.Error("PowerShellTool should not be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		if _, ok := props["command"]; !ok {
			t.Error("schema missing 'command' property")
		}
		if _, ok := props["timeout"]; !ok {
			t.Error("schema missing 'timeout' property")
		}
		if _, ok := props["description"]; !ok {
			t.Error("schema missing 'description' property")
		}
		if _, ok := props["run_in_background"]; !ok {
			t.Error("schema missing 'run_in_background' property")
		}
		if ap, ok := parsed["additionalProperties"]; !ok || ap != false {
			t.Error("additionalProperties should be false")
		}
	})

	t.Run("missing_command", func(t *testing.T) {
		input := json.RawMessage(`{"command": ""}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty command")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("non_windows_without_pwsh", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping non-Windows test on Windows")
		}
		input := json.RawMessage(`{"command": "Write-Output 'hello'"}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError && !strings.Contains(out.Content, "not available") {
			t.Logf("got error: %s", out.Content)
		}
	})

	t.Run("plan_mode_rejects_write_commands", func(t *testing.T) {
		tc := &tools.ToolContext{PlanMode: true}
		input := json.RawMessage(`{"command": "Remove-Item foo.txt"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for write command in plan mode")
		}
		if !strings.Contains(out.Content, "plan mode") {
			t.Errorf("expected plan mode error message, got: %s", out.Content)
		}
	})

	t.Run("concurrency_safe_for_read_commands", func(t *testing.T) {
		readInput := json.RawMessage(`{"command": "Get-ChildItem"}`)
		if !tool.IsConcurrencySafe(readInput) {
			t.Error("expected Get-ChildItem to be concurrency safe")
		}

		writeInput := json.RawMessage(`{"command": "Remove-Item foo"}`)
		if tool.IsConcurrencySafe(writeInput) {
			t.Error("expected Remove-Item to NOT be concurrency safe")
		}
	})
}

// TestPowerShellPrompt verifies the Description contains key verbatim phrases
// from the TS source prompt.ts:73-144.
func TestPowerShellPrompt(t *testing.T) {
	tool := &tools.PowerShellTool{}
	desc := tool.Description()

	// Source: prompt.ts:78 — header line
	requiredPhrases := []string{
		"Executes a given PowerShell command with optional timeout",
		"Working directory persists between commands; shell state (variables, functions) does not",
		"IMPORTANT: This tool is for terminal operations via PowerShell: git, npm, docker, and PS cmdlets",
		"DO NOT use it for file operations",
		"Directory Verification",
		"Command Execution",
		"PowerShell Syntax Notes",
		"Variables use $ prefix",
		"Escape character is backtick",
		"Verb-Noun cmdlet naming",
		"Registry access uses PSDrive prefixes",
		"NEVER use `Read-Host`",
		"`Get-Credential`",
		"`Out-GridView`",
		"`pause`",
		"-Confirm:$false",
		"here-string",
		"closing `'@` MUST be at column 0",
		"stop-parsing token",
		"run_in_background",
		"Avoid unnecessary `Start-Sleep`",
		"Glob (NOT Get-ChildItem -Recurse)",
		"Grep (NOT Select-String)",
		"Read (NOT Get-Content)",
		"Write (NOT Set-Content/Out-File)",
		"Do NOT prefix commands with `cd` or `Set-Location`",
		"git reset --hard",
		"git push --force",
		"--no-verify",
		"--no-gpg-sign",
	}

	for _, phrase := range requiredPhrases {
		if !strings.Contains(desc, phrase) {
			t.Errorf("prompt missing required phrase: %q", phrase)
		}
	}
}

// TestPowerShellPromptEditions verifies edition-specific sections.
func TestPowerShellPromptEditions(t *testing.T) {
	t.Run("desktop_edition", func(t *testing.T) {
		prompt := tools.GetPowerShellPrompt("desktop")
		desktopPhrases := []string{
			"Windows PowerShell 5.1 (powershell.exe)",
			"`&&` and `||` are NOT available",
			"parser error",
			"A; if ($?) { B }",
			"Ternary",
			"null-coalescing",
			"null-conditional",
			"2>&1",
			"NativeCommandError",
			"UTF-16 LE",
			"ConvertFrom-Json",
			"-AsHashtable",
		}
		for _, phrase := range desktopPhrases {
			if !strings.Contains(prompt, phrase) {
				t.Errorf("desktop prompt missing: %q", phrase)
			}
		}
	})

	t.Run("core_edition", func(t *testing.T) {
		prompt := tools.GetPowerShellPrompt("core")
		corePhrases := []string{
			"PowerShell 7+ (pwsh)",
			"`&&` and `||` ARE available",
			"Ternary",
			"null-coalescing",
			"UTF-8 without BOM",
		}
		for _, phrase := range corePhrases {
			if !strings.Contains(prompt, phrase) {
				t.Errorf("core prompt missing: %q", phrase)
			}
		}
	})

	t.Run("unknown_edition", func(t *testing.T) {
		prompt := tools.GetPowerShellPrompt("unknown")
		unknownPhrases := []string{
			"unknown — assume Windows PowerShell 5.1 for compatibility",
			"Do NOT use `&&`",
			"PowerShell 7+ only",
		}
		for _, phrase := range unknownPhrases {
			if !strings.Contains(prompt, phrase) {
				t.Errorf("unknown prompt missing: %q", phrase)
			}
		}
	})
}

// TestPSDestructiveCommandWarning verifies all 14 destructive patterns from TS source.
// Source: destructiveCommandWarning.ts:12-96
func TestPSDestructiveCommandWarning(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		// Remove-Item with -Recurse -Force
		{"rm_recurse_force", "Remove-Item -Recurse -Force ./build", "Note: may recursively force-remove files"},
		{"rm_force_recurse", "rm -Force -Recurse ./build", "Note: may recursively force-remove files"},
		{"del_recurse_force", "del -Recurse -Force ./tmp", "Note: may recursively force-remove files"},
		// Remove-Item with -Recurse only
		{"rm_recurse", "Remove-Item -Recurse ./build", "Note: may recursively remove files"},
		{"ri_recurse", "ri -Recurse ./dist", "Note: may recursively remove files"},
		// Remove-Item with -Force only
		{"rm_force", "rm -Force ./locked.txt", "Note: may force-remove files"},
		{"rmdir_force", "rmdir -Force ./locked", "Note: may force-remove files"},
		// Clear-Content on broad paths
		{"clear_content_wildcard", "Clear-Content *.log", "Note: may clear content of multiple files"},
		// Format-Volume
		{"format_volume", "Format-Volume -DriveLetter D", "Note: may format a disk volume"},
		// Clear-Disk
		{"clear_disk", "Clear-Disk -Number 1", "Note: may clear a disk"},
		// Git destructive ops
		{"git_reset_hard", "git reset --hard HEAD~1", "Note: may discard uncommitted changes"},
		{"git_push_force", "git push --force origin main", "Note: may overwrite remote history"},
		{"git_push_force_lease", "git push --force-with-lease origin main", "Note: may overwrite remote history"},
		{"git_push_f", "git push -f origin main", "Note: may overwrite remote history"},
		{"git_clean_f", "git clean -fd", "Note: may permanently delete untracked files"},
		{"git_stash_drop", "git stash drop", "Note: may permanently remove stashed changes"},
		{"git_stash_clear", "git stash clear", "Note: may permanently remove stashed changes"},
		// Database
		{"drop_table", "DROP TABLE users", "Note: may drop or truncate database objects"},
		{"truncate_table", "TRUNCATE TABLE logs", "Note: may drop or truncate database objects"},
		// System
		{"stop_computer", "Stop-Computer", "Note: will shut down the computer"},
		{"restart_computer", "Restart-Computer", "Note: will restart the computer"},
		{"clear_recyclebin", "Clear-RecycleBin", "Note: permanently deletes recycled files"},
		// Safe commands: no warning
		{"safe_get_childitem", "Get-ChildItem .", ""},
		{"safe_git_status", "git status", ""},
		{"safe_git_log", "git log --oneline", ""},
		// git clean with dry-run should NOT warn
		{"git_clean_dry_run", "git clean -fdn", ""},
		{"git_clean_dry_run_long", "git clean -fd --dry-run", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tools.GetPSDestructiveCommandWarning(tt.command)
			if got != tt.want {
				t.Errorf("GetPSDestructiveCommandWarning(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

// TestPSDestructivePatternAnchoring verifies statement-start anchoring.
// `git rm --force` should NOT match Remove-Item patterns.
// Source: destructiveCommandWarning.ts:14-20 — anchoring commentary
func TestPSDestructivePatternAnchoring(t *testing.T) {
	// `git rm --force` should NOT match the rm -Force pattern (anchored to statement start)
	warning := tools.GetPSDestructiveCommandWarning("git rm --force foo.txt")
	if strings.Contains(warning, "force-remove") {
		t.Errorf("git rm --force should NOT match Remove-Item pattern, got: %q", warning)
	}

	// After semicolon should match
	warning = tools.GetPSDestructiveCommandWarning("echo hi; rm -Force foo.txt")
	if warning == "" {
		t.Error("rm -Force after semicolon should match")
	}

	// After pipe should match
	warning = tools.GetPSDestructiveCommandWarning("Get-ChildItem | rm -Recurse -Force")
	if warning == "" {
		t.Error("rm -Recurse -Force after pipe should match")
	}

	// Inside scriptblock { rm -Force } should match
	warning = tools.GetPSDestructiveCommandWarning("if ($true) { rm -Force foo }")
	if warning == "" {
		t.Error("rm -Force inside scriptblock should match")
	}
}

// TestPSGitSafetyPatterns verifies git safety patterns.
// Source: destructiveCommandWarning.ts:58-75
func TestPSGitSafetyPatterns(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantWarn bool
	}{
		{"reset_hard", "git reset --hard", true},
		{"push_force", "git push --force origin main", true},
		{"push_f", "git push -f origin main", true},
		{"push_force_with_lease", "git push --force-with-lease", true},
		{"clean_force", "git clean -f", true},
		{"clean_fd", "git clean -fd", true},
		{"stash_drop", "git stash drop", true},
		{"stash_clear", "git stash clear", true},
		// safe git commands
		{"status", "git status", false},
		{"log", "git log", false},
		{"diff", "git diff", false},
		{"push_normal", "git push origin main", false},
		{"clean_dry", "git clean -fd --dry-run", false},
		{"clean_n", "git clean -fdn", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warning := tools.GetPSDestructiveCommandWarning(tt.command)
			if tt.wantWarn && warning == "" {
				t.Errorf("expected warning for %q, got none", tt.command)
			}
			if !tt.wantWarn && warning != "" {
				t.Errorf("expected no warning for %q, got %q", tt.command, warning)
			}
		})
	}
}

// TestPSReadOnlyCommand verifies read-only command classification.
// Source: readOnlyValidation.ts — isReadOnlyCommand()
func TestPSReadOnlyCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		readOnly bool
	}{
		// Read-only cmdlets
		{"get_childitem", "Get-ChildItem .", true},
		{"get_content", "Get-Content file.txt", true},
		{"test_path", "Test-Path ./foo", true},
		{"select_string", "Select-String -Pattern foo *.txt", true},
		{"format_table", "Get-Process | Format-Table", true},
		{"write_output", "Write-Output hello", true},
		{"get_help", "Get-Help Get-Process", true},
		// Aliases resolve to read-only
		{"ls_alias", "ls", true},
		{"dir_alias", "dir", true},
		{"cat_alias", "cat foo.txt", true},
		{"sls_alias", "sls foo *.txt", true},
		{"pwd_alias", "pwd", true},
		// Git read-only
		{"git_status", "git status", true},
		{"git_log", "git log", true},
		{"git_diff", "git diff", true},
		// Write commands
		{"remove_item", "Remove-Item foo.txt", false},
		{"rm", "rm foo.txt", false},
		{"set_content", "Set-Content -Path foo.txt -Value bar", false},
		{"new_item", "New-Item -Path foo -ItemType Directory", false},
		{"invoke_expression", "Invoke-Expression $cmd", false},
		// Git write commands
		{"git_push", "git push", false},
		{"git_commit", "git commit -m msg", false},
		{"git_reset", "git reset --hard", false},
		// Multiple statements — all must be read-only
		{"multi_read", "Get-ChildItem; Get-Content foo.txt", true},
		{"multi_mixed", "Get-ChildItem; Remove-Item foo.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tools.IsPSReadOnlyCommand(tt.command)
			if got != tt.readOnly {
				t.Errorf("IsPSReadOnlyCommand(%q) = %v, want %v", tt.command, got, tt.readOnly)
			}
		})
	}
}

// TestPSCmdletAliasResolution verifies alias to canonical cmdlet resolution.
// Source: readOnlyValidation.ts — resolveToCanonical()
func TestPSCmdletAliasResolution(t *testing.T) {
	tests := []struct {
		alias     string
		canonical string
	}{
		{"dir", "get-childitem"},
		{"ls", "get-childitem"},
		{"gci", "get-childitem"},
		{"cat", "get-content"},
		{"cd", "set-location"},
		{"rm", "remove-item"},
		{"del", "remove-item"},
		{"cp", "copy-item"},
		{"mv", "move-item"},
		{"echo", "write-output"},
		{"pwd", "get-location"},
		{"iex", "invoke-expression"},
		{"sls", "select-string"},
		{"iwr", "invoke-webrequest"},
		{"wget", "invoke-webrequest"},
		{"curl", "invoke-webrequest"},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			got := tools.ResolvePSToCanonical(tt.alias)
			if got != tt.canonical {
				t.Errorf("ResolvePSToCanonical(%q) = %q, want %q", tt.alias, got, tt.canonical)
			}
		})
	}
}

// TestCLMAllowedTypes verifies the CLM type allowlist.
// Source: clmTypes.ts:18-188
func TestCLMAllowedTypes(t *testing.T) {
	t.Run("allowed_short_names", func(t *testing.T) {
		allowed := []string{
			"string", "int", "bool", "datetime", "guid", "hashtable",
			"array", "regex", "uri", "xml", "void", "double", "float",
			"ipaddress", "pscustomobject", "securestring",
		}
		for _, name := range allowed {
			if !tools.IsClmAllowedType(name) {
				t.Errorf("expected %q to be CLM-allowed", name)
			}
		}
	})

	t.Run("allowed_fq_names", func(t *testing.T) {
		allowed := []string{
			"System.String", "System.Int32", "System.Boolean",
			"System.Collections.Hashtable",
			"System.Management.Automation.PSCustomObject",
			"Microsoft.Management.Infrastructure.CimInstance",
		}
		for _, name := range allowed {
			if !tools.IsClmAllowedType(name) {
				t.Errorf("expected %q to be CLM-allowed", name)
			}
		}
	})

	t.Run("removed_types_blocked", func(t *testing.T) {
		blocked := []string{
			"adsi", "adsisearcher", "wmi", "wmiclass", "wmisearcher", "cimsession",
		}
		for _, name := range blocked {
			if tools.IsClmAllowedType(name) {
				t.Errorf("expected %q to be CLM-BLOCKED (security: network bind)", name)
			}
		}
	})

	t.Run("unknown_types_blocked", func(t *testing.T) {
		unknown := []string{
			"System.Diagnostics.Process",
			"System.Runtime.InteropServices.Marshal",
			"System.Net.Sockets.TcpClient",
			"SomeCustomType",
		}
		for _, name := range unknown {
			if tools.IsClmAllowedType(name) {
				t.Errorf("expected unknown type %q to be CLM-blocked", name)
			}
		}
	})

	t.Run("array_suffix_stripped", func(t *testing.T) {
		if !tools.IsClmAllowedType("string[]") {
			t.Error("string[] should be allowed (array of allowed type)")
		}
		if !tools.IsClmAllowedType("Int32[]") {
			t.Error("Int32[] should be allowed")
		}
	})

	t.Run("generic_stripped", func(t *testing.T) {
		// "List[int]" should check "list" which is NOT in the set
		if tools.IsClmAllowedType("List[int]") {
			t.Error("List[int] should NOT be allowed (generic wrapper not in allowlist)")
		}
		// "hashtable" is allowed
		if !tools.IsClmAllowedType("Hashtable") {
			t.Error("Hashtable should be allowed")
		}
	})
}

// TestNormalizePSTypeName verifies type name normalization.
// Source: clmTypes.ts:194-203
func TestNormalizePSTypeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"String", "string"},
		{"string[]", "string"},
		{"Int32[]", "int32"},
		{"List[int]", "list"},
		{"System.Collections.Generic.Dictionary[string,int]", "system.collections.generic.dictionary"},
		{"  GUID  ", "guid"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := tools.NormalizePSTypeName(tt.input)
			if got != tt.want {
				t.Errorf("NormalizePSTypeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestPSCommonParameters verifies common parameter set.
// Source: commonParameters.ts:12-30
func TestPSCommonParameters(t *testing.T) {
	expected := []string{
		"-verbose", "-debug",
		"-erroraction", "-warningaction", "-informationaction", "-progressaction",
		"-errorvariable", "-warningvariable", "-informationvariable",
		"-outvariable", "-outbuffer", "-pipelinevariable",
	}
	for _, p := range expected {
		if !tools.IsPSCommonParameter(p) {
			t.Errorf("expected %q to be a common parameter", p)
		}
	}

	// Non-common parameters
	notCommon := []string{"-Path", "-Force", "-Recurse", "-Name"}
	for _, p := range notCommon {
		if tools.IsPSCommonParameter(p) {
			t.Errorf("expected %q to NOT be a common parameter", p)
		}
	}
}

// TestPSCommandSemantics verifies per-command exit code interpretation.
// Source: commandSemantics.ts
func TestPSCommandSemantics(t *testing.T) {
	t.Run("grep_no_match", func(t *testing.T) {
		sem := tools.InterpretPSCommandResult("findstr foo bar.txt", 1)
		if sem.IsError {
			t.Error("findstr exit 1 should not be error (no matches)")
		}
		if sem.Message != "No matches found" {
			t.Errorf("expected 'No matches found', got %q", sem.Message)
		}
	})

	t.Run("robocopy_sync", func(t *testing.T) {
		sem := tools.InterpretPSCommandResult("robocopy src dst", 0)
		if sem.IsError {
			t.Error("robocopy exit 0 should not be error")
		}
		if sem.Message != "No files copied (already in sync)" {
			t.Errorf("expected 'No files copied (already in sync)', got %q", sem.Message)
		}
	})

	t.Run("robocopy_copied", func(t *testing.T) {
		sem := tools.InterpretPSCommandResult("robocopy src dst", 1)
		if sem.IsError {
			t.Error("robocopy exit 1 should not be error")
		}
		if sem.Message != "Files copied successfully" {
			t.Errorf("expected 'Files copied successfully', got %q", sem.Message)
		}
	})

	t.Run("robocopy_no_error", func(t *testing.T) {
		sem := tools.InterpretPSCommandResult("robocopy src dst", 2)
		if sem.IsError {
			t.Error("robocopy exit 2 should not be error")
		}
		if sem.Message != "Robocopy completed (no errors)" {
			t.Errorf("expected 'Robocopy completed (no errors)', got %q", sem.Message)
		}
	})

	t.Run("robocopy_error", func(t *testing.T) {
		sem := tools.InterpretPSCommandResult("robocopy src dst", 8)
		if !sem.IsError {
			t.Error("robocopy exit 8 should be error")
		}
	})

	t.Run("default_nonzero_error", func(t *testing.T) {
		sem := tools.InterpretPSCommandResult("some-command", 1)
		if !sem.IsError {
			t.Error("default: non-zero exit should be error")
		}
	})

	t.Run("default_zero_success", func(t *testing.T) {
		sem := tools.InterpretPSCommandResult("some-command", 0)
		if sem.IsError {
			t.Error("default: zero exit should not be error")
		}
	})
}

// TestIsSearchOrReadPSCommand verifies search/read classification.
// Source: PowerShellTool.tsx:54-95
func TestIsSearchOrReadPSCommand(t *testing.T) {
	t.Run("search_commands", func(t *testing.T) {
		cmds := []string{"Select-String foo", "findstr /s foo *.txt"}
		for _, cmd := range cmds {
			isSearch, _ := tools.IsSearchOrReadPSCommand(cmd)
			if !isSearch {
				t.Errorf("expected %q to be search command", cmd)
			}
		}
	})

	t.Run("read_commands", func(t *testing.T) {
		cmds := []string{"Get-Content foo.txt", "Test-Path ./bar", "Get-Process", "Format-Hex file.bin"}
		for _, cmd := range cmds {
			_, isRead := tools.IsSearchOrReadPSCommand(cmd)
			if !isRead {
				t.Errorf("expected %q to be read command", cmd)
			}
		}
	})

	t.Run("neither", func(t *testing.T) {
		isSearch, isRead := tools.IsSearchOrReadPSCommand("Remove-Item foo.txt")
		if isSearch || isRead {
			t.Error("Remove-Item should be neither search nor read")
		}
	})
}

// TestWindowsSandboxPolicyRefusal verifies the verbatim enterprise policy message.
// Source: PowerShellTool.tsx:49-50
func TestWindowsSandboxPolicyRefusal(t *testing.T) {
	expected := "Enterprise policy requires sandboxing, but sandboxing is not available on native Windows. Shell command execution is blocked on this platform by policy."
	if tools.WindowsSandboxPolicyRefusal != expected {
		t.Errorf("WindowsSandboxPolicyRefusal mismatch:\ngot:  %q\nwant: %q", tools.WindowsSandboxPolicyRefusal, expected)
	}
}

// TestPSConstants verifies numeric constants match TS source.
func TestPSConstants(t *testing.T) {
	if tools.DefaultPowerShellTimeoutMs != 120_000 {
		t.Errorf("DefaultPowerShellTimeoutMs = %d, want 120000", tools.DefaultPowerShellTimeoutMs)
	}
	if tools.MaxPowerShellTimeoutMs != 600_000 {
		t.Errorf("MaxPowerShellTimeoutMs = %d, want 600000", tools.MaxPowerShellTimeoutMs)
	}
	if tools.PSAssistantBlockingBudgetMs != 15_000 {
		t.Errorf("PSAssistantBlockingBudgetMs = %d, want 15000", tools.PSAssistantBlockingBudgetMs)
	}
	if tools.PSProgressThresholdMs != 2000 {
		t.Errorf("PSProgressThresholdMs = %d, want 2000", tools.PSProgressThresholdMs)
	}
	if tools.PSProgressIntervalMs != 1000 {
		t.Errorf("PSProgressIntervalMs = %d, want 1000", tools.PSProgressIntervalMs)
	}
}

// TestValidatePSCommand verifies the validation pipeline.
func TestValidatePSCommand(t *testing.T) {
	t.Run("plan_mode_rejects_write", func(t *testing.T) {
		out := tools.ValidatePSCommand("Remove-Item foo.txt", "", "", true)
		if out == nil {
			t.Fatal("expected error for write command in plan mode")
		}
		if !strings.Contains(out.Content, "plan mode") {
			t.Errorf("expected plan mode error, got: %s", out.Content)
		}
	})

	t.Run("plan_mode_allows_read", func(t *testing.T) {
		out := tools.ValidatePSCommand("Get-ChildItem", "", "", true)
		if out != nil {
			t.Errorf("expected nil for read command in plan mode, got: %s", out.Content)
		}
	})

	t.Run("normal_mode_allows_write", func(t *testing.T) {
		out := tools.ValidatePSCommand("Remove-Item foo.txt", "", "", false)
		if out != nil {
			t.Errorf("expected nil for write command in normal mode, got: %s", out.Content)
		}
	})
}

// TestCLMRemovedTypes verifies the explicit removed types list.
func TestCLMRemovedTypes(t *testing.T) {
	for name := range tools.CLMRemovedTypes {
		if tools.IsClmAllowedType(name) {
			t.Errorf("removed type %q should NOT be in CLM allowlist", name)
		}
	}
}
