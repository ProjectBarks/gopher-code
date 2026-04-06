package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TeamMaxResultSizeChars is the per-tool max result size for team tools.
// Source: TeamCreateTool.ts:77, TeamDeleteTool.ts:35 — maxResultSizeChars: 100_000
const TeamMaxResultSizeChars = 100_000

// TeamLeadName is the constant name for the team leader agent.
// Source: utils/swarm/constants.ts — TEAM_LEAD_NAME
const TeamLeadName = "team-lead"

// TeamMember represents a member of a team.
type TeamMember struct {
	Name      string `json:"name"`
	AgentType string `json:"agentType,omitempty"`
	IsActive  bool   `json:"isActive"`
}

// TeamStore manages teams in memory.
type TeamStore struct {
	mu    sync.RWMutex
	teams map[string]*Team
	// leaderTeam tracks which team the leader is currently managing (1 team per leader).
	leaderTeam string
}

// Team represents a named team of agent members.
type Team struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Members     []TeamMember `json:"members"`
}

// ListTeams returns the names of all teams in the store.
func (s *TeamStore) ListTeams() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.teams))
	for name := range s.teams {
		names = append(names, name)
	}
	return names
}

// GetTeam returns the team with the given name, or nil if not found.
func (s *TeamStore) GetTeam(name string) *Team {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.teams[name]
}

// --- TeamCreateTool ---

// TeamCreateTool creates a team of agents for coordinating parallel work.
// Source: TeamCreateTool.ts — 240 LOC
type TeamCreateTool struct {
	store *TeamStore
}

func (t *TeamCreateTool) Name() string { return "TeamCreate" }

// Description returns the tool description.
// Source: TeamCreateTool.ts:108 — verbatim
func (t *TeamCreateTool) Description() string {
	return "Create a new team for coordinating multiple agents"
}

func (t *TeamCreateTool) IsReadOnly() bool { return false }

// ShouldDefer implements DeferrableTool.
// Source: TeamCreateTool.ts:78
func (t *TeamCreateTool) ShouldDefer() bool { return true }

// SearchHint implements SearchHinter.
// Source: TeamCreateTool.ts:76
func (t *TeamCreateTool) SearchHint() string { return "create a multi-agent swarm team" }

// MaxResultSizeChars implements MaxResultSizeCharsProvider.
// Source: TeamCreateTool.ts:77
func (t *TeamCreateTool) MaxResultSizeChars() int { return TeamMaxResultSizeChars }

// Prompt implements ToolPrompter.
// Source: TeamCreateTool/prompt.ts — verbatim
func (t *TeamCreateTool) Prompt() string {
	return teamCreatePrompt
}

func (t *TeamCreateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"team_name": {"type": "string", "description": "Name for the new team to create."},
			"description": {"type": "string", "description": "Team description/purpose."},
			"agent_type": {"type": "string", "description": "Type/role of the team lead (e.g., \"researcher\", \"test-runner\"). Used for team file and inter-agent coordination."}
		},
		"required": ["team_name"],
		"additionalProperties": false
	}`)
}

func (t *TeamCreateTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		TeamName    string `json:"team_name"`
		Description string `json:"description"`
		AgentType   string `json:"agent_type"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if strings.TrimSpace(params.TeamName) == "" {
		return ErrorOutput("team_name is required for TeamCreate"), nil
	}

	t.store.mu.Lock()
	defer t.store.mu.Unlock()

	// Check if already leading a team — one team per leader.
	// Source: TeamCreateTool.ts:136-139
	if t.store.leaderTeam != "" {
		return ErrorOutput(fmt.Sprintf(
			"Already leading team %q. A leader can only manage one team at a time. Use TeamDelete to end the current team before creating a new one.",
			t.store.leaderTeam,
		)), nil
	}

	if _, exists := t.store.teams[params.TeamName]; exists {
		return ErrorOutput(fmt.Sprintf("team %q already exists", params.TeamName)), nil
	}

	leadAgentType := params.AgentType
	if leadAgentType == "" {
		leadAgentType = TeamLeadName
	}

	team := &Team{
		Name:        params.TeamName,
		Description: params.Description,
		Members: []TeamMember{
			{
				Name:      TeamLeadName,
				AgentType: leadAgentType,
				IsActive:  true,
			},
		},
	}
	t.store.teams[params.TeamName] = team
	t.store.leaderTeam = params.TeamName

	// Return structured data matching TS Output type.
	// Source: TeamCreateTool.ts:230-235
	leadAgentID := fmt.Sprintf("%s@%s", TeamLeadName, params.TeamName)
	result := map[string]string{
		"team_name":      params.TeamName,
		"team_file_path": fmt.Sprintf("~/.claude/teams/%s/config.json", params.TeamName),
		"lead_agent_id":  leadAgentID,
	}
	data, _ := json.Marshal(result)
	return SuccessOutput(string(data)), nil
}

