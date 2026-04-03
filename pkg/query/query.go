package query

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

const microCompactThreshold = 10000

const maxRetries = 3

// loadClaudeMD loads CLAUDE.md files following the same hierarchy as the TS source:
// 1. Global ~/.claude/CLAUDE.md
// 2. Walk up from CWD (up to 10 levels) looking for CLAUDE.md
// 3. Check .claude/CLAUDE.md in CWD (distinct from CWD/CLAUDE.md)
func loadClaudeMD(cwd string) string {
	var parts []string

	// 1. Global CLAUDE.md
	if home, err := os.UserHomeDir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md")); err == nil {
			parts = append(parts, string(data))
		}
	}

	// 2. Walk up from CWD (up to 10 levels)
	dir := cwd
	for i := 0; i < 10; i++ {
		if data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md")); err == nil {
			parts = append(parts, string(data))
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// 3. Check .claude/CLAUDE.md in CWD (distinct from CLAUDE.md in CWD)
	dotClaudePath := filepath.Join(cwd, ".claude", "CLAUDE.md")
	cwdPath := filepath.Join(cwd, "CLAUDE.md")
	if dotClaudePath != cwdPath {
		if data, err := os.ReadFile(dotClaudePath); err == nil {
			parts = append(parts, string(data))
		}
	}

	return strings.Join(parts, "\n\n")
}

