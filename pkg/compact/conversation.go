package compact

import (
	"context"
	"errors"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// Source: services/compact/compact.ts:299-763

// CompactionTrigger distinguishes manual /compact from auto-compact.
type CompactionTrigger string

const (
	TriggerManual CompactionTrigger = "manual"
	TriggerAuto   CompactionTrigger = "auto"
)

// CompactionResult is the output of a full or partial compaction.
// Source: compact.ts:299-310
type CompactionResult struct {
	// BoundaryMarker is the system message inserted at the compaction boundary.
	BoundaryMarker message.Message
	// SummaryMessages are the user messages containing the formatted summary.
	SummaryMessages []message.Message
	// Attachments are restored file/skill/plan/agent context messages.
	Attachments []message.Message
	// HookResults are messages produced by session-start hooks post-compact.
	HookResults []message.Message
	// MessagesToKeep are messages preserved across compaction (partial compact).
	MessagesToKeep []message.Message
	// UserDisplayMessage is an optional message shown to the user.
	UserDisplayMessage string
	// PreCompactTokenCount is the estimated tokens before compaction.
	PreCompactTokenCount int
	// PostCompactTokenCount is the compact API call's total token usage.
	PostCompactTokenCount int
	// TruePostCompactTokenCount is the estimated size of the resulting context.
	TruePostCompactTokenCount int
}

// RecompactionInfo tracks compact history for diagnostics.
// Source: compact.ts:317-323
type RecompactionInfo struct {
	IsRecompactionInChain    bool
	TurnsSincePreviousCompact int
	PreviousCompactTurnID    string
	AutoCompactThreshold     int
	QuerySource              string
}

// SummaryFunc is the callback that sends messages to the LLM for summarization
// and returns the summary text. This is the seam for testing — callers inject
// a real API client; tests inject a stub.
type SummaryFunc func(ctx context.Context, messages []message.Message, systemPrompt string) (string, error)

// BuildPostCompactMessages constructs the ordered post-compact message list.
// Order: boundaryMarker, summaryMessages, messagesToKeep, attachments, hookResults.
// Source: compact.ts:330-338
func BuildPostCompactMessages(result CompactionResult) []message.Message {
	var out []message.Message
	out = append(out, result.BoundaryMarker)
	out = append(out, result.SummaryMessages...)
	out = append(out, result.MessagesToKeep...)
	out = append(out, result.Attachments...)
	out = append(out, result.HookResults...)
	return out
}

// CompactConversation summarizes a full conversation, replacing all messages
// with a compact boundary + summary + restored attachments.
// Source: compact.ts:387-763
func CompactConversation(
	ctx context.Context,
	messages []message.Message,
	summarize SummaryFunc,
	suppressFollowUp bool,
	customInstructions string,
	trigger CompactionTrigger,
	transcriptPath string,
) (CompactionResult, error) {
	if len(messages) == 0 {
		return CompactionResult{}, errors.New(ErrorMessageNotEnoughMessages)
	}

	preCompactTokenCount := EstimateMessageTokens(messages)

	// Build the compaction system prompt.
	prompt := GetCompactPrompt(customInstructions)

	// Strip images before sending to summarizer — images waste tokens and
	// can cause the compact call itself to hit prompt-too-long.
	stripped := StripImagesFromMessages(messages)

	// Call the summarizer (LLM or mock).
	summary, err := summarize(ctx, stripped, prompt)
	if err != nil {
		// Detect user abort.
		if errors.Is(err, context.Canceled) {
			return CompactionResult{}, errors.New(ErrorMessageUserAbort)
		}
		return CompactionResult{}, err
	}

	if strings.TrimSpace(summary) == "" {
		return CompactionResult{}, errors.New(ErrorMessageIncompleteResponse)
	}

	// Build the summary user message.
	formattedSummary := GetCompactUserSummaryMessage(summary, suppressFollowUp, transcriptPath, false)
	summaryMsg := message.UserMessage(formattedSummary)

	// Build the compact boundary marker.
	boundaryMarker := message.UserMessage("[compact boundary]")

	postCompactTokenCount := EstimateMessageTokens([]message.Message{boundaryMarker, summaryMsg})

	return CompactionResult{
		BoundaryMarker:           boundaryMarker,
		SummaryMessages:          []message.Message{summaryMsg},
		Attachments:              nil, // filled by caller with createPostCompactFileAttachments etc.
		HookResults:              nil,
		PreCompactTokenCount:     preCompactTokenCount,
		PostCompactTokenCount:    postCompactTokenCount,
		TruePostCompactTokenCount: postCompactTokenCount,
	}, nil
}

// PartialCompactConversation summarizes part of a conversation around a pivot.
// Direction "up_to": summarizes messages before pivotIndex, keeps later ones.
// Direction "from": summarizes messages after pivotIndex, keeps earlier ones.
// Source: compact.ts:772-1060
func PartialCompactConversation(
	ctx context.Context,
	allMessages []message.Message,
	pivotIndex int,
	summarize SummaryFunc,
	direction string,
	customInstructions string,
	transcriptPath string,
) (CompactionResult, error) {
	var messagesToSummarize, messagesToKeep []message.Message

	switch direction {
	case "up_to":
		if pivotIndex <= 0 || pivotIndex > len(allMessages) {
			return CompactionResult{}, errors.New("invalid pivot index for up_to compaction")
		}
		messagesToSummarize = allMessages[:pivotIndex]
		messagesToKeep = allMessages[pivotIndex:]
	case "from":
		if pivotIndex < 0 || pivotIndex >= len(allMessages) {
			return CompactionResult{}, errors.New("invalid pivot index for from compaction")
		}
		messagesToSummarize = allMessages[pivotIndex:]
		messagesToKeep = allMessages[:pivotIndex]
	default:
		return CompactionResult{}, errors.New("direction must be 'up_to' or 'from'")
	}

	if len(messagesToSummarize) == 0 {
		if direction == "up_to" {
			return CompactionResult{}, errors.New("Nothing to summarize before the selected message.")
		}
		return CompactionResult{}, errors.New("Nothing to summarize after the selected message.")
	}

	preCompactTokenCount := EstimateMessageTokens(allMessages)

	prompt := GetCompactPrompt(customInstructions)
	stripped := StripImagesFromMessages(messagesToSummarize)

	summary, err := summarize(ctx, stripped, prompt)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return CompactionResult{}, errors.New(ErrorMessageUserAbort)
		}
		return CompactionResult{}, err
	}

	if strings.TrimSpace(summary) == "" {
		return CompactionResult{}, errors.New(ErrorMessageIncompleteResponse)
	}

	formattedSummary := GetCompactUserSummaryMessage(summary, false, transcriptPath, len(messagesToKeep) > 0)
	summaryMsg := message.UserMessage(formattedSummary)
	boundaryMarker := message.UserMessage("[compact boundary]")

	return CompactionResult{
		BoundaryMarker:       boundaryMarker,
		SummaryMessages:      []message.Message{summaryMsg},
		MessagesToKeep:       messagesToKeep,
		PreCompactTokenCount: preCompactTokenCount,
	}, nil
}

