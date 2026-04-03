package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// helper to find a tool by name in the slice
func findTool(ts []tools.Tool, name string) tools.Tool {
	for _, t := range ts {
		if t.Name() == name {
			return t
		}
	}
	return nil
}

func TestTaskCreateTool(t *testing.T) {
	ts := tools.NewTaskTools()
	create := findTool(ts, "TaskCreate")
	if create == nil {
		t.Fatal("TaskCreate not found")
	}

	t.Run("name", func(t *testing.T) {
		if create.Name() != "TaskCreate" {
			t.Errorf("expected TaskCreate, got %q", create.Name())
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if create.IsReadOnly() {
			t.Error("TaskCreate should not be read-only")
		}
	})

	t.Run("schema_valid", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(create.InputSchema(), &parsed); err != nil {
			t.Fatalf("invalid schema JSON: %v", err)
		}
	})

	t.Run("create_task", func(t *testing.T) {
		input := json.RawMessage(`{"subject": "Build feature", "description": "Implement the new feature"}`)
		out, err := create.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "Task 1") {
			t.Errorf("expected task ID in output, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "Build feature") {
			t.Errorf("expected subject in output, got %q", out.Content)
		}
	})

	t.Run("create_assigns_sequential_ids", func(t *testing.T) {
		ts2 := tools.NewTaskTools()
		c := findTool(ts2, "TaskCreate")
		out1, _ := c.Execute(context.Background(), nil, json.RawMessage(`{"subject": "A", "description": "first"}`))
		out2, _ := c.Execute(context.Background(), nil, json.RawMessage(`{"subject": "B", "description": "second"}`))
		if !strings.Contains(out1.Content, "Task 1") {
			t.Errorf("first task should have ID 1, got %q", out1.Content)
		}
		if !strings.Contains(out2.Content, "Task 2") {
			t.Errorf("second task should have ID 2, got %q", out2.Content)
		}
	})

	t.Run("missing_subject", func(t *testing.T) {
		out, err := create.Execute(context.Background(), nil, json.RawMessage(`{"subject": "", "description": "d"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing subject")
		}
	})

	t.Run("missing_description", func(t *testing.T) {
		out, err := create.Execute(context.Background(), nil, json.RawMessage(`{"subject": "s", "description": ""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing description")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		out, err := create.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestTaskListTool(t *testing.T) {
	ts := tools.NewTaskTools()
	create := findTool(ts, "TaskCreate")
	list := findTool(ts, "TaskList")
	if list == nil {
		t.Fatal("TaskList not found")
	}

	t.Run("name", func(t *testing.T) {
		if list.Name() != "TaskList" {
			t.Errorf("expected TaskList, got %q", list.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !list.IsReadOnly() {
			t.Error("TaskList should be read-only")
		}
	})

	t.Run("empty_list", func(t *testing.T) {
		ts2 := tools.NewTaskTools()
		l := findTool(ts2, "TaskList")
		out, err := l.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Content != "No tasks" {
			t.Errorf("expected 'No tasks', got %q", out.Content)
		}
	})

	t.Run("lists_non_deleted_tasks", func(t *testing.T) {
		create.Execute(context.Background(), nil, json.RawMessage(`{"subject": "Task A", "description": "a"}`))
		create.Execute(context.Background(), nil, json.RawMessage(`{"subject": "Task B", "description": "b"}`))
		out, err := list.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "Task A") {
			t.Errorf("expected Task A, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "Task B") {
			t.Errorf("expected Task B, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "[ ]") {
			t.Errorf("expected pending icon, got %q", out.Content)
		}
	})
}

func TestTaskGetTool(t *testing.T) {
	ts := tools.NewTaskTools()
	create := findTool(ts, "TaskCreate")
	get := findTool(ts, "TaskGet")
	if get == nil {
		t.Fatal("TaskGet not found")
	}

	t.Run("name", func(t *testing.T) {
		if get.Name() != "TaskGet" {
			t.Errorf("expected TaskGet, got %q", get.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !get.IsReadOnly() {
			t.Error("TaskGet should be read-only")
		}
	})

	t.Run("get_existing_task", func(t *testing.T) {
		create.Execute(context.Background(), nil, json.RawMessage(`{"subject": "My task", "description": "details"}`))
		out, err := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Should be valid JSON
		var task map[string]interface{}
		if err := json.Unmarshal([]byte(out.Content), &task); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}
		if task["subject"] != "My task" {
			t.Errorf("expected subject 'My task', got %v", task["subject"])
		}
		if task["status"] != "pending" {
			t.Errorf("expected status 'pending', got %v", task["status"])
		}
	})

	t.Run("get_unknown_task", func(t *testing.T) {
		out, err := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "nonexistent"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for unknown task ID")
		}
		if !strings.Contains(out.Content, "not found") {
			t.Errorf("expected 'not found' in error, got %q", out.Content)
		}
	})

	t.Run("missing_taskId", func(t *testing.T) {
		out, err := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": ""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty taskId")
		}
	})
}

func TestTaskUpdateTool(t *testing.T) {
	ts := tools.NewTaskTools()
	create := findTool(ts, "TaskCreate")
	update := findTool(ts, "TaskUpdate")
	get := findTool(ts, "TaskGet")
	list := findTool(ts, "TaskList")

	t.Run("name", func(t *testing.T) {
		if update.Name() != "TaskUpdate" {
			t.Errorf("expected TaskUpdate, got %q", update.Name())
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if update.IsReadOnly() {
			t.Error("TaskUpdate should not be read-only")
		}
	})

	t.Run("update_status_pending_to_in_progress", func(t *testing.T) {
		create.Execute(context.Background(), nil, json.RawMessage(`{"subject": "Status test", "description": "d"}`))
		out, err := update.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1", "status": "in_progress"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		gout, _ := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1"}`))
		var task map[string]interface{}
		json.Unmarshal([]byte(gout.Content), &task)
		if task["status"] != "in_progress" {
			t.Errorf("expected in_progress, got %v", task["status"])
		}
	})

	t.Run("update_status_to_completed", func(t *testing.T) {
		out, err := update.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1", "status": "completed"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		gout, _ := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1"}`))
		var task map[string]interface{}
		json.Unmarshal([]byte(gout.Content), &task)
		if task["status"] != "completed" {
			t.Errorf("expected completed, got %v", task["status"])
		}
	})

	t.Run("update_subject", func(t *testing.T) {
		create.Execute(context.Background(), nil, json.RawMessage(`{"subject": "Old subject", "description": "d"}`))
		update.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "2", "subject": "New subject"}`))

		gout, _ := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "2"}`))
		var task map[string]interface{}
		json.Unmarshal([]byte(gout.Content), &task)
		if task["subject"] != "New subject" {
			t.Errorf("expected 'New subject', got %v", task["subject"])
		}
	})

	t.Run("update_description", func(t *testing.T) {
		update.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "2", "description": "New desc"}`))
		gout, _ := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "2"}`))
		var task map[string]interface{}
		json.Unmarshal([]byte(gout.Content), &task)
		if task["description"] != "New desc" {
			t.Errorf("expected 'New desc', got %v", task["description"])
		}
	})

	t.Run("update_owner", func(t *testing.T) {
		update.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "2", "owner": "agent-1"}`))
		gout, _ := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "2"}`))
		var task map[string]interface{}
		json.Unmarshal([]byte(gout.Content), &task)
		if task["owner"] != "agent-1" {
			t.Errorf("expected 'agent-1', got %v", task["owner"])
		}
	})

	t.Run("add_blocks", func(t *testing.T) {
		create.Execute(context.Background(), nil, json.RawMessage(`{"subject": "Blocker", "description": "blocks others"}`))
		update.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "3", "addBlocks": ["1", "2"]}`))
		gout, _ := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "3"}`))
		var task map[string]interface{}
		json.Unmarshal([]byte(gout.Content), &task)
		blocks, ok := task["blocks"].([]interface{})
		if !ok || len(blocks) != 2 {
			t.Errorf("expected 2 blocks, got %v", task["blocks"])
		}
	})

	t.Run("add_blocked_by", func(t *testing.T) {
		update.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1", "addBlockedBy": ["3"]}`))
		gout, _ := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1"}`))
		var task map[string]interface{}
		json.Unmarshal([]byte(gout.Content), &task)
		blockedBy, ok := task["blockedBy"].([]interface{})
		if !ok || len(blockedBy) != 1 {
			t.Errorf("expected 1 blockedBy, got %v", task["blockedBy"])
		}
	})

	t.Run("delete_via_update", func(t *testing.T) {
		create.Execute(context.Background(), nil, json.RawMessage(`{"subject": "To delete", "description": "d"}`))
		update.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "4", "status": "deleted"}`))

		lout, _ := list.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if strings.Contains(lout.Content, "To delete") {
			t.Errorf("deleted task should not appear in list, got %q", lout.Content)
		}
	})

	t.Run("invalid_status", func(t *testing.T) {
		out, err := update.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1", "status": "invalid"}`))
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

	t.Run("unknown_task_id", func(t *testing.T) {
		out, err := update.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "nonexistent", "status": "completed"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for unknown task ID")
		}
		if !strings.Contains(out.Content, "not found") {
			t.Errorf("expected 'not found' in error, got %q", out.Content)
		}
	})

	t.Run("metadata_null_deletes_key", func(t *testing.T) {
		// Source: TaskUpdateTool.ts:200-210 — setting a metadata key to null deletes it
		ts2 := tools.NewTaskTools()
		c := findTool(ts2, "TaskCreate")
		u := findTool(ts2, "TaskUpdate")
		g := findTool(ts2, "TaskGet")

		// Create a task with metadata
		c.Execute(context.Background(), nil, json.RawMessage(`{"subject": "Meta test", "description": "d", "metadata": {"keep": "yes", "remove": "please"}}`))

		// Update: set "remove" to null to delete it, add "new" key
		u.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1", "metadata": {"remove": null, "new": "added"}}`))

		// Get and verify
		gout, _ := g.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1"}`))
		var task map[string]interface{}
		json.Unmarshal([]byte(gout.Content), &task)
		meta, ok := task["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected metadata map, got %v", task["metadata"])
		}
		if meta["keep"] != "yes" {
			t.Errorf("expected keep=yes, got %v", meta["keep"])
		}
		if _, exists := meta["remove"]; exists {
			t.Error("expected 'remove' key to be deleted when set to null")
		}
		if meta["new"] != "added" {
			t.Errorf("expected new=added, got %v", meta["new"])
		}
	})
}

