// Package bridge — inbound message parsing for the CCR bridge protocol.
// Source: src/bridge/inboundMessages.ts
package bridge

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------------
// Content block types (subset needed for inbound parsing)
// ---------------------------------------------------------------------------

// ContentBlock represents a single content block in a user message.
// It is a discriminated union on the Type field.
type ContentBlock struct {
	Type   string           `json:"type"`
	Text   string           `json:"text,omitempty"`
	Source *Base64ImageSource `json:"source,omitempty"`
}

// Base64ImageSource is the source payload for an image content block.
// Accepts both "media_type" (correct) and "mediaType" (camelCase from
// mobile clients) via custom unmarshal.
type Base64ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`

	// mediaTypeCamel captures the camelCase "mediaType" field sent by
	// some bridge clients. Used as fallback when media_type is empty.
	mediaTypeCamel string
}

// UnmarshalJSON handles both "media_type" and "mediaType" JSON keys.
func (s *Base64ImageSource) UnmarshalJSON(data []byte) error {
	// Use a raw map to capture both key variants.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["type"]; ok {
		_ = json.Unmarshal(v, &s.Type)
	}
	if v, ok := raw["data"]; ok {
		_ = json.Unmarshal(v, &s.Data)
	}
	if v, ok := raw["media_type"]; ok {
		_ = json.Unmarshal(v, &s.MediaType)
	}
	if v, ok := raw["mediaType"]; ok {
		_ = json.Unmarshal(v, &s.mediaTypeCamel)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Inbound message envelope
// ---------------------------------------------------------------------------

// InboundMessage is the raw JSON envelope received from poll/WebSocket.
// Only user-type messages with non-empty content are processed.
type InboundMessage struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message,omitempty"`
	UUID    string          `json:"uuid,omitempty"`
}

// inboundMessagePayload is the inner "message" field of a user InboundMessage.
type inboundMessagePayload struct {
	Content json.RawMessage `json:"content"`
}

// ---------------------------------------------------------------------------
// Extracted fields (return type of ExtractInboundMessageFields)
// ---------------------------------------------------------------------------

// InboundFields holds the extracted content and optional UUID from an
// inbound user message. Content is either a plain string or a slice of
// ContentBlock.
type InboundFields struct {
	ContentString string         // set when content is a bare string
	ContentBlocks []ContentBlock // set when content is []ContentBlock
	UUID          string         // optional; empty if absent
}

// IsString reports whether the content is a plain string.
func (f *InboundFields) IsString() bool {
	return f.ContentBlocks == nil
}

// ---------------------------------------------------------------------------
// ExtractInboundMessageFields
// ---------------------------------------------------------------------------

// ExtractInboundMessageFields processes an inbound SDK message, extracting
// content and UUID for enqueueing. Returns nil when the message should be
// skipped (non-user type, missing/empty content).
//
// Normalizes image blocks from bridge clients that may use camelCase
// "mediaType" instead of snake_case "media_type".
func ExtractInboundMessageFields(data []byte) (*InboundFields, error) {
	var msg InboundMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("invalid inbound message JSON: %w", err)
	}

	// Only process user messages.
	if msg.Type != "user" {
		return nil, nil
	}

	// Extract inner message payload.
	if len(msg.Message) == 0 {
		return nil, nil
	}
	var payload inboundMessagePayload
	if err := json.Unmarshal(msg.Message, &payload); err != nil {
		return nil, fmt.Errorf("invalid message payload: %w", err)
	}
	if len(payload.Content) == 0 {
		return nil, nil
	}

	// Content is either a string or an array of content blocks.
	// Try string first (most common).
	var str string
	if err := json.Unmarshal(payload.Content, &str); err == nil {
		if str == "" {
			return nil, nil
		}
		return &InboundFields{ContentString: str, UUID: msg.UUID}, nil
	}

	// Try array of content blocks.
	var blocks []ContentBlock
	if err := json.Unmarshal(payload.Content, &blocks); err != nil {
		return nil, fmt.Errorf("content is neither string nor array: %w", err)
	}
	if len(blocks) == 0 {
		return nil, nil
	}

	blocks = NormalizeImageBlocks(blocks)

	return &InboundFields{ContentBlocks: blocks, UUID: msg.UUID}, nil
}

// ---------------------------------------------------------------------------
// NormalizeImageBlocks
// ---------------------------------------------------------------------------

// NormalizeImageBlocks fixes image content blocks from bridge clients that
// send "mediaType" (camelCase) instead of "media_type" (snake_case), or
// omit the field entirely. Without normalization the API rejects with
// "media_type: Field required".
//
// Fast path: returns the original slice when no normalization is needed.
func NormalizeImageBlocks(blocks []ContentBlock) []ContentBlock {
	needsFix := false
	for i := range blocks {
		if isMalformedBase64Image(&blocks[i]) {
			needsFix = true
			break
		}
	}
	if !needsFix {
		return blocks
	}

	out := make([]ContentBlock, len(blocks))
	copy(out, blocks)
	for i := range out {
		if !isMalformedBase64Image(&out[i]) {
			continue
		}
		src := out[i].Source
		mt := src.mediaTypeCamel
		if mt == "" {
			mt = DetectImageFormatFromBase64(src.Data)
		}
		out[i].Source = &Base64ImageSource{
			Type:      "base64",
			MediaType: mt,
			Data:      src.Data,
		}
	}
	return out
}

// isMalformedBase64Image returns true when a block is an image with
// base64 source but missing the media_type field.
func isMalformedBase64Image(b *ContentBlock) bool {
	if b.Type != "image" || b.Source == nil || b.Source.Type != "base64" {
		return false
	}
	return b.Source.MediaType == ""
}

// ---------------------------------------------------------------------------
// DetectImageFormatFromBase64
// ---------------------------------------------------------------------------

// DetectImageFormatFromBase64 sniffs the image format from the first bytes
// of base64-encoded data. Returns a MIME type string, or "application/octet-stream"
// if the format cannot be determined.
func DetectImageFormatFromBase64(data string) string {
	// Decode enough bytes to check magic numbers. 16 bytes of decoded data
	// requires ~24 base64 chars.
	if len(data) < 8 {
		return "application/octet-stream"
	}
	prefix := data
	if len(prefix) > 24 {
		prefix = prefix[:24]
	}
	decoded, err := base64.StdEncoding.DecodeString(prefix)
	if err != nil {
		// Try with padding.
		for len(prefix)%4 != 0 {
			prefix += "="
		}
		decoded, err = base64.StdEncoding.DecodeString(prefix)
		if err != nil {
			return "application/octet-stream"
		}
	}

	switch {
	case len(decoded) >= 2 && decoded[0] == 0xFF && decoded[1] == 0xD8:
		return "image/jpeg"
	case len(decoded) >= 4 && decoded[0] == 0x89 && decoded[1] == 0x50 && decoded[2] == 0x4E && decoded[3] == 0x47:
		return "image/png"
	case len(decoded) >= 3 && decoded[0] == 0x47 && decoded[1] == 0x49 && decoded[2] == 0x46:
		return "image/gif"
	case len(decoded) >= 12 && decoded[0] == 0x52 && decoded[1] == 0x49 && decoded[2] == 0x46 && decoded[3] == 0x46 &&
		decoded[8] == 0x57 && decoded[9] == 0x45 && decoded[10] == 0x42 && decoded[11] == 0x50:
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
