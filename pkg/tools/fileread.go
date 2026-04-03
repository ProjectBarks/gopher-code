package tools

import (
	"bufio"
	"context"
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
// Source: FileReadTool.ts:98-128
var blockedDevicePaths = map[string]bool{
	"/dev/zero":    true,
	"/dev/null":    true,
	"/dev/random":  true,
	"/dev/urandom": true,
	"/dev/stdin":   true,
	"/dev/stdout":  true,
	"/dev/stderr":  true,
	"/dev/tty":     true,
	"/dev/fd/0":    true,
	"/dev/fd/1":    true,
	"/dev/fd/2":    true,
}

// isBlockedDevicePath checks if a path is a dangerous device file.
func isBlockedDevicePath(path string) bool {
	return blockedDevicePaths[path]
}

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

func (f *FileReadTool) Name() string        { return "Read" }
func (f *FileReadTool) Description() string { return "Reads a file from the local filesystem." }
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
	// Source: FileReadTool.ts:98-128 — blockedDevicePaths
	if isBlockedDevicePath(path) {
		return ErrorOutput(fmt.Sprintf("Cannot read %s: this path could cause the process to hang.", path)), nil
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
	// Source: FileReadTool.ts:472-482 — PDF and images excluded
	ext := strings.ToLower(filepath.Ext(path))
	if hasBinaryExtension(path) && ext != ".pdf" && !imageExtensions[ext] {
		return ErrorOutput(fmt.Sprintf(
			"This tool cannot read binary files. The file appears to be a binary %s file. Please use appropriate tools for binary file analysis.", ext,
		)), nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrorOutput(fmt.Sprintf("File does not exist at path: %s", path)), nil
		}
		return ErrorOutput(fmt.Sprintf("failed to open file: %s", err)), nil
	}
	defer file.Close()

	limit := 2000
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
