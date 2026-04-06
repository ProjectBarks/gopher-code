// Package remote implements the remote permission bridge for CCR sessions.
// Source: src/remote/remotePermissionBridge.ts
package remote

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/projectbarks/gopher-code/pkg/message"
)

// ---------------------------------------------------------------------------
// T74: CreateSyntheticAssistantMessage
// Source: remotePermissionBridge.ts — createSyntheticAssistantMessage
// ---------------------------------------------------------------------------

// SDKControlPermissionRequest is the inbound permission request from a remote
// CCR session. It mirrors the TS SDKControlPermissionRequest type.
type SDKControlPermissionRequest struct {
	ToolName  string         `json:"tool_name"`
	ToolUseID string         `json:"tool_use_id"`
	Input     map[string]any `json:"input"`
}

// SyntheticAssistantMessage wraps a remote tool_use request in an
// AssistantMessage shape that ToolUseConfirm can consume.
// Source: remotePermissionBridge.ts:15-42
type SyntheticAssistantMessage struct {
	// ID is "remote-{requestId}".
	ID string `json:"id"`
	// Model is empty for synthetic messages.
	Model string `json:"model"`
	// StopReason is nil for synthetic messages.
	StopReason *string `json:"stop_reason"`
	// Usage is all-zero.
	Usage SyntheticUsage `json:"usage"`
	// Content contains the single tool_use block.
	Content []message.ContentBlock `json:"content"`
	// Timestamp is the creation time in ISO 8601.
	Timestamp string `json:"timestamp"`
}

// SyntheticUsage is an all-zero usage struct for synthetic messages.
type SyntheticUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// CreateSyntheticAssistantMessage wraps a remote SDKControlPermissionRequest
// into an AssistantMessage shape. The ToolUseConfirm component needs a real
// AssistantMessage; this bridges remote tool_use requests into that contract.
//
// Source: remotePermissionBridge.ts:15-42
func CreateSyntheticAssistantMessage(request SDKControlPermissionRequest, requestID string) SyntheticAssistantMessage {
	toolUseID := request.ToolUseID
	if toolUseID == "" {
		toolUseID = uuid.New().String()
	}

	inputJSON, _ := json.Marshal(request.Input)

	return SyntheticAssistantMessage{
		ID:         fmt.Sprintf("remote-%s", requestID),
		Model:      "",
		StopReason: nil,
		Usage:      SyntheticUsage{},
		Content: []message.ContentBlock{
			message.ToolUseBlock(toolUseID, request.ToolName, inputJSON),
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// ---------------------------------------------------------------------------
// T75: CreateToolStub
// Source: remotePermissionBridge.ts — createToolStub
// ---------------------------------------------------------------------------

// ToolStub is a minimal Tool interface implementation for MCP tools unknown
// to the local CLI. It routes to FallbackPermissionRequest with conservative
// defaults (needsPermissions=true, isReadOnly=false, isMcp=false).
//
// Source: remotePermissionBridge.ts:44-78
type ToolStub struct {
	// Name is the user-facing tool name.
	Name_ string
}

// CreateToolStub creates a minimal Tool stub for an unknown tool name.
// The stub has conservative permission defaults so users always see the
// full permission prompt.
//
// Source: remotePermissionBridge.ts:44-78
func CreateToolStub(toolName string) *ToolStub {
	return &ToolStub{Name_: toolName}
}

// UserFacingName returns the tool name as displayed to the user.
func (t *ToolStub) UserFacingName() string { return t.Name_ }

// IsEnabled always returns true for stubs.
func (t *ToolStub) IsEnabled() bool { return true }

// NeedsPermissions always returns true (conservative default).
func (t *ToolStub) NeedsPermissions() bool { return true }

// IsReadOnly always returns false (conservative default).
func (t *ToolStub) IsReadOnly() bool { return false }

// IsMCP always returns false for stubs.
func (t *ToolStub) IsMCP() bool { return false }

// RenderInput formats the first 3 input entries as "key: value, key: value".
// Source: remotePermissionBridge.ts:62-73
func (t *ToolStub) RenderInput(input map[string]any) string {
	return FormatToolInput(input, 3)
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// FormatToolInput formats up to maxEntries input key-value pairs as a
// comma-separated string. Non-string values are JSON-serialized.
// Source: remotePermissionBridge.ts:62-73
func FormatToolInput(input map[string]any, maxEntries int) string {
	if len(input) == 0 {
		return ""
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for i, k := range keys {
		if i >= maxEntries {
			break
		}
		v := input[k]
		var vs string
		switch val := v.(type) {
		case string:
			vs = val
		default:
			b, _ := json.Marshal(val)
			vs = string(b)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", k, vs))
	}
	return strings.Join(parts, ", ")
}

// ---------------------------------------------------------------------------
// PermissionBridge ties the synthetic message builder and tool stub together.
// Source: remotePermissionBridge.ts — module-level exports
// ---------------------------------------------------------------------------

// PermissionBridge is the top-level bridge that remote CCR permission
// requests flow through. It creates synthetic messages and tool stubs.
type PermissionBridge struct{}

// NewPermissionBridge creates a new PermissionBridge instance.
func NewPermissionBridge() *PermissionBridge {
	return &PermissionBridge{}
}

// WrapRequest creates a synthetic AssistantMessage and ToolStub for a
// remote permission request. This is the main entry point for CCR
// permission bridging.
func (pb *PermissionBridge) WrapRequest(request SDKControlPermissionRequest, requestID string) (SyntheticAssistantMessage, *ToolStub) {
	msg := CreateSyntheticAssistantMessage(request, requestID)
	stub := CreateToolStub(request.ToolName)
	return msg, stub
}
