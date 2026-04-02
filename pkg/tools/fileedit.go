package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileEdit error codes matching TS FileEditTool.ts.
// Source: FileEditTool.ts:146-341
const (
	EditErrSameStrings   = 1  // old_string == new_string
	EditErrDeniedByPerms = 2  // path denied by permission settings
	EditErrFileExists    = 3  // file exists, old_string empty (create attempt on non-empty)
	EditErrFileNotFound  = 4  // file does not exist
	EditErrNotebookFile  = 5  // .ipynb — use NotebookEdit
	EditErrNotReadYet    = 6  // file not read yet
	EditErrModifiedSince = 7  // file modified since last read
	EditErrStringNotFound = 8 // old_string not found in file
	EditErrMultipleMatch = 9  // multiple matches, replace_all=false
	EditErrFileTooLarge  = 10 // file too large
)

// MaxEditFileSize is the maximum file size for editing (10MB).
const MaxEditFileSize = 10 * 1024 * 1024

// FileEditTool performs string replacements in files.
type FileEditTool struct{}

type fileEditInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func (f *FileEditTool) Name() string        { return "Edit" }
func (f *FileEditTool) Description() string { return "Performs exact string replacements in files." }
func (f *FileEditTool) IsReadOnly() bool    { return false }

// Source: FileEditTool.ts inputSchema
func (f *FileEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "The absolute path to the file to modify"},
			"old_string": {"type": "string", "description": "The text to replace"},
			"new_string": {"type": "string", "description": "The text to replace it with (must be different from old_string)"},
			"replace_all": {"type": "boolean", "description": "Replace all occurrences of old_string (default false)", "default": false}
		},
		"required": ["file_path", "old_string", "new_string"],
		"additionalProperties": false
	}`)
}

// Source: FileEditTool.ts:137-355
func (f *FileEditTool) Execute(_ context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in fileEditInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.FilePath == "" {
		return ErrorOutput("file_path is required"), nil
	}

	// Error code 1: old_string == new_string
	// Source: FileEditTool.ts:148-155
	if in.OldString == in.NewString {
		return ErrorOutput("No changes to make: old_string and new_string are exactly the same."), nil
	}

	path := in.FilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(tc.CWD, path)
	}

	// Error code 10: File too large
	// Source: FileEditTool.ts:186-195
	if info, err := os.Stat(path); err == nil {
		if info.Size() > MaxEditFileSize {
			return ErrorOutput(fmt.Sprintf("File is too large to edit (%d bytes). Maximum editable file size is %d bytes.", info.Size(), MaxEditFileSize)), nil
		}
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Error code 4: File does not exist
			// Source: FileEditTool.ts:224-245
			if in.OldString == "" {
				// Empty old_string on nonexistent file = new file creation
				// Source: FileEditTool.ts:226-227
			} else {
				return ErrorOutput(fmt.Sprintf("File does not exist. Please verify the path and try again. Current working directory: %s.", tc.CWD)), nil
			}
		} else {
			return ErrorOutput(fmt.Sprintf("failed to read file: %s", err)), nil
		}
	}

	content := string(data)

	// File exists but old_string is empty — creating new content
	// Source: FileEditTool.ts:249-263
	if in.OldString == "" {
		if os.IsNotExist(err) {
			// Create new file
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return ErrorOutput(fmt.Sprintf("failed to create directory: %s", err)), nil
			}
			if err := os.WriteFile(path, []byte(in.NewString), 0644); err != nil {
				return ErrorOutput(fmt.Sprintf("failed to write file: %s", err)), nil
			}
			return SuccessOutput(fmt.Sprintf("Successfully created %s", path)), nil
		}
		// Error code 3: file exists and has content
		if strings.TrimSpace(content) != "" {
			return ErrorOutput("Cannot create new file - file already exists."), nil
		}
		// Empty file — replace with new content
		if err := os.WriteFile(path, []byte(in.NewString), 0644); err != nil {
			return ErrorOutput(fmt.Sprintf("failed to write file: %s", err)), nil
		}
		return SuccessOutput(fmt.Sprintf("Successfully edited %s", path)), nil
	}

	// Error code 5: .ipynb file
	// Source: FileEditTool.ts:266-273
	if strings.HasSuffix(path, ".ipynb") {
		return ErrorOutput("File is a Jupyter Notebook. Use the NotebookEdit tool to edit this file."), nil
	}

	// Error code 6: File not read yet (staleness guard)
	// Source: FileEditTool.ts:275-287
	if tc.ReadFileState != nil {
		entry := tc.ReadFileState.Get(path)
		if entry == nil || entry.IsPartialView {
			return ErrorOutput("File has not been read yet. Read it first before writing to it."), nil
		}

		// Error code 7: File modified since last read
		// Source: FileEditTool.ts:290-311
		if info, statErr := os.Stat(path); statErr == nil {
			if info.ModTime().After(entry.Timestamp) {
				// Content comparison fallback for full reads
				// Source: FileEditTool.ts:298-300
				if content != entry.Content {
					return ErrorOutput("File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."), nil
				}
			}
		}
	}

	// Error code 8: old_string not found
	// Source: FileEditTool.ts:316-327
	count := strings.Count(content, in.OldString)
	if count == 0 {
		return ErrorOutput(fmt.Sprintf("String to replace not found in file.\nString: %s", in.OldString)), nil
	}

	// Error code 9: multiple matches without replace_all
	// Source: FileEditTool.ts:332-343
	if count > 1 && !in.ReplaceAll {
		return ErrorOutput(fmt.Sprintf(
			"Found %d matches of the string to replace, but replace_all is false. To replace all occurrences, set replace_all to true. To replace only one occurrence, please provide more context to uniquely identify the instance.\nString: %s",
			count, in.OldString,
		)), nil
	}

	// Perform replacement
	var newContent string
	if in.ReplaceAll {
		newContent = strings.ReplaceAll(content, in.OldString, in.NewString)
	} else {
		newContent = strings.Replace(content, in.OldString, in.NewString, 1)
	}

	info, statErr := os.Stat(path)
	mode := os.FileMode(0644)
	if statErr == nil {
		mode = info.Mode()
	}

	if err := os.WriteFile(path, []byte(newContent), mode); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to write file: %s", err)), nil
	}

	// Update read file state after successful write
	if tc.ReadFileState != nil {
		tc.ReadFileState.Record(path, newContent, false)
	}

	return SuccessOutput(fmt.Sprintf("Successfully edited %s", path)), nil
}