// LoadClaudeMDPublic is the exported version of loadClaudeMD for use by the REPL.
func LoadClaudeMDPublic(cwd string) string {
	return loadClaudeMD(cwd)
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

	// Token budget tracker for +500k feature
	// Source: query.ts:111, 1308-1355
	var budgetTracker *compact.BudgetTracker
	if sess.Config.TokenBudgetTarget > 0 {
		budgetTracker = compact.NewBudgetTracker()
	}

	// Memory prefetch: load CLAUDE.md async, consume before first API call.
	// Source: query.ts — memory prefetch runs in parallel with skill discovery
	memCh := make(chan string, 1)
	go func() {
		memCh <- loadClaudeMD(sess.CWD)
	}()
	// Consume prefetch result (blocks until ready, but file read is fast)
	memoryContent := <-memCh
	systemPrompt := buildSystemPrompt(sess.Config.SystemPrompt, memoryContent)

	for {
		// 1. Check context cancellation
		if ctx.Err() != nil {
			return &AgentError{Kind: ErrAborted}
		}

		// 1.5. Proactive compaction: check token budget before building request
		if sess.LastInputTokens > 0 && sess.Config.TokenBudget.ShouldCompact(sess.LastInputTokens) {
			CompactSession(sess)
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

		// Add thinking config if enabled
		if sess.Config.ThinkingEnabled {
			budget := sess.Config.ThinkingBudget
			if budget <= 0 {
				budget = 10000
			}
			req.Thinking = &provider.ThinkingConfig{
				Type:         "enabled",
				BudgetTokens: budget,
			}
		}

		// Add JSON schema for structured output if configured
		if sess.Config.JSONSchema != "" {
			req.JSONSchema = json.RawMessage(sess.Config.JSONSchema)
		}

		// 4. Call provider.Stream - with error classification for L2
		apiStart := time.Now()
		ch, err := prov.Stream(ctx, req)
		if err != nil {
			// Fallback model switch: when 529 retries are exhausted and a fallback is configured
			// Source: query.ts:894-951
			if fte, ok := IsFallbackTriggered(err); ok && sess.Config.FallbackModel != "" {
				sess.Config.Model = fte.FallbackModel
				emit(onEvent, QueryEvent{
					Type: QEventTextDelta,
					Text: fmt.Sprintf("\n[Switched to %s due to high demand for %s]\n", fte.FallbackModel, fte.OriginalModel),
				})
				continue // Retry with fallback model
			}

			errStr := strings.ToLower(err.Error())

			// Context too long: compact and retry once
			if strings.Contains(errStr, "context_too_long") || strings.Contains(errStr, "prompt is too long") {
				if compactedOnce {
					return &AgentError{Kind: ErrContextTooLong, Wrapped: err}
				}
				compactedOnce = true
				CompactSession(sess)
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

		// Track API call duration.
		// Source: bootstrap/state.ts — totalAPIDuration
		apiDuration := time.Since(apiStart)
		sess.TotalAPIDuration += float64(apiDuration.Milliseconds())

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

		// Compute accurate cost and update session tracking.
		// Source: bootstrap/state.ts — addToTotalCostState()
		turnUsage := provider.TokenUsage{
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
		}
		if usage.CacheReadInputTokens != nil {
			turnUsage.CacheReadInputTokens = *usage.CacheReadInputTokens
		}
		if usage.CacheCreationInputTokens != nil {
			turnUsage.CacheCreationInputTokens = *usage.CacheCreationInputTokens
		}
		turnCost := provider.CalculateUSDCost(sess.Config.Model, turnUsage)
		sess.AddCost(sess.Config.Model, turnCost, turnUsage)

		// Budget check: stop if spend exceeds --max-budget-usd
		if sess.Config.MaxBudgetUSD > 0 {
			if sess.TotalCostUSD > sess.Config.MaxBudgetUSD {
				return &AgentError{Kind: ErrProvider, Detail: fmt.Sprintf("budget exceeded: %s > %s", provider.FormatCost(sess.TotalCostUSD), provider.FormatCost(sess.Config.MaxBudgetUSD))}
			}
		}

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

		// Execute post-sampling hooks (fire-and-forget, before tool execution)
		// Source: query.ts:999-1009
		if len(sess.PostSamplingHooks) > 0 {
			var assistantTexts []string
			for _, b := range contentBlocks {
				if b.Type == message.ContentText && b.Text != "" {
					assistantTexts = append(assistantTexts, b.Text)
				}
			}
			for _, hook := range sess.PostSamplingHooks {
				if h, ok := hook.(PostSamplingHook); ok {
					go h(assistantTexts) // Fire-and-forget (void in TS)
				}
			}
		}

		// 8. If no tool calls, check stop reason
		if len(toolCalls) == 0 {
			if stopReason == provider.StopReasonMaxTokens {
				// L2: Auto-continue (matches TS source query.ts:1226-1227)
				sess.PushMessage(message.UserMessage("Output token limit hit. Resume directly — no apology, no recap of what you were doing. Pick up mid-thought if that is where the cut happened. Break remaining work into smaller pieces."))
				continue
			}

			// Stop hooks: run after model response, can prevent continuation
			// Source: query.ts:1267-1305
			if runner, ok := sess.StopHookRunner.(StopHookRunner); ok && runner != nil {
				var assistantTexts []string
				for _, b := range contentBlocks {
					if b.Type == message.ContentText && b.Text != "" {
						assistantTexts = append(assistantTexts, b.Text)
					}
				}
				hookResult := runner(assistantTexts)
				if hookResult.PreventContinuation {
					emit(onEvent, QueryEvent{Type: QEventTurnComplete, StopReason: stopReason})
					return nil
				}
				if len(hookResult.BlockingErrors) > 0 {
					for _, errMsg := range hookResult.BlockingErrors {
						sess.PushMessage(message.UserMessage(errMsg))
					}
					continue
				}
			}

			// Token budget nudge: if user specified +500k, check if we should continue
			// Source: query.ts:1308-1355
			if sess.Config.TokenBudgetTarget > 0 && budgetTracker != nil {
				decision := budgetTracker.CheckTokenBudget(sess.Config.TokenBudgetTarget, sess.TotalOutputTokens)
				if decision.Action == compact.BudgetContinue {
					nudgeMsg := compact.GetBudgetContinuationMessage(decision.Pct, decision.TurnTokens, decision.Budget)
					sess.PushMessage(message.UserMessage(nudgeMsg))
					continue
				}
			}

			emit(onEvent, QueryEvent{Type: QEventTurnComplete, StopReason: stopReason})
			return nil
		}

		// 9. Emit ToolUseStart events
		for _, tc := range toolCalls {
			emit(onEvent, QueryEvent{Type: QEventToolUseStart, ToolUseID: tc.ID, ToolName: tc.Name})
		}

		// 10. Execute tools via orchestrator
		var permPolicy permissions.PermissionPolicy
		if pp, ok := sess.PermissionPolicy.(permissions.PermissionPolicy); ok && pp != nil {
			permPolicy = pp
		} else {
			permPolicy = permissions.NewRuleBasedPolicy(sess.Config.PermissionMode)
		}
		toolCtx := &tools.ToolContext{
			CWD:         sess.CWD,
			Permissions: permPolicy,
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

// isRetryable checks if an error message indicates a retryable error (429, 529, or 5xx).
func isRetryable(errStr string) bool {
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate") {
		return true
	}
	if strings.Contains(errStr, "529") || strings.Contains(errStr, "overload") {
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

// CompactSession removes middle messages from the session to reduce context size.
// It keeps the first message and the last few messages.
func CompactSession(sess *session.SessionState) {
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
