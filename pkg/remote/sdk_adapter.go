// Package remote — SDK message adapter for CCR remote sessions.
// Source: src/remote/sdkMessageAdapter.ts
package remote

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// ---------------------------------------------------------------------------
// T84: ConvertedMessage union
// Source: sdkMessageAdapter.ts:145-149
// ---------------------------------------------------------------------------

// ConvertedMessageType discriminates the ConvertedMessage union.
type ConvertedMessageType string

const (
	// ConvertedMsg indicates the message was converted to a display message.
	ConvertedMsg ConvertedMessageType = "message"
	// ConvertedStreamEvent indicates a streaming event.
	ConvertedStreamEvent ConvertedMessageType = "stream_event"
	// ConvertedIgnored indicates the message was intentionally ignored.
	ConvertedIgnored ConvertedMessageType = "ignored"
)

// ConvertedMessage is the result of converting an SDKMessage from CCR.
// It is a tagged union discriminated by Type.
// Source: sdkMessageAdapter.ts:145-149
type ConvertedMessage struct {
	Type ConvertedMessageType

	// Message is set when Type == ConvertedMsg.
	// It contains the display-ready message data.
	Message *DisplayMessage

	// StreamEvent is set when Type == ConvertedStreamEvent.
	StreamEvent *StreamEventData
}

// DisplayMessage holds a converted message for display in the REPL.
// Source: sdkMessageAdapter.ts — various convert* functions
type DisplayMessage struct {
	// Kind is "assistant", "system", or "user".
	Kind string `json:"kind"`
	// Subtype refines system messages (e.g. "informational", "compact_boundary").
	Subtype string `json:"subtype,omitempty"`
	// Content is the text content for display.
	Content string `json:"content"`
	// Level is "info" or "warning" for system messages.
	Level string `json:"level,omitempty"`
	// UUID is the message UUID from the server.
	UUID string `json:"uuid,omitempty"`
	// Timestamp is ISO 8601 creation time.
	Timestamp string `json:"timestamp"`
	// ToolUseID is set for tool_progress messages.
	ToolUseID string `json:"tool_use_id,omitempty"`
	// Raw is the original SDK message (for assistant messages that carry
	// the full API message shape).
	Raw json.RawMessage `json:"raw,omitempty"`
}

// StreamEventData holds a streaming event from a partial assistant message.
type StreamEventData struct {
	// Event is the raw stream event payload.
	Event json.RawMessage `json:"event"`
}

// ---------------------------------------------------------------------------
// T83: convertSDKMessage converter
// Source: sdkMessageAdapter.ts:168-278
// ---------------------------------------------------------------------------

// ConvertOptions controls optional conversion behavior.
type ConvertOptions struct {
	// ConvertToolResults converts user messages containing tool_result content
	// blocks into display messages. Used by direct-connect mode.
	ConvertToolResults bool
	// ConvertUserTextMessages converts user text messages for historical display.
	ConvertUserTextMessages bool
}

// ConvertSDKMessage converts an SDK wire-format message (JSON) to a
// ConvertedMessage for display in the REPL.
// Source: sdkMessageAdapter.ts:168-278
func ConvertSDKMessage(raw json.RawMessage, opts *ConvertOptions) ConvertedMessage {
	if opts == nil {
		opts = &ConvertOptions{}
	}

	var envelope struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype,omitempty"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		slog.Debug("sdk adapter: failed to decode message type", "err", err)
		return ConvertedMessage{Type: ConvertedIgnored}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	switch envelope.Type {
	case "assistant":
		return convertAssistantMsg(raw, now)

	case "user":
		return convertUserMsg(raw, opts, now)

	case "stream_event":
		return convertStreamEventMsg(raw)

	case "result":
		return convertResultMsg(raw, now)

	case "system":
		return convertSystemMsg(raw, envelope.Subtype, now)

	case "tool_progress":
		return convertToolProgressMsg(raw, now)

	case "auth_status":
		slog.Debug("sdk adapter: ignoring auth_status message")
		return ConvertedMessage{Type: ConvertedIgnored}

	case "tool_use_summary":
		slog.Debug("sdk adapter: ignoring tool_use_summary message")
		return ConvertedMessage{Type: ConvertedIgnored}

	case "rate_limit_event":
		slog.Debug("sdk adapter: ignoring rate_limit_event message")
		return ConvertedMessage{Type: ConvertedIgnored}

	default:
		slog.Debug("sdk adapter: unknown message type", "type", envelope.Type)
		return ConvertedMessage{Type: ConvertedIgnored}
	}
}

