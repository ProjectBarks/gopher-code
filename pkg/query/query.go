package query

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

const microCompactThreshold = 10000

const maxRetries = 3

// loadClaudeMD reads CLAUDE.md from the given directory, if it exists.
func loadClaudeMD(cwd string) string {
	data, err := os.ReadFile(filepath.Join(cwd, "CLAUDE.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// buildSystemPrompt combines the base system prompt with memory content.
func buildSystemPrompt(base, memory string) string {
	if memory == "" {
		return base
	}
	if base == "" {
		return memory
	}
	return base + "\n\n" + memory
}

// Query is the recursive agent loop — the beating heart of the runtime.
//
// This drives multi-turn conversations by:
// 1. Building the model request from session state
// 2. Streaming the model response
// 3. Collecting assistant text and tool_use blocks
// 4. Executing tool calls via the orchestrator
// 5. Appending tool_result messages
// 6. Looping if the model wants to continue
func Query(
	ctx context.Context,
	sess *session.SessionState,
	prov provider.ModelProvider,
	registry *tools.ToolRegistry,
	orchestrator *tools.ToolOrchestrator,
	onEvent EventCallback,
) error {
	retryCount := 0
	compactedOnce := false

	// Memory prefetch: load CLAUDE.md once before the loop
	memoryContent := loadClaudeMD(sess.CWD)
	systemPrompt := buildSystemPrompt(sess.Config.SystemPrompt, memoryContent)

	for {
		// 1. Check context cancellation
		if ctx.Err() != nil {
			return &AgentError{Kind: ErrAborted}
		}

		// 1.5. Proactive compaction: check token budget before building request
		if sess.LastInputTokens > 0 && sess.Config.TokenBudget.ShouldCompact(sess.LastInputTokens) {
			compactSession(sess)
		}

		// 2. Max turns check
		if sess.Config.MaxTurns > 0 && sess.TurnCount >= sess.Config.MaxTurns {
			return &AgentError{
				Kind:   ErrMaxTurnsExceeded,
				Detail: fmt.Sprintf("reached %d turns", sess.Config.MaxTurns),
			}
		}

		// 3. Build ModelRequest
		req := provider.ModelRequest{
			Model:     sess.Config.Model,
			System:    systemPrompt,
			Messages:  sess.ToRequestMessages(),
			MaxTokens: sess.Config.TokenBudget.MaxOutputTokens,
			Tools:     registry.ToolDefinitions(),
		}

		// 4. Call provider.Stream - with error classification for L2
		ch, err := prov.Stream(ctx, req)
		if err != nil {
			errStr := strings.ToLower(err.Error())

			// Context too long: compact and retry once
			if strings.Contains(errStr, "context_too_long") {
				if compactedOnce {
					return &AgentError{Kind: ErrContextTooLong, Wrapped: err}
				}
				compactedOnce = true
				compactSession(sess)
				continue
			}

			// Auth errors: fail immediately
			if strings.Contains(errStr, "401") || strings.Contains(errStr, "auth") {
				return &AgentError{Kind: ErrProvider, Wrapped: err}
			}

			// Rate limit (429) or server errors (5xx): retry up to maxRetries
			if isRetryable(errStr) {
				retryCount++
				if retryCount <= maxRetries {
					continue
				}
				return &AgentError{Kind: ErrProvider, Wrapped: err}
			}

			return &AgentError{Kind: ErrProvider, Wrapped: err}
		}

		// Reset retry count on successful Stream() call
		retryCount = 0

		// 5. Consume stream events from channel
		var textBuilder strings.Builder
		type toolAccum struct {
			ID      string
			Name    string
			JSONBuf strings.Builder
		}
		var toolUses []*toolAccum
		var usage provider.Usage
		var stopReason provider.StopReason

		for result := range ch {
			if result.Err != nil {
				return &AgentError{Kind: ErrProvider, Wrapped: result.Err}
			}
			evt := result.Event
			if evt == nil {
				continue
			}

			switch evt.Type {
			case provider.EventTextDelta:
				textBuilder.WriteString(evt.Text)
				emit(onEvent, QueryEvent{Type: QEventTextDelta, Text: evt.Text})

			case provider.EventContentBlockStart:
				if evt.Content != nil && evt.Content.Type == "tool_use" {
					toolUses = append(toolUses, &toolAccum{
						ID:   evt.Content.ID,
						Name: evt.Content.Name,
					})
				}

			case provider.EventInputJsonDelta:
				if len(toolUses) > 0 {
					toolUses[len(toolUses)-1].JSONBuf.WriteString(evt.PartialJSON)
				}

			case provider.EventMessageDone:
				if evt.Response != nil {
					usage = evt.Response.Usage
					if evt.Response.StopReason != nil {
						stopReason = *evt.Response.StopReason
					}
				}
			}
		}

		// 6. Update session usage
		sess.TotalInputTokens += usage.InputTokens
		sess.TotalOutputTokens += usage.OutputTokens
		sess.LastInputTokens = usage.InputTokens
		if usage.CacheCreationInputTokens != nil {
			sess.TotalCacheCreationTokens += *usage.CacheCreationInputTokens
		}
		if usage.CacheReadInputTokens != nil {
			sess.TotalCacheReadTokens += *usage.CacheReadInputTokens
		}
		sess.TurnCount++

		// Emit usage event
		ue := QueryEvent{
			Type:         QEventUsage,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
		}
		if usage.CacheCreationInputTokens != nil {
			ue.CacheCreation = usage.CacheCreationInputTokens
		}
		if usage.CacheReadInputTokens != nil {
			ue.CacheRead = usage.CacheReadInputTokens
		}
		emit(onEvent, ue)

		// 7. Build assistant message
		var contentBlocks []message.ContentBlock
		text := textBuilder.String()
		if text != "" {
			contentBlocks = append(contentBlocks, message.TextBlock(text))
		}

		var toolCalls []tools.ToolCall
		for _, tu := range toolUses {
			assembledJSON := tu.JSONBuf.String()
			// Validate JSON; fall back to {} if malformed
			inputJSON := json.RawMessage(assembledJSON)
			if !json.Valid(inputJSON) {
				inputJSON = json.RawMessage(`{}`)
			}
			contentBlocks = append(contentBlocks, message.ToolUseBlock(tu.ID, tu.Name, inputJSON))
			toolCalls = append(toolCalls, tools.ToolCall{ID: tu.ID, Name: tu.Name, Input: inputJSON})
		}

		sess.PushMessage(message.Message{Role: message.RoleAssistant, Content: contentBlocks})

		// 8. If no tool calls, check stop reason
		if len(toolCalls) == 0 {
			if stopReason == provider.StopReasonMaxTokens {
				// L2: Auto-continue
				sess.PushMessage(message.UserMessage("Please continue from where you left off."))
				continue
			}
			emit(onEvent, QueryEvent{Type: QEventTurnComplete, StopReason: stopReason})
			return nil
		}

		// 9. Emit ToolUseStart events
		for _, tc := range toolCalls {
			emit(onEvent, QueryEvent{Type: QEventToolUseStart, ToolUseID: tc.ID, ToolName: tc.Name})
		}

		// 10. Execute tools via orchestrator
		toolCtx := &tools.ToolContext{
			CWD:         sess.CWD,
			Permissions: permissions.NewRuleBasedPolicy(sess.Config.PermissionMode),
			SessionID:   sess.ID,
		}
		results := orchestrator.ExecuteBatch(ctx, toolCalls, toolCtx)

		// 11. Build tool result message + emit events (with micro-compaction)
		var resultBlocks []message.ContentBlock
		for _, r := range results {
			content := microCompact(r.Output.Content)
			resultBlocks = append(resultBlocks, message.ToolResultBlock(r.ToolUseID, content, r.Output.IsError))
			emit(onEvent, QueryEvent{
				Type:      QEventToolResult,
				ToolUseID: r.ToolUseID,
				Content:   content,
				IsError:   r.Output.IsError,
			})
		}
		sess.PushMessage(message.Message{Role: message.RoleUser, Content: resultBlocks})
	}
}

// emit safely calls the event callback if non-nil.
func emit(cb EventCallback, evt QueryEvent) {
	if cb != nil {
		cb(evt)
	}
}

// isRetryable checks if an error message indicates a retryable error (429 or 5xx).
func isRetryable(errStr string) bool {
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate") {
		return true
	}
	if strings.Contains(errStr, "5xx") ||
		strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") {
		return true
	}
	return false
}

// microCompact truncates tool results that exceed microCompactThreshold.
func microCompact(content string) string {
	if len(content) <= microCompactThreshold {
		return content
	}
	return content[:microCompactThreshold] + "...[truncated]"
}

// compactSession removes middle messages from the session to reduce context size.
// It keeps the first message and the last few messages.
func compactSession(sess *session.SessionState) {
	msgs := sess.Messages
	if len(msgs) <= 4 {
		return
	}
	// Keep the first message and the last 2 messages.
	keep := make([]message.Message, 0, 3)
	keep = append(keep, msgs[0])
	keep = append(keep, msgs[len(msgs)-2:]...)
	sess.Messages = keep
}
