package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// CronEntry represents a scheduled cron job.
type CronEntry struct {
	ID     string `json:"id"`
	Cron   string `json:"cron"`
	Prompt string `json:"prompt"`
	Active bool   `json:"active"`
}

// CronStore is the in-memory store for cron entries.
type CronStore struct {
	mu      sync.Mutex
	entries map[string]*CronEntry
	nextID  int
}

// CronCreateTool creates a new cron entry.
type CronCreateTool struct {
	store *CronStore
}

func (t *CronCreateTool) Name() string        { return "CronCreate" }
func (t *CronCreateTool) Description() string { return "Schedule a prompt to run on a cron schedule" }
func (t *CronCreateTool) IsReadOnly() bool    { return false }

func (t *CronCreateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"cron": {"type": "string", "description": "Cron expression (e.g. '*/5 * * * *')"},
			"prompt": {"type": "string", "description": "The prompt to execute on schedule"},
			"recurring": {"type": "boolean", "description": "Whether the job recurs (default true)"}
		},
		"required": ["cron", "prompt"],
		"additionalProperties": false
	}`)
}

func (t *CronCreateTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Cron      string `json:"cron"`
		Prompt    string `json:"prompt"`
		Recurring *bool  `json:"recurring"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Cron == "" {
		return ErrorOutput("cron expression is required"), nil
	}
	if params.Prompt == "" {
		return ErrorOutput("prompt is required"), nil
	}

	t.store.mu.Lock()
	t.store.nextID++
	id := fmt.Sprintf("cron-%d", t.store.nextID)
	t.store.entries[id] = &CronEntry{
		ID:     id,
		Cron:   params.Cron,
		Prompt: params.Prompt,
		Active: true,
	}
	t.store.mu.Unlock()

	return SuccessOutput(fmt.Sprintf("Created cron job %s: %s", id, params.Cron)), nil
}

// CronDeleteTool deletes a cron entry.
type CronDeleteTool struct {
	store *CronStore
}

func (t *CronDeleteTool) Name() string        { return "CronDelete" }
func (t *CronDeleteTool) Description() string { return "Cancel a scheduled cron job" }
func (t *CronDeleteTool) IsReadOnly() bool    { return false }

func (t *CronDeleteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {"type": "string", "description": "The cron job ID to cancel"}
		},
		"required": ["id"],
		"additionalProperties": false
	}`)
}

func (t *CronDeleteTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.ID == "" {
		return ErrorOutput("id is required"), nil
	}

	t.store.mu.Lock()
	_, exists := t.store.entries[params.ID]
	if exists {
		delete(t.store.entries, params.ID)
	}
	t.store.mu.Unlock()

	if !exists {
		return ErrorOutput(fmt.Sprintf("cron job %s not found", params.ID)), nil
	}
	return SuccessOutput(fmt.Sprintf("Deleted cron job %s", params.ID)), nil
}

// CronListTool lists all cron entries.
type CronListTool struct {
	store *CronStore
}

func (t *CronListTool) Name() string        { return "CronList" }
func (t *CronListTool) Description() string { return "List all scheduled cron jobs" }
func (t *CronListTool) IsReadOnly() bool    { return true }

func (t *CronListTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": [],
		"additionalProperties": false
	}`)
}

func (t *CronListTool) Execute(_ context.Context, _ *ToolContext, _ json.RawMessage) (*ToolOutput, error) {
	t.store.mu.Lock()
	entries := make([]*CronEntry, 0, len(t.store.entries))
	for _, e := range t.store.entries {
		entries = append(entries, e)
	}
	t.store.mu.Unlock()

	if len(entries) == 0 {
		return SuccessOutput("No cron jobs scheduled"), nil
	}

	var sb strings.Builder
	for _, e := range entries {
		status := "active"
		if !e.Active {
			status = "inactive"
		}
		sb.WriteString(fmt.Sprintf("%s  %s  [%s]  %s\n", e.ID, e.Cron, status, e.Prompt))
	}
	return SuccessOutput(strings.TrimRight(sb.String(), "\n")), nil
}

// NewCronTools creates the cron tool set sharing the same store.
func NewCronTools() []Tool {
	store := &CronStore{entries: make(map[string]*CronEntry)}
	return []Tool{
		&CronCreateTool{store: store},
		&CronDeleteTool{store: store},
		&CronListTool{store: store},
	}
}