// ---------------------------------------------------------------------------
// Per-type converters
// Source: sdkMessageAdapter.ts:31-123
// ---------------------------------------------------------------------------

func convertAssistantMsg(raw json.RawMessage, now string) ConvertedMessage {
	var msg struct {
		UUID string `json:"uuid"`
	}
	_ = json.Unmarshal(raw, &msg)

	return ConvertedMessage{
		Type: ConvertedMsg,
		Message: &DisplayMessage{
			Kind:      "assistant",
			UUID:      msg.UUID,
			Timestamp: now,
			Raw:       raw,
		},
	}
}

func convertUserMsg(raw json.RawMessage, opts *ConvertOptions, now string) ConvertedMessage {
	var msg struct {
		UUID    string `json:"uuid"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
		ToolUseResult json.RawMessage `json:"tool_use_result,omitempty"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return ConvertedMessage{Type: ConvertedIgnored}
	}

	// Check if content is an array containing tool_result blocks.
	isToolResult := false
	var contentArray []struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(msg.Message.Content, &contentArray) == nil {
		for _, block := range contentArray {
			if block.Type == "tool_result" {
				isToolResult = true
				break
			}
		}
	}

	if opts.ConvertToolResults && isToolResult {
		return ConvertedMessage{
			Type: ConvertedMsg,
			Message: &DisplayMessage{
				Kind:      "user",
				Subtype:   "tool_result",
				UUID:      msg.UUID,
				Timestamp: now,
				Raw:       raw,
			},
		}
	}

	if opts.ConvertUserTextMessages && !isToolResult {
		// Check if content is a string or array.
		var str string
		if json.Unmarshal(msg.Message.Content, &str) == nil {
			return ConvertedMessage{
				Type: ConvertedMsg,
				Message: &DisplayMessage{
					Kind:      "user",
					Content:   str,
					UUID:      msg.UUID,
					Timestamp: now,
					Raw:       raw,
				},
			}
		}
		if len(contentArray) > 0 {
			return ConvertedMessage{
				Type: ConvertedMsg,
				Message: &DisplayMessage{
					Kind:      "user",
					UUID:      msg.UUID,
					Timestamp: now,
					Raw:       raw,
				},
			}
		}
	}

	return ConvertedMessage{Type: ConvertedIgnored}
}

func convertStreamEventMsg(raw json.RawMessage) ConvertedMessage {
	var msg struct {
		Event json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return ConvertedMessage{Type: ConvertedIgnored}
	}

	return ConvertedMessage{
		Type: ConvertedStreamEvent,
		StreamEvent: &StreamEventData{
			Event: msg.Event,
		},
	}
}

func convertResultMsg(raw json.RawMessage, now string) ConvertedMessage {
	var msg struct {
		Subtype string   `json:"subtype"`
		UUID    string   `json:"uuid"`
		Errors  []string `json:"errors,omitempty"`
		Result  string   `json:"result,omitempty"`
	}
	_ = json.Unmarshal(raw, &msg)

	// Only show result messages for errors. Success results are noise.
	// Source: sdkMessageAdapter.ts:222-226
	if msg.Subtype == "success" {
		return ConvertedMessage{Type: ConvertedIgnored}
	}

	content := "Unknown error"
	if len(msg.Errors) > 0 {
		content = ""
		for i, e := range msg.Errors {
			if i > 0 {
				content += ", "
			}
			content += e
		}
	}

	return ConvertedMessage{
		Type: ConvertedMsg,
		Message: &DisplayMessage{
			Kind:      "system",
			Subtype:   "informational",
			Content:   content,
			Level:     "warning",
			UUID:      msg.UUID,
			Timestamp: now,
		},
	}
}

