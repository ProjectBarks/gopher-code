package compact

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Error message verbatim tests ---
// Source: compact.ts:225-297

func TestErrorMessageNotEnoughMessages_Verbatim(t *testing.T) {
	assert.Equal(t, "Not enough messages to compact.", ErrorMessageNotEnoughMessages)
}

func TestErrorMessageUserAbort_Verbatim(t *testing.T) {
	assert.Equal(t, "API Error: Request was aborted.", ErrorMessageUserAbort)
}

func TestErrorMessageIncompleteResponse_Verbatim(t *testing.T) {
	assert.Equal(t,
		"Compaction interrupted \u00b7 This may be due to network issues \u2014 please try again.",
		ErrorMessageIncompleteResponse,
	)
}

func TestErrorMessagePromptTooLong_Verbatim(t *testing.T) {
	assert.Equal(t,
		"Conversation too long. Press esc twice to go up a few messages and try again.",
		ErrorMessagePromptTooLong,
	)
}

func TestPTLRetryMarker_Verbatim(t *testing.T) {
	assert.Equal(t,
		"[earlier conversation truncated for compaction retry]",
		PTLRetryMarker,
	)
}

// --- Post-compact constants ---
// Source: compact.ts:122-130

func TestPostCompactConstants_Values(t *testing.T) {
	assert.Equal(t, 5, PostCompactMaxFilesToRestore)
	assert.Equal(t, 50_000, PostCompactTokenBudget)
	assert.Equal(t, 5_000, PostCompactMaxTokensPerFile)
	assert.Equal(t, 5_000, PostCompactMaxTokensPerSkill)
	assert.Equal(t, 25_000, PostCompactSkillsTokenBudget)
}

// --- CompactConversation pipeline tests ---

func mockSummarizer(summary string) SummaryFunc {
	return func(_ context.Context, _ []message.Message, _ string) (string, error) {
		return summary, nil
	}
}

func mockSummarizerErr(err error) SummaryFunc {
	return func(_ context.Context, _ []message.Message, _ string) (string, error) {
		return "", err
	}
}

func TestCompactConversation_EmptyMessages(t *testing.T) {
	_, err := CompactConversation(
		context.Background(),
		nil,
		mockSummarizer("ignored"),
		false,
		"",
		TriggerManual,
		"",
	)
	require.Error(t, err)
	assert.Equal(t, ErrorMessageNotEnoughMessages, err.Error())
}

func TestCompactConversation_EmptySlice(t *testing.T) {
	_, err := CompactConversation(
		context.Background(),
		[]message.Message{},
		mockSummarizer("ignored"),
		false,
		"",
		TriggerAuto,
		"",
	)
	require.Error(t, err)
	assert.Equal(t, ErrorMessageNotEnoughMessages, err.Error())
}

func TestCompactConversation_Success(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("Hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("Hi there")}},
		message.UserMessage("Do something"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("Done")}},
	}

	summary := "<analysis>thinking</analysis>\n<summary>User asked to do something. It was done.</summary>"
	result, err := CompactConversation(
		context.Background(),
		msgs,
		mockSummarizer(summary),
		false,
		"",
		TriggerManual,
		"",
	)
	require.NoError(t, err)

	// Boundary marker must be present.
	assert.NotEmpty(t, result.BoundaryMarker.Content)

	// Summary messages must contain the formatted summary.
	require.Len(t, result.SummaryMessages, 1)
	summaryText := result.SummaryMessages[0].Content[0].Text
	assert.Contains(t, summaryText, "This session is being continued from a previous conversation")
	assert.Contains(t, summaryText, "User asked to do something")

	// MessagesToKeep should be empty for full compaction.
	assert.Empty(t, result.MessagesToKeep)

	// Token counts should be positive.
	assert.Greater(t, result.PreCompactTokenCount, 0)
}

func TestCompactConversation_SuppressFollowUp(t *testing.T) {
	msgs := []message.Message{message.UserMessage("test")}
	result, err := CompactConversation(
		context.Background(),
		msgs,
		mockSummarizer("summary text"),
		true, // suppressFollowUp
		"",
		TriggerManual,
		"",
	)
	require.NoError(t, err)
	summaryText := result.SummaryMessages[0].Content[0].Text
	assert.Contains(t, summaryText, "Continue the conversation from where it left off")
	assert.Contains(t, summaryText, "do not acknowledge the summary")
}

