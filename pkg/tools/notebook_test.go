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
	})

	t.Run("replace_cell", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_index": 0,
			"new_source": "print('world')\n",
			"operation": "replace"
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

	t.Run("replace_default_operation", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		// Omit operation — should default to "replace"
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_index": 1,
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

	t.Run("insert_cell", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_index": 1,
			"new_source": "import os\n",
			"operation": "insert",
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
			"cell_index": 3,
			"new_source": "# Appendix\n",
			"operation": "insert",
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

	t.Run("delete_cell", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_index": 1,
			"new_source": "",
			"operation": "delete"
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

	t.Run("index_out_of_range_replace", func(t *testing.T) {
		dir := t.TempDir()
		nbPath := writeNotebook(t, dir, sampleNotebook)
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{
			"notebook_path": %q,
			"cell_index": 10,
			"new_source": "x"
		}`, nbPath))

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for out-of-range index")
		}
		if !strings.Contains(out.Content, "out of range") {
			t.Errorf("expected 'out of range' in error, got %q", out.Content)
		}
	})

	t.Run("file_not_found", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{
			"notebook_path": "/nonexistent/path.ipynb",
			"cell_index": 0,
			"new_source": "x"
		}`)

		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
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
			"cell_index": 0,
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
			"cell_index": 0,
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
