package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NotebookEditTool edits Jupyter notebook (.ipynb) cells.
type NotebookEditTool struct{}

type notebookEditInput struct {
	NotebookPath string `json:"notebook_path"`
	CellIndex    int    `json:"cell_index"`
	NewSource    string `json:"new_source"`
	CellType     string `json:"cell_type"`
	Operation    string `json:"operation"`
}

// notebookCell represents a single cell in a Jupyter notebook.
type notebookCell struct {
	CellType       string            `json:"cell_type"`
	Source         []string          `json:"source"`
	Metadata       json.RawMessage   `json:"metadata,omitempty"`
	Outputs        json.RawMessage   `json:"outputs,omitempty"`
	ExecutionCount json.RawMessage   `json:"execution_count,omitempty"`
}

// notebook represents a Jupyter notebook's top-level structure.
type notebook struct {
	Cells         []notebookCell  `json:"cells"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	NBFormat      int             `json:"nbformat"`
	NBFormatMinor int             `json:"nbformat_minor"`
}

func (t *NotebookEditTool) Name() string        { return "NotebookEdit" }
func (t *NotebookEditTool) Description() string { return "Edit a Jupyter notebook cell" }
func (t *NotebookEditTool) IsReadOnly() bool    { return false }

func (t *NotebookEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"notebook_path": {"type": "string", "description": "Path to the .ipynb file"},
			"cell_index": {"type": "integer", "description": "Index of the cell to edit (0-based)"},
			"new_source": {"type": "string", "description": "New source content for the cell"},
			"cell_type": {"type": "string", "enum": ["code", "markdown"], "description": "Cell type (default: code)"},
			"operation": {"type": "string", "enum": ["replace", "insert", "delete"], "description": "Operation (default: replace)"}
		},
		"required": ["notebook_path", "cell_index", "new_source"],
		"additionalProperties": false
	}`)
}

// splitSourceLines splits source text into the line format used by .ipynb files.
// Each line except the last ends with "\n".
func splitSourceLines(source string) []string {
	if source == "" {
		return []string{}
	}
	lines := strings.SplitAfter(source, "\n")
	// SplitAfter produces an empty trailing element when source ends with "\n"
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (t *NotebookEditTool) Execute(_ context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in notebookEditInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.NotebookPath == "" {
		return ErrorOutput("notebook_path is required"), nil
	}

	// Defaults
	if in.CellType == "" {
		in.CellType = "code"
	}
	if in.Operation == "" {
		in.Operation = "replace"
	}

	// Validate cell_type
	if in.CellType != "code" && in.CellType != "markdown" {
		return ErrorOutput(fmt.Sprintf("invalid cell_type %q: must be 'code' or 'markdown'", in.CellType)), nil
	}

	// Validate operation
	if in.Operation != "replace" && in.Operation != "insert" && in.Operation != "delete" {
		return ErrorOutput(fmt.Sprintf("invalid operation %q: must be 'replace', 'insert', or 'delete'", in.Operation)), nil
	}

	path := in.NotebookPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(tc.CWD, path)
	}

	// Read the notebook file
	data, err := os.ReadFile(path)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to read notebook: %s", err)), nil
	}

	var nb notebook
	if err := json.Unmarshal(data, &nb); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to parse notebook JSON: %s", err)), nil
	}

	sourceLines := splitSourceLines(in.NewSource)

	switch in.Operation {
	case "replace":
		if in.CellIndex < 0 || in.CellIndex >= len(nb.Cells) {
			return ErrorOutput(fmt.Sprintf("cell_index %d out of range (notebook has %d cells)", in.CellIndex, len(nb.Cells))), nil
		}
		nb.Cells[in.CellIndex].Source = sourceLines
		nb.Cells[in.CellIndex].CellType = in.CellType

	case "insert":
		if in.CellIndex < 0 || in.CellIndex > len(nb.Cells) {
			return ErrorOutput(fmt.Sprintf("cell_index %d out of range for insert (notebook has %d cells)", in.CellIndex, len(nb.Cells))), nil
		}
		newCell := notebookCell{
			CellType: in.CellType,
			Source:   sourceLines,
			Metadata: json.RawMessage(`{}`),
		}
		if in.CellType == "code" {
			newCell.Outputs = json.RawMessage(`[]`)
			newCell.ExecutionCount = json.RawMessage(`null`)
		}
		// Insert at index
		nb.Cells = append(nb.Cells, notebookCell{})
		copy(nb.Cells[in.CellIndex+1:], nb.Cells[in.CellIndex:])
		nb.Cells[in.CellIndex] = newCell

	case "delete":
		if in.CellIndex < 0 || in.CellIndex >= len(nb.Cells) {
			return ErrorOutput(fmt.Sprintf("cell_index %d out of range for delete (notebook has %d cells)", in.CellIndex, len(nb.Cells))), nil
		}
		nb.Cells = append(nb.Cells[:in.CellIndex], nb.Cells[in.CellIndex+1:]...)
	}

	// Write back
	output, err := json.MarshalIndent(nb, "", " ")
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to marshal notebook: %s", err)), nil
	}
	// Ensure trailing newline
	output = append(output, '\n')

	if err := os.WriteFile(path, output, 0644); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to write notebook: %s", err)), nil
	}

	switch in.Operation {
	case "replace":
		return SuccessOutput(fmt.Sprintf("Successfully replaced cell %d in %s", in.CellIndex, path)), nil
	case "insert":
		return SuccessOutput(fmt.Sprintf("Successfully inserted cell at index %d in %s", in.CellIndex, path)), nil
	case "delete":
		return SuccessOutput(fmt.Sprintf("Successfully deleted cell %d from %s", in.CellIndex, path)), nil
	default:
		return SuccessOutput(fmt.Sprintf("Successfully edited %s", path)), nil
	}
}