func TestTaskStopTool(t *testing.T) {
	ts := tools.NewTaskTools()
	create := findTool(ts, "TaskCreate")
	stop := findTool(ts, "TaskStop")
	list := findTool(ts, "TaskList")
	get := findTool(ts, "TaskGet")

	if stop == nil {
		t.Fatal("TaskStop not found")
	}

	t.Run("name", func(t *testing.T) {
		if stop.Name() != "TaskStop" {
			t.Errorf("expected TaskStop, got %q", stop.Name())
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if stop.IsReadOnly() {
			t.Error("TaskStop should not be read-only")
		}
	})

	t.Run("stop_sets_deleted", func(t *testing.T) {
		create.Execute(context.Background(), nil, json.RawMessage(`{"subject": "Running task", "description": "d"}`))
		out, err := stop.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "stopped") {
			t.Errorf("expected 'stopped' in output, got %q", out.Content)
		}

		// Verify status is deleted via get
		gout, _ := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1"}`))
		var task map[string]interface{}
		json.Unmarshal([]byte(gout.Content), &task)
		if task["status"] != "deleted" {
			t.Errorf("expected deleted status, got %v", task["status"])
		}

		// Should not appear in list
		lout, _ := list.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if strings.Contains(lout.Content, "Running task") {
			t.Errorf("stopped task should not appear in list, got %q", lout.Content)
		}
	})

	t.Run("stop_unknown_task", func(t *testing.T) {
		out, err := stop.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "nonexistent"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for unknown task")
		}
	})
}

