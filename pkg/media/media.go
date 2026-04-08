// Package media provides image and media processing utilities.
// Source: utils/imageResizer.ts, utils/imageValidation.ts, utils/imageStore.ts,
//         constants/apiLimits.ts (image limits)
package media

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// API and image size limits matching the TS constants.
// Source: constants/apiLimits.ts
const (
	APIImageMaxBase64Size = 5 * 1024 * 1024         // 5 MB base64 string
	ImageTargetRawSize    = APIImageMaxBase64Size * 3 / 4 // ~3.75 MB decoded
	ImageMaxWidth         = 2000
	ImageMaxHeight        = 2000
)

// ImageMediaType is the MIME type for images the API accepts.
type ImageMediaType string

const (
	MediaPNG  ImageMediaType = "image/png"
	MediaJPEG ImageMediaType = "image/jpeg"
	MediaGIF  ImageMediaType = "image/gif"
	MediaWebP ImageMediaType = "image/webp"
)

// SupportedImageTypes are the media types accepted by the Anthropic API.
var SupportedImageTypes = map[ImageMediaType]bool{
	MediaPNG: true, MediaJPEG: true, MediaGIF: true, MediaWebP: true,
}

// DetectMediaType sniffs the media type from file content.
// Returns "" if the type is not a supported image.
func DetectMediaType(data []byte) ImageMediaType {
	ct := http.DetectContentType(data)
	switch {
	case strings.HasPrefix(ct, "image/png"):
		return MediaPNG
	case strings.HasPrefix(ct, "image/jpeg"):
		return MediaJPEG
	case strings.HasPrefix(ct, "image/gif"):
		return MediaGIF
	case strings.HasPrefix(ct, "image/webp"):
		return MediaWebP
	default:
		return ""
	}
}

// DetectMediaTypeFromPath determines image type by extension.
func DetectMediaTypeFromPath(path string) ImageMediaType {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return MediaPNG
	case ".jpg", ".jpeg":
		return MediaJPEG
	case ".gif":
		return MediaGIF
	case ".webp":
		return MediaWebP
	default:
		return ""
	}
}

// ValidateBase64Size checks if a base64-encoded image string is within the API limit.
// Returns an error if oversized.
// Source: utils/imageValidation.ts — validateMessageImages
func ValidateBase64Size(b64 string) error {
	if len(b64) > APIImageMaxBase64Size {
		return fmt.Errorf("image base64 size (%s) exceeds API limit (%s); please resize the image",
			FormatFileSize(len(b64)), FormatFileSize(APIImageMaxBase64Size))
	}
	return nil
}

// EncodeFileToBase64 reads a file and returns its base64-encoded content.
func EncodeFileToBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// FormatFileSize returns a human-readable file size string.
func FormatFileSize(bytes int) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// ImageInfo holds metadata about an image.
type ImageInfo struct {
	Path      string
	MediaType ImageMediaType
	RawSize   int
	Base64Len int
}

// ReadImageInfo reads an image file and returns its metadata.
func ReadImageInfo(path string) (*ImageInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	mt := DetectMediaType(data)
	if mt == "" {
		mt = DetectMediaTypeFromPath(path)
	}
	if mt == "" {
		return nil, fmt.Errorf("unsupported image format: %s", path)
	}

	b64Len := base64.StdEncoding.EncodedLen(len(data))
	return &ImageInfo{
		Path:      path,
		MediaType: mt,
		RawSize:   len(data),
		Base64Len: b64Len,
	}, nil
}

// NeedsResize returns true if the image exceeds API limits.
func (info *ImageInfo) NeedsResize() bool {
	return info.Base64Len > APIImageMaxBase64Size
}
