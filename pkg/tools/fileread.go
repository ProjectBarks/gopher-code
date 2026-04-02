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

// FileReadTool reads files with line numbers.
type FileReadTool struct{}

type fileReadInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

func (f *FileReadTool) Name() string        { return "Read" }
func (f *FileReadTool) Description() string { return "Read a file from the filesystem" }
func (f *FileReadTool) IsReadOnly() bool    { return true }

func (f *FileReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "The path to the file to read"},
			"offset": {"type": "integer", "description": "Line number to start reading from (0-based)"},
			"limit": {"type": "integer", "description": "Maximum number of lines to read (default 2000)"}
		},
		"required": ["file_path"],
		"additionalProperties": false
	}`)
}

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

	file, err := os.Open(path)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to open file: %s", err)), nil
	}
	defer file.Close()

	limit := 2000
	if in.Limit > 0 {
		limit = in.Limit
	}

	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lines []string
	lineNum := 0
	linesRead := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= in.Offset {
			continue
		}
		if linesRead >= limit {
			break
		}
		lines = append(lines, fmt.Sprintf("%d\t%s", lineNum, scanner.Text()))
		linesRead++
	}

	if err := scanner.Err(); err != nil {
		return ErrorOutput(fmt.Sprintf("error reading file: %s", err)), nil
	}

	if len(lines) == 0 {
		return SuccessOutput(""), nil
	}

	return SuccessOutput(strings.Join(lines, "\n") + "\n"), nil
}
