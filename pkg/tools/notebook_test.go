package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

const sampleNotebook = `{
 "cells": [
  {
   "cell_type": "code",
   "source": [
    "print('hello')\n"
   ],
   "metadata": {},
   "outputs": [],
   "execution_count": null
  },
  {
   "cell_type": "markdown",
   "source": [
    "# Title\n"
   ],
   "metadata": {}
  },
  {
   "cell_type": "code",
   "source": [
    "x = 1\n",
    "y = 2\n"
   ],
   "metadata": {},
   "outputs": [],
   "execution_count": null
  }
 ],
 "metadata": {},
 "nbformat": 4,
 "nbformat_minor": 5
}
`

func writeNotebook(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "test.ipynb")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write notebook: %v", err)
	}
	return path
}

func readNotebookCells(t *testing.T, path string) []struct {
	CellType string   `json:"cell_type"`
	Source   []string `json:"source"`
} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read notebook: %v", err)
	}
	var nb struct {
		Cells []struct {
			CellType string   `json:"cell_type"`
			Source   []string `json:"source"`
		} `json:"cells"`
	}
	if err := json.Unmarshal(data, &nb); err != nil {
		t.Fatalf("failed to parse notebook: %v", err)
	}
	return nb.Cells
}

func TestNotebookEditTool(t *testing.T) {
	tool := &tools.NotebookEditTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "NotebookEdit" {
			t.Errorf("expected 'NotebookEdit', got %q", tool.Name())
		}
	})

	t.Run("description_verbatim", func(t *testing.T) {
		want := "Replace the contents of a specific cell in a Jupyter notebook."
		if tool.Description() != want {
			t.Errorf("expected %q, got %q", want, tool.Description())
		}
	})

	t.Run("prompt_verbatim", func(t *testing.T) {
		want := "Completely replaces the contents of a specific cell in a Jupyter notebook (.ipynb file) with new source. Jupyter notebooks are interactive documents that combine code, text, and visualizations, commonly used for data analysis and scientific computing. The notebook_path parameter must be an absolute path, not a relative path. The cell_number is 0-indexed. Use edit_mode=insert to add a new cell at the index specified by cell_number. Use edit_mode=delete to delete the cell at the index specified by cell_number."
		if tool.Prompt() != want {
			t.Errorf("expected prompt verbatim, got %q", tool.Prompt())
		}
	})

	t.Run("max_result_size_chars", func(t *testing.T) {
		if tool.MaxResultSizeChars() != 100_000 {
			t.Errorf("expected 100000, got %d", tool.MaxResultSizeChars())
		}
	})

	t.Run("is_not_read_only", func(t *testing.T) {
		if tool.IsReadOnly() {
			t.Error("NotebookEditTool should not be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		schema := tool.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
		props := parsed["properties"].(map[string]interface{})
		if _, ok := props["cell_number"]; !ok {
			t.Error("schema should have cell_number property")
		}
		if _, ok := props["edit_mode"]; !ok {
			t.Error("schema should have edit_mode property")
		}
	})

	t.Run("replace_cell", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 0,
			"new_source": "print('world')\n",
			"edit_mode": "replace"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		cells := readNotebookCells(t, nbPath)
		if len(cells) != 3 {
			t.Fatalf("expected 3 cells, got %d", len(cells))
		}
		if len(cells[0].Source) != 1 || cells[0].Source[0] != "print('world')\n" {
			t.Errorf("expected replaced source, got %v", cells[0].Source)
		}
	})

	t.Run("replace_default_edit_mode", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		// Omit edit_mode — should default to "replace"
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 1,
			"new_source": "# Updated Title\n"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		cells := readNotebookCells(t, nbPath)
		if cells[1].Source[0] != "# Updated Title\n" {
			t.Errorf("expected updated source, got %v", cells[1].Source)
		}
	})

	t.Run("replace_preserves_cell_type", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		// Replace markdown cell without specifying cell_type — should stay markdown
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 1,
			"new_source": "# New Header\n"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		cells := readNotebookCells(t, nbPath)
		if cells[1].CellType != "markdown" {
			t.Errorf("expected cell_type preserved as 'markdown', got %q", cells[1].CellType)
		}
	})

	t.Run("replace_changes_cell_type", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		// Replace markdown cell specifying cell_type=code
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 1,
			"new_source": "x = 42\n",
			"cell_type": "code"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		cells := readNotebookCells(t, nbPath)
		if cells[1].CellType != "code" {
			t.Errorf("expected cell_type changed to 'code', got %q", cells[1].CellType)
		}
	})

	t.Run("insert_cell", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 1,
			"new_source": "import os\n",
			"edit_mode": "insert",
			"cell_type": "code"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		cells := readNotebookCells(t, nbPath)
		if len(cells) != 4 {
			t.Fatalf("expected 4 cells after insert, got %d", len(cells))
		}
		if cells[1].Source[0] != "import os\n" {
			t.Errorf("expected inserted source at index 1, got %v", cells[1].Source)
		}
		if cells[1].CellType != "code" {
			t.Errorf("expected cell_type 'code', got %q", cells[1].CellType)
		}
		// Original cell at index 1 should now be at index 2
		if cells[2].CellType != "markdown" {
			t.Errorf("expected original markdown cell shifted to index 2, got %q", cells[2].CellType)
		}
	})

	t.Run("insert_at_end", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 3,
			"new_source": "# Appendix\n",
			"edit_mode": "insert",
			"cell_type": "markdown"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		cells := readNotebookCells(t, nbPath)
		if len(cells) != 4 {
			t.Fatalf("expected 4 cells, got %d", len(cells))
		}
		if cells[3].CellType != "markdown" {
			t.Errorf("expected markdown cell at end, got %q", cells[3].CellType)
		}
	})

	t.Run("insert_markdown_cell", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 0,
			"new_source": "# Intro\n",
			"edit_mode": "insert",
			"cell_type": "markdown"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		// Verify markdown cell has no outputs/execution_count fields
		data, _ := os.ReadFile(nbPath)
		var nb struct {
			Cells []json.RawMessage `json:"cells"`
		}
		json.Unmarshal(data, &nb)
		var cell map[string]interface{}
		json.Unmarshal(nb.Cells[0], &cell)
		if _, ok := cell["outputs"]; ok {
			t.Error("markdown cell should not have outputs field")
		}
		if _, ok := cell["execution_count"]; ok {
			t.Error("markdown cell should not have execution_count field")
		}
	})

	t.Run("delete_cell", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 1,
			"new_source": "",
			"edit_mode": "delete"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		cells := readNotebookCells(t, nbPath)
		if len(cells) != 2 {
			t.Fatalf("expected 2 cells after delete, got %d", len(cells))
		}
		// The remaining cells should be the original first and third
		if cells[0].CellType != "code" {
			t.Errorf("expected first cell to still be code, got %q", cells[0].CellType)
		}
		if cells[1].CellType != "code" {
			t.Errorf("expected second cell to be the original third (code), got %q", cells[1].CellType)
		}
	})

	t.Run("auto_append_replace_at_end_becomes_insert", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		// Replace at index 3 (one past end) should auto-convert to insert
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 3,
			"new_source": "# Appended\n"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		cells := readNotebookCells(t, nbPath)
		if len(cells) != 4 {
			t.Fatalf("expected 4 cells after auto-append, got %d", len(cells))
		}
		// Auto-append defaults cell_type to "code"
		if cells[3].CellType != "code" {
			t.Errorf("expected auto-appended cell_type 'code', got %q", cells[3].CellType)
		}
	})

	// --- Error cases with verbatim messages ---

	t.Run("error_file_not_found", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{
			"notebook_path": "/nonexistent/path.ipynb",
			"cell_number": 0,
			"new_source": "x"
		}`)

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent file")
		}
		if out.Content != "Notebook file does not exist." {
			t.Errorf("expected verbatim error, got %q", out.Content)
		}
	})

	t.Run("error_not_ipynb", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{
			"notebook_path": "/some/file.txt",
			"cell_number": 0,
			"new_source": "x"
		}`)

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for non-.ipynb file")
		}
		if !strings.Contains(out.Content, "must be a Jupyter notebook (.ipynb file)") {
			t.Errorf("expected .ipynb extension error, got %q", out.Content)
		}
	})

	t.Run("error_invalid_edit_mode", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 0,
			"new_source": "x",
			"edit_mode": "bogus"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid edit_mode")
		}
		if out.Content != "Edit mode must be replace, insert, or delete." {
			t.Errorf("expected verbatim error, got %q", out.Content)
		}
	})

	t.Run("error_insert_missing_cell_type", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 0,
			"new_source": "x",
			"edit_mode": "insert"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for insert without cell_type")
		}
		if out.Content != "Cell type is required when using edit_mode=insert." {
			t.Errorf("expected verbatim error, got %q", out.Content)
		}
	})

	t.Run("error_invalid_json_notebook", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.ipynb")
		os.WriteFile(path, []byte("{not valid json!!!}"), 0644)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 0,
			"new_source": "x"
		}`, path))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON notebook")
		}
		if out.Content != "Notebook is not valid JSON." {
			t.Errorf("expected verbatim error, got %q", out.Content)
		}
	})

	t.Run("error_replace_out_of_range", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 10,
			"new_source": "x"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for out-of-range index")
		}
		if !strings.Contains(out.Content, "Cell with index 10 does not exist in notebook.") {
			t.Errorf("expected verbatim OOB error, got %q", out.Content)
		}
	})

	t.Run("error_delete_out_of_range", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 5,
			"new_source": "",
			"edit_mode": "delete"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for delete out-of-range")
		}
		if !strings.Contains(out.Content, "Cell with index 5 does not exist in notebook.") {
			t.Errorf("expected verbatim OOB error, got %q", out.Content)
		}
	})

	t.Run("error_insert_out_of_range", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 10,
			"new_source": "x",
			"edit_mode": "insert",
			"cell_type": "code"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for insert out-of-range")
		}
		if !strings.Contains(out.Content, "Cell with index 10 does not exist in notebook.") {
			t.Errorf("expected verbatim OOB error, got %q", out.Content)
		}
	})

	t.Run("invalid_input_json", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{bad}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("relative_path", func(t *testing.T) {
		dir := t.TempDir()
		// Write notebook with a simple name
		nbPath := filepath.Join(dir, "nb.ipynb")
		if err := os.WriteFile(nbPath, []byte(sampleNotebook), 0644); err != nil {
			t.Fatalf("failed to write: %v", err)
		}
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{
			"notebook_path": "nb.ipynb",
			"cell_number": 0,
			"new_source": "print('relative')\n"
		}`)

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		cells := readNotebookCells(t, nbPath)
		if cells[0].Source[0] != "print('relative')\n" {
			t.Errorf("expected relative path to work, got %v", cells[0].Source)
		}
	})

	t.Run("multiline_source_split", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_number": 0,
			"new_source": "line1\nline2\nline3\n"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		cells := readNotebookCells(t, nbPath)
		if len(cells[0].Source) != 3 {
			t.Fatalf("expected 3 source lines, got %d: %v", len(cells[0].Source), cells[0].Source)
		}
		if cells[0].Source[0] != "line1\n" || cells[0].Source[1] != "line2\n" || cells[0].Source[2] != "line3\n" {
			t.Errorf("unexpected source lines: %v", cells[0].Source)
		}
	})
}
