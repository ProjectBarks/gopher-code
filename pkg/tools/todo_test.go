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
		_, reader := tools.NewTodoTools()
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

	// --- T534: full schema (content, status, activeForm) ---

	t.Run("write_and_read_full_schema", func(t *testing.T) {
		writer, reader := tools.NewTodoTools()
		input := json.RawMessage(`{
			"todos": [
				{"content": "First task", "status": "pending", "activeForm": "Working on first task"},
				{"content": "Second task", "status": "in_progress", "activeForm": "Working on second task"},
				{"content": "Third task", "status": "completed", "activeForm": "Completing third task"}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		// TS result message: "Todos have been modified successfully..."
		const wantResult = "Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current tasks if applicable"
		if out.Content != wantResult {
			t.Errorf("result mismatch\n got: %q\nwant: %q", out.Content, wantResult)
		}

		// Read back
		out, err = reader.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "First task") {
			t.Errorf("expected 'First task' in read output, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "Second task") {
			t.Errorf("expected 'Second task' in read output, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "Third task") {
			t.Errorf("expected 'Third task' in read output, got %q", out.Content)
		}
	})

	t.Run("status_validation_completed_accepted", func(t *testing.T) {
		writer, _ := tools.NewTodoTools()
		input := json.RawMessage(`{
			"todos": [
				{"content": "Done task", "status": "completed", "activeForm": "Completing task"}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error for 'completed' status: %s", out.Content)
		}
	})

	t.Run("status_validation_done_rejected", func(t *testing.T) {
		// TS uses "completed" not "done" — "done" should be invalid
		writer, _ := tools.NewTodoTools()
		input := json.RawMessage(`{
			"todos": [
				{"content": "Bad status", "status": "done", "activeForm": "Doing bad status"}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for 'done' status — TS uses 'completed'")
		}
	})

	t.Run("status_validation_cancelled_rejected", func(t *testing.T) {
		writer, _ := tools.NewTodoTools()
		input := json.RawMessage(`{
			"todos": [
				{"content": "Cancelled task", "status": "cancelled", "activeForm": "Cancelling task"}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for 'cancelled' status — not a valid TS status")
		}
	})

	t.Run("status_pending_in_progress_completed_are_valid", func(t *testing.T) {
		writer, _ := tools.NewTodoTools()
		for _, status := range []string{"pending", "in_progress", "completed"} {
			input, _ := json.Marshal(map[string]interface{}{
				"todos": []map[string]string{
					{"content": "Task", "status": status, "activeForm": "Tasking"},
				},
			})
			out, err := writer.Execute(context.Background(), nil, input)
			if err != nil {
				t.Fatalf("status %q: unexpected error: %v", status, err)
			}
			if out.IsError {
				t.Errorf("status %q should be valid, got error: %s", status, out.Content)
			}
		}
	})

	t.Run("empty_content_rejected", func(t *testing.T) {
		writer, _ := tools.NewTodoTools()
		input := json.RawMessage(`{
			"todos": [
				{"content": "", "status": "pending", "activeForm": "Working"}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty content")
		}
	})

	t.Run("empty_activeForm_rejected", func(t *testing.T) {
		writer, _ := tools.NewTodoTools()
		input := json.RawMessage(`{
			"todos": [
				{"content": "A task", "status": "pending", "activeForm": ""}
			]
		}`)
		out, err := writer.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty activeForm")
		}
	})

	t.Run("update_replaces_list", func(t *testing.T) {
		writer, reader := tools.NewTodoTools()
		// First write 3 tasks
		input1 := json.RawMessage(`{
			"todos": [
				{"content": "Task A", "status": "pending", "activeForm": "Working on A"},
				{"content": "Task B", "status": "pending", "activeForm": "Working on B"},
				{"content": "Task C", "status": "pending", "activeForm": "Working on C"}
			]
		}`)
		writer.Execute(context.Background(), nil, input1)

		// Now replace with 1 task
		input2 := json.RawMessage(`{
			"todos": [
				{"content": "Task A updated", "status": "completed", "activeForm": "Updating A"}
			]
		}`)
		writer.Execute(context.Background(), nil, input2)

		// Read back - should only have 1 task
		out, err := reader.Execute(context.Background(), nil, json.RawMessage(`{}`))
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
		writer, reader := tools.NewTodoTools()
		// First add something
		writer.Execute(context.Background(), nil, json.RawMessage(`{
			"todos": [{"content": "temp", "status": "pending", "activeForm": "Temping"}]
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

	t.Run("invalid_json_write", func(t *testing.T) {
		out, err := writer.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	// --- T534: prompt text ---

	t.Run("description_matches_ts", func(t *testing.T) {
		const wantDesc = "Update the todo list for the current session. To be used proactively and often to track progress and pending tasks. Make sure that at least one task is in_progress at all times. Always provide both content (imperative) and activeForm (present continuous) for each task."
		got := writer.Description()
		if got != wantDesc {
			t.Errorf("description mismatch\n got: %q\nwant: %q", got, wantDesc)
		}
	})

	t.Run("prompt_contains_key_sections", func(t *testing.T) {
		prompt := tools.GetToolPrompt(writer)
		if prompt == "" {
			t.Fatal("TodoWriteTool must implement ToolPrompter (Prompt() string)")
		}

		requiredFragments := []string{
			"Use this tool to create and manage a structured task list for your current coding session.",
			"It also helps the user understand the progress of the task and overall progress of their requests.",
			"## When to Use This Tool",
			"Complex multi-step tasks",
			"## When NOT to Use This Tool",
			"NOTE that you should not use this tool if there is only one trivial task to do.",
			"## Examples of When to Use the Todo List",
			"## Examples of When NOT to Use the Todo List",
			"## Task States and Management",
			"pending: Task not yet started",
			"in_progress: Currently working on",
			"completed: Task finished successfully",
			"Ideally you should only have one todo as in_progress at a time",
			"**Task Breakdown**",
			"content: ",
			"activeForm: ",
		}
		for _, frag := range requiredFragments {
			if !strings.Contains(prompt, frag) {
				t.Errorf("prompt missing fragment: %q", frag)
			}
		}
	})

	// --- T534: search hint ---

	t.Run("search_hint", func(t *testing.T) {
		hint := tools.GetSearchHint(writer)
		if hint == "" {
			t.Fatal("TodoWriteTool must implement SearchHinter")
		}
		if hint != "manage the session task checklist" {
			t.Errorf("search hint mismatch: got %q", hint)
		}
	})

	// --- T534: maxResultSizeChars ---

	t.Run("max_result_size_chars", func(t *testing.T) {
		got := tools.GetMaxResultSizeChars(writer)
		if got != 100_000 {
			t.Errorf("maxResultSizeChars: got %d, want 100000", got)
		}
	})

	// --- T534: input schema has content, status, activeForm (not id/description) ---

	t.Run("input_schema_fields", func(t *testing.T) {
		schema := writer.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("InputSchema() is not valid JSON: %v", err)
		}
		props := parsed["properties"].(map[string]interface{})
		todosObj := props["todos"].(map[string]interface{})
		items := todosObj["items"].(map[string]interface{})
		itemProps := items["properties"].(map[string]interface{})

		// Must have content, status, activeForm
		for _, field := range []string{"content", "status", "activeForm"} {
			if _, ok := itemProps[field]; !ok {
				t.Errorf("input schema items missing field %q", field)
			}
		}
		// Must NOT have old id/description fields
		for _, field := range []string{"id", "description"} {
			if _, ok := itemProps[field]; ok {
				t.Errorf("input schema items should not have old field %q", field)
			}
		}

		// required should list content, status, activeForm
		required := items["required"].([]interface{})
		requiredSet := map[string]bool{}
		for _, r := range required {
			requiredSet[r.(string)] = true
		}
		for _, field := range []string{"content", "status", "activeForm"} {
			if !requiredSet[field] {
				t.Errorf("input schema items missing required field %q", field)
			}
		}
	})

	// --- T534: read output format with new schema ---

	t.Run("read_output_format", func(t *testing.T) {
		writer, reader := tools.NewTodoTools()
		input := json.RawMessage(`{
			"todos": [
				{"content": "Fix bug", "status": "pending", "activeForm": "Fixing bug"},
				{"content": "Write tests", "status": "in_progress", "activeForm": "Writing tests"},
				{"content": "Deploy", "status": "completed", "activeForm": "Deploying"}
			]
		}`)
		writer.Execute(context.Background(), nil, input)

		out, err := reader.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should show content with status icons
		if !strings.Contains(out.Content, "[ ]") {
			t.Errorf("expected [ ] for pending, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "[~]") {
			t.Errorf("expected [~] for in_progress, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "[x]") {
			t.Errorf("expected [x] for completed, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "Fix bug") {
			t.Errorf("expected 'Fix bug' in output, got %q", out.Content)
		}
	})
}

func TestTodoToolsSharedState(t *testing.T) {
	// Verify that separate NewTodoTools calls create independent state
	writer1, reader1 := tools.NewTodoTools()
	writer2, reader2 := tools.NewTodoTools()

	writer1.Execute(context.Background(), nil, json.RawMessage(`{
		"todos": [{"content": "From set 1", "status": "pending", "activeForm": "Working on set 1"}]
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
		"todos": [{"content": "From set 2", "status": "completed", "activeForm": "Working on set 2"}]
	}`))

	out, _ = reader1.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if strings.Contains(out.Content, "From set 2") {
		t.Errorf("set 2 should not affect set 1, got %q", out.Content)
	}
}
