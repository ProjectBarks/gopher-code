package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileWriteTool writes content to a file.
type FileWriteTool struct{}

type fileWriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (f *FileWriteTool) Name() string        { return "Write" }
func (f *FileWriteTool) Description() string { return "Write content to a file" }
func (f *FileWriteTool) IsReadOnly() bool    { return false }

func (f *FileWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "The path to the file to write"},
			"content": {"type": "string", "description": "The content to write to the file"}
		},
		"required": ["file_path", "content"],
		"additionalProperties": false
	}`)
}

// Source: FileWriteTool.ts:186-219
func (f *FileWriteTool) Execute(_ context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in fileWriteInput
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

	// Check if file exists — if so, require prior read (staleness guard)
	// Source: FileWriteTool.ts:198-219
	if _, statErr := os.Stat(path); statErr == nil {
		// File exists — check ReadFileState
		if tc.ReadFileState != nil {
			entry := tc.ReadFileState.Get(path)
			if entry == nil || entry.IsPartialView {
				// Source: FileWriteTool.ts:199-205
				return ErrorOutput("File has not been read yet. Read it first before writing to it."), nil
			}

			// Source: FileWriteTool.ts:211-218 — check mtime
			if info, err := os.Stat(path); err == nil {
				if info.ModTime().After(entry.Timestamp) {
					return ErrorOutput("File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."), nil
				}
			}
		}
	}
	// File doesn't exist — new file creation is always allowed
	// Source: FileWriteTool.ts:192-193

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to create directories: %s", err)), nil
	}

	if err := os.WriteFile(path, []byte(in.Content), 0644); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to write file: %s", err)), nil
	}

	// Update ReadFileState after successful write
	if tc.ReadFileState != nil {
		tc.ReadFileState.Record(path, in.Content, false)
	}

	return SuccessOutput(fmt.Sprintf("Successfully wrote to %s", path)), nil
}