// --- TeamDeleteTool ---

// TeamDeleteTool cleans up team and task directories when the swarm is complete.
// Source: TeamDeleteTool.ts — 139 LOC
type TeamDeleteTool struct {
	store *TeamStore
}

func (t *TeamDeleteTool) Name() string { return "TeamDelete" }

// Description returns the tool description.
// Source: TeamDeleteTool.ts:51 — verbatim
func (t *TeamDeleteTool) Description() string {
	return "Clean up team and task directories when the swarm is complete"
}

func (t *TeamDeleteTool) IsReadOnly() bool { return false }

// ShouldDefer implements DeferrableTool.
// Source: TeamDeleteTool.ts:36
func (t *TeamDeleteTool) ShouldDefer() bool { return true }

// SearchHint implements SearchHinter.
// Source: TeamDeleteTool.ts:34
func (t *TeamDeleteTool) SearchHint() string { return "disband a swarm team and clean up" }

// MaxResultSizeChars implements MaxResultSizeCharsProvider.
// Source: TeamDeleteTool.ts:35
func (t *TeamDeleteTool) MaxResultSizeChars() int { return TeamMaxResultSizeChars }

// Prompt implements ToolPrompter.
// Source: TeamDeleteTool/prompt.ts — verbatim
func (t *TeamDeleteTool) Prompt() string {
	return teamDeletePrompt
}

func (t *TeamDeleteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"team_name": {"type": "string", "description": "Name of the team to delete."}
		},
		"required": ["team_name"],
		"additionalProperties": false
	}`)
}

func (t *TeamDeleteTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		TeamName string `json:"team_name"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if strings.TrimSpace(params.TeamName) == "" {
		// Source: TeamDeleteTool.ts:127-131 — no-team message
		result := map[string]interface{}{
			"success": true,
			"message": "No team name found, nothing to clean up",
		}
		data, _ := json.Marshal(result)
		return SuccessOutput(string(data)), nil
	}

	t.store.mu.Lock()
	defer t.store.mu.Unlock()

	team, exists := t.store.teams[params.TeamName]
	if !exists {
		return ErrorOutput(fmt.Sprintf("team %q not found", params.TeamName)), nil
	}

	// Active-members guard: filter out the team lead, then check for active members.
	// Source: TeamDeleteTool.ts:81-98
	var activeNames []string
	for _, m := range team.Members {
		if m.Name == TeamLeadName {
			continue
		}
		if m.IsActive {
			activeNames = append(activeNames, m.Name)
		}
	}
	if len(activeNames) > 0 {
		// Source: TeamDeleteTool.ts:94 — verbatim error message
		result := map[string]interface{}{
			"success":   false,
			"message":   fmt.Sprintf("Cannot cleanup team with %d active member(s): %s. Use requestShutdown to gracefully terminate teammates first.", len(activeNames), strings.Join(activeNames, ", ")),
			"team_name": params.TeamName,
		}
		data, _ := json.Marshal(result)
		return ErrorOutput(string(data)), nil
	}

	delete(t.store.teams, params.TeamName)
	if t.store.leaderTeam == params.TeamName {
		t.store.leaderTeam = ""
	}

	// Source: TeamDeleteTool.ts:127-131 — success message
	result := map[string]interface{}{
		"success":   true,
		"message":   fmt.Sprintf("Cleaned up directories and worktrees for team %q", params.TeamName),
		"team_name": params.TeamName,
	}
	data, _ := json.Marshal(result)
	return SuccessOutput(string(data)), nil
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