// TruncateHeadForPTLRetry drops the oldest API-round groups from messages
// until the prompt-too-long gap is covered. Returns nil when nothing can be
// dropped without leaving an empty summarize set.
// Source: compact.ts:243-291
func TruncateHeadForPTLRetry(messages []message.Message, dropFraction float64) []message.Message {
	if len(messages) < 2 {
		return nil
	}

	// Strip our own synthetic marker from a previous retry.
	input := messages
	if len(input) > 0 && input[0].Role == message.RoleUser &&
		len(input[0].Content) == 1 && input[0].Content[0].Text == PTLRetryMarker {
		input = input[1:]
	}

	if len(input) < 2 {
		return nil
	}

	// Default: drop 20% of messages.
	if dropFraction <= 0 {
		dropFraction = 0.2
	}
	dropCount := int(float64(len(input)) * dropFraction)
	if dropCount < 1 {
		dropCount = 1
	}
	// Keep at least one message so there's something to summarize.
	if dropCount >= len(input) {
		dropCount = len(input) - 1
	}

	sliced := input[dropCount:]

	// If first remaining message is assistant, prepend a synthetic user marker
	// (Anthropic API requires first message to be user role).
	if len(sliced) > 0 && sliced[0].Role == message.RoleAssistant {
		return append(
			[]message.Message{message.UserMessage(PTLRetryMarker)},
			sliced...,
		)
	}
	return sliced
}