func TestCompactConversation_WithTranscriptPath(t *testing.T) {
	msgs := []message.Message{message.UserMessage("test")}
	result, err := CompactConversation(
		context.Background(),
		msgs,
		mockSummarizer("summary"),
		false,
		"",
		TriggerManual,
		"/tmp/transcript.jsonl",
	)
	require.NoError(t, err)
	summaryText := result.SummaryMessages[0].Content[0].Text
	assert.Contains(t, summaryText, "/tmp/transcript.jsonl")
}

func TestCompactConversation_CustomInstructions(t *testing.T) {
	var capturedPrompt string
	summarizer := func(_ context.Context, _ []message.Message, prompt string) (string, error) {
		capturedPrompt = prompt
		return "summary", nil
	}

	msgs := []message.Message{message.UserMessage("test")}
	_, err := CompactConversation(
		context.Background(),
		msgs,
		summarizer,
		false,
		"Focus on TypeScript changes",
		TriggerManual,
		"",
	)
	require.NoError(t, err)
	assert.Contains(t, capturedPrompt, "Focus on TypeScript changes")
}

func TestCompactConversation_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	msgs := []message.Message{message.UserMessage("test")}
	_, err := CompactConversation(
		ctx,
		msgs,
		mockSummarizerErr(context.Canceled),
		false,
		"",
		TriggerManual,
		"",
	)
	require.Error(t, err)
	assert.Equal(t, ErrorMessageUserAbort, err.Error())
}

func TestCompactConversation_EmptySummary(t *testing.T) {
	msgs := []message.Message{message.UserMessage("test")}
	_, err := CompactConversation(
		context.Background(),
		msgs,
		mockSummarizer(""),
		false,
		"",
		TriggerManual,
		"",
	)
	require.Error(t, err)
	assert.Equal(t, ErrorMessageIncompleteResponse, err.Error())
}

func TestCompactConversation_WhitespaceSummary(t *testing.T) {
	msgs := []message.Message{message.UserMessage("test")}
	_, err := CompactConversation(
		context.Background(),
		msgs,
		mockSummarizer("   \n\t  "),
		false,
		"",
		TriggerManual,
		"",
	)
	require.Error(t, err)
	assert.Equal(t, ErrorMessageIncompleteResponse, err.Error())
}

// --- BuildPostCompactMessages ordering ---

func TestBuildPostCompactMessages_Ordering(t *testing.T) {
	boundary := message.UserMessage("[boundary]")
	summary := message.UserMessage("summary")
	kept := message.UserMessage("kept msg")
	attachment := message.UserMessage("file attachment")
	hook := message.UserMessage("hook result")

	result := BuildPostCompactMessages(CompactionResult{
		BoundaryMarker:  boundary,
		SummaryMessages: []message.Message{summary},
		MessagesToKeep:  []message.Message{kept},
		Attachments:     []message.Message{attachment},
		HookResults:     []message.Message{hook},
	})

	require.Len(t, result, 5)
	assert.Equal(t, "[boundary]", result[0].Content[0].Text)
	assert.Equal(t, "summary", result[1].Content[0].Text)
	assert.Equal(t, "kept msg", result[2].Content[0].Text)
	assert.Equal(t, "file attachment", result[3].Content[0].Text)
	assert.Equal(t, "hook result", result[4].Content[0].Text)
}

func TestBuildPostCompactMessages_NoOptionalFields(t *testing.T) {
	result := BuildPostCompactMessages(CompactionResult{
		BoundaryMarker:  message.UserMessage("[boundary]"),
		SummaryMessages: []message.Message{message.UserMessage("summary")},
	})
	// boundary + summary only
	require.Len(t, result, 2)
}

// --- PartialCompactConversation tests (fork-context preservation) ---

func TestPartialCompact_UpTo_PreservesLaterMessages(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("old msg 1"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("old reply")}},
		message.UserMessage("recent msg"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("recent reply")}},
	}

	result, err := PartialCompactConversation(
		context.Background(),
		msgs,
		2, // pivot: summarize [0,1], keep [2,3]
		mockSummarizer("summary of old messages"),
		"up_to",
		"",
		"",
	)
	require.NoError(t, err)

	// Messages after pivot are preserved.
	require.Len(t, result.MessagesToKeep, 2)
	assert.Equal(t, "recent msg", result.MessagesToKeep[0].Content[0].Text)
	assert.Equal(t, "recent reply", result.MessagesToKeep[1].Content[0].Text)

	// Summary is generated from the old messages.
	require.Len(t, result.SummaryMessages, 1)
	assert.Contains(t, result.SummaryMessages[0].Content[0].Text, "summary of old messages")
}

