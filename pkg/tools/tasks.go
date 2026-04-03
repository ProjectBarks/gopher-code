package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// TaskStatus represents the state of a task.
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskDeleted    TaskStatus = "deleted"
)

var validTaskStatuses = map[TaskStatus]bool{
	TaskPending:    true,
	TaskInProgress: true,
	TaskCompleted:  true,
	TaskDeleted:    true,
}

// TaskItem represents a single task.
type TaskItem struct {
	ID          string                 `json:"id"`
	Subject     string                 `json:"subject"`
	Description string                 `json:"description,omitempty"`
	Status      TaskStatus             `json:"status"`
	Owner       string                 `json:"owner,omitempty"`
	ActiveForm  string                 `json:"activeForm,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Blocks      []string               `json:"blocks,omitempty"`
	BlockedBy   []string               `json:"blockedBy,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TaskStore manages tasks in memory.
type TaskStore struct {
	mu     sync.RWMutex
	tasks  map[string]*TaskItem
	nextID int
}

// NewTaskStore creates a new empty task store.
func NewTaskStore() *TaskStore {
	return &TaskStore{tasks: make(map[string]*TaskItem)}
}

func (s *TaskStore) create(subject, description, activeForm string, metadata map[string]interface{}) *TaskItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	// TS uses numeric string IDs ("1", "2", "3"...).
	// Source: utils/tasks.ts:297 — const id = String(highestId + 1)
	id := fmt.Sprintf("%d", s.nextID)
	now := time.Now()
	task := &TaskItem{
		ID:          id,
		Subject:     subject,
		Description: description,
		Status:      TaskPending,
		ActiveForm:  activeForm,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    metadata,
	}
	s.tasks[id] = task
	return task
}

func (s *TaskStore) get(id string) (*TaskItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, false
	}
	// Return a copy
	cp := *t
	if t.Blocks != nil {
		cp.Blocks = make([]string, len(t.Blocks))
		copy(cp.Blocks, t.Blocks)
	}
	if t.BlockedBy != nil {
		cp.BlockedBy = make([]string, len(t.BlockedBy))
		copy(cp.BlockedBy, t.BlockedBy)
	}
	if t.Metadata != nil {
		cp.Metadata = make(map[string]interface{}, len(t.Metadata))
		for k, v := range t.Metadata {
			cp.Metadata[k] = v
		}
	}
	return &cp, true
}

func (s *TaskStore) listNonDeleted() []*TaskItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*TaskItem
	for _, t := range s.tasks {
		if t.Status != TaskDeleted {
			cp := *t
			result = append(result, &cp)
		}
	}
	return result
}

// --- TaskCreateTool ---

// TaskCreateTool creates a new task.
type TaskCreateTool struct {
	store *TaskStore
}

type taskCreateInput struct {
	Subject     string                 `json:"subject"`
	Description string                 `json:"description"`
	ActiveForm  string                 `json:"activeForm,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func (t *TaskCreateTool) Name() string        { return "TaskCreate" }
func (t *TaskCreateTool) Description() string { return "Create a new task." }
func (t *TaskCreateTool) IsReadOnly() bool    { return false }

func (t *TaskCreateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
	"type": "object",
	"properties": {
		"subject": {
			"type": "string",
			"description": "A brief title for the task"
		},
		"description": {
			"type": "string",
			"description": "What needs to be done"
		},
		"activeForm": {
			"type": "string",
			"description": "Present continuous form shown in spinner when in_progress"
		},
		"metadata": {
			"type": "object",
			"description": "Arbitrary metadata to attach to the task"
		}
	},
	"required": ["subject", "description"],
	"additionalProperties": false
}`)
}

func (t *TaskCreateTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in taskCreateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.Subject == "" {
		return ErrorOutput("subject is required"), nil
	}
	if in.Description == "" {
		return ErrorOutput("description is required"), nil
	}
	task := t.store.create(in.Subject, in.Description, in.ActiveForm, in.Metadata)
	return SuccessOutput(fmt.Sprintf("Task %s created: %s", task.ID, task.Subject)), nil
}

// --- TaskListTool ---

// TaskListTool lists all non-deleted tasks.
type TaskListTool struct {
	store *TaskStore
}

func (t *TaskListTool) Name() string        { return "TaskList" }
func (t *TaskListTool) Description() string { return "List all tasks." }
func (t *TaskListTool) IsReadOnly() bool    { return true }

