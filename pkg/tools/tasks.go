package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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

// completedIDs returns the set of task IDs with status completed.
func (s *TaskStore) completedIDs() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make(map[string]bool)
	for id, t := range s.tasks {
		if t.Status == TaskCompleted {
			ids[id] = true
		}
	}
	return ids
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

func (t *TaskCreateTool) Name() string { return "TaskCreate" }

// Source: TaskCreateTool/prompt.ts — DESCRIPTION
func (t *TaskCreateTool) Description() string { return "Create a new task in the task list" }
func (t *TaskCreateTool) IsReadOnly() bool     { return false }
func (t *TaskCreateTool) ShouldDefer() bool    { return true }
func (t *TaskCreateTool) SearchHint() string   { return "create a task in the task list" }

// Prompt implements ToolPrompter.
// Source: TaskCreateTool/prompt.ts — getPrompt()
func (t *TaskCreateTool) Prompt() string {
	return `Use this tool to create a structured task list for your current coding session. This helps you track progress, organize complex tasks, and demonstrate thoroughness to the user.
It also helps the user understand the progress of the task and overall progress of their requests.

## When to Use This Tool
- When a task involves complex multi-step work (3+ distinct steps)
- When the task is non-trivial and complex
- When in plan mode to outline the work before executing
- When the user explicitly requests a todo list or task tracking
- When the user provides multiple tasks or requests at once
- After receiving new instructions that change the scope of work
- When starting a task -- mark it in_progress BEFORE beginning work
- After completing a task -- mark it completed and add follow-up tasks if needed

## When NOT to Use This Tool
- For trivial single-step tasks that can be done directly
- When there is only one simple task to do
- When the overhead of task tracking exceeds the benefit
- When the user explicitly asks you not to use tasks

NOTE that you should not use this tool if there is only one trivial task to do. In this case you are better off just doing the task directly.

## Task Fields
- **subject**: A brief title for the task in imperative form (e.g., "Add validation to login form")
- **description**: What needs to be done -- provide enough detail for context
- **activeForm** (optional): Present continuous form shown in spinner when in_progress (e.g., "Running tests")

All tasks are created with status ` + "`pending`" + `.

## Tips
- Write clear, specific subjects so progress is easy to understand at a glance
- Use TaskUpdate to set blocks/blockedBy relationships between tasks
- Check TaskList first to avoid creating duplicate tasks`
}

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
			"description": "Present continuous form shown in spinner when in_progress (e.g., \"Running tests\")"
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
	// Source: TaskCreateTool.ts — result: 'Task #{task.id} created successfully: {task.subject}'
	return SuccessOutput(fmt.Sprintf("Task #%s created successfully: %s", task.ID, task.Subject)), nil
}

// --- TaskListTool ---

// TaskListTool lists all non-deleted tasks.
type TaskListTool struct {
	store *TaskStore
}

func (t *TaskListTool) Name() string { return "TaskList" }

// Source: TaskListTool/prompt.ts — DESCRIPTION
func (t *TaskListTool) Description() string { return "List all tasks in the task list" }
func (t *TaskListTool) IsReadOnly() bool     { return true }
func (t *TaskListTool) ShouldDefer() bool    { return true }
func (t *TaskListTool) SearchHint() string   { return "list all tasks" }