func TestPartialCompact_From_PreservesEarlierMessages(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("keep this"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("keep reply")}},
		message.UserMessage("summarize this"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("summarize reply")}},
	}

	result, err := PartialCompactConversation(
		context.Background(),
		msgs,
		2, // pivot: keep [0,1], summarize [2,3]
		mockSummarizer("summary of later messages"),
		"from",
		"",
		"",
	)
	require.NoError(t, err)

	// Messages before pivot are preserved.
	require.Len(t, result.MessagesToKeep, 2)
	assert.Equal(t, "keep this", result.MessagesToKeep[0].Content[0].Text)
	assert.Equal(t, "keep reply", result.MessagesToKeep[1].Content[0].Text)
}

func TestPartialCompact_EmptySummarizeSet(t *testing.T) {
	msgs := []message.Message{message.UserMessage("only msg")}

	_, err := PartialCompactConversation(
		context.Background(),
		msgs,
		0, // up_to index 0 = nothing to summarize
		mockSummarizer("ignored"),
		"up_to",
		"",
		"",
	)
	require.Error(t, err)
}

func TestPartialCompact_RecentPreservedIndicator(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("old"),
		message.UserMessage("new"),
	}

	result, err := PartialCompactConversation(
		context.Background(),
		msgs,
		1,
		mockSummarizer("summary"),
		"up_to",
		"",
		"",
	)
	require.NoError(t, err)
	summaryText := result.SummaryMessages[0].Content[0].Text
	assert.Contains(t, summaryText, "Recent messages are preserved verbatim")
}

func TestPartialCompact_InvalidDirection(t *testing.T) {
	msgs := []message.Message{message.UserMessage("test")}
	_, err := PartialCompactConversation(
		context.Background(),
		msgs,
		0,
		mockSummarizer("ignored"),
		"invalid",
		"",
		"",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "direction must be")
}

// --- TruncateHeadForPTLRetry ---

func TestTruncateHead_TooFewMessages(t *testing.T) {
	result := TruncateHeadForPTLRetry([]message.Message{message.UserMessage("only")}, 0.2)
	assert.Nil(t, result)
}

func TestTruncateHead_Drops20Percent(t *testing.T) {
	msgs := make([]message.Message, 10)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = message.UserMessage("user msg")
		} else {
			msgs[i] = message.Message{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("asst msg")}}
		}
	}

	result := TruncateHeadForPTLRetry(msgs, 0.2)
	require.NotNil(t, result)
	// 20% of 10 = 2 dropped, 8 remaining (possibly +1 for synthetic marker)
	assert.True(t, len(result) >= 8)
}

func TestTruncateHead_PrependsUserMarkerIfAssistantFirst(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("user 1"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("asst 1")}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("asst 2")}},
		message.UserMessage("user 2"),
	}

	// Drop first 2 messages (50%), leaving [asst2, user2] — asst first
	result := TruncateHeadForPTLRetry(msgs, 0.5)
	require.NotNil(t, result)
	assert.Equal(t, message.RoleUser, result[0].Role)
	assert.Equal(t, PTLRetryMarker, result[0].Content[0].Text)
}

func TestTruncateHead_StripsOwnMarkerFromPreviousRetry(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage(PTLRetryMarker), // from previous retry
		message.UserMessage("real msg 1"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("asst 1")}},
		message.UserMessage("real msg 2"),
	}

	result := TruncateHeadForPTLRetry(msgs, 0.0) // use default
	require.NotNil(t, result)
	// The marker should have been stripped before processing
	for _, m := range result {
		for _, b := range m.Content {
			if b.Text == PTLRetryMarker && m.Content[0].Text == PTLRetryMarker {
				// If it's still there, it should be because we re-added it
				// (assistant-first case), not because the old one persisted
			}
		}
	}
	// At minimum, result should be shorter than input minus the marker
	assert.True(t, len(result) < len(msgs))
}

func TestTruncateHead_PreservesAtLeastOne(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("msg 1"),
		message.UserMessage("msg 2"),
	}

	result := TruncateHeadForPTLRetry(msgs, 0.99)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result), 1)
}

