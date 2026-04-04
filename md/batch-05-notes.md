# Batch 5 Notes — Agent & Team Tools

## What was done

### AgentTool: Full input schema parity
The Go AgentTool only had `prompt` and `description` parameters. TS has 8 parameters:
- `prompt` (required) — task description
- `description` (required) — short summary
- `subagent_type` — selects agent type (Explore, Plan, etc.)
- `model` — override model (haiku/sonnet/opus enum)
- `name` — names the agent for SendMessage targeting
- `run_in_background` — background execution
- `isolation` — "worktree" for git worktree isolation
- `mode` — permission mode ("plan", "acceptEdits", etc.)

All parameters now in Go schema. Model alias resolution added. Max turns increased from 20 to 30 to match TS. ParentSessionID set on child sessions.

### Other tools reviewed — no changes needed
- **SendMessageTool**: Go has file-based mailbox with direct message and broadcast. Matches TS core behavior. TS has additional team member enumeration for broadcast that Go stubs.
- **TeamCreate/TeamDelete**: Go uses in-memory TeamStore. TS uses file-based team persistence. Core operations (create/delete/check exists) match.
- **SkillTool**: Go looks up skill by name, returns prompt with optional args. Matches TS.
- **ToolSearchTool**: Go has keyword scoring (exact name > contains name > description match). TS has similar fuzzy matching. Default max_results=5 in both.

## What's NOT done (deferred)

### AgentTool: Built-in agent types and tool filtering per type
TS has 6+ built-in agent definitions (general-purpose, Explore, Plan, claude-code-guide, statusline-setup, verification) each with specific tool allowlists/denylists and model overrides. Go has the agent filtering infrastructure (FilterToolsForAgent, ResolveAgentTools) but doesn't actually use `subagent_type` to select agent definitions at execution time yet. The parameter is now accepted but the agent type lookup needs wiring.

### AgentTool: Fork subagent feature
TS has a "fork" mode where the agent inherits the parent's full conversation context (no subagent_type → fork). This is a cache optimization feature. Go always creates a fresh session.

### AgentTool: Background execution
`run_in_background` is parsed but not implemented. Needs task system (Batch 6).

### AgentTool: Worktree isolation
`isolation: "worktree"` is parsed but not implemented. Needs git worktree management.

### AgentTool: MCP server initialization per agent
TS agents can define their own MCP servers in frontmatter. Not in Go.

### TeamTools: File-based persistence
TS uses file-based team storage (.claude/teams/). Go uses in-memory. Teams don't survive process restart.

### SendMessageTool: Broadcast to team members
TS reads the team file to enumerate members for broadcast. Go stubs this with an error.

## Patterns noticed

1. **Agent tool filtering** is well-implemented in Go (AllAgentDisallowedTools, AsyncAgentAllowedTools, InProcessTeammateAllowedTools, FilterToolsForAgent, ResolveAgentTools). The infrastructure is there — it just needs the agent definition lookup to wire it up.

2. **Model resolution** follows a priority chain in TS: env var (CLAUDE_CODE_SUBAGENT_MODEL) > tool-specified model > agent definition model > parent model. Go now has model alias resolution but doesn't check the env var or agent definition. The `GetAgentModel` function in agent_tools.go handles this correctly for the existing test cases.

3. **The ONE_SHOT_BUILTIN_AGENT_TYPES** (Explore, Plan) skip the agentId/SendMessage/usage trailer to save tokens. This optimization isn't in Go yet but would be easy to add once agent type lookup is wired up.
