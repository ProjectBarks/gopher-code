package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Todo represents a single task in the todo list.
// Source: utils/todo/types.ts — TodoItemSchema
type Todo struct {
	Content    string `json:"content"`
	Status     string `json:"status"` // "pending", "in_progress", "completed"
	ActiveForm string `json:"activeForm"`
}

// validTodoStatuses are the allowed status values.
// Source: utils/todo/types.ts — TodoStatusSchema
var validTodoStatuses = map[string]bool{
	"pending":     true,
	"in_progress": true,
	"completed":   true,
}

// todoWriteResult is the verbatim result message from TS.
// Source: TodoWriteTool.ts:105-106
const todoWriteResult = "Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current tasks if applicable"

// todoWriteDescription is the verbatim description from TS.
// Source: prompt.ts:183-184
const todoWriteDescription = "Update the todo list for the current session. To be used proactively and often to track progress and pending tasks. Make sure that at least one task is in_progress at all times. Always provide both content (imperative) and activeForm (present continuous) for each task."

// todoWritePrompt is the verbatim prompt from TS.
// Source: prompt.ts:3-181
const todoWritePrompt = `Use this tool to create and manage a structured task list for your current coding session. This helps you track progress, organize complex tasks, and demonstrate thoroughness to the user.
It also helps the user understand the progress of the task and overall progress of their requests.

## When to Use This Tool
Use this tool proactively in these scenarios:

1. Complex multi-step tasks - When a task requires 3 or more distinct steps or actions
2. Non-trivial and complex tasks - Tasks that require careful planning or multiple operations
3. User explicitly requests todo list - When the user directly asks you to use the todo list
4. User provides multiple tasks - When users provide a list of things to be done (numbered or comma-separated)
5. After receiving new instructions - Immediately capture user requirements as todos
6. When you start working on a task - Mark it as in_progress BEFORE beginning work. Ideally you should only have one todo as in_progress at a time
7. After completing a task - Mark it as completed and add any new follow-up tasks discovered during implementation

## When NOT to Use This Tool

Skip using this tool when:
1. There is only a single, straightforward task
2. The task is trivial and tracking it provides no organizational benefit
3. The task can be completed in less than 3 trivial steps
4. The task is purely conversational or informational

NOTE that you should not use this tool if there is only one trivial task to do. In this case you are better off just doing the task directly.

## Examples of When to Use the Todo List

<example>
User: I want to add a dark mode toggle to the application settings. Make sure you run the tests and build when you're done!
Assistant: *Creates todo list with the following items:*
1. Creating dark mode toggle component in Settings page
2. Adding dark mode state management (context/store)
3. Implementing CSS-in-JS styles for dark theme
4. Updating existing components to support theme switching
5. Running tests and build process, addressing any failures or errors that occur
*Begins working on the first task*

<reasoning>
The assistant used the todo list because:
1. Adding dark mode is a multi-step feature requiring UI, state management, and styling changes
2. The user explicitly requested tests and build be run afterward
3. The assistant inferred that tests and build need to pass by adding "Ensure tests and build succeed" as the final task
</reasoning>
</example>

<example>
User: Help me rename the function getCwd to getCurrentWorkingDirectory across my project
Assistant: *Uses grep or search tools to locate all instances of getCwd in the codebase*
I've found 15 instances of 'getCwd' across 8 different files.
*Creates todo list with specific items for each file that needs updating*

<reasoning>
The assistant used the todo list because:
1. First, the assistant searched to understand the scope of the task
2. Upon finding multiple occurrences across different files, it determined this was a complex task with multiple steps
3. The todo list helps ensure every instance is tracked and updated systematically
4. This approach prevents missing any occurrences and maintains code consistency
</reasoning>
</example>


<example>
User: I need to implement these features for my e-commerce site: user registration, product catalog, shopping cart, and checkout flow.
Assistant: *Creates a todo list breaking down each feature into specific tasks based on the project architecture*
Let's start with user registration. This will involve creating database models, API endpoints, and frontend forms that integrate with your existing authentication system.

<reasoning>
The assistant used the todo list because:
1. The user provided multiple complex features to implement in a comma separated list
2. The todo list helps organize these large features into manageable tasks
3. This approach allows for tracking progress across the entire implementation
</reasoning>
</example>

<example>
User: Can you help optimize my React application? It's rendering slowly and has performance issues.
Assistant: *Reviews component structure, render patterns, state management, and data fetching*
After analyzing your codebase, I've identified several performance issues.
*Creates todo list with items like: 1) Implementing memoization for expensive calculations in ProductList, 2) Adding virtualization for long lists in Dashboard, 3) Optimizing image loading in Gallery component, 4) Fixing state update loops in ShoppingCart, 5) Reviewing bundle size and implementing code splitting*
Let's start by implementing memoization for the expensive calculations in your ProductList component.

<reasoning>
The assistant used the todo list because:
1. First, the assistant examined the codebase to identify specific performance issues
2. Based on this analysis, it identified multiple optimization opportunities
3. Performance optimization is a non-trivial task requiring multiple steps
4. The todo list helps methodically track improvements across different components
5. This systematic approach ensures all performance bottlenecks are addressed
</reasoning>
</example>

## Examples of When NOT to Use the Todo List

<example>
User: How do I print 'Hello World' in Python?
Assistant: In Python, you can print "Hello World" with this simple code:

python
print("Hello World")

This will output the text "Hello World" to the console when executed.

<reasoning>
The assistant did not use the todo list because this is a single, trivial task that can be completed in one step. There's no need to track multiple tasks or steps for such a straightforward request.
</reasoning>
</example>

<example>
User: What does the git status command do?
Assistant: The git status command shows the current state of your working directory and staging area. It displays which changes have been staged, which haven't, and which files aren't being tracked by Git. This is a fundamental command for understanding the current state of your repository before making commits.

<reasoning>
The assistant did not use the todo list because this is an informational request with no actual coding task to complete. The user is simply asking for an explanation, not for the assistant to perform multiple steps or tasks.
</reasoning>
</example>

<example>
User: Can you add a comment to the calculateTotal function to explain what it does?
Assistant: Sure, let me add a comment to the calculateTotal function to explain what it does.
* Uses the FileEdit tool to add a comment to the calculateTotal function *

<reasoning>
The assistant did not use the todo list because this is a single, straightforward task confined to one location in the code. Adding a comment doesn't require tracking multiple steps or systematic organization.
</reasoning>
</example>

<example>
User: Run npm install for me and tell me what happens.
Assistant: I'll run the npm install command for you.

*Executes: npm install*

The command completed successfully. Here's the output:
[Output of npm install command]

All dependencies have been installed according to your package.json file.

<reasoning>
The assistant did not use the todo list because this is a single command execution with immediate results. There are no multiple steps to track or organize, making the todo list unnecessary for this straightforward task.
</reasoning>
</example>

## Task States and Management

1. **Task States**: Use these states to track progress:
   - pending: Task not yet started
   - in_progress: Currently working on (limit to ONE task at a time)
   - completed: Task finished successfully

   **IMPORTANT**: Task descriptions must have two forms:
   - content: The imperative form describing what needs to be done (e.g., "Run tests", "Build the project")
   - activeForm: The present continuous form shown during execution (e.g., "Running tests", "Building the project")

2. **Task Management**:
   - Update task status in real-time as you work
   - Mark tasks complete IMMEDIATELY after finishing (don't batch completions)
   - Exactly ONE task must be in_progress at any time (not less, not more)
   - Complete current tasks before starting new ones
   - Remove tasks that are no longer relevant from the list entirely

3. **Task Completion Requirements**:
   - ONLY mark a task as completed when you have FULLY accomplished it
   - If you encounter errors, blockers, or cannot finish, keep the task as in_progress
   - When blocked, create a new task describing what needs to be resolved
   - Never mark a task as completed if:
     - Tests are failing
     - Implementation is partial
     - You encountered unresolved errors
     - You couldn't find necessary files or dependencies

4. **Task Breakdown**:
   - Create specific, actionable items
   - Break complex tasks into smaller, manageable steps
   - Use clear, descriptive task names
   - Always provide both forms:
     - content: "Fix authentication bug"
     - activeForm: "Fixing authentication bug"

When in doubt, use this tool. Being proactive with task management demonstrates attentiveness and ensures you complete all requirements successfully.`

