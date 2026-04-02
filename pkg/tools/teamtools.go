package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TeamStore manages teams in memory.
type TeamStore struct {
	mu    sync.RWMutex
	teams map[string]*Team
}

// Team represents a named team of agent members.
type Team struct {
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// --- TeamCreateTool ---

// TeamCreateTool creates a team of agents for parallel work.
type TeamCreateTool struct {
	store *TeamStore
}

func (t *TeamCreateTool) Name() string        { return "TeamCreate" }
func (t *TeamCreateTool) Description() string { return "Create a team of agents for parallel work" }
func (t *TeamCreateTool) IsReadOnly() bool    { return false }

func (t *TeamCreateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "Name for the team"},
			"members": {"type": "array", "items": {"type": "string"}, "description": "List of agent descriptions"}
		},
		"required": ["name"],
		"additionalProperties": false
	}`)
}

func (t *TeamCreateTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Name    string   `json:"name"`
		Members []string `json:"members"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Name == "" {
		return ErrorOutput("name is required"), nil
	}

	t.store.mu.Lock()
	defer t.store.mu.Unlock()

	if _, exists := t.store.teams[params.Name]; exists {
		return ErrorOutput(fmt.Sprintf("team %q already exists", params.Name)), nil
	}

	team := &Team{
		Name:    params.Name,
		Members: params.Members,
	}
	t.store.teams[params.Name] = team

	memberInfo := "0 members"
	if len(params.Members) > 0 {
		memberInfo = fmt.Sprintf("%d member(s): %s", len(params.Members), strings.Join(params.Members, ", "))
	}
	return SuccessOutput(fmt.Sprintf("Team %q created with %s", params.Name, memberInfo)), nil
}

// --- TeamDeleteTool ---

// TeamDeleteTool deletes a team.
type TeamDeleteTool struct {
	store *TeamStore
}

func (t *TeamDeleteTool) Name() string        { return "TeamDelete" }
func (t *TeamDeleteTool) Description() string { return "Delete a team" }
func (t *TeamDeleteTool) IsReadOnly() bool    { return false }

func (t *TeamDeleteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "Name of the team to delete"}
		},
		"required": ["name"],
		"additionalProperties": false
	}`)
}

func (t *TeamDeleteTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Name == "" {
		return ErrorOutput("name is required"), nil
	}

	t.store.mu.Lock()
	defer t.store.mu.Unlock()

	if _, exists := t.store.teams[params.Name]; !exists {
		return ErrorOutput(fmt.Sprintf("team %q not found", params.Name)), nil
	}

	delete(t.store.teams, params.Name)
	return SuccessOutput(fmt.Sprintf("Team %q deleted", params.Name)), nil
}

// --- Factory ---

// NewTeamTools creates TeamCreate and TeamDelete tools sharing a single store.
func NewTeamTools() []Tool {
	store := &TeamStore{teams: make(map[string]*Team)}
	return []Tool{
		&TeamCreateTool{store: store},
		&TeamDeleteTool{store: store},
	}
}