func (t *TaskListTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
	"type": "object",
	"properties": {},
	"required": [],
	"additionalProperties": false
}`)
}

func (t *TaskListTool) Execute(_ context.Context, _ *ToolContext, _ json.RawMessage) (*ToolOutput, error) {
	tasks := t.store.listNonDeleted()
	if len(tasks) == 0 {
		return SuccessOutput("No tasks"), nil
	}

	statusIcon := map[TaskStatus]string{
		TaskPending:    "[ ]",
		TaskInProgress: "[~]",
		TaskCompleted:  "[x]",
	}

	var sb strings.Builder
	for i, task := range tasks {
		icon := statusIcon[task.Status]
		if icon == "" {
			icon = "[?]"
		}
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("%s %s %s", icon, task.ID, task.Subject))
	}
	return SuccessOutput(sb.String()), nil
}

// FormatTaskList returns a formatted task list string for use by the /tasks command.
func FormatTaskList(store *TaskStore) string {
	tasks := store.listNonDeleted()
	if len(tasks) == 0 {
		return "No tasks"
	}

	statusIcon := map[TaskStatus]string{
		TaskPending:    "[ ]",
		TaskInProgress: "[~]",
		TaskCompleted:  "[x]",
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tasks (%d):\n", len(tasks)))
	for _, task := range tasks {
		icon := statusIcon[task.Status]
		if icon == "" {
			icon = "[?]"
		}
		sb.WriteString(fmt.Sprintf("  %s %-10s %-12s %s\n", icon, task.ID, task.Status, task.Subject))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// --- TaskGetTool ---

// TaskGetTool retrieves details of a specific task.
type TaskGetTool struct {
	store *TaskStore
}

type taskGetInput struct {
	TaskID string `json:"taskId"`
}

func (t *TaskGetTool) Name() string        { return "TaskGet" }
func (t *TaskGetTool) Description() string { return "Retrieve task details." }
func (t *TaskGetTool) IsReadOnly() bool    { return true }

func (t *TaskGetTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
	"type": "object",
	"properties": {
		"taskId": {
			"type": "string",
			"description": "The ID of the task to retrieve"
		}
	},
	"required": ["taskId"],
	"additionalProperties": false
}`)
}

func (t *TaskGetTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in taskGetInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.TaskID == "" {
		return ErrorOutput("taskId is required"), nil
	}
	task, ok := t.store.get(in.TaskID)
	if !ok {
		return ErrorOutput(fmt.Sprintf("task %q not found", in.TaskID)), nil
	}
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to marshal task: %s", err)), nil
	}
	return SuccessOutput(string(data)), nil
}

// --- TaskUpdateTool ---

// TaskUpdateTool updates an existing task.
type TaskUpdateTool struct {
	store *TaskStore
}

type taskUpdateInput struct {
	TaskID      string                 `json:"taskId"`
	Status      string                 `json:"status,omitempty"`
	Subject     string                 `json:"subject,omitempty"`
	Description string                 `json:"description,omitempty"`
	Owner       string                 `json:"owner,omitempty"`
	ActiveForm  string                 `json:"activeForm,omitempty"`
	AddBlocks   []string               `json:"addBlocks,omitempty"`
	AddBlockedBy []string              `json:"addBlockedBy,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func (t *TaskUpdateTool) Name() string        { return "TaskUpdate" }
func (t *TaskUpdateTool) Description() string { return "Update an existing task." }
func (t *TaskUpdateTool) IsReadOnly() bool    { return false }

func (t *TaskUpdateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
	"type": "object",
	"properties": {
		"taskId": {
			"type": "string",
			"description": "The ID of the task to update"
		},
		"subject": {
			"type": "string"
		},
		"description": {
			"type": "string"
		},
		"activeForm": {
			"type": "string"
		},
		"status": {
			"type": "string",
			"enum": ["pending", "in_progress", "completed", "deleted"]
		},
		"addBlocks": {
			"type": "array",
			"items": {"type": "string"},
			"description": "Task IDs that this task blocks"
		},
		"addBlockedBy": {
			"type": "array",
			"items": {"type": "string"},
			"description": "Task IDs that block this task"
		},
		"owner": {
			"type": "string"
		},
		"metadata": {
			"type": "object"
		}
	},
	"required": ["taskId"],
	"additionalProperties": false
}`)
}

