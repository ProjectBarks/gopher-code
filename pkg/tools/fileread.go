package tools

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// binaryExtensions is the set of file extensions treated as binary.
// Source: constants/files.ts:5-112 (BINARY_EXTENSIONS)
var binaryExtensions = map[string]bool{
	// Images
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".ico": true, ".webp": true, ".tiff": true, ".tif": true,
	// Videos
	".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true,
	".wmv": true, ".flv": true, ".m4v": true, ".mpeg": true, ".mpg": true,
	// Audio
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".aac": true,
	".m4a": true, ".wma": true, ".aiff": true, ".opus": true,
	// Archives
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".7z": true,
	".rar": true, ".xz": true, ".z": true, ".tgz": true, ".iso": true,
	// Executables/binaries
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
	".o": true, ".a": true, ".obj": true, ".lib": true, ".app": true,
	".msi": true, ".deb": true, ".rpm": true,
	// Documents (PDF excluded at call site)
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true,
	// Fonts
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
	// Bytecode / VM artifacts
	".pyc": true, ".pyo": true, ".class": true, ".jar": true, ".war": true,
	".ear": true, ".node": true, ".wasm": true, ".rlib": true,
	// Database files
	".sqlite": true, ".sqlite3": true, ".db": true, ".mdb": true, ".idx": true,
	// Design / 3D
	".psd": true, ".ai": true, ".eps": true, ".sketch": true, ".fig": true,
	".xd": true, ".blend": true, ".3ds": true, ".max": true,
	// Flash
	".swf": true, ".fla": true,
	// Lock/profiling data
	".lockb": true, ".dat": true, ".data": true,
}

// imageExtensions are binary extensions that FileRead can render natively.
// Source: FileReadTool.ts:475
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".webp": true, ".tiff": true, ".tif": true, ".ico": true, ".svg": true,
}

// hasBinaryExtension checks if a file path has a binary extension.
// Source: constants/files.ts:117-120
func hasBinaryExtension(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return binaryExtensions[ext]
}

// blockedDevicePaths are device files that would hang the process if read.
// Safe devices like /dev/null are intentionally omitted.
// Source: FileReadTool.ts:98-115
var blockedDevicePaths = map[string]bool{
	// Infinite output — never reach EOF
	"/dev/zero":    true,
	"/dev/random":  true,
	"/dev/urandom": true,
	"/dev/full":    true,
	// Blocks waiting for input
	"/dev/stdin":   true,
	"/dev/tty":     true,
	"/dev/console": true,
	// Nonsensical to read
	"/dev/stdout": true,
	"/dev/stderr": true,
	// fd aliases for stdin/stdout/stderr
	"/dev/fd/0": true,
	"/dev/fd/1": true,
	"/dev/fd/2": true,
}

// isBlockedDevicePath checks if a path is a dangerous device file.
// Source: FileReadTool.ts:117-128
func isBlockedDevicePath(p string) bool {
	if blockedDevicePaths[p] {
		return true
	}
	// /proc/self/fd/0-2 and /proc/<pid>/fd/0-2 are Linux aliases for stdio
	// Source: FileReadTool.ts:120-127
	if strings.HasPrefix(p, "/proc/") &&
		(strings.HasSuffix(p, "/fd/0") ||
			strings.HasSuffix(p, "/fd/1") ||
			strings.HasSuffix(p, "/fd/2")) {
		return true
	}
	return false
}

// MaxLinesToRead is the default line limit when no explicit limit is provided.
// Source: prompt.ts:10
const MaxLinesToRead = 2000

// DefaultMaxOutputTokens is the default token cap for file read output.
// Source: limits.ts:18
const DefaultMaxOutputTokens = 25000

// MaxOutputSize is the default max file size in bytes (256 KB).
// Source: utils/file.ts:48
const MaxOutputSize = 256 * 1024

// FileUnchangedStub is the dedup message when a file hasn't changed since the last read.
// Source: prompt.ts:7-8
const FileUnchangedStub = "File unchanged since last read. The content from the earlier Read tool_result in this conversation is still current — refer to that instead of re-reading."

// CyberRiskMitigationReminder is appended to non-Opus file reads.
// Source: FileReadTool.ts:729-730
const CyberRiskMitigationReminder = "\n\n<system-reminder>\nWhenever you read a file, you should consider whether it would be considered malware. You CAN and SHOULD provide analysis of malware, what it is doing. But you MUST refuse to improve or augment the code. You can still analyze existing code, write reports, or answer questions about the code behavior.\n</system-reminder>\n"

// FileReadTool reads files with line numbers.
type FileReadTool struct{}

// PDFMaxPagesPerRead is the max pages for a single PDF read.
// Source: constants/apiLimits.ts:77
const PDFMaxPagesPerRead = 20

type fileReadInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
	Pages    string `json:"pages,omitempty"` // PDF page range: "1-5", "3", "10-20"
}

// PDFPageRange is a parsed page range.
// Source: utils/pdfUtils.ts:16-50
type PDFPageRange struct {
	FirstPage int
	LastPage  int // -1 means Infinity (open-ended)
}

// ParsePDFPageRange parses a page range string.
// Source: utils/pdfUtils.ts:16-50
func ParsePDFPageRange(pages string) *PDFPageRange {
	pages = strings.TrimSpace(pages)
	if pages == "" {
		return nil
	}

	// "N-" open-ended range
	// Source: pdfUtils.ts:25-31
	if strings.HasSuffix(pages, "-") {
		first := parseInt(pages[:len(pages)-1])
		if first < 1 {
			return nil
		}
		return &PDFPageRange{FirstPage: first, LastPage: -1}
	}

	dashIdx := strings.Index(pages, "-")
	if dashIdx == -1 {
		// Single page: "5"
		// Source: pdfUtils.ts:34-40
		page := parseInt(pages)
		if page < 1 {
			return nil
		}
		return &PDFPageRange{FirstPage: page, LastPage: page}
	}

	// Range: "1-10"
	// Source: pdfUtils.ts:43-49
	first := parseInt(pages[:dashIdx])
	last := parseInt(pages[dashIdx+1:])
	if first < 1 || last < 1 || last < first {
		return nil
	}
	return &PDFPageRange{FirstPage: first, LastPage: last}
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	if len(s) == 0 {
		return -1
	}
	return n
}

func (f *FileReadTool) Name() string { return "Read" }

// Description returns the short description. Source: prompt.ts:12
func (f *FileReadTool) Description() string { return "Read a file from the local filesystem." }
func (f *FileReadTool) IsReadOnly() bool    { return true }