func TestTaskOutputTool(t *testing.T) {
	ts := tools.NewTaskTools()
	create := findTool(ts, "TaskOutput")
	_ = create // just check it exists
	output := findTool(ts, "TaskOutput")
	get := findTool(ts, "TaskGet")
	createTool := findTool(ts, "TaskCreate")

	if output == nil {
		t.Fatal("TaskOutput not found")
	}

	t.Run("name", func(t *testing.T) {
		if output.Name() != "TaskOutput" {
			t.Errorf("expected TaskOutput, got %q", output.Name())
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if output.IsReadOnly() {
			t.Error("TaskOutput should not be read-only")
		}
	})

	t.Run("stores_output_in_metadata", func(t *testing.T) {
		createTool.Execute(context.Background(), nil, json.RawMessage(`{"subject": "Output test", "description": "d"}`))
		out, err := output.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1", "output": "Build succeeded"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "Output captured") {
			t.Errorf("expected confirmation, got %q", out.Content)
		}

		// Verify output in metadata
		gout, _ := get.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1"}`))
		var task map[string]interface{}
		json.Unmarshal([]byte(gout.Content), &task)
		meta, ok := task["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected metadata map, got %v", task["metadata"])
		}
		if meta["output"] != "Build succeeded" {
			t.Errorf("expected 'Build succeeded', got %v", meta["output"])
		}
	})

	t.Run("unknown_task", func(t *testing.T) {
		out, err := output.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "nonexistent", "output": "x"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for unknown task")
		}
	})

	t.Run("missing_output", func(t *testing.T) {
		out, err := output.Execute(context.Background(), nil, json.RawMessage(`{"taskId": "1", "output": ""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty output")
		}
	})
}

func TestTaskToolsConcurrentAccess(t *testing.T) {
	ts := tools.NewTaskTools()
	create := findTool(ts, "TaskCreate")
	list := findTool(ts, "TaskList")
	update := findTool(ts, "TaskUpdate")

	var wg sync.WaitGroup
	n := 50

	// Concurrently create tasks
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			input := json.RawMessage(`{"subject": "concurrent task", "description": "test"}`)
			out, err := create.Execute(context.Background(), nil, input)
			if err != nil {
				t.Errorf("create error: %v", err)
			}
			if out.IsError {
				t.Errorf("create tool error: %s", out.Content)
			}
		}(i)
	}
	wg.Wait()

	// List should show all 50
	out, _ := list.Execute(context.Background(), nil, json.RawMessage(`{}`))
	count := strings.Count(out.Content, "concurrent task")
	if count != n {
		t.Errorf("expected %d tasks, got %d", n, count)
	}

	// Concurrently update tasks
	for i := 1; i <= n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			inp, _ := json.Marshal(map[string]string{
				"taskId": fmt.Sprintf("task_%d", i),
				"status": "completed",
			})
			update.Execute(context.Background(), nil, inp)
		}(i)
	}
	wg.Wait()
}

func TestTaskToolsIndependentStores(t *testing.T) {
	ts1 := tools.NewTaskTools()
	ts2 := tools.NewTaskTools()

	create1 := findTool(ts1, "TaskCreate")
	list1 := findTool(ts1, "TaskList")
	list2 := findTool(ts2, "TaskList")

	create1.Execute(context.Background(), nil, json.RawMessage(`{"subject": "Store 1 task", "description": "d"}`))

	out1, _ := list1.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if !strings.Contains(out1.Content, "Store 1 task") {
		t.Errorf("store 1 should have the task, got %q", out1.Content)
	}

	out2, _ := list2.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if out2.Content != "No tasks" {
		t.Errorf("store 2 should be empty, got %q", out2.Content)
	}
}

func TestNewTaskToolsReturns6Tools(t *testing.T) {
	ts := tools.NewTaskTools()
	if len(ts) != 6 {
		t.Errorf("expected 6 task tools, got %d", len(ts))
	}
	expected := []string{"TaskCreate", "TaskList", "TaskGet", "TaskUpdate", "TaskStop", "TaskOutput"}
	for _, name := range expected {
		if findTool(ts, name) == nil {
			t.Errorf("missing tool: %s", name)
		}
	}
}
