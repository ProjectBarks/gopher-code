package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Source: utils/mcpOutputStorage.ts, utils/toolResultStorage.ts

// PreviewSizeBytes is the number of bytes to include as a preview for large results.
const PreviewSizeBytes = 2000

// GetLargeOutputInstructions generates instruction text for Claude to read from a
// saved output file.
// Source: utils/mcpOutputStorage.ts:39-58
func GetLargeOutputInstructions(rawOutputPath string, contentLength int, formatDescription string) string {
	return fmt.Sprintf(
		"Error: result (%d characters) exceeds maximum allowed tokens. Output has been saved to %s.\n"+
			"Format: %s\n"+
			"Use offset and limit parameters to read specific portions of the file, search within it for specific content, and jq to make structured queries.\n"+
			"REQUIREMENTS FOR SUMMARIZATION/ANALYSIS/REVIEW:\n"+
			"- You MUST read the content from the file at %s in sequential chunks until 100%% of the content has been read.\n"+
			"- If you receive truncation warnings when reading the file, reduce the chunk size until you have read 100%% of the content without truncation.\n"+
			"- Before producing ANY summary or analysis, you MUST explicitly describe what portion of the content you have read. ***If you did not read the entire content, you MUST explicitly state this.***\n",
		contentLength, rawOutputPath, formatDescription, rawOutputPath,
	)
}

// ExtensionForMimeType maps a MIME type to a file extension.
// Source: utils/mcpOutputStorage.ts:66-118
func ExtensionForMimeType(mimeType string) string {
	if mimeType == "" {
		return "bin"
	}
	// Strip charset/boundary parameters
	mt := strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])
	mt = strings.ToLower(mt)

	switch mt {
	case "application/pdf":
		return "pdf"
	case "application/json":
		return "json"
	case "text/csv":
		return "csv"
	case "text/plain":
		return "txt"
	case "text/html":
		return "html"
	case "text/markdown":
		return "md"
	case "application/zip":
		return "zip"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return "pptx"
	case "application/msword":
		return "doc"
	case "application/vnd.ms-excel":
		return "xls"
	case "audio/mpeg":
		return "mp3"
	case "audio/wav":
		return "wav"
	case "audio/ogg":
		return "ogg"
	case "video/mp4":
		return "mp4"
	case "video/webm":
		return "webm"
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	default:
		return "bin"
	}
}

// IsBinaryContentType checks if a content type indicates binary content.
// Source: utils/mcpOutputStorage.ts:125-136
func IsBinaryContentType(contentType string) bool {
	if contentType == "" {
		return false
	}
	mt := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	mt = strings.ToLower(mt)

	if strings.HasPrefix(mt, "text/") {
		return false
	}
	if strings.HasSuffix(mt, "+json") || mt == "application/json" {
		return false
	}
	if strings.HasSuffix(mt, "+xml") || mt == "application/xml" {
		return false
	}
	if strings.HasPrefix(mt, "application/javascript") {
		return false
	}
	if mt == "application/x-www-form-urlencoded" {
		return false
	}
	return true
}

// PersistBinaryResult is the outcome of writing binary content to disk.
// Source: utils/mcpOutputStorage.ts:138-140
type PersistBinaryResult struct {
	Filepath string
	Size     int
	Ext      string
	Error    string // non-empty on failure
}

// PersistBinaryContent writes raw binary bytes to the tool-results directory.
// Source: utils/mcpOutputStorage.ts:148-174
func PersistBinaryContent(bytes []byte, mimeType, persistID, toolResultsDir string) PersistBinaryResult {
	if err := os.MkdirAll(toolResultsDir, 0755); err != nil {
		return PersistBinaryResult{Error: fmt.Sprintf("create dir: %s", err)}
	}
	ext := ExtensionForMimeType(mimeType)
	fp := filepath.Join(toolResultsDir, persistID+"."+ext)

	if err := os.WriteFile(fp, bytes, 0644); err != nil {
		return PersistBinaryResult{Error: err.Error()}
	}
	return PersistBinaryResult{Filepath: fp, Size: len(bytes), Ext: ext}
}

// PersistLargeOutput writes a large text result to disk and returns instructions.
// Source: utils/toolResultStorage.ts + mcpOutputStorage.ts
func PersistLargeOutput(content, persistID, toolResultsDir, formatDescription string) (string, error) {
	if err := os.MkdirAll(toolResultsDir, 0755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}
	fp := filepath.Join(toolResultsDir, persistID+".txt")

	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		return "", err
	}

	// Return instructions with preview
	instructions := GetLargeOutputInstructions(fp, len(content), formatDescription)
	if len(content) > PreviewSizeBytes {
		instructions += fmt.Sprintf("\nPreview (first %d bytes):\n%s\n...\n", PreviewSizeBytes, content[:PreviewSizeBytes])
	}
	return instructions, nil
}
