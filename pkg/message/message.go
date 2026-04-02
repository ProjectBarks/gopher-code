package message

import "encoding/json"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type ContentBlockType string

const (
	ContentText       ContentBlockType = "text"
	ContentToolUse    ContentBlockType = "tool_use"
	ContentToolResult ContentBlockType = "tool_result"
)

// ContentBlock is a tagged union. Go lacks sum types, so we use a struct with a Type discriminator.
type ContentBlock struct {
	Type      ContentBlockType `json:"type"`
	Text      string           `json:"text,omitempty"`        // for text blocks
	ID        string           `json:"id,omitempty"`          // for tool_use
	Name      string           `json:"name,omitempty"`        // for tool_use
	Input     json.RawMessage  `json:"input,omitempty"`       // for tool_use (deferred parsing)
	ToolUseID string           `json:"tool_use_id,omitempty"` // for tool_result
	Content   string           `json:"content,omitempty"`     // for tool_result
	IsError   bool             `json:"is_error,omitempty"`    // for tool_result
}

type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// UserMessage creates a user message with a single text block.
func UserMessage(text string) Message {
	return Message{
		Role:    RoleUser,
		Content: []ContentBlock{{Type: ContentText, Text: text}},
	}
}

// TextBlock creates a text content block.
func TextBlock(text string) ContentBlock {
	return ContentBlock{Type: ContentText, Text: text}
}

// ToolUseBlock creates a tool_use content block.
func ToolUseBlock(id, name string, input json.RawMessage) ContentBlock {
	return ContentBlock{Type: ContentToolUse, ID: id, Name: name, Input: input}
}

// ToolResultBlock creates a tool_result content block.
func ToolResultBlock(toolUseID, content string, isError bool) ContentBlock {
	return ContentBlock{Type: ContentToolResult, ToolUseID: toolUseID, Content: content, IsError: isError}
}

// ToolUses returns all tool_use blocks from this message.
func (m Message) ToolUses() []ContentBlock {
	var uses []ContentBlock
	for _, b := range m.Content {
		if b.Type == ContentToolUse {
			uses = append(uses, b)
		}
	}
	return uses
}