// Prompt implements ToolPrompter.
// Source: TaskListTool/prompt.ts — getPrompt()
func (t *TaskListTool) Prompt() string {
	return `Use this tool to list all tasks in the task list.

## When to Use This Tool
- To see what tasks are available to work on (status: 'pending', no owner, not blocked)
- To check overall progress on the project
- To find tasks that are blocked and need dependencies resolved
- After completing a task, to check for newly unblocked work or claim the next available task
- **Prefer working on tasks in ID order** (lowest ID first) when multiple tasks are available, as earlier tasks often set up context for later ones

## Output
Returns a summary of each task with these fields:
- **id**: Task identifier
- **subject**: Task title
- **status**: 'pending', 'in_progress', or 'completed'
- **owner**: Who is working on it (if assigned)
- **blockedBy**: Tasks that must complete before this one can start

Use TaskGet with a specific task ID to view full details including description and comments.`
}

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

	// Filter out internal tasks (metadata._internal).
	// Source: TaskListTool.ts — filters out tasks with metadata._internal flag
	var visible []*TaskItem
	for _, task := range tasks {
		if task.Metadata != nil {
			if _, isInternal := task.Metadata["_internal"]; isInternal {
				continue
			}
		}
		visible = append(visible, task)
	}

	if len(visible) == 0 {
		// Source: TaskListTool.ts — empty result: 'No tasks found'
		return SuccessOutput("No tasks found"), nil
	}

	// Sort by ID (numeric order) for deterministic output.
	sort.Slice(visible, func(i, j int) bool {
		return visible[i].ID < visible[j].ID
	})

	// Auto-resolve blockedBy: exclude completed task IDs.
	// Source: TaskListTool.ts — build resolvedTaskIds set from completed tasks
	completedIDs := t.store.completedIDs()

	// Source: TaskListTool.ts — per-task line: '#{id} [{status}] {subject}{ ({owner})}{ [blocked by #id1, #id2]}'
	var sb strings.Builder
	for i, task := range visible {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("#%s [%s] %s", task.ID, task.Status, task.Subject))

		if task.Owner != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", task.Owner))
		}

		// Filter blockedBy to exclude resolved (completed) IDs.
		var unresolvedBlockers []string
		for _, bid := range task.BlockedBy {
			if !completedIDs[bid] {
				unresolvedBlockers = append(unresolvedBlockers, "#"+bid)
			}
		}
		if len(unresolvedBlockers) > 0 {
			sb.WriteString(fmt.Sprintf(" [blocked by %s]", strings.Join(unresolvedBlockers, ", ")))
		}
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

func (t *TaskGetTool) Name() string { return "TaskGet" }

// Source: TaskGetTool/prompt.ts — DESCRIPTION
func (t *TaskGetTool) Description() string { return "Get a task by ID from the task list" }
func (t *TaskGetTool) IsReadOnly() bool     { return true }
func (t *TaskGetTool) ShouldDefer() bool    { return true }
func (t *TaskGetTool) SearchHint() string   { return "retrieve a task by ID" }

// Prompt implements ToolPrompter.
// Source: TaskGetTool/prompt.ts — PROMPT
func (t *TaskGetTool) Prompt() string {
	return `Use this tool to retrieve a task by its ID from the task list.

## When to Use This Tool
- When you need the full description and context before starting work on a task
- To understand task dependencies (what it blocks, what blocks it)
- After being assigned a task, to get complete requirements

## Output
Returns full task details:
- **subject**: Task title
- **description**: Detailed requirements and context
- **status**: 'pending', 'in_progress', or 'completed'
- **blocks**: Tasks waiting on this one to complete
- **blockedBy**: Tasks that must complete before this one can start

## Tips
- After fetching a task, verify its blockedBy list is empty before beginning work.
- Use TaskList to see all tasks in summary form.`
}

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
		// Source: TaskGetTool.ts — not-found: 'Task not found'
		return ErrorOutput("Task not found"), nil
	}

	// Source: TaskGetTool.ts — result lines:
	//   'Task #{id}: {subject}'
	//   'Status: {status}'
	//   'Description: {description}'
	//   'Blocked by: #{id1}, #{id2}, ...' (only if non-empty)
	//   'Blocks: #{id1}, #{id2}, ...' (only if non-empty)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Task #%s: %s\n", task.ID, task.Subject))
	sb.WriteString(fmt.Sprintf("Status: %s\n", task.Status))
	sb.WriteString(fmt.Sprintf("Description: %s", task.Description))

	if len(task.BlockedBy) > 0 {
		refs := make([]string, len(task.BlockedBy))
		for i, bid := range task.BlockedBy {
			refs[i] = "#" + bid
		}
		sb.WriteString(fmt.Sprintf("\nBlocked by: %s", strings.Join(refs, ", ")))
	}
	if len(task.Blocks) > 0 {
		refs := make([]string, len(task.Blocks))
		for i, bid := range task.Blocks {
			refs[i] = "#" + bid
		}
		sb.WriteString(fmt.Sprintf("\nBlocks: %s", strings.Join(refs, ", ")))
	}

	return SuccessOutput(sb.String()), nil
}

// --- TaskUpdateTool ---

// TaskUpdateTool updates an existing task.
type TaskUpdateTool struct {
	store *TaskStore
}

