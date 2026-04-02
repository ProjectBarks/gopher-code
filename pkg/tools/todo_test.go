package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestTodoTools(t *testing.T) {
	writer, reader := tools.NewTodoTools()

	t.Run("writer_name", func(t *testing.T) {
		if writer.Name() != "TodoWrite" {
			t.Errorf("expected 'TodoWrite', got %q", writer.Name())
		}
	})

	t.Run("reader_name", func(t *testing.T) {
		if reader.Name() != "TodoRead" {
			t.Errorf("expected 'TodoRead', got %q", reader.Name())
		}
	})

	t.Run("writer_not_read_only", func(t *testing.T) {
		if writer.IsReadOnly() {
			t.Error("TodoWriteTool should not be read-only")
		}
	})

	t.Run("reader_is_read_only", func(t *testing.T) {
		if !reader.IsReadOnly() {
			t.Error("TodoReadTool should be read-only")
		}
	})

	t.Run("writer_schema_valid", func(t *testing.T) {
		schema := writer.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("InputSchema() is not valid JSON: %v", err)
		}
	})

	t.Run("reader_schema_valid", func(t *testing.T) {
		schema := reader.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("InputSchema() is not valid JSON: %v", err)
		}
	})

	t.Run("read_empty_list", func(t *testing.T) {
		out, err := reader.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if out.Content != "No todos" {
			t.Errorf("expected 'No todos', got %q", out.Content)
		}
	})

	t.Run("write_and_read", func(t *testing.T) {
		input := json.RawMessage(`{
			"todos": [
				{"id": "1", "description": "First task", "status": "pending"},
				{"id": "2", "description": "Second task", "status": "in_progress"},
				{"id": "3", "description": "Third task", "status": "done"}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "3 tasks") {
			t.Errorf("expected '3 tasks' in output, got %q", out.Content)
		}

		// Read back
		out, err = reader.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "[ ] 1 First task") {
			t.Errorf("expected pending task, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "[~] 2 Second task") {
			t.Errorf("expected in_progress task, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "[x] 3 Third task") {
			t.Errorf("expected done task, got %q", out.Content)
		}
	})

	t.Run("update_replaces_list", func(t *testing.T) {
		// First write 3 tasks
		input1 := json.RawMessage(`{
			"todos": [
				{"id": "1", "description": "Task A", "status": "pending"},
				{"id": "2", "description": "Task B", "status": "pending"},
				{"id": "3", "description": "Task C", "status": "pending"}
			]
		}`)
		writer.Execute(context.Background(), nil, input1)

		// Now replace with 1 task
		input2 := json.RawMessage(`{
			"todos": [
				{"id": "1", "description": "Task A updated", "status": "done"}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "1 tasks") {
			t.Errorf("expected '1 tasks', got %q", out.Content)
		}

		// Read back - should only have 1 task
		out, err = reader.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(out.Content, "Task B") {
			t.Errorf("old tasks should be replaced, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "Task A updated") {
			t.Errorf("expected updated task, got %q", out.Content)
		}
	})

	t.Run("clear_all_todos", func(t *testing.T) {
		// First add something
		writer.Execute(context.Background(), nil, json.RawMessage(`{
			"todos": [{"id": "1", "description": "temp", "status": "pending"}]
		}`))

		// Clear by writing empty list
		out, err := writer.Execute(context.Background(), nil, json.RawMessage(`{"todos": []}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		// Read should show empty
		out, err = reader.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Content != "No todos" {
			t.Errorf("expected 'No todos' after clear, got %q", out.Content)
		}
	})

	t.Run("invalid_status_rejected", func(t *testing.T) {
		input := json.RawMessage(`{
			"todos": [
				{"id": "1", "description": "Bad status", "status": "invalid"}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid status")
		}
		if !strings.Contains(out.Content, "invalid status") {
			t.Errorf("expected 'invalid status' in error, got %q", out.Content)
		}
	})

	t.Run("missing_id_rejected", func(t *testing.T) {
		input := json.RawMessage(`{
			"todos": [
				{"id": "", "description": "No ID", "status": "pending"}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing ID")
		}
	})

	t.Run("missing_description_rejected", func(t *testing.T) {
		input := json.RawMessage(`{
			"todos": [
				{"id": "1", "description": "", "status": "pending"}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing description")
		}
	})

	t.Run("invalid_json_write", func(t *testing.T) {
		out, err := writer.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestTodoToolsSharedState(t *testing.T) {
	// Verify that separate NewTodoTools calls create independent state
	writer1, reader1 := tools.NewTodoTools()
	writer2, reader2 := tools.NewTodoTools()

	writer1.Execute(context.Background(), nil, json.RawMessage(`{
		"todos": [{"id": "1", "description": "From set 1", "status": "pending"}]
	}`))

	// Reader2 should not see set1's todos
	out, _ := reader2.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if out.Content != "No todos" {
		t.Errorf("separate instances should have independent state, got %q", out.Content)
	}

	// Reader1 should see its own todos
	out, _ = reader1.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if !strings.Contains(out.Content, "From set 1") {
		t.Errorf("expected todo from set 1, got %q", out.Content)
	}

	// Writer2 should not affect reader1
	writer2.Execute(context.Background(), nil, json.RawMessage(`{
		"todos": [{"id": "x", "description": "From set 2", "status": "done"}]
	}`))

	out, _ = reader1.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if strings.Contains(out.Content, "From set 2") {
		t.Errorf("set 2 should not affect set 1, got %q", out.Content)
	}
}