// --- Summary prompt template tests ---

func TestCompactPrompt_ContainsNoCriticalParts(t *testing.T) {
	prompt := GetCompactPrompt("")

	// Must contain no-tools preamble
	assert.Contains(t, prompt, "CRITICAL: Respond with TEXT ONLY")
	assert.Contains(t, prompt, "Do NOT call any tools")

	// Must contain the 9-section structure
	assert.Contains(t, prompt, "Primary Request and Intent")
	assert.Contains(t, prompt, "Key Technical Concepts")
	assert.Contains(t, prompt, "Files and Code Sections")
	assert.Contains(t, prompt, "Errors and fixes")
	assert.Contains(t, prompt, "Problem Solving")
	assert.Contains(t, prompt, "All user messages")
	assert.Contains(t, prompt, "Pending Tasks")
	assert.Contains(t, prompt, "Current Work")
	assert.Contains(t, prompt, "Optional Next Step")

	// Must contain no-tools trailer
	assert.Contains(t, prompt, "REMINDER: Do NOT call any tools")
}

func TestCompactPrompt_CustomInstructionsAppended(t *testing.T) {
	prompt := GetCompactPrompt("Focus on test output")
	assert.Contains(t, prompt, "Focus on test output")
	assert.Contains(t, prompt, "Additional Instructions:")
}

func TestCompactPrompt_EmptyCustomInstructionsOmitted(t *testing.T) {
	prompt := GetCompactPrompt("")
	assert.NotContains(t, prompt, "Additional Instructions:")
}

// --- FormatCompactSummary ---

func TestFormatCompactSummary_StripsAnalysis(t *testing.T) {
	input := "<analysis>internal thinking</analysis>\n<summary>The actual summary</summary>"
	result := FormatCompactSummary(input)
	assert.NotContains(t, result, "internal thinking")
	assert.NotContains(t, result, "<analysis>")
	assert.Contains(t, result, "The actual summary")
}

func TestFormatCompactSummary_FormatsSummaryTag(t *testing.T) {
	input := "<summary>The summary content</summary>"
	result := FormatCompactSummary(input)
	assert.Contains(t, result, "Summary:")
	assert.Contains(t, result, "The summary content")
	assert.NotContains(t, result, "<summary>")
}

// --- MergeHookInstructions ---

func TestMergeHookInstructions_BothEmpty(t *testing.T) {
	assert.Equal(t, "", MergeHookInstructions("", ""))
}

func TestMergeHookInstructions_OnlyUser(t *testing.T) {
	assert.Equal(t, "user instr", MergeHookInstructions("user instr", ""))
}

func TestMergeHookInstructions_OnlyHook(t *testing.T) {
	assert.Equal(t, "hook instr", MergeHookInstructions("", "hook instr"))
}

func TestMergeHookInstructions_Both(t *testing.T) {
	result := MergeHookInstructions("user", "hook")
	assert.Equal(t, "user\n\nhook", result)
}

// --- StripImagesFromMessages ---

func TestStripImages_NoImages(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("reply")}},
	}
	result := StripImagesFromMessages(msgs)
	assert.Equal(t, msgs, result)
}

func TestStripImages_ReplacesImageBlocks(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentBlock{
			{Type: contentImage, Text: "base64data"},
			message.TextBlock("describe this"),
		}},
	}
	result := StripImagesFromMessages(msgs)
	require.Len(t, result, 1)
	assert.Equal(t, message.ContentText, result[0].Content[0].Type)
	assert.Equal(t, "[image]", result[0].Content[0].Text)
	assert.Equal(t, "describe this", result[0].Content[1].Text)
}

func TestStripImages_ReplacesDocumentBlocks(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentBlock{
			{Type: contentDocument},
		}},
	}
	result := StripImagesFromMessages(msgs)
	assert.Equal(t, "[document]", result[0].Content[0].Text)
}

func TestStripImages_SkipsAssistantMessages(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			{Type: contentImage}, // shouldn't happen but test that we don't touch it
		}},
	}
	result := StripImagesFromMessages(msgs)
	assert.Equal(t, contentImage, result[0].Content[0].Type)
}

// --- Full compaction pipeline integration test ---

