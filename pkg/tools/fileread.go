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

// FileReadTool reads files with line numbers.
type FileReadTool struct{}

type fileReadInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
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
			"limit": {"type": "integer", "description": "The number of lines to read. Only provide if the file is too large to read at once."}
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
	if !filepath.IsAbs(path) {
		path = filepath.Join(tc.CWD, path)
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
		if lineNum <= in.Offset {
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
		return SuccessOutput(""), nil
	}

	return SuccessOutput(strings.Join(lines, "\n") + "\n"), nil
}
