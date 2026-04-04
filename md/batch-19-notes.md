# Batch 19 Notes — Coordinator & Tasks Runtime

## What was done

All 5 directories reviewed. All are framework-specific or advanced runtime — no Go changes needed.

### hooks/ — 104 React hooks
TS uses React/Ink for the TUI, with hooks for everything: useInput, useSearchInput, useCanUseTool, useTerminalSize, useSwarmInitialization, etc. Go uses Bubbletea with its Update/View architecture — these are fundamentally different UI paradigms. Go's TUI is in `pkg/ui/` with Bubbletea models.

### coordinator/ — Worker agent management
Single file: coordinatorMode.ts. Feature-gated behind `COORDINATOR_MODE`. Defines worker agent types, internal worker tools (TeamCreate, TeamDelete, SendMessage, SyntheticOutput), and coordinator-specific system prompts. Enterprise feature for multi-agent orchestration.

### tasks/ — Background task execution runtime (12 files)
Manages running background tasks:
- **LocalShellTask**: Background Bash commands (run_in_background)
- **LocalAgentTask**: Background agent execution
- **RemoteAgentTask**: CCR remote agent execution
- **InProcessTeammateTask**: In-process teammate with shared message history
- **DreamTask**: Auto-dream background task
- **types.ts**: TaskState, TaskProgress interfaces
- **stopTask.ts**: Task termination logic

Go has in-memory task CRUD (TaskCreate/Update/List/Get/Stop/Output from Batch 6) but no background execution runtime. The `run_in_background` parameter is parsed but not implemented.

### buddy/ — Companion sprite/mascot (6 files)
Visual companion feature: animated sprite, reaction system, notification handling. Pure UI cosmetic — not functional.

### assistant/ — Session history API (1 file)
Fetches session history from CCR cloud API with pagination. OAuth-dependent, remote-only feature.

## Patterns noticed

1. **React hooks are the biggest architectural gap**: 104 hooks representing the entire React/Ink UI layer. Go's Bubbletea architecture is fundamentally different (message passing vs hooks), so these can never be "ported" — they're replaced by Bubbletea's Update/View pattern.

2. **Background task runtime is the most impactful missing feature**: The tasks/ directory implements the execution layer for `run_in_background` in BashTool and AgentTool. Without this, Go can't execute tools in the background. The data layer (TaskCreate/Update) exists but the execution layer doesn't.

3. **Coordinator mode builds on existing infrastructure**: It uses the agent system, team tools, and task system that Go already has. The coordinator-specific pieces are a system prompt variant and worker agent type definitions.