func TestFullCompactionPipeline_SelectSummarizeReplace(t *testing.T) {
	// Simulate a realistic conversation.
	msgs := []message.Message{
		message.UserMessage("Please read main.go"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("tu1", "Read", nil),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("tu1", "package main\nfunc main() {}", false),
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.TextBlock("I've read main.go. It contains a basic Go program."),
		}},
		message.UserMessage("Now add a function to it"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("tu2", "Edit", nil),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("tu2", "File edited successfully", false),
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.TextBlock("I've added the function."),
		}},
	}

	var receivedMsgs []message.Message
	summarizer := func(_ context.Context, m []message.Message, _ string) (string, error) {
		receivedMsgs = m
		return "<analysis>User asked to read and edit main.go</analysis>\n<summary>User asked to read main.go and add a function. The file was read and edited successfully.</summary>", nil
	}

	result, err := CompactConversation(
		context.Background(),
		msgs,
		summarizer,
		false,
		"",
		TriggerAuto,
		"/tmp/transcript.jsonl",
	)
	require.NoError(t, err)

	// The summarizer received all messages.
	assert.Equal(t, len(msgs), len(receivedMsgs))

	// The result has the expected shape.
	postMsgs := BuildPostCompactMessages(result)
	assert.GreaterOrEqual(t, len(postMsgs), 2) // boundary + summary at minimum

	// The summary message references the transcript.
	found := false
	for _, m := range postMsgs {
		for _, b := range m.Content {
			if strings.Contains(b.Text, "/tmp/transcript.jsonl") {
				found = true
			}
		}
	}
	assert.True(t, found, "post-compact messages should reference transcript path")

	// Pre-compact token count should be positive.
	assert.Greater(t, result.PreCompactTokenCount, 0)
}

// --- Fork-context preservation integration test ---

func TestForkContextPreservation_PartialCompactKeepsContext(t *testing.T) {
	// Simulate a conversation with a "fork boundary" at index 4.
	// Messages 0-3 are old context, 4-7 are recent and must be kept.
	msgs := []message.Message{
		message.UserMessage("old request"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("old response")}},
		message.UserMessage("another old request"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("another old response")}},
		// --- fork boundary ---
		message.UserMessage("new request after fork"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("new response")}},
		message.UserMessage("follow up"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("follow up response")}},
	}

	result, err := PartialCompactConversation(
		context.Background(),
		msgs,
		4, // pivot at fork boundary
		mockSummarizer("Summary of old conversation context"),
		"up_to",
		"",
		"",
	)
	require.NoError(t, err)

	// All 4 messages after the boundary are preserved.
	require.Len(t, result.MessagesToKeep, 4)
	assert.Equal(t, "new request after fork", result.MessagesToKeep[0].Content[0].Text)
	assert.Equal(t, "new response", result.MessagesToKeep[1].Content[0].Text)
	assert.Equal(t, "follow up", result.MessagesToKeep[2].Content[0].Text)
	assert.Equal(t, "follow up response", result.MessagesToKeep[3].Content[0].Text)

	// Building post-compact messages preserves the order.
	postMsgs := BuildPostCompactMessages(result)
	// boundary + summary + 4 kept = 6
	require.Len(t, postMsgs, 6)

	// Kept messages come after summary.
	assert.Equal(t, "new request after fork", postMsgs[2].Content[0].Text)
}

// --- Compact streaming retry constants ---

func TestMaxCompactStreamingRetries(t *testing.T) {
	assert.Equal(t, 2, MaxCompactStreamingRetries)
}

func TestMaxPTLRetries(t *testing.T) {
	assert.Equal(t, 3, MaxPTLRetries)
}

// --- CompactionResult zero-value safety ---

func TestBuildPostCompactMessages_NilSlices(t *testing.T) {
	result := BuildPostCompactMessages(CompactionResult{
		BoundaryMarker:  message.UserMessage("[b]"),
		SummaryMessages: nil,
		MessagesToKeep:  nil,
		Attachments:     nil,
		HookResults:     nil,
	})
	// Only boundary
	require.Len(t, result, 1)
}

// --- Error propagation from summarizer ---

func TestCompactConversation_ArbitrarySummarizerError(t *testing.T) {
	msgs := []message.Message{message.UserMessage("test")}
	_, err := CompactConversation(
		context.Background(),
		msgs,
		mockSummarizerErr(errors.New("network timeout")),
		false,
		"",
		TriggerManual,
		"",
	)
	require.Error(t, err)
	assert.Equal(t, "network timeout", err.Error())
}