// TodoWriteTool manages the in-memory task list.
// Source: TodoWriteTool.ts
type TodoWriteTool struct {
	mu    sync.Mutex
	todos *[]Todo // shared pointer with TodoReadTool
}

type todoWriteInput struct {
	Todos []Todo `json:"todos"`
}

func (t *TodoWriteTool) Name() string        { return "TodoWrite" }
func (t *TodoWriteTool) Description() string { return todoWriteDescription }
func (t *TodoWriteTool) IsReadOnly() bool    { return false }

// Prompt implements ToolPrompter.
// Source: prompt.ts — PROMPT export
func (t *TodoWriteTool) Prompt() string { return todoWritePrompt }

// SearchHint implements SearchHinter.
// Source: TodoWriteTool.ts:33 — searchHint
func (t *TodoWriteTool) SearchHint() string { return "manage the session task checklist" }

// MaxResultSizeChars implements MaxResultSizeCharsProvider.
// Source: TodoWriteTool.ts:34 — maxResultSizeChars: 100_000
func (t *TodoWriteTool) MaxResultSizeChars() int { return 100_000 }

func (t *TodoWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"todos": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"content": {"type": "string", "description": "The imperative form describing what needs to be done"},
						"status": {"type": "string", "enum": ["pending", "in_progress", "completed"], "description": "Task status"},
						"activeForm": {"type": "string", "description": "The present continuous form shown during execution"}
					},
					"required": ["content", "status", "activeForm"]
				},
				"description": "The updated todo list"
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

	// Validate fields.
	// Source: utils/todo/types.ts — content min(1), activeForm min(1), status enum
	for i, todo := range in.Todos {
		if todo.Content == "" {
			return ErrorOutput(fmt.Sprintf("todo[%d]: content cannot be empty", i)), nil
		}
		if todo.ActiveForm == "" {
			return ErrorOutput(fmt.Sprintf("todo[%d]: activeForm cannot be empty", i)), nil
		}
		if !validTodoStatuses[todo.Status] {
			return ErrorOutput(fmt.Sprintf("todo[%d] %q has invalid status %q (must be pending, in_progress, or completed)", i, todo.Content, todo.Status)), nil
		}
	}

	t.mu.Lock()
	*t.todos = make([]Todo, len(in.Todos))
	copy(*t.todos, in.Todos)
	t.mu.Unlock()

	// Source: TodoWriteTool.ts:105-106 — verbatim base result
	return SuccessOutput(todoWriteResult), nil
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
		"completed":   "[x]",
	}

	for _, todo := range todos {
		icon := statusIcon[todo.Status]
		if icon == "" {
			icon = "[?]"
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", icon, todo.Content))
	}

	return SuccessOutput(strings.TrimRight(sb.String(), "\n")), nil
}

// NewTodoTools creates a TodoWriteTool and TodoReadTool that share the same task list.
func NewTodoTools() (*TodoWriteTool, *TodoReadTool) {
	todos := make([]Todo, 0)
	return &TodoWriteTool{todos: &todos}, &TodoReadTool{todos: &todos}
}
