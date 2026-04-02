package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Todo represents a single task in the todo list.
type Todo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"` // "pending", "in_progress", "done"
}

// TodoWriteTool manages the in-memory task list.
type TodoWriteTool struct {
	mu    sync.Mutex
	todos *[]Todo // shared pointer with TodoReadTool
}

type todoWriteInput struct {
	Todos []Todo `json:"todos"`
}

func (t *TodoWriteTool) Name() string { return "TodoWrite" }
func (t *TodoWriteTool) Description() string {
	return "Create and update the todo list for tracking tasks"
}
func (t *TodoWriteTool) IsReadOnly() bool { return false }

func (t *TodoWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"todos": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "string", "description": "Unique task ID"},
						"description": {"type": "string", "description": "Task description"},
						"status": {"type": "string", "description": "Task status: pending, in_progress, or done"}
					},
					"required": ["id", "description", "status"]
				},
				"description": "The complete list of todos (replaces existing list)"
			}
		},
		"required": ["todos"],
		"additionalProperties": false
	}`)
}

func (t *TodoWriteTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in todoWriteInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}

	// Validate statuses
	validStatuses := map[string]bool{"pending": true, "in_progress": true, "done": true}
	for _, todo := range in.Todos {
		if todo.ID == "" {
			return ErrorOutput("each todo must have an id"), nil
		}
		if todo.Description == "" {
			return ErrorOutput(fmt.Sprintf("todo %q must have a description", todo.ID)), nil
		}
		if !validStatuses[todo.Status] {
			return ErrorOutput(fmt.Sprintf("todo %q has invalid status %q (must be pending, in_progress, or done)", todo.ID, todo.Status)), nil
		}
	}

	t.mu.Lock()
	*t.todos = make([]Todo, len(in.Todos))
	copy(*t.todos, in.Todos)
	t.mu.Unlock()

	return SuccessOutput(fmt.Sprintf("Updated todo list: %d tasks", len(in.Todos))), nil
}

// TodoReadTool reads the in-memory task list.
type TodoReadTool struct {
	mu    sync.Mutex
	todos *[]Todo // shared pointer with TodoWriteTool
}

func (t *TodoReadTool) Name() string        { return "TodoRead" }
func (t *TodoReadTool) Description() string { return "Read the current todo list" }
func (t *TodoReadTool) IsReadOnly() bool    { return true }

func (t *TodoReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": [],
		"additionalProperties": false
	}`)
}

func (t *TodoReadTool) Execute(_ context.Context, _ *ToolContext, _ json.RawMessage) (*ToolOutput, error) {
	t.mu.Lock()
	todos := make([]Todo, len(*t.todos))
	copy(todos, *t.todos)
	t.mu.Unlock()

	if len(todos) == 0 {
		return SuccessOutput("No todos"), nil
	}

	var sb strings.Builder
	statusIcon := map[string]string{
		"pending":     "[ ]",
		"in_progress": "[~]",
		"done":        "[x]",
	}

	for _, todo := range todos {
		icon := statusIcon[todo.Status]
		if icon == "" {
			icon = "[?]"
		}
		sb.WriteString(fmt.Sprintf("%s %s %s\n", icon, todo.ID, todo.Description))
	}

	return SuccessOutput(strings.TrimRight(sb.String(), "\n")), nil
}

// NewTodoTools creates a TodoWriteTool and TodoReadTool that share the same task list.
func NewTodoTools() (*TodoWriteTool, *TodoReadTool) {
	todos := make([]Todo, 0)
	return &TodoWriteTool{todos: &todos}, &TodoReadTool{todos: &todos}
}
