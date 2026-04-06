// Package bridge — stream_accumulator.go implements text_delta coalescing
// for stream events, producing full-so-far snapshots per content block.
// Source: src/cli/transports/ccrClient.ts (StreamAccumulatorState)
package bridge

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// StreamAccumulatorState accumulates text_delta chunks into full-so-far
// snapshots per content block. Keyed by API message ID; cleared when the
// complete assistant message arrives via WriteEvent.
type StreamAccumulatorState struct {
	mu sync.Mutex

	// ByMessage maps API message ID (msg_...) -> blocks[blockIndex] -> chunk slice.
	ByMessage map[string][][]string

	// ScopeToMessage maps "{session_id}:{parent_tool_use_id}" -> active message ID.
	// content_block_delta events don't carry the message ID, so we track which
	// message is currently streaming for each scope.
	ScopeToMessage map[string]string
}

// NewStreamAccumulator creates an empty StreamAccumulatorState.
func NewStreamAccumulator() *StreamAccumulatorState {
	return &StreamAccumulatorState{
		ByMessage:      make(map[string][][]string),
		ScopeToMessage: make(map[string]string),
	}
}

// scopeKey returns the scope key for a stream event.
func scopeKey(sessionID, parentToolUseID string) string {
	return fmt.Sprintf("%s:%s", sessionID, parentToolUseID)
}

// AccumulateStreamEvents processes a buffer of stream events, coalescing
// text_delta events for the same content block into a single full-so-far
// snapshot per flush. Non-text-delta events pass through unchanged.
//
// Each emitted text_delta event is self-contained — a client connecting
// mid-stream sees the complete text from the start of the block.
func AccumulateStreamEvents(buffer []map[string]any, state *StreamAccumulatorState) []EventPayload {
	state.mu.Lock()
	defer state.mu.Unlock()

	out := make([]EventPayload, 0, len(buffer))

	// Track which chunk slices we've already emitted a snapshot for in this
	// flush. Key = pointer identity via the index into ByMessage. We use a
	// string key "{msgID}:{blockIndex}" since Go slices aren't comparable.
	type touchKey struct {
		msgID      string
		blockIndex int
	}
	touched := make(map[touchKey]int) // value = index into out

	for _, msg := range buffer {
		evtObj, _ := msg["event"].(map[string]any)
		var evtType string
		if evtObj != nil {
			evtType, _ = evtObj["type"].(string)
		}

		sessionID, _ := msg["session_id"].(string)
		parentToolUseID, _ := msg["parent_tool_use_id"].(string)
		sk := scopeKey(sessionID, parentToolUseID)

		switch evtType {
		case "message_start":
			// Extract message ID from event.message.id
			msgMsg, _ := evtObj["message"].(map[string]any)
			var msgID string
			if msgMsg != nil {
				msgID, _ = msgMsg["id"].(string)
			}
			if msgID != "" {
				// Clear previous message for this scope
				if prevID, ok := state.ScopeToMessage[sk]; ok {
					delete(state.ByMessage, prevID)
				}
				state.ScopeToMessage[sk] = msgID
				state.ByMessage[msgID] = nil // empty blocks slice
			}
			out = append(out, EventPayload(msg))

		case "content_block_delta":
			delta, _ := evtObj["delta"].(map[string]any)
			var deltaType string
			if delta != nil {
				deltaType, _ = delta["type"].(string)
			}

			if deltaType != "text_delta" {
				out = append(out, EventPayload(msg))
				break
			}

			text, _ := delta["text"].(string)
			indexF, _ := evtObj["index"].(float64)
			blockIndex := int(indexF)

			messageID := state.ScopeToMessage[sk]
			blocks, hasBlocks := state.ByMessage[messageID]
			if messageID == "" || !hasBlocks {
				// Delta without a preceding message_start — pass through raw.
				out = append(out, EventPayload(msg))
				break
			}

			// Ensure blocks slice is big enough.
			for len(blocks) <= blockIndex {
				blocks = append(blocks, nil)
			}
			state.ByMessage[messageID] = blocks

			// Append chunk.
			if blocks[blockIndex] == nil {
				blocks[blockIndex] = []string{}
			}
			blocks[blockIndex] = append(blocks[blockIndex], text)
			state.ByMessage[messageID] = blocks

			tk := touchKey{msgID: messageID, blockIndex: blockIndex}
			if idx, ok := touched[tk]; ok {
				// Update the existing snapshot in-place.
				existing := out[idx]
				existEvt, _ := existing["event"].(map[string]any)
				if existEvt != nil {
					existDelta, _ := existEvt["delta"].(map[string]any)
					if existDelta != nil {
						existDelta["text"] = strings.Join(blocks[blockIndex], "")
					}
				}
			} else {
				// Create a new snapshot event.
				snapshot := EventPayload{
					"type":               "stream_event",
					"uuid":               uuidFromMsg(msg),
					"session_id":         sessionID,
					"parent_tool_use_id": parentToolUseID,
					"event": map[string]any{
						"type":  "content_block_delta",
						"index": blockIndex,
						"delta": map[string]any{
							"type": "text_delta",
							"text": strings.Join(blocks[blockIndex], ""),
						},
					},
				}
				touched[tk] = len(out)
				out = append(out, snapshot)
			}

		default:
			out = append(out, EventPayload(msg))
		}
	}

	return out
}

// uuidFromMsg extracts uuid from a message map, or generates a new one.
func uuidFromMsg(msg map[string]any) string {
	if u, ok := msg["uuid"].(string); ok && u != "" {
		return u
	}
	return uuid.New().String()
}

// ClearStreamAccumulatorForMessage removes accumulator state for a completed
// assistant message. Called when the complete SDKAssistantMessage arrives.
func ClearStreamAccumulatorForMessage(state *StreamAccumulatorState, sessionID, parentToolUseID, messageID string) {
	state.mu.Lock()
	defer state.mu.Unlock()

	delete(state.ByMessage, messageID)
	sk := scopeKey(sessionID, parentToolUseID)
	if state.ScopeToMessage[sk] == messageID {
		delete(state.ScopeToMessage, sk)
	}
}
