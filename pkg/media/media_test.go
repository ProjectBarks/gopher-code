package media

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectMediaType(t *testing.T) {
	// PNG header
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	if got := DetectMediaType(png); got != MediaPNG {
		t.Errorf("PNG = %q, want image/png", got)
	}

	// JPEG header
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0, 0, 0, 0, 0, 0}
	if got := DetectMediaType(jpeg); got != MediaJPEG {
		t.Errorf("JPEG = %q, want image/jpeg", got)
	}

	// Not an image
	text := []byte("hello world this is plain text")
	if got := DetectMediaType(text); got != "" {
		t.Errorf("text = %q, want empty", got)
	}
}

func TestDetectMediaTypeFromPath(t *testing.T) {
	tests := map[string]ImageMediaType{
		"photo.png":    MediaPNG,
		"photo.jpg":    MediaJPEG,
		"photo.jpeg":   MediaJPEG,
		"anim.gif":     MediaGIF,
		"modern.webp":  MediaWebP,
		"doc.pdf":      "",
		"readme.md":    "",
	}
	for path, want := range tests {
		if got := DetectMediaTypeFromPath(path); got != want {
			t.Errorf("DetectMediaTypeFromPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestValidateBase64Size(t *testing.T) {
	small := strings.Repeat("A", 100)
	if err := ValidateBase64Size(small); err != nil {
		t.Errorf("small image should pass: %v", err)
	}

	big := strings.Repeat("A", APIImageMaxBase64Size+1)
	if err := ValidateBase64Size(big); err == nil {
		t.Error("oversized image should fail")
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes int
		want  string
	}{
		{500, "500 B"},
		{2048, "2.0 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
	}
	for _, tt := range tests {
		got := FormatFileSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestEncodeFileToBase64(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	content := []byte{1, 2, 3, 4, 5}
	os.WriteFile(path, content, 0644)

	b64, err := EncodeFileToBase64(path)
	if err != nil {
		t.Fatalf("EncodeFileToBase64 error: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if string(decoded) != string(content) {
		t.Error("round-trip mismatch")
	}
}

func TestReadImageInfo(t *testing.T) {
	dir := t.TempDir()
	// Write a minimal PNG (just the header — enough for content sniffing)
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0, 0, 0, 13, 'I', 'H', 'D', 'R', 0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0}
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, pngHeader, 0644)

	info, err := ReadImageInfo(path)
	if err != nil {
		t.Fatalf("ReadImageInfo error: %v", err)
	}
	if info.MediaType != MediaPNG {
		t.Errorf("MediaType = %q, want image/png", info.MediaType)
	}
	if info.RawSize != len(pngHeader) {
		t.Errorf("RawSize = %d, want %d", info.RawSize, len(pngHeader))
	}
	if info.NeedsResize() {
		t.Error("small image should not need resize")
	}
}
