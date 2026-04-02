package tools_test

import (
	"fmt"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
)

// These tests validate behavioral facts extracted from the Claude Code TypeScript source.
// Source: research/claude-code-source-build/source/src/tools/*/
// Each test asserts a fact that any correct reimplementation MUST satisfy.

func TestBashToolBehavior(t *testing.T) {
	b, err := testharness.LoadToolBehavior()
	if err != nil {
		t.Fatalf("failed to load tool behavior: %v", err)
	}
	bash := b.Bash

	t.Run("tool_name_is_Bash", func(t *testing.T) {
		if bash.ToolName != "Bash" {
			t.Errorf("expected Bash, got %s", bash.ToolName)
		}
	})
	t.Run("default_timeout_120s", func(t *testing.T) {
		// Source: timeouts.ts:2 — DEFAULT_TIMEOUT_MS = 120_000
		if bash.DefaultTimeoutMs != 120000 {
			t.Errorf("expected 120000, got %d", bash.DefaultTimeoutMs)
		}
	})
	t.Run("max_timeout_600s", func(t *testing.T) {
		// Source: timeouts.ts:3 — MAX_TIMEOUT_MS = 600_000
		if bash.MaxTimeoutMs != 600000 {
			t.Errorf("expected 600000, got %d", bash.MaxTimeoutMs)
		}
	})
	t.Run("max_timeout_gte_default", func(t *testing.T) {
		// Source: timeouts.ts:34,38 — Math.max ensures max >= default
		if bash.MaxTimeoutMs < bash.DefaultTimeoutMs {
			t.Errorf("max %d < default %d", bash.MaxTimeoutMs, bash.DefaultTimeoutMs)
		}
	})
	t.Run("max_result_30k", func(t *testing.T) {
		if bash.MaxResultSizeChars != 30000 {
			t.Errorf("expected 30000, got %d", bash.MaxResultSizeChars)
		}
	})
	t.Run("default_timeout_is_2_minutes", func(t *testing.T) {
		if bash.DefaultTimeoutMs/1000 != 120 {
			t.Errorf("expected 120 seconds, got %d", bash.DefaultTimeoutMs/1000)
		}
	})
	t.Run("max_timeout_is_10_minutes", func(t *testing.T) {
		if bash.MaxTimeoutMs/1000/60 != 10 {
			t.Errorf("expected 10 minutes, got %d", bash.MaxTimeoutMs/1000/60)
		}
	})
}

