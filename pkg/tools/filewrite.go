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

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to create directories: %s", err)), nil
	}

	if err := os.WriteFile(path, []byte(in.Content), 0644); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to write file: %s", err)), nil
	}

	return SuccessOutput(fmt.Sprintf("Successfully wrote to %s", path)), nil
}