type taskUpdateInput struct {
	TaskID       string                 `json:"taskId"`
	Status       string                 `json:"status,omitempty"`
	Subject      string                 `json:"subject,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Owner        string                 `json:"owner,omitempty"`
	ActiveForm   string                 `json:"activeForm,omitempty"`
	AddBlocks    []string               `json:"addBlocks,omitempty"`
	AddBlockedBy []string               `json:"addBlockedBy,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

func (t *TaskUpdateTool) Name() string { return "TaskUpdate" }

// Source: TaskUpdateTool/prompt.ts — DESCRIPTION
func (t *TaskUpdateTool) Description() string { return "Update a task in the task list" }
func (t *TaskUpdateTool) IsReadOnly() bool     { return false }
func (t *TaskUpdateTool) ShouldDefer() bool    { return true }
func (t *TaskUpdateTool) SearchHint() string   { return "update a task in the task list" }

// Prompt implements ToolPrompter.
// Source: TaskUpdateTool/prompt.ts — PROMPT
func (t *TaskUpdateTool) Prompt() string {
	return `Use this tool to update a task in the task list.

## Mark tasks as resolved
- When you have completed the work
- When a task is no longer needed or has been superseded
- IMPORTANT: always mark your assigned tasks as resolved when done
- After resolving a task, call TaskList to see what's next

## Completion Guardrails
- ONLY mark a task as completed when you have FULLY accomplished it
- If you encounter errors, blockers, or cannot finish, keep the task as in_progress
- When blocked, create a new task describing what needs to be resolved
- Never mark a task as completed if:
  - Tests are failing
  - Implementation is partial
  - You encountered unresolved errors
  - You couldn't find necessary files or dependencies

## Delete tasks
Setting status to 'deleted' permanently removes the task.

## Update task details
You can modify any combination of fields in a single call.

## Fields You Can Update
- **status**: pending, in_progress, completed, or deleted
- **subject**: Task title
- **description**: Detailed requirements
- **activeForm**: Present continuous form for spinner display
- **owner**: Who is working on it
- **metadata**: Arbitrary key-value data (set a key to null to delete it)
- **addBlocks**: Task IDs that this task blocks (append-only)
- **addBlockedBy**: Task IDs that block this task (append-only)

## Status Workflow
` + "`pending` -> `in_progress` -> `completed`" + `
Use ` + "`deleted`" + ` to permanently remove a task.

## Staleness
Make sure to read a task's latest state using ` + "`TaskGet`" + ` before updating it.

## Examples
Start working: ` + "`" + `{"taskId": "1", "status": "in_progress"}` + "`" + `
Complete: ` + "`" + `{"taskId": "1", "status": "completed"}` + "`" + `
Delete: ` + "`" + `{"taskId": "1", "status": "deleted"}` + "`" + `
Claim: ` + "`" + `{"taskId": "1", "owner": "agent-1"}` + "`" + `
Add dependency: ` + "`" + `{"taskId": "1", "addBlockedBy": ["2"]}` + "`"
}

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
	// Source: TaskUpdateTool.ts — addBlocks dedup via filter (only new entries added)
	if len(in.AddBlocks) > 0 {
		existing := make(map[string]bool, len(task.Blocks))
		for _, b := range task.Blocks {
			existing[b] = true
		}
		for _, b := range in.AddBlocks {
			if !existing[b] {
				task.Blocks = append(task.Blocks, b)
				existing[b] = true
			}
		}
	}
	// Source: TaskUpdateTool.ts — addBlockedBy dedup via filter
	if len(in.AddBlockedBy) > 0 {
		existing := make(map[string]bool, len(task.BlockedBy))
		for _, b := range task.BlockedBy {
			existing[b] = true
		}
		for _, b := range in.AddBlockedBy {
			if !existing[b] {
				task.BlockedBy = append(task.BlockedBy, b)
				existing[b] = true
			}
		}
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
	TaskID  string `json:"task_id"`
	ShellID string `json:"shell_id,omitempty"`
}

func (t *TaskStopTool) Name() string { return "TaskStop" }

// Source: TaskStopTool/prompt.ts — DESCRIPTION (4-bullet)
func (t *TaskStopTool) Description() string {
	return "- Stops a running background task by its ID\n- Takes a task_id parameter identifying the task to stop\n- Returns a success or failure status\n- Use this tool when you need to terminate a long-running task"
}
func (t *TaskStopTool) IsReadOnly() bool  { return false }
func (t *TaskStopTool) ShouldDefer() bool { return true }
func (t *TaskStopTool) SearchHint() string {
	return "kill a running background task"
}

func (t *TaskStopTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
	"type": "object",
	"properties": {
		"task_id": {
			"type": "string",
			"description": "The ID of the background task to stop"
		},
		"shell_id": {
			"type": "string",
			"description": "Deprecated: use task_id instead"
		}
	},
	"additionalProperties": false
}`)
}

func (t *TaskStopTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in taskStopInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	// Source: TaskStopTool.ts — shell_id backwards compat: either task_id or shell_id
	id := in.TaskID
	if id == "" {
		id = in.ShellID
	}
	if id == "" {
		// Source: TaskStopTool.ts — errorCode 1: 'Missing required parameter: task_id'
		return ErrorOutput("Missing required parameter: task_id"), nil
	}

	t.store.mu.Lock()
	defer t.store.mu.Unlock()

	task, ok := t.store.tasks[id]
	if !ok {
		// Source: TaskStopTool.ts — errorCode 1: 'No task found with ID: {id}'
		return ErrorOutput(fmt.Sprintf("No task found with ID: %s", id)), nil
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
	TaskID  string `json:"task_id"`
	Output  string `json:"output,omitempty"`
	Block   *bool  `json:"block,omitempty"`
	Timeout *int   `json:"timeout,omitempty"`
}

func (t *TaskOutputTool) Name() string { return "TaskOutput" }

// Source: TaskOutputTool.tsx — description
func (t *TaskOutputTool) Description() string {
	return "[Deprecated] -- prefer Read on the task output file path"
}
func (t *TaskOutputTool) IsReadOnly() bool  { return false }
func (t *TaskOutputTool) ShouldDefer() bool { return true }
func (t *TaskOutputTool) SearchHint() string {
	return "read output/logs from a background task"
}

// Prompt implements ToolPrompter.
// Source: TaskOutputTool.tsx — prompt
func (t *TaskOutputTool) Prompt() string {
	return `DEPRECATED: Prefer using the Read tool on the task's output file path instead. Background tasks return their output file path in the tool result, and you receive a <task-notification> with the same path when the task completes -- Read that file directly.

- Retrieves output from a running or completed task (background shell, agent, or remote session)
- Takes a task_id parameter identifying the task
- Returns the task output along with status information
- Use block=true (default) to wait for task completion
- Use block=false for non-blocking check of current status
- Task IDs can be found using the /tasks command
- Works with all task types: background shells, async agents, and remote sessions`
}

func (t *TaskOutputTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
	"type": "object",
	"properties": {
		"task_id": {
			"type": "string",
			"description": "The ID of the task to read output from"
		},
		"output": {
			"type": "string",
			"description": "The output to capture"
		},
		"block": {
			"type": "boolean",
			"description": "Whether to block until the task completes (default true)"
		},
		"timeout": {
			"type": "number",
			"description": "Timeout in milliseconds (0-600000, default 30000)"
		}
	},
	"required": ["task_id"],
	"additionalProperties": false
}`)
}

