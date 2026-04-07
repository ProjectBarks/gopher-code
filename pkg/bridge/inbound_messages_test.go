package bridge

import (
	"encoding/base64"
	"testing"
)

// ---------------------------------------------------------------------------
// ExtractInboundMessageFields — user message with string content
// ---------------------------------------------------------------------------

func TestExtractInboundMessageFields_StringContent(t *testing.T) {
	data := `{"type":"user","message":{"content":"hello world"},"uuid":"abc-123"}`
	fields, err := ExtractInboundMessageFields([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fields == nil {
		t.Fatal("expected non-nil fields")
	}
	if !fields.IsString() {
		t.Fatal("expected string content")
	}
	if fields.ContentString != "hello world" {
		t.Errorf("content = %q, want %q", fields.ContentString, "hello world")
	}
	if fields.UUID != "abc-123" {
		t.Errorf("uuid = %q, want %q", fields.UUID, "abc-123")
	}
}

// ---------------------------------------------------------------------------
// ExtractInboundMessageFields — user message with block array content
// ---------------------------------------------------------------------------

func TestExtractInboundMessageFields_BlockContent(t *testing.T) {
	data := `{"type":"user","message":{"content":[{"type":"text","text":"describe this"}]}}`
	fields, err := ExtractInboundMessageFields([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fields == nil {
		t.Fatal("expected non-nil fields")
	}
	if fields.IsString() {
		t.Fatal("expected block content, got string")
	}
	if len(fields.ContentBlocks) != 1 {
		t.Fatalf("len(blocks) = %d, want 1", len(fields.ContentBlocks))
	}
	if fields.ContentBlocks[0].Text != "describe this" {
		t.Errorf("block text = %q", fields.ContentBlocks[0].Text)
	}
	if fields.UUID != "" {
		t.Errorf("uuid = %q, want empty", fields.UUID)
	}
}

// ---------------------------------------------------------------------------
// Non-user message → nil (skipped)
// ---------------------------------------------------------------------------

func TestExtractInboundMessageFields_NonUserType(t *testing.T) {
	for _, msgType := range []string{"assistant", "result", "system"} {
		data := `{"type":"` + msgType + `","message":{"content":"hi"}}`
		fields, err := ExtractInboundMessageFields([]byte(data))
		if err != nil {
			t.Fatalf("type=%s: unexpected error: %v", msgType, err)
		}
		if fields != nil {
			t.Errorf("type=%s: expected nil, got %+v", msgType, fields)
		}
	}
}

// ---------------------------------------------------------------------------
// Missing/empty content → nil
// ---------------------------------------------------------------------------

func TestExtractInboundMessageFields_MissingContent(t *testing.T) {
	data := `{"type":"user","message":{}}`
	fields, err := ExtractInboundMessageFields([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fields != nil {
		t.Error("expected nil for missing content")
	}
}

func TestExtractInboundMessageFields_EmptyArrayContent(t *testing.T) {
	data := `{"type":"user","message":{"content":[]}}`
	fields, err := ExtractInboundMessageFields([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fields != nil {
		t.Error("expected nil for empty array content")
	}
}

func TestExtractInboundMessageFields_MissingMessage(t *testing.T) {
	data := `{"type":"user"}`
	fields, err := ExtractInboundMessageFields([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fields != nil {
		t.Error("expected nil for missing message")
	}
}

// ---------------------------------------------------------------------------
// Malformed JSON → error
// ---------------------------------------------------------------------------

func TestExtractInboundMessageFields_MalformedJSON(t *testing.T) {
	_, err := ExtractInboundMessageFields([]byte(`{not valid json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestExtractInboundMessageFields_ContentNeitherStringNorArray(t *testing.T) {
	data := `{"type":"user","message":{"content":42}}`
	_, err := ExtractInboundMessageFields([]byte(data))
	if err == nil {
		t.Fatal("expected error for numeric content")
	}
}

// ---------------------------------------------------------------------------
// UUID extraction — present and absent
// ---------------------------------------------------------------------------

func TestExtractInboundMessageFields_UUIDPresent(t *testing.T) {
	data := `{"type":"user","message":{"content":"hi"},"uuid":"550e8400-e29b-41d4-a716-446655440000"}`
	fields, err := ExtractInboundMessageFields([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if fields.UUID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("uuid = %q", fields.UUID)
	}
}

func TestExtractInboundMessageFields_UUIDAbsent(t *testing.T) {
	data := `{"type":"user","message":{"content":"hi"}}`
	fields, err := ExtractInboundMessageFields([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if fields.UUID != "" {
		t.Errorf("uuid = %q, want empty", fields.UUID)
	}
}

// ---------------------------------------------------------------------------
// NormalizeImageBlocks — no-op fast path (zero allocation)
// ---------------------------------------------------------------------------

func TestNormalizeImageBlocks_NoOpFastPath(t *testing.T) {
	blocks := []ContentBlock{
		{Type: "text", Text: "hello"},
		{
			Type: "image",
			Source: &Base64ImageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      "iVBORw0KGgo=",
			},
		},
	}
	result := NormalizeImageBlocks(blocks)
	// Fast path: should return same slice.
	if &result[0] != &blocks[0] {
		t.Error("expected same slice reference on fast path")
	}
}

// ---------------------------------------------------------------------------
// NormalizeImageBlocks — camelCase mediaType fallback
// ---------------------------------------------------------------------------

func TestNormalizeImageBlocks_CamelCaseMediaType(t *testing.T) {
	// Simulate a block with mediaType (camelCase) but no media_type.
	blocks := []ContentBlock{
		{
			Type: "image",
			Source: &Base64ImageSource{
				Type:           "base64",
				MediaType:      "", // missing
				Data:           "iVBORw0KGgo=",
				mediaTypeCamel: "image/png",
			},
		},
	}
	result := NormalizeImageBlocks(blocks)
	if result[0].Source.MediaType != "image/png" {
		t.Errorf("media_type = %q, want image/png", result[0].Source.MediaType)
	}
}

// ---------------------------------------------------------------------------
// NormalizeImageBlocks — last-resort format detection
// ---------------------------------------------------------------------------

func TestNormalizeImageBlocks_FormatDetectionFallback(t *testing.T) {
	// PNG magic bytes: 89 50 4E 47
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	pngB64 := base64.StdEncoding.EncodeToString(pngMagic)
	blocks := []ContentBlock{
		{
			Type: "image",
			Source: &Base64ImageSource{
				Type: "base64",
				Data: pngB64,
				// No media_type, no mediaType → should sniff as PNG.
			},
		},
	}
	result := NormalizeImageBlocks(blocks)
	if result[0].Source.MediaType != "image/png" {
		t.Errorf("media_type = %q, want image/png", result[0].Source.MediaType)
	}
}

// ---------------------------------------------------------------------------
// isMalformedBase64Image predicate
// ---------------------------------------------------------------------------

func TestIsMalformedBase64Image_True(t *testing.T) {
	b := &ContentBlock{
		Type:   "image",
		Source: &Base64ImageSource{Type: "base64", Data: "abc"},
	}
	if !isMalformedBase64Image(b) {
		t.Error("expected true for image+base64+missing media_type")
	}
}

func TestIsMalformedBase64Image_FalseWhenMediaTypePresent(t *testing.T) {
	b := &ContentBlock{
		Type:   "image",
		Source: &Base64ImageSource{Type: "base64", MediaType: "image/jpeg", Data: "abc"},
	}
	if isMalformedBase64Image(b) {
		t.Error("expected false when media_type is present")
	}
}

func TestIsMalformedBase64Image_FalseForTextBlock(t *testing.T) {
	b := &ContentBlock{Type: "text", Text: "hi"}
	if isMalformedBase64Image(b) {
		t.Error("expected false for text block")
	}
}

func TestIsMalformedBase64Image_FalseForNonBase64Source(t *testing.T) {
	b := &ContentBlock{
		Type:   "image",
		Source: &Base64ImageSource{Type: "url", Data: "https://example.com/img.png"},
	}
	if isMalformedBase64Image(b) {
		t.Error("expected false for non-base64 source type")
	}
}

// ---------------------------------------------------------------------------
// DetectImageFormatFromBase64
// ---------------------------------------------------------------------------

func TestDetectImageFormatFromBase64_JPEG(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10})
	if got := DetectImageFormatFromBase64(data); got != "image/jpeg" {
		t.Errorf("got %q, want image/jpeg", got)
	}
}

func TestDetectImageFormatFromBase64_PNG(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	if got := DetectImageFormatFromBase64(data); got != "image/png" {
		t.Errorf("got %q, want image/png", got)
	}
}

func TestDetectImageFormatFromBase64_GIF(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61})
	if got := DetectImageFormatFromBase64(data); got != "image/gif" {
		t.Errorf("got %q, want image/gif", got)
	}
}

func TestDetectImageFormatFromBase64_WebP(t *testing.T) {
	// RIFF....WEBP
	data := base64.StdEncoding.EncodeToString([]byte{
		0x52, 0x49, 0x46, 0x46,
		0x00, 0x00, 0x00, 0x00,
		0x57, 0x45, 0x42, 0x50,
	})
	if got := DetectImageFormatFromBase64(data); got != "image/webp" {
		t.Errorf("got %q, want image/webp", got)
	}
}

func TestDetectImageFormatFromBase64_Unknown(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05})
	if got := DetectImageFormatFromBase64(data); got != "application/octet-stream" {
		t.Errorf("got %q, want application/octet-stream", got)
	}
}

func TestDetectImageFormatFromBase64_TooShort(t *testing.T) {
	if got := DetectImageFormatFromBase64("YQ=="); got != "application/octet-stream" {
		t.Errorf("got %q, want application/octet-stream", got)
	}
}

// ---------------------------------------------------------------------------
// Base64ImageSource custom UnmarshalJSON — both key variants
// ---------------------------------------------------------------------------

func TestBase64ImageSourceUnmarshal_SnakeCase(t *testing.T) {
	data := `{"type":"base64","media_type":"image/jpeg","data":"abc"}`
	var src Base64ImageSource
	if err := src.UnmarshalJSON([]byte(data)); err != nil {
		t.Fatal(err)
	}
	if src.MediaType != "image/jpeg" {
		t.Errorf("MediaType = %q", src.MediaType)
	}
}

func TestBase64ImageSourceUnmarshal_CamelCase(t *testing.T) {
	data := `{"type":"base64","mediaType":"image/png","data":"abc"}`
	var src Base64ImageSource
	if err := src.UnmarshalJSON([]byte(data)); err != nil {
		t.Fatal(err)
	}
	// camelCase goes to the private fallback field.
	if src.MediaType != "" {
		t.Errorf("MediaType should be empty, got %q", src.MediaType)
	}
	if src.mediaTypeCamel != "image/png" {
		t.Errorf("mediaTypeCamel = %q, want image/png", src.mediaTypeCamel)
	}
}

func TestBase64ImageSourceUnmarshal_BothKeys(t *testing.T) {
	// If both are present, media_type (snake_case) wins.
	data := `{"type":"base64","media_type":"image/jpeg","mediaType":"image/png","data":"abc"}`
	var src Base64ImageSource
	if err := src.UnmarshalJSON([]byte(data)); err != nil {
		t.Fatal(err)
	}
	if src.MediaType != "image/jpeg" {
		t.Errorf("MediaType = %q, want image/jpeg", src.MediaType)
	}
}

// ---------------------------------------------------------------------------
// End-to-end: inbound message with camelCase image block
// ---------------------------------------------------------------------------

func TestExtractInboundMessageFields_NormalizesImageBlocks(t *testing.T) {
	data := `{
		"type": "user",
		"message": {
			"content": [
				{"type": "text", "text": "what is this?"},
				{"type": "image", "source": {"type": "base64", "mediaType": "image/jpeg", "data": "/9j/4AAQ"}}
			]
		},
		"uuid": "msg-1"
	}`
	fields, err := ExtractInboundMessageFields([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fields == nil {
		t.Fatal("expected non-nil fields")
	}
	if len(fields.ContentBlocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(fields.ContentBlocks))
	}
	imgBlock := fields.ContentBlocks[1]
	if imgBlock.Source == nil {
		t.Fatal("image source is nil")
	}
	if imgBlock.Source.MediaType != "image/jpeg" {
		t.Errorf("media_type = %q, want image/jpeg", imgBlock.Source.MediaType)
	}
}

// ---------------------------------------------------------------------------
// Integration: table-driven round-trip from raw JSON to typed InboundFields
// ---------------------------------------------------------------------------

func TestExtractInboundMessageFields_Integration(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantNil   bool
		wantErr   bool
		wantStr   string
		wantUUID  string
		wantBlock int // expected len(ContentBlocks); -1 = don't check
	}{
		{
			name:     "string content with uuid",
			json:     `{"type":"user","message":{"content":"run tests"},"uuid":"u1"}`,
			wantStr:  "run tests",
			wantUUID: "u1",
		},
		{
			name:      "text block array",
			json:      `{"type":"user","message":{"content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}}`,
			wantBlock: 2,
		},
		{
			name:    "assistant message skipped",
			json:    `{"type":"assistant","message":{"content":"ignored"}}`,
			wantNil: true,
		},
		{
			name:    "empty string content skipped",
			json:    `{"type":"user","message":{"content":""}}`,
			wantNil: true,
		},
		{
			name:    "null message field skipped",
			json:    `{"type":"user","message":null}`,
			wantNil: true,
		},
		{
			name:    "boolean content errors",
			json:    `{"type":"user","message":{"content":true}}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ExtractInboundMessageFields([]byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if fields != nil {
					t.Fatalf("expected nil, got %+v", fields)
				}
				return
			}
			if fields == nil {
				t.Fatal("expected non-nil fields")
			}
			if tt.wantStr != "" && fields.ContentString != tt.wantStr {
				t.Errorf("content = %q, want %q", fields.ContentString, tt.wantStr)
			}
			if tt.wantUUID != "" && fields.UUID != tt.wantUUID {
				t.Errorf("uuid = %q, want %q", fields.UUID, tt.wantUUID)
			}
			if tt.wantBlock >= 0 && len(fields.ContentBlocks) != tt.wantBlock {
				t.Errorf("len(blocks) = %d, want %d", len(fields.ContentBlocks), tt.wantBlock)
			}
		})
	}
}
