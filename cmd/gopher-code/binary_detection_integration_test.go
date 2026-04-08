package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// T372: Binary content detection
func TestBinaryDetection_WiredIntoBinary(t *testing.T) {
	// Extension detection.
	if !tools.HasBinaryExtension("image.png") {
		t.Error("should detect .png as binary")
	}
	if !tools.HasBinaryExtension("archive.tar.gz") {
		t.Error("should detect .gz as binary")
	}
	if tools.HasBinaryExtension("code.go") {
		t.Error("should not detect .go as binary")
	}
	if tools.HasBinaryExtension("readme.md") {
		t.Error("should not detect .md as binary")
	}

	// Content detection.
	if tools.IsBinaryContent([]byte("hello world\n")) {
		t.Error("text should not be detected as binary")
	}
	if !tools.IsBinaryContent([]byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00}) {
		t.Error("PNG header with null byte should be detected as binary")
	}
	if tools.IsBinaryContent(nil) {
		t.Error("empty data should not be binary")
	}
}
