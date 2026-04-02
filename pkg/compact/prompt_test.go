package compact

import (
	"strings"
	"testing"
)

// Source: services/compact/prompt.ts

func TestGetCompactPrompt(t *testing.T) {
	// Source: services/compact/prompt.ts:293-303

	t.Run("without_instructions", func(t *testing.T) {
		prompt := GetCompactPrompt("")
		// Source: services/compact/prompt.ts:19 — starts with CRITICAL
		if !strings.HasPrefix(prompt, "CRITICAL: Respond with TEXT ONLY") {
			t.Error("should start with no-tools preamble")
		}
		// Source: services/compact/prompt.ts:269-272 — ends with REMINDER
		if !strings.HasSuffix(prompt, "Tool calls will be rejected and you will fail the task.") {
			t.Error("should end with no-tools trailer")
		}
		// Should contain the base prompt
		if !strings.Contains(prompt, "Primary Request and Intent") {
			t.Error("should contain base compact prompt sections")
		}
		// Should NOT contain custom instructions marker
		if strings.Contains(prompt, "Additional Instructions:") {
			t.Error("should not contain additional instructions when empty")
		}
	})

	t.Run("with_instructions", func(t *testing.T) {
		// Source: services/compact/prompt.ts:296-298
		prompt := GetCompactPrompt("Focus on typescript changes")
		if !strings.Contains(prompt, "Additional Instructions:\nFocus on typescript changes") {
			t.Error("should include custom instructions")
		}
		// Trailer should still be present after instructions
		if !strings.Contains(prompt, "REMINDER: Do NOT call any tools") {
			t.Error("should still have no-tools trailer after instructions")
		}
	})

	t.Run("whitespace_instructions_ignored", func(t *testing.T) {
		prompt := GetCompactPrompt("   ")
		if strings.Contains(prompt, "Additional Instructions:") {
			t.Error("whitespace-only instructions should be ignored")
		}
	})
}

func TestFormatCompactSummary(t *testing.T) {
	// Source: services/compact/prompt.ts:311-335

	t.Run("strips_analysis_block", func(t *testing.T) {
		// Source: services/compact/prompt.ts:316-319
		input := "<analysis>\nThinking about stuff...\n</analysis>\n\n<summary>\n1. Primary Request\n</summary>"
		result := FormatCompactSummary(input)
		if strings.Contains(result, "<analysis>") {
			t.Error("analysis block should be stripped")
		}
		if strings.Contains(result, "Thinking about stuff") {
			t.Error("analysis content should be stripped")
		}
	})

	t.Run("formats_summary_section", func(t *testing.T) {
		// Source: services/compact/prompt.ts:322-329
		input := "<summary>\n1. Primary Request: Build a CLI\n</summary>"
		result := FormatCompactSummary(input)
		if !strings.HasPrefix(result, "Summary:\n") {
			t.Errorf("should start with 'Summary:', got %q", result[:20])
		}
		if strings.Contains(result, "<summary>") {
			t.Error("summary tags should be removed")
		}
		if !strings.Contains(result, "1. Primary Request: Build a CLI") {
			t.Error("summary content should be preserved")
		}
	})

	t.Run("collapses_multiple_newlines", func(t *testing.T) {
		// Source: services/compact/prompt.ts:332
		input := "line1\n\n\n\nline2"
		result := FormatCompactSummary(input)
		if strings.Contains(result, "\n\n\n") {
			t.Error("should collapse multiple newlines to double")
		}
	})

	t.Run("full_pipeline", func(t *testing.T) {
		input := `<analysis>
Let me think about this...
I need to consider the user's request.
</analysis>

<summary>
1. Primary Request and Intent:
   Build a Go CLI tool

2. Key Technical Concepts:
   - Go modules
   - CLI parsing
</summary>`
		result := FormatCompactSummary(input)
		if strings.Contains(result, "Let me think") {
			t.Error("analysis should be stripped")
		}
		if !strings.Contains(result, "Summary:\n1. Primary Request") {
			t.Error("summary should be formatted")
		}
	})

	t.Run("no_tags_passthrough", func(t *testing.T) {
		input := "Just a plain summary without any XML tags."
		result := FormatCompactSummary(input)
		if result != input {
			t.Errorf("plain text should pass through unchanged, got %q", result)
		}
	})
}

func TestGetCompactUserSummaryMessage(t *testing.T) {
	// Source: services/compact/prompt.ts:337-374

	t.Run("basic_summary", func(t *testing.T) {
		msg := GetCompactUserSummaryMessage("<summary>test</summary>", false, "", false)
		if !strings.Contains(msg, "continued from a previous conversation") {
			t.Error("should contain continuation header")
		}
		if !strings.Contains(msg, "Summary:\ntest") {
			t.Error("should contain formatted summary")
		}
	})

	t.Run("with_transcript_path", func(t *testing.T) {
		// Source: services/compact/prompt.ts:349-351
		msg := GetCompactUserSummaryMessage("test", false, "/tmp/transcript.jsonl", false)
		if !strings.Contains(msg, "read the full transcript at: /tmp/transcript.jsonl") {
			t.Error("should include transcript path")
		}
	})

	t.Run("with_recent_preserved", func(t *testing.T) {
		// Source: services/compact/prompt.ts:353-355
		msg := GetCompactUserSummaryMessage("test", false, "", true)
		if !strings.Contains(msg, "Recent messages are preserved verbatim") {
			t.Error("should note preserved messages")
		}
	})

	t.Run("suppress_follow_up", func(t *testing.T) {
		// Source: services/compact/prompt.ts:357-369
		msg := GetCompactUserSummaryMessage("test", true, "", false)
		if !strings.Contains(msg, "Continue the conversation from where it left off") {
			t.Error("should contain continuation instruction")
		}
		if !strings.Contains(msg, "do not acknowledge the summary") {
			t.Error("should suppress recap")
		}
	})

	t.Run("no_suppress", func(t *testing.T) {
		msg := GetCompactUserSummaryMessage("test", false, "", false)
		if strings.Contains(msg, "Continue the conversation") {
			t.Error("should NOT contain continuation instruction when not suppressed")
		}
	})
}
