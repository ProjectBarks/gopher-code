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
	CellNumber   int    `json:"cell_number"`
	NewSource    string `json:"new_source"`
	CellType     string `json:"cell_type"`
	EditMode     string `json:"edit_mode"`
}

// notebookCell represents a single cell in a Jupyter notebook.
type notebookCell struct {
	CellType       string          `json:"cell_type"`
	Source         []string        `json:"source"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	Outputs        json.RawMessage `json:"outputs,omitempty"`
	ExecutionCount json.RawMessage `json:"execution_count,omitempty"`
}

// notebook represents a Jupyter notebook's top-level structure.
type notebook struct {
	Cells         []notebookCell  `json:"cells"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	NBFormat      int             `json:"nbformat"`
	NBFormatMinor int             `json:"nbformat_minor"`
}

func (t *NotebookEditTool) Name() string { return "NotebookEdit" }
func (t *NotebookEditTool) Description() string {
	return "Replace the contents of a specific cell in a Jupyter notebook."
}
func (t *NotebookEditTool) IsReadOnly() bool { return false }

func (t *NotebookEditTool) Prompt() string {
	return "Completely replaces the contents of a specific cell in a Jupyter notebook (.ipynb file) with new source. Jupyter notebooks are interactive documents that combine code, text, and visualizations, commonly used for data analysis and scientific computing. The notebook_path parameter must be an absolute path, not a relative path. The cell_number is 0-indexed. Use edit_mode=insert to add a new cell at the index specified by cell_number. Use edit_mode=delete to delete the cell at the index specified by cell_number."
}

func (t *NotebookEditTool) MaxResultSizeChars() int { return 100_000 }

func (t *NotebookEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"notebook_path": {"type": "string", "description": "The absolute path to the Jupyter notebook file to edit (must be absolute, not relative)"},
			"cell_number": {"type": "integer", "description": "The 0-indexed cell number to edit"},
			"new_source": {"type": "string", "description": "The new source for the cell"},
			"cell_type": {"type": "string", "enum": ["code", "markdown"], "description": "The type of the cell (code or markdown). If not specified, it defaults to the current cell type. If using edit_mode=insert, this is required."},
			"edit_mode": {"type": "string", "enum": ["replace", "insert", "delete"], "description": "The type of edit to make (replace, insert, delete). Defaults to replace."}
		},
		"required": ["notebook_path", "cell_number", "new_source"],
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
	if in.EditMode == "" {
		in.EditMode = "replace"
	}

	// Validate .ipynb extension
	if filepath.Ext(in.NotebookPath) != ".ipynb" {
		return ErrorOutput("File must be a Jupyter notebook (.ipynb file). For editing other file types, use the FileEdit tool."), nil
	}

	// Validate edit_mode
	if in.EditMode != "replace" && in.EditMode != "insert" && in.EditMode != "delete" {
		return ErrorOutput("Edit mode must be replace, insert, or delete."), nil
	}

	// Validate cell_type required for insert
	if in.EditMode == "insert" && in.CellType == "" {
		return ErrorOutput("Cell type is required when using edit_mode=insert."), nil
	}

	// Validate cell_type enum when provided
	if in.CellType != "" && in.CellType != "code" && in.CellType != "markdown" {
		return ErrorOutput(fmt.Sprintf("invalid cell_type %q: must be 'code' or 'markdown'", in.CellType)), nil
	}

	path := in.NotebookPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(tc.CWD, path)
	}

	// Read the notebook file
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrorOutput("Notebook file does not exist."), nil
		}
		return ErrorOutput(fmt.Sprintf("failed to read notebook: %s", err)), nil
	}

	var nb notebook
	if err := json.Unmarshal(data, &nb); err != nil {
		return ErrorOutput("Notebook is not valid JSON."), nil
	}

	sourceLines := splitSourceLines(in.NewSource)
	cellIndex := in.CellNumber
	editMode := in.EditMode

	// Auto-append: replace at end → insert
	if editMode == "replace" && cellIndex == len(nb.Cells) {
		editMode = "insert"
		if in.CellType == "" {
			in.CellType = "code"
		}
	}

	switch editMode {
	case "replace":
		if cellIndex < 0 || cellIndex >= len(nb.Cells) {
			return ErrorOutput(fmt.Sprintf("Cell with index %d does not exist in notebook.", cellIndex)), nil
		}
		// Default cell_type to target cell's type when not specified
		if in.CellType == "" {
			in.CellType = nb.Cells[cellIndex].CellType
		}
		nb.Cells[cellIndex].Source = sourceLines
		if in.CellType != nb.Cells[cellIndex].CellType {
			nb.Cells[cellIndex].CellType = in.CellType
		}
		// Reset execution_count and outputs for code cells since source changed
		if nb.Cells[cellIndex].CellType == "code" {
			nb.Cells[cellIndex].ExecutionCount = json.RawMessage(`null`)
			nb.Cells[cellIndex].Outputs = json.RawMessage(`[]`)
		}

	case "insert":
		if cellIndex < 0 || cellIndex > len(nb.Cells) {
			return ErrorOutput(fmt.Sprintf("Cell with index %d does not exist in notebook.", cellIndex)), nil
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
		copy(nb.Cells[cellIndex+1:], nb.Cells[cellIndex:])
		nb.Cells[cellIndex] = newCell

	case "delete":
		if cellIndex < 0 || cellIndex >= len(nb.Cells) {
			return ErrorOutput(fmt.Sprintf("Cell with index %d does not exist in notebook.", cellIndex)), nil
		}
		nb.Cells = append(nb.Cells[:cellIndex], nb.Cells[cellIndex+1:]...)
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

	switch editMode {
	case "replace":
		return SuccessOutput(fmt.Sprintf("Successfully replaced cell %d in %s", cellIndex, path)), nil
	case "insert":
		return SuccessOutput(fmt.Sprintf("Successfully inserted cell at index %d in %s", cellIndex, path)), nil
	case "delete":
		return SuccessOutput(fmt.Sprintf("Successfully deleted cell %d from %s", cellIndex, path)), nil
	default:
		return SuccessOutput(fmt.Sprintf("Successfully edited %s", path)), nil
	}
}