// --- Prompts ---

// teamCreatePrompt is the verbatim system prompt for TeamCreate.
// Source: TeamCreateTool/prompt.ts — getPrompt()
const teamCreatePrompt = `# TeamCreate

## When to Use

Use this tool proactively whenever:
- The user explicitly asks to use a team, swarm, or group of agents
- The user mentions wanting agents to work together, coordinate, or collaborate
- A task is complex enough that it would benefit from parallel work by multiple agents (e.g., building a full-stack feature with frontend and backend work, refactoring a codebase while keeping tests passing, implementing a multi-step project with research, planning, and coding phases)

When in doubt about whether a task warrants a team, prefer spawning a team.

## Choosing Agent Types for Teammates

When spawning teammates via the Agent tool, choose the ` + "`subagent_type`" + ` based on what tools the agent needs for its task. Each agent type has a different set of available tools — match the agent to the work:

- **Read-only agents** (e.g., Explore, Plan) cannot edit or write files. Only assign them research, search, or planning tasks. Never assign them implementation work.
- **Full-capability agents** (e.g., general-purpose) have access to all tools including file editing, writing, and bash. Use these for tasks that require making changes.
- **Custom agents** defined in ` + "`.claude/agents/`" + ` may have their own tool restrictions. Check their descriptions to understand what they can and cannot do.

Always review the agent type descriptions and their available tools listed in the Agent tool prompt before selecting a ` + "`subagent_type`" + ` for a teammate.

Create a new team to coordinate multiple agents working on a project. Teams have a 1:1 correspondence with task lists (Team = TaskList).

` + "```" + `
{
  "team_name": "my-project",
  "description": "Working on feature X"
}
` + "```" + `

This creates:
- A team file at ` + "`~/.claude/teams/{team-name}/config.json`" + `
- A corresponding task list directory at ` + "`~/.claude/tasks/{team-name}/`" + `

## Team Workflow

1. **Create a team** with TeamCreate - this creates both the team and its task list
2. **Create tasks** using the Task tools (TaskCreate, TaskList, etc.) - they automatically use the team's task list
3. **Spawn teammates** using the Agent tool with ` + "`team_name`" + ` and ` + "`name`" + ` parameters to create teammates that join the team
4. **Assign tasks** using TaskUpdate with ` + "`owner`" + ` to give tasks to idle teammates
5. **Teammates work on assigned tasks** and mark them completed via TaskUpdate
6. **Teammates go idle between turns** - after each turn, teammates automatically go idle and send a notification. IMPORTANT: Be patient with idle teammates! Don't comment on their idleness until it actually impacts your work.
7. **Shutdown your team** - when the task is completed, gracefully shut down your teammates via SendMessage with ` + "`message: {type: \"shutdown_request\"}`" + `.

## Task Ownership

Tasks are assigned using TaskUpdate with the ` + "`owner`" + ` parameter. Any agent can set or change task ownership via TaskUpdate.

## Automatic Message Delivery

**IMPORTANT**: Messages from teammates are automatically delivered to you. You do NOT need to manually check your inbox.

When you spawn teammates:
- They will send you messages when they complete tasks or need help
- These messages appear automatically as new conversation turns (like user messages)
- If you're busy (mid-turn), messages are queued and delivered when your turn ends
- The UI shows a brief notification with the sender's name when messages are waiting

Messages will be delivered automatically.

When reporting on teammate messages, you do NOT need to quote the original message—it's already rendered to the user.

## Teammate Idle State

Teammates go idle after every turn—this is completely normal and expected. A teammate going idle immediately after sending you a message does NOT mean they are done or unavailable. Idle simply means they are waiting for input.

- **Idle teammates can receive messages.** Sending a message to an idle teammate wakes them up and they will process it normally.
- **Idle notifications are automatic.** The system sends an idle notification whenever a teammate's turn ends. You do not need to react to idle notifications unless you want to assign new work or send a follow-up message.
- **Do not treat idle as an error.** A teammate sending a message and then going idle is the normal flow—they sent their message and are now waiting for a response.
- **Peer DM visibility.** When a teammate sends a DM to another teammate, a brief summary is included in their idle notification. This gives you visibility into peer collaboration without the full message content. You do not need to respond to these summaries — they are informational.

## Discovering Team Members

Teammates can read the team config file to discover other team members:
- **Team config location**: ` + "`~/.claude/teams/{team-name}/config.json`" + `

The config file contains a ` + "`members`" + ` array with each teammate's:
- ` + "`name`" + `: Human-readable name (**always use this** for messaging and task assignment)
- ` + "`agentId`" + `: Unique identifier (for reference only - do not use for communication)
- ` + "`agentType`" + `: Role/type of the agent

**IMPORTANT**: Always refer to teammates by their NAME (e.g., "team-lead", "researcher", "tester"). Names are used for:
- ` + "`to`" + ` when sending messages
- Identifying task owners

Example of reading team config:
` + "```" + `
Use the Read tool to read ~/.claude/teams/{team-name}/config.json
` + "```" + `

## Task List Coordination

Teams share a task list that all teammates can access at ` + "`~/.claude/tasks/{team-name}/`" + `.

Teammates should:
1. Check TaskList periodically, **especially after completing each task**, to find available work or see newly unblocked tasks
2. Claim unassigned, unblocked tasks with TaskUpdate (set ` + "`owner`" + ` to your name). **Prefer tasks in ID order** (lowest ID first) when multiple tasks are available, as earlier tasks often set up context for later ones
3. Create new tasks with ` + "`TaskCreate`" + ` when identifying additional work
4. Mark tasks as completed with ` + "`TaskUpdate`" + ` when done, then check TaskList for next work
5. Coordinate with other teammates by reading the task list status
6. If all available tasks are blocked, notify the team lead or help resolve blocking tasks

**IMPORTANT notes for communication with your team**:
- Do not use terminal tools to view your team's activity; always send a message to your teammates (and remember, refer to them by name).
- Your team cannot hear you if you do not use the SendMessage tool. Always send a message to your teammates if you are responding to them.
- Do NOT send structured JSON status messages like ` + "`{\"type\":\"idle\",...}`" + ` or ` + "`{\"type\":\"task_completed\",...}`" + `. Just communicate in plain text when you need to message teammates.
- Use TaskUpdate to mark tasks completed.
- If you are an agent in the team, the system will automatically send idle notifications to the team lead when you stop.`

// teamDeletePrompt is the verbatim system prompt for TeamDelete.
// Source: TeamDeleteTool/prompt.ts — getPrompt()
const teamDeletePrompt = `# TeamDelete

Remove team and task directories when the swarm work is complete.

This operation:
- Removes the team directory (` + "`~/.claude/teams/{team-name}/`" + `)
- Removes the task directory (` + "`~/.claude/tasks/{team-name}/`" + `)
- Clears team context from the current session

**IMPORTANT**: TeamDelete will fail if the team still has active members. Gracefully terminate teammates first, then call TeamDelete after all teammates have shut down.

Use this when all teammates have finished their work and you want to clean up the team resources. The team name is automatically determined from the current session's team context.`
