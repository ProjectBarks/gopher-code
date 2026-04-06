package compact

import (
	"github.com/projectbarks/gopher-code/pkg/message"
)

// Source: services/compact/compact.ts:145-223

// ContentImage and ContentDocument are block types that carry media.
// These may not be defined in the message package yet, so we define
// the type strings here for matching purposes.
const (
	contentImage    message.ContentBlockType = "image"
	contentDocument message.ContentBlockType = "document"
)

// StripImagesFromMessages replaces image and document blocks with text markers.
// Images are not needed for generating a conversation summary and can cause
// the compaction API call itself to hit the prompt-too-long limit.
// Only user messages contain images (directly attached or within tool_result content).
// Source: compact.ts:145-200
func StripImagesFromMessages(messages []message.Message) []message.Message {
	result := make([]message.Message, len(messages))
	for i, msg := range messages {
		if msg.Role != message.RoleUser {
			result[i] = msg
			continue
		}

		hasMedia := false
		newContent := make([]message.ContentBlock, 0, len(msg.Content))
		for _, block := range msg.Content {
			switch block.Type {
			case contentImage:
				hasMedia = true
				newContent = append(newContent, message.ContentBlock{
					Type: message.ContentText,
					Text: "[image]",
				})
			case contentDocument:
				hasMedia = true
				newContent = append(newContent, message.ContentBlock{
					Type: message.ContentText,
					Text: "[document]",
				})
			default:
				newContent = append(newContent, block)
			}
		}

		if !hasMedia {
			result[i] = msg
		} else {
			result[i] = message.Message{
				Role:    msg.Role,
				Content: newContent,
			}
		}
	}
	return result
}

// StripReinjectedAttachments removes attachment types that are re-injected
// post-compaction anyway (skill_discovery, skill_listing). These waste tokens
// and pollute the summary with stale skill suggestions.
// Source: compact.ts:211-223
//
// In Go, this is a no-op until skill search attachments are implemented.
// The function exists for API parity and to be wired up later.
func StripReinjectedAttachments(messages []message.Message) []message.Message {
	return messages
}