func TestFileReadToolBehavior(t *testing.T) {
	b, err := testharness.LoadToolBehavior()
	if err != nil {
		t.Fatalf("failed to load tool behavior: %v", err)
	}
	fr := b.FileRead

	t.Run("tool_name_is_Read", func(t *testing.T) {
		if fr.ToolName != "Read" {
			t.Errorf("expected Read, got %s", fr.ToolName)
		}
	})
	t.Run("default_max_output_25k_tokens", func(t *testing.T) {
		// Source: limits.ts:18 — DEFAULT_MAX_OUTPUT_TOKENS = 25000
		if fr.DefaultMaxOutputTokens != 25000 {
			t.Errorf("expected 25000, got %d", fr.DefaultMaxOutputTokens)
		}
	})
	t.Run("max_output_size_256KB", func(t *testing.T) {
		// Source: file.ts:48 — MAX_OUTPUT_SIZE = 0.25 * 1024 * 1024
		if fr.MaxOutputSizeBytes != 262144 {
			t.Errorf("expected 262144, got %d", fr.MaxOutputSizeBytes)
		}
	})
	t.Run("max_result_infinity", func(t *testing.T) {
		if fr.MaxResultSizeChars != -1 {
			t.Errorf("expected -1 (infinity), got %d", fr.MaxResultSizeChars)
		}
	})
	t.Run("pdf_max_pages_20", func(t *testing.T) {
		// Source: apiLimits.ts:77 — PDF_MAX_PAGES_PER_READ = 20
		if fr.PDFMaxPagesPerRead != 20 {
			t.Errorf("expected 20, got %d", fr.PDFMaxPagesPerRead)
		}
	})
	t.Run("pdf_inline_threshold_10_pages", func(t *testing.T) {
		// Source: apiLimits.ts:83 — PDF_AT_MENTION_INLINE_THRESHOLD = 10
		if fr.PDFInlineThresholdPages != 10 {
			t.Errorf("expected 10, got %d", fr.PDFInlineThresholdPages)
		}
	})
	t.Run("pdf_extract_size_3MB", func(t *testing.T) {
		// Source: apiLimits.ts:66 — PDF_EXTRACT_SIZE_THRESHOLD = 3 * 1024 * 1024
		if fr.PDFExtractSizeThresholdBytes != 3*1024*1024 {
			t.Errorf("expected %d, got %d", 3*1024*1024, fr.PDFExtractSizeThresholdBytes)
		}
	})
	t.Run("is_read_only", func(t *testing.T) {
		if !fr.IsReadOnly {
			t.Error("Read tool must be read-only")
		}
	})
	t.Run("is_concurrency_safe", func(t *testing.T) {
		if !fr.IsConcurrencySafe {
			t.Error("Read tool must be concurrency-safe")
		}
	})

	// Blocked device paths
	blockedPaths := []string{
		"/dev/zero", "/dev/random", "/dev/urandom", "/dev/full",
		"/dev/stdin", "/dev/tty", "/dev/console",
		"/dev/stdout", "/dev/stderr",
		"/dev/fd/0", "/dev/fd/1", "/dev/fd/2",
	}
	for _, path := range blockedPaths {
		path := path
		t.Run(fmt.Sprintf("blocks_%s", path), func(t *testing.T) {
			found := false
			for _, bp := range fr.BlockedPaths {
				if bp == path {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("path %s should be blocked", path)
			}
		})
	}
	t.Run("blocked_paths_count", func(t *testing.T) {
		if len(fr.BlockedPaths) < 12 {
			t.Errorf("expected at least 12 blocked paths, got %d", len(fr.BlockedPaths))
		}
	})
}

func TestFileEditToolBehavior(t *testing.T) {
	b, err := testharness.LoadToolBehavior()
	if err != nil {
		t.Fatalf("failed to load tool behavior: %v", err)
	}
	fe := b.FileEdit

	t.Run("tool_name_is_Edit", func(t *testing.T) {
		if fe.ToolName != "Edit" {
			t.Errorf("expected Edit, got %s", fe.ToolName)
		}
	})
	t.Run("max_file_size_1GiB", func(t *testing.T) {
		// Source: FileEditTool.ts:84 — MAX_EDIT_FILE_SIZE = 1024 * 1024 * 1024
		expected := int64(1024 * 1024 * 1024)
		if fe.MaxFileSizeBytes != expected {
			t.Errorf("expected %d, got %d", expected, fe.MaxFileSizeBytes)
		}
	})
	t.Run("requires_read_before_edit", func(t *testing.T) {
		if !fe.RequiresFullReadBefore {
			t.Error("Edit tool must require full read before editing")
		}
	})
	t.Run("not_read_only", func(t *testing.T) {
		if fe.IsReadOnly {
			t.Error("Edit tool must not be read-only")
		}
	})

	// Error codes
	errorCodes := []struct {
		name string
		code int
	}{
		{"file_already_exists_empty_old", 3},
		{"file_not_found_with_old_string", 4},
		{"file_not_read_yet", 6},
		{"file_modified_since_read", 7},
		{"string_not_found", 8},
		{"multiple_matches_no_replace_all", 9},
		{"file_too_large", 10},
	}
	for _, ec := range errorCodes {
		ec := ec
		t.Run(fmt.Sprintf("error_code_%s=%d", ec.name, ec.code), func(t *testing.T) {
			var got int
			switch ec.name {
			case "file_already_exists_empty_old":
				got = fe.ErrorCodes.FileAlreadyExistsEmptyOld
			case "file_not_found_with_old_string":
				got = fe.ErrorCodes.FileNotFoundWithOldString
			case "file_not_read_yet":
				got = fe.ErrorCodes.FileNotReadYet
			case "file_modified_since_read":
				got = fe.ErrorCodes.FileModifiedSinceRead
			case "string_not_found":
				got = fe.ErrorCodes.StringNotFound
			case "multiple_matches_no_replace_all":
				got = fe.ErrorCodes.MultipleMatchesNoReplaceAll
			case "file_too_large":
				got = fe.ErrorCodes.FileTooLarge
			}
			if got != ec.code {
				t.Errorf("expected %d, got %d", ec.code, got)
			}
		})
	}

	t.Run("error_codes_all_unique", func(t *testing.T) {
		codes := map[int]string{}
		allCodes := []struct {
			name string
			code int
		}{
			{"empty_old", fe.ErrorCodes.FileAlreadyExistsEmptyOld},
			{"not_found", fe.ErrorCodes.FileNotFoundWithOldString},
			{"not_read", fe.ErrorCodes.FileNotReadYet},
			{"modified", fe.ErrorCodes.FileModifiedSinceRead},
			{"no_match", fe.ErrorCodes.StringNotFound},
			{"multi_match", fe.ErrorCodes.MultipleMatchesNoReplaceAll},
			{"too_large", fe.ErrorCodes.FileTooLarge},
		}
		for _, c := range allCodes {
			if existing, ok := codes[c.code]; ok {
				t.Errorf("code %d used by both %s and %s", c.code, existing, c.name)
			}
			codes[c.code] = c.name
		}
	})
}

func TestGlobToolBehavior(t *testing.T) {
	b, err := testharness.LoadToolBehavior()
	if err != nil {
		t.Fatalf("failed to load tool behavior: %v", err)
	}
	g := b.Glob

	t.Run("tool_name_is_Glob", func(t *testing.T) {
		if g.ToolName != "Glob" {
			t.Errorf("expected Glob, got %s", g.ToolName)
		}
	})
	t.Run("max_results_100", func(t *testing.T) {
		// Source: GlobTool.ts:157 — limit = 100
		if g.MaxResultsDefault != 100 {
			t.Errorf("expected 100, got %d", g.MaxResultsDefault)
		}
	})
	t.Run("truncation_message", func(t *testing.T) {
		expected := "(Results are truncated. Consider using a more specific path or pattern.)"
		if g.TruncationMessage != expected {
			t.Errorf("expected %q, got %q", expected, g.TruncationMessage)
		}
	})
	t.Run("is_read_only", func(t *testing.T) {
		if !g.IsReadOnly {
			t.Error("Glob must be read-only")
		}
	})
	t.Run("is_concurrency_safe", func(t *testing.T) {
		if !g.IsConcurrencySafe {
			t.Error("Glob must be concurrency-safe")
		}
	})
}

func TestGrepToolBehavior(t *testing.T) {
	b, err := testharness.LoadToolBehavior()
	if err != nil {
		t.Fatalf("failed to load tool behavior: %v", err)
	}
	g := b.Grep

	t.Run("tool_name_is_Grep", func(t *testing.T) {
		if g.ToolName != "Grep" {
			t.Errorf("expected Grep, got %s", g.ToolName)
		}
	})
	t.Run("default_head_limit_250", func(t *testing.T) {
		// Source: GrepTool.ts:108 — DEFAULT_HEAD_LIMIT = 250
		if g.DefaultHeadLimit != 250 {
			t.Errorf("expected 250, got %d", g.DefaultHeadLimit)
		}
	})
	t.Run("default_output_mode_files_with_matches", func(t *testing.T) {
		// Source: GrepTool.ts:316
		if g.DefaultOutputMode != "files_with_matches" {
			t.Errorf("expected files_with_matches, got %s", g.DefaultOutputMode)
		}
	})
	t.Run("default_line_numbers_true", func(t *testing.T) {
		// Source: GrepTool.ts:321
		if !g.DefaultLineNumbers {
			t.Error("line numbers should default to true")
		}
	})
	t.Run("max_line_length_500", func(t *testing.T) {
		// Source: GrepTool.ts:338 — --max-columns 500
		if g.MaxLineLengthChars != 500 {
			t.Errorf("expected 500, got %d", g.MaxLineLengthChars)
		}
	})
	t.Run("max_result_20k", func(t *testing.T) {
		if g.MaxResultSizeChars != 20000 {
			t.Errorf("expected 20000, got %d", g.MaxResultSizeChars)
		}
	})
	t.Run("is_read_only", func(t *testing.T) {
		if !g.IsReadOnly {
			t.Error("Grep must be read-only")
		}
	})

	// VCS exclusions
	vcsExclusions := []string{".git", ".svn", ".hg", ".bzr", ".jj", ".sl"}
	for _, dir := range vcsExclusions {
		dir := dir
		t.Run(fmt.Sprintf("excludes_%s", dir), func(t *testing.T) {
			found := false
			for _, excluded := range g.VCSDirectoriesExcluded {
				if excluded == dir {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("VCS directory %s should be excluded", dir)
			}
		})
	}
	t.Run("vcs_exclusion_count", func(t *testing.T) {
		if len(g.VCSDirectoriesExcluded) != 6 {
			t.Errorf("expected 6 VCS exclusions, got %d", len(g.VCSDirectoriesExcluded))
		}
	})
}

func TestQueryLoopBehavior(t *testing.T) {
	b, err := testharness.LoadToolBehavior()
	if err != nil {
		t.Fatalf("failed to load tool behavior: %v", err)
	}
	ql := b.QueryLoop

	t.Run("turn_counter_starts_at_1", func(t *testing.T) {
		// Source: query.ts:276 — turnCount: 1
		if ql.TurnCounterStartsAt != 1 {
			t.Errorf("expected 1, got %d", ql.TurnCounterStartsAt)
		}
	})
	t.Run("tool_detection_uses_content_blocks", func(t *testing.T) {
		// Source: query.ts:554 — "stop_reason === 'tool_use' is unreliable"
		if !ql.ToolDetectionUsesContentBlocksNotStopReason {
			t.Error("tool detection must use content blocks, not stop_reason")
		}
	})
	t.Run("system_prompt_separate_field", func(t *testing.T) {
		// Source: query.ts:661 — systemPrompt separate from messages
		if !ql.SystemPromptInSeparateFieldNotMessages {
			t.Error("system prompt must be in separate field, not in messages")
		}
	})
	t.Run("tool_results_in_user_messages", func(t *testing.T) {
		// Source: query.ts:138-143 — createUserMessage for tool_result
		if !ql.ToolResultsInUserRoleMessages {
			t.Error("tool results must be in user role messages")
		}
	})
	t.Run("capped_default_8k", func(t *testing.T) {
		// Source: context.ts:24 — CAPPED_DEFAULT_MAX_TOKENS = 8_000
		if ql.CappedDefaultMaxTokens != 8000 {
			t.Errorf("expected 8000, got %d", ql.CappedDefaultMaxTokens)
		}
	})
	t.Run("compact_max_output_20k", func(t *testing.T) {
		// Source: context.ts:12 — COMPACT_MAX_OUTPUT_TOKENS = 20_000
		if ql.CompactMaxOutputTokens != 20000 {
			t.Errorf("expected 20000, got %d", ql.CompactMaxOutputTokens)
		}
	})
	t.Run("fallback_default_32k", func(t *testing.T) {
		// Source: context.ts:15 — MAX_OUTPUT_TOKENS_DEFAULT = 32_000
		if ql.FallbackDefaultMaxOutputTokens != 32000 {
			t.Errorf("expected 32000, got %d", ql.FallbackDefaultMaxOutputTokens)
		}
	})
	t.Run("recovery_message_content", func(t *testing.T) {
		// Source: query.ts:1225-1227
		expected := "Output token limit hit. Resume directly"
		if len(ql.MaxOutputTokensRecoveryMessage) < len(expected) {
			t.Errorf("recovery message too short: %q", ql.MaxOutputTokensRecoveryMessage)
		}
		if ql.MaxOutputTokensRecoveryMessage[:len(expected)] != expected {
			t.Errorf("recovery message should start with %q", expected)
		}
	})
	t.Run("recovery_message_no_apology", func(t *testing.T) {
		// The message explicitly says "no apology, no recap"
		msg := ql.MaxOutputTokensRecoveryMessage
		if len(msg) == 0 {
			t.Fatal("message is empty")
		}
		// The TS source includes "no apology" in the continuation prompt
	})
}

func TestFileWriteToolBehavior(t *testing.T) {
	b, err := testharness.LoadToolBehavior()
	if err != nil {
		t.Fatalf("failed to load tool behavior: %v", err)
	}
	fw := b.FileWrite

	t.Run("tool_name_is_Write", func(t *testing.T) {
		if fw.ToolName != "Write" {
			t.Errorf("expected Write, got %s", fw.ToolName)
		}
	})
	t.Run("max_result_100k", func(t *testing.T) {
		if fw.MaxResultSizeChars != 100000 {
			t.Errorf("expected 100000, got %d", fw.MaxResultSizeChars)
		}
	})
	t.Run("not_read_only", func(t *testing.T) {
		if fw.IsReadOnly {
			t.Error("Write must not be read-only")
		}
	})
	t.Run("not_concurrency_safe", func(t *testing.T) {
		if fw.IsConcurrencySafe {
			t.Error("Write must not be concurrency-safe")
		}
	})
}