func (t *TaskUpdateTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in taskUpdateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.TaskID == "" {
		return ErrorOutput("taskId is required"), nil
	}

	t.store.mu.Lock()
	defer t.store.mu.Unlock()

	task, ok := t.store.tasks[in.TaskID]
	if !ok {
		return ErrorOutput(fmt.Sprintf("task %q not found", in.TaskID)), nil
	}

	if in.Status != "" {
		status := TaskStatus(in.Status)
		if !validTaskStatuses[status] {
			return ErrorOutput(fmt.Sprintf("invalid status %q (must be pending, in_progress, completed, or deleted)", in.Status)), nil
		}
		task.Status = status
	}
	if in.Subject != "" {
		task.Subject = in.Subject
	}
	if in.Description != "" {
		task.Description = in.Description
	}
	if in.Owner != "" {
		task.Owner = in.Owner
	}
	if in.ActiveForm != "" {
		task.ActiveForm = in.ActiveForm
	}
	if len(in.AddBlocks) > 0 {
		task.Blocks = append(task.Blocks, in.AddBlocks...)
	}
	if len(in.AddBlockedBy) > 0 {
		task.BlockedBy = append(task.BlockedBy, in.AddBlockedBy...)
	}
	if in.Metadata != nil {
		if task.Metadata == nil {
			task.Metadata = make(map[string]interface{})
		}
		// Merge metadata: setting a key to null deletes it.
		// Source: TaskUpdateTool.ts:200-210
		for k, v := range in.Metadata {
			if v == nil {
				delete(task.Metadata, k)
			} else {
				task.Metadata[k] = v
			}
		}
	}
	task.UpdatedAt = time.Now()

	return SuccessOutput(fmt.Sprintf("Task %s updated", task.ID)), nil
}

// --- TaskStopTool ---

// TaskStopTool stops/cancels a running task by setting its status to deleted.
type TaskStopTool struct {
	store *TaskStore
}

type taskStopInput struct {
	TaskID string `json:"taskId"`
}

func (t *TaskStopTool) Name() string        { return "TaskStop" }
func (t *TaskStopTool) Description() string { return "Stop/cancel a running task." }
func (t *TaskStopTool) IsReadOnly() bool    { return false }

func (t *TaskStopTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
	"type": "object",
	"properties": {
		"taskId": {
			"type": "string",
			"description": "The ID of the task to stop"
		}
	},
	"required": ["taskId"],
	"additionalProperties": false
}`)
}

func (t *TaskStopTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in taskStopInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.TaskID == "" {
		return ErrorOutput("taskId is required"), nil
	}

	t.store.mu.Lock()
	defer t.store.mu.Unlock()

	task, ok := t.store.tasks[in.TaskID]
	if !ok {
		return ErrorOutput(fmt.Sprintf("task %q not found", in.TaskID)), nil
	}

	task.Status = TaskDeleted
	task.UpdatedAt = time.Now()

	return SuccessOutput(fmt.Sprintf("Task %s stopped", task.ID)), nil
}

// --- TaskOutputTool ---

// TaskOutputTool captures output for a task, storing it in the task's metadata.
type TaskOutputTool struct {
	store *TaskStore
}

type taskOutputInput struct {
	TaskID string `json:"taskId"`
	Output string `json:"output"`
}

func (t *TaskOutputTool) Name() string        { return "TaskOutput" }
func (t *TaskOutputTool) Description() string { return "Capture output for a task." }
func (t *TaskOutputTool) IsReadOnly() bool    { return false }

func (t *TaskOutputTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
	"type": "object",
	"properties": {
		"taskId": {
			"type": "string",
			"description": "The ID of the task to record output for"
		},
		"output": {
			"type": "string",
			"description": "The output to capture"
		}
	},
	"required": ["taskId", "output"],
	"additionalProperties": false
}`)
}

func (t *TaskOutputTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in taskOutputInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.TaskID == "" {
		return ErrorOutput("taskId is required"), nil
	}
	if in.Output == "" {
		return ErrorOutput("output is required"), nil
	}

	t.store.mu.Lock()
	defer t.store.mu.Unlock()

	task, ok := t.store.tasks[in.TaskID]
	if !ok {
		return ErrorOutput(fmt.Sprintf("task %q not found", in.TaskID)), nil
	}

	if task.Metadata == nil {
		task.Metadata = make(map[string]interface{})
	}
	task.Metadata["output"] = in.Output
	task.UpdatedAt = time.Now()

	return SuccessOutput(fmt.Sprintf("Output captured for task %s", task.ID)), nil
}

// --- Factory ---

// NewTaskTools creates all task management tools sharing a single store.
func NewTaskTools() []Tool {
	store := NewTaskStore()
	return []Tool{
		&TaskCreateTool{store: store},
		&TaskListTool{store: store},
		&TaskGetTool{store: store},
		&TaskUpdateTool{store: store},
		&TaskStopTool{store: store},
		&TaskOutputTool{store: store},
	}
}

// GetTaskStoreFromRegistry extracts the TaskStore from a registry by finding a
// TaskListTool. Returns nil if no task tools are registered.
func GetTaskStoreFromRegistry(registry *ToolRegistry) *TaskStore {
	t := registry.Get("TaskList")
	if t == nil {
		return nil
	}
	if tl, ok := t.(*TaskListTool); ok {
		return tl.store
	}
	return nil
}