func convertSystemMsg(raw json.RawMessage, subtype, now string) ConvertedMessage {
	var msg struct {
		UUID   string `json:"uuid"`
		Model  string `json:"model,omitempty"`
		Status string `json:"status,omitempty"`
	}
	_ = json.Unmarshal(raw, &msg)

	switch subtype {
	case "init":
		// Source: sdkMessageAdapter.ts:74-83
		return ConvertedMessage{
			Type: ConvertedMsg,
			Message: &DisplayMessage{
				Kind:      "system",
				Subtype:   "informational",
				Content:   fmt.Sprintf("Remote session initialized (model: %s)", msg.Model),
				Level:     "info",
				UUID:      msg.UUID,
				Timestamp: now,
			},
		}

	case "status":
		// Source: sdkMessageAdapter.ts:88-104
		if msg.Status == "" {
			return ConvertedMessage{Type: ConvertedIgnored}
		}
		content := fmt.Sprintf("Status: %s", msg.Status)
		if msg.Status == "compacting" {
			content = "Compacting conversation\u2026"
		}
		return ConvertedMessage{
			Type: ConvertedMsg,
			Message: &DisplayMessage{
				Kind:      "system",
				Subtype:   "informational",
				Content:   content,
				Level:     "info",
				UUID:      msg.UUID,
				Timestamp: now,
			},
		}

	case "compact_boundary":
		// Source: sdkMessageAdapter.ts:128-140
		return ConvertedMessage{
			Type: ConvertedMsg,
			Message: &DisplayMessage{
				Kind:      "system",
				Subtype:   "compact_boundary",
				Content:   "Conversation compacted",
				Level:     "info",
				UUID:      msg.UUID,
				Timestamp: now,
			},
		}

	default:
		slog.Debug("sdk adapter: ignoring system subtype", "subtype", subtype)
		return ConvertedMessage{Type: ConvertedIgnored}
	}
}

func convertToolProgressMsg(raw json.RawMessage, now string) ConvertedMessage {
	var msg struct {
		UUID               string  `json:"uuid"`
		ToolName           string  `json:"tool_name"`
		ToolUseID          string  `json:"tool_use_id"`
		ElapsedTimeSeconds float64 `json:"elapsed_time_seconds"`
	}
	_ = json.Unmarshal(raw, &msg)

	// Source: sdkMessageAdapter.ts:111-123
	return ConvertedMessage{
		Type: ConvertedMsg,
		Message: &DisplayMessage{
			Kind:      "system",
			Subtype:   "informational",
			Content:   fmt.Sprintf("Tool %s running for %.0fs\u2026", msg.ToolName, msg.ElapsedTimeSeconds),
			Level:     "info",
			UUID:      msg.UUID,
			Timestamp: now,
			ToolUseID: msg.ToolUseID,
		},
	}
}

// ---------------------------------------------------------------------------
// T85: Predicates
// Source: sdkMessageAdapter.ts:280-302
// ---------------------------------------------------------------------------

// IsSessionEndMessage returns true if the raw SDK message indicates the
// session has ended (type == "result").
// Source: sdkMessageAdapter.ts:283-285
func IsSessionEndMessage(raw json.RawMessage) bool {
	var envelope struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return false
	}
	return envelope.Type == "result"
}

// IsSuccessResult returns true if the raw SDK result message has
// subtype == "success".
// Source: sdkMessageAdapter.ts:290-292
func IsSuccessResult(raw json.RawMessage) bool {
	var msg struct {
		Subtype string `json:"subtype"`
	}
	if json.Unmarshal(raw, &msg) != nil {
		return false
	}
	return msg.Subtype == "success"
}

// GetResultText extracts the result text from a successful SDKResultMessage.
// Returns empty string if the message is not a success result.
// Source: sdkMessageAdapter.ts:297-302
func GetResultText(raw json.RawMessage) string {
	var msg struct {
		Subtype string `json:"subtype"`
		Result  string `json:"result"`
	}
	if json.Unmarshal(raw, &msg) != nil {
		return ""
	}
	if msg.Subtype == "success" {
		return msg.Result
	}
	return ""
}