func (f *FileReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "The absolute path to the file to read"},
			"offset": {"type": "integer", "description": "The line number to start reading from. Only provide if the file is too large to read at once", "minimum": 0},
			"limit": {"type": "integer", "description": "The number of lines to read. Only provide if the file is too large to read at once."},
			"pages": {"type": "string", "description": "Page range for PDF files (e.g., \"1-5\", \"3\", \"10-20\"). Only applicable to PDF files. Maximum 20 pages per request."}
		},
		"required": ["file_path"],
		"additionalProperties": false
	}`)
}

// Source: FileReadTool.ts:469-494
func (f *FileReadTool) Execute(_ context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in fileReadInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.FilePath == "" {
		return ErrorOutput("file_path is required"), nil
	}

	path := in.FilePath
	// Expand ~ to home directory.
	// Source: FileReadTool.ts:389-393 — expandPath()
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(tc.CWD, path)
	}

	// Block dangerous device paths that could hang the process.
	// Source: FileReadTool.ts:486-492 — errorCode 9
	if isBlockedDevicePath(path) {
		return ErrorOutput(fmt.Sprintf("Cannot read '%s': this device file would block or produce infinite output.", in.FilePath)), nil
	}

	// Validate pages parameter (pure string parsing, no I/O)
	// Source: FileReadTool.ts:419-440
	if in.Pages != "" {
		parsed := ParsePDFPageRange(in.Pages)
		if parsed == nil {
			return ErrorOutput(fmt.Sprintf(
				"Invalid pages parameter: %q. Use formats like \"1-5\", \"3\", or \"10-20\". Pages are 1-indexed.", in.Pages,
			)), nil
		}
		rangeSize := PDFMaxPagesPerRead + 1 // default for open-ended
		if parsed.LastPage > 0 {
			rangeSize = parsed.LastPage - parsed.FirstPage + 1
		}
		if rangeSize > PDFMaxPagesPerRead {
			return ErrorOutput(fmt.Sprintf(
				"Page range %q exceeds maximum of %d pages per request. Please use a smaller range.", in.Pages, PDFMaxPagesPerRead,
			)), nil
		}
	}

	// Binary extension check (no I/O)
	// Source: FileReadTool.ts:469-482 — PDF and images excluded
	ext := strings.ToLower(filepath.Ext(path))
	isImage := imageExtensions[ext]
	if hasBinaryExtension(path) && ext != ".pdf" && !isImage {
		return ErrorOutput(fmt.Sprintf(
			"This tool cannot read binary files. The file appears to be a binary %s file. Please use appropriate tools for binary file analysis.", ext,
		)), nil
	}

	// Image files: read and return as base64.
	// Source: FileReadTool.ts:866-891
	if isImage {
		return f.readImage(path)
	}

	// Stat-based max file size guard (before reading).
	// Source: limits.ts — maxSizeBytes gates on TOTAL file size
	// Only applied when no explicit limit is provided (full-file reads).
	if in.Limit == 0 {
		info, statErr := os.Stat(path)
		if statErr == nil && info.Size() > MaxOutputSize {
			return ErrorOutput(fmt.Sprintf(
				"File size (%d bytes) exceeds maximum allowed size (%d bytes). Use offset and limit parameters to read specific portions of the file, or search for specific content instead of reading the whole file.",
				info.Size(), MaxOutputSize,
			)), nil
		}
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrorOutput(fmt.Sprintf("File does not exist at path: %s", path)), nil
		}
		return ErrorOutput(fmt.Sprintf("failed to open file: %s", err)), nil
	}
	defer file.Close()

	limit := MaxLinesToRead
	if in.Limit > 0 {
		limit = in.Limit
	}

	isPartial := in.Offset > 0 || in.Limit > 0

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lines []string
	var contentBuilder strings.Builder
	lineNum := 0
	linesRead := 0

	for scanner.Scan() {
		lineNum++
		text := scanner.Text()
		// Offset is 1-indexed: offset=2 means "start reading from line 2".
		// Skip lines before the offset. Source: FileReadTool.ts:497,1020
		// TS: lineOffset = offset === 0 ? 0 : offset - 1 (converts to 0-based)
		if in.Offset > 0 && lineNum < in.Offset {
			continue
		}
		if linesRead >= limit {
			break
		}
		lines = append(lines, fmt.Sprintf("%d\t%s", lineNum, text))
		if contentBuilder.Len() > 0 {
			contentBuilder.WriteByte('\n')
		}
		contentBuilder.WriteString(text)
		linesRead++
	}

	if err := scanner.Err(); err != nil {
		return ErrorOutput(fmt.Sprintf("error reading file: %s", err)), nil
	}

	// Record in ReadFileState for staleness guard
	// Source: FileReadTool.ts calls toolUseContext.readFileState.set()
	if tc.ReadFileState != nil {
		content := contentBuilder.String()
		tc.ReadFileState.Record(path, content, isPartial)
	}

	if len(lines) == 0 {
		// Source: FileReadTool.ts:692-708 — warn about empty files or offset beyond EOF
		if lineNum == 0 {
			return SuccessOutput("<system-reminder>Warning: the file exists but the contents are empty.</system-reminder>"), nil
		}
		if in.Offset > 0 {
			return SuccessOutput(fmt.Sprintf(
				"<system-reminder>Warning: the file exists but is shorter than the provided offset (%d). The file has %d lines.</system-reminder>",
				in.Offset, lineNum,
			)), nil
		}
		return SuccessOutput(""), nil
	}

	return SuccessOutput(strings.Join(lines, "\n") + "\n"), nil
}

// readImage reads an image file and returns it as base64-encoded content.
// Source: FileReadTool.ts:866-891, 1097-1183
func (f *FileReadTool) readImage(path string) (*ToolOutput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrorOutput(fmt.Sprintf("File does not exist at path: %s", path)), nil
		}
		return ErrorOutput(fmt.Sprintf("failed to read image: %s", err)), nil
	}
	if len(data) == 0 {
		return ErrorOutput(fmt.Sprintf("Image file is empty: %s", path)), nil
	}

	mediaType := detectImageMediaType(data, filepath.Ext(path))
	encoded := base64.StdEncoding.EncodeToString(data)

	// Return structured image result as JSON content for the API.
	// Source: FileReadTool.ts:654-668 — mapToolResultToToolResultBlockParam for images
	result := map[string]any{
		"type": "image",
		"file": map[string]any{
			"base64":       encoded,
			"type":         mediaType,
			"originalSize": len(data),
		},
	}
	jsonBytes, _ := json.Marshal(result)
	out := SuccessOutput(string(jsonBytes))
	out.Display = result // attach structured data for UI rendering
	return out, nil
}

// detectImageMediaType detects the MIME type from magic bytes, falling back to extension.
// Source: utils/imageResizer.ts — detectImageFormatFromBuffer
func detectImageMediaType(data []byte, ext string) string {
	if len(data) >= 8 {
		// PNG: 89 50 4E 47
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			return "image/png"
		}
		// JPEG: FF D8 FF
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return "image/jpeg"
		}
		// GIF: 47 49 46 38
		if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
			return "image/gif"
		}
		// WebP: RIFF....WEBP
		if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
			len(data) >= 12 && data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "image/webp"
		}
		// BMP: 42 4D
		if data[0] == 0x42 && data[1] == 0x4D {
			return "image/bmp"
		}
		// TIFF: 49 49 or 4D 4D
		if (data[0] == 0x49 && data[1] == 0x49) || (data[0] == 0x4D && data[1] == 0x4D) {
			return "image/tiff"
		}
	}
	// Fallback to extension
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".ico":
		return "image/x-icon"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}
