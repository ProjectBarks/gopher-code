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

// MaxEditFileSize is the maximum file size for editing (1 GiB).
// Source: FileEditTool.ts:79-84 — V8/Bun string limit ~2^30 chars
const MaxEditFileSize = 1024 * 1024 * 1024

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

	// Try exact match first; fall back to quote-normalized match.
	// Source: FileEditTool/utils.ts:73-93 — findActualString()
	oldString := in.OldString
	count := strings.Count(content, oldString)
	if count == 0 {
		// Try with normalized quotes (curly → straight)
		actualOld := findActualString(content, oldString)
		if actualOld != "" {
			oldString = actualOld
			count = strings.Count(content, oldString)
		}
	}
	// Error code 8: old_string not found
	// Source: FileEditTool.ts:316-327
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

	// Perform replacement using applyEditToFile logic.
	// Source: FileEditTool/utils.ts:206-228 — special newline handling on deletion
	newContent := applyEditToFile(content, oldString, in.NewString, in.ReplaceAll)

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

// normalizeQuotes converts curly quotes to straight quotes.
// Source: FileEditTool/utils.ts:31-37
func normalizeQuotes(s string) string {
	s = strings.ReplaceAll(s, "\u2018", "'")  // LEFT SINGLE CURLY QUOTE
	s = strings.ReplaceAll(s, "\u2019", "'")  // RIGHT SINGLE CURLY QUOTE
	s = strings.ReplaceAll(s, "\u201C", "\"") // LEFT DOUBLE CURLY QUOTE
	s = strings.ReplaceAll(s, "\u201D", "\"") // RIGHT DOUBLE CURLY QUOTE
	return s
}

// findActualString finds the actual string in file content that matches via
// quote normalization. Returns the original file substring if found.
// Source: FileEditTool/utils.ts:73-93
func findActualString(fileContent, searchString string) string {
	// First try exact match
	if strings.Contains(fileContent, searchString) {
		return searchString
	}
	// Try with normalized quotes. normalizeQuotes replaces each curly quote
	// rune with a straight quote rune (1-to-1 rune mapping), so rune positions
	// are preserved between original and normalized. Use rune indexing to map
	// back from normalized to original.
	normalizedSearch := normalizeQuotes(searchString)
	normalizedFile := normalizeQuotes(fileContent)
	byteIdx := strings.Index(normalizedFile, normalizedSearch)
	if byteIdx == -1 {
		return ""
	}
	// Convert byte offset in normalizedFile to rune offset
	runeStart := len([]rune(normalizedFile[:byteIdx]))
	runeLen := len([]rune(normalizedSearch))
	// Extract from original file content by rune offset
	fileRunes := []rune(fileContent)
	if runeStart+runeLen > len(fileRunes) {
		return ""
	}
	return string(fileRunes[runeStart : runeStart+runeLen])
}

// applyEditToFile applies a string replacement with special deletion handling.
// When new_string is empty (deletion), also strips the trailing newline after
// old_string to avoid leaving blank lines.
// Source: FileEditTool/utils.ts:206-228
func applyEditToFile(content, oldString, newString string, replaceAll bool) string {
	if newString != "" {
		if replaceAll {
			return strings.ReplaceAll(content, oldString, newString)
		}
		return strings.Replace(content, oldString, newString, 1)
	}
	// Deletion: strip trailing newline if old_string doesn't end with \n
	// but appears followed by \n in the file
	stripTrailingNewline := !strings.HasSuffix(oldString, "\n") &&
		strings.Contains(content, oldString+"\n")
	target := oldString
	if stripTrailingNewline {
		target = oldString + "\n"
	}
	if replaceAll {
		return strings.ReplaceAll(content, target, newString)
	}
	return strings.Replace(content, target, newString, 1)
}
