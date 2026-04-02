package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestCronTools(t *testing.T) {
	cronTools := tools.NewCronTools()

	var createTool, deleteTool, listTool tools.Tool
	for _, ct := range cronTools {
		switch ct.Name() {
		case "CronCreate":
			createTool = ct
		case "CronDelete":
			deleteTool = ct
		case "CronList":
			listTool = ct
		}
	}
	if createTool == nil || deleteTool == nil || listTool == nil {
		t.Fatal("NewCronTools must return CronCreate, CronDelete, and CronList")
	}

	t.Run("create_name", func(t *testing.T) {
		if createTool.Name() != "CronCreate" {
			t.Errorf("expected 'CronCreate', got %q", createTool.Name())
		}
	})

	t.Run("delete_name", func(t *testing.T) {
		if deleteTool.Name() != "CronDelete" {
			t.Errorf("expected 'CronDelete', got %q", deleteTool.Name())
		}
	})

	t.Run("list_name", func(t *testing.T) {
		if listTool.Name() != "CronList" {
			t.Errorf("expected 'CronList', got %q", listTool.Name())
		}
	})

	t.Run("create_not_read_only", func(t *testing.T) {
		if createTool.IsReadOnly() {
			t.Error("CronCreate should not be read-only")
		}
	})

	t.Run("delete_not_read_only", func(t *testing.T) {
		if deleteTool.IsReadOnly() {
			t.Error("CronDelete should not be read-only")
		}
	})

	t.Run("list_is_read_only", func(t *testing.T) {
		if !listTool.IsReadOnly() {
			t.Error("CronList should be read-only")
		}
	})

	t.Run("schemas_valid", func(t *testing.T) {
		for _, tool := range cronTools {
			var parsed map[string]interface{}
			if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
				t.Fatalf("%s schema is not valid JSON: %v", tool.Name(), err)
			}
		}
	})

	t.Run("list_empty", func(t *testing.T) {
		out, err := listTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if out.Content != "No cron jobs scheduled" {
			t.Errorf("expected 'No cron jobs scheduled', got %q", out.Content)
		}
	})

	t.Run("create_and_list", func(t *testing.T) {
		input := json.RawMessage(`{"cron": "*/5 * * * *", "prompt": "check status"}`)
		out, err := createTool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "Created cron job") {
			t.Errorf("expected creation message, got %q", out.Content)
		}

		// Extract the ID from the output
		// Format: "Created cron job cron-N: ..."
		parts := strings.Fields(out.Content)
		var createdID string
		for i, p := range parts {
			if p == "job" && i+1 < len(parts) {
				createdID = strings.TrimSuffix(parts[i+1], ":")
				break
			}
		}

		// List should show the entry
		out, err = listTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "*/5 * * * *") {
			t.Errorf("expected cron expression in list, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "check status") {
			t.Errorf("expected prompt in list, got %q", out.Content)
		}
		if !strings.Contains(out.Content, createdID) {
			t.Errorf("expected ID %s in list, got %q", createdID, out.Content)
		}
	})

	t.Run("create_missing_cron", func(t *testing.T) {
		input := json.RawMessage(`{"cron": "", "prompt": "test"}`)
		out, err := createTool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing cron")
		}
	})

	t.Run("create_missing_prompt", func(t *testing.T) {
		input := json.RawMessage(`{"cron": "* * * * *", "prompt": ""}`)
		out, err := createTool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing prompt")
		}
	})

	t.Run("create_invalid_json", func(t *testing.T) {
		out, err := createTool.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("delete_nonexistent", func(t *testing.T) {
		input := json.RawMessage(`{"id": "cron-999"}`)
		out, err := deleteTool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent cron job")
		}
		if !strings.Contains(out.Content, "not found") {
			t.Errorf("expected 'not found' in error, got %q", out.Content)
		}
	})

	t.Run("delete_missing_id", func(t *testing.T) {
		input := json.RawMessage(`{"id": ""}`)
		out, err := deleteTool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing id")
		}
	})

	t.Run("delete_invalid_json", func(t *testing.T) {
		out, err := deleteTool.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("returns_three_tools", func(t *testing.T) {
		if len(cronTools) != 3 {
			t.Errorf("expected 3 tools, got %d", len(cronTools))
		}
	})
}

func TestCronCreateAndDelete(t *testing.T) {
	cronTools := tools.NewCronTools()
	var createTool, deleteTool, listTool tools.Tool
	for _, ct := range cronTools {
		switch ct.Name() {
		case "CronCreate":
			createTool = ct
		case "CronDelete":
			deleteTool = ct
		case "CronList":
			listTool = ct
		}
	}

	// Create a job
	out, _ := createTool.Execute(context.Background(), nil, json.RawMessage(`{"cron": "0 * * * *", "prompt": "hourly check"}`))
	// Extract ID
	parts := strings.Fields(out.Content)
	var id string
	for i, p := range parts {
		if p == "job" && i+1 < len(parts) {
			id = strings.TrimSuffix(parts[i+1], ":")
			break
		}
	}

	// Delete it
	out, err := deleteTool.Execute(context.Background(), nil, json.RawMessage(fmt.Sprintf(`{"id": %q}`, id)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.IsError {
		t.Fatalf("unexpected tool error: %s", out.Content)
	}
	if !strings.Contains(out.Content, "Deleted") {
		t.Errorf("expected deletion message, got %q", out.Content)
	}

	// List should be empty
	out, _ = listTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if out.Content != "No cron jobs scheduled" {
		t.Errorf("expected empty list after delete, got %q", out.Content)
	}
}