func (t *TaskOutputTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in taskOutputInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.TaskID == "" {
		// Source: TaskOutputTool.tsx — errorCode 1: 'Task ID is required'
		return ErrorOutput("Task ID is required"), nil
	}

	// Validate timeout bounds.
	// Source: TaskOutputTool.tsx — timeout max: 600000ms
	if in.Timeout != nil && (*in.Timeout < 0 || *in.Timeout > 600000) {
		return ErrorOutput("timeout must be between 0 and 600000"), nil
	}

	t.store.mu.Lock()
	defer t.store.mu.Unlock()

	task, ok := t.store.tasks[in.TaskID]
	if !ok {
		// Source: TaskOutputTool.tsx — 'No task found with ID: {task_id}'
		return ErrorOutput(fmt.Sprintf("No task found with ID: %s", in.TaskID)), nil
	}

	// If output was provided, store it in metadata.
	if in.Output != "" {
		if task.Metadata == nil {
			task.Metadata = make(map[string]interface{})
		}
		task.Metadata["output"] = in.Output
		task.UpdatedAt = time.Now()
		return SuccessOutput(fmt.Sprintf("Output captured for task %s", task.ID)), nil
	}

	// If no output provided, return current task output from metadata.
	if task.Metadata != nil {
		if out, ok := task.Metadata["output"]; ok {
			return SuccessOutput(fmt.Sprintf("%v", out)), nil
		}
	}
	return SuccessOutput("No output available"), nil
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
