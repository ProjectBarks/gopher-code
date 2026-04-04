# Batch 16 Notes — Git, Task & Plugin Utils

## What was done

All 6 directories reviewed. No code changes needed.

### utils/git/ — Git utilities
TS has 3 files: gitignore.ts (check-ignore, global gitignore), gitFilesystem.ts (git operations), gitConfigParser.ts. Go handles git operations through:
- `pkg/tools/bash.go` — git commands run directly
- `pkg/prompt/system.go` — git info in system prompt
- `pkg/session/worktree.go` — git worktree management
- `pkg/tools/glob.go` — gitignore-aware file listing

### utils/github/ — GitHub auth
TS has ghAuthStatus.ts for checking `gh auth status`. Not needed in Go — GitHub operations run through Bash tool.

### utils/todo/ — Todo types
TS has types.ts defining TodoItem schema (content, activeForm, status). Go's TodoWriteTool (audited in Batch 6) uses equivalent struct with different field names (ID, Description, Status).

### utils/task/ — Task framework
TS has 5 files: framework.ts (task scheduling), TaskOutput.ts (background task output), diskOutput.ts (disk persistence), outputFormatting.ts, sdkProgress.ts. Go's task tools (audited in Batch 6) use in-memory TaskStore. The TS framework is for background task execution (shell/agent tasks), while Go's tasks are the simple TaskCreate/Update/List/Get/Stop/Output tools.

### utils/skills/ — Skill change detection
TS has skillChangeDetector.ts for detecting when skill files change during a session. Go's skill loading (`pkg/skills/loader.go`) is comprehensive:
- Full frontmatter parsing with 15+ fields
- User and project skill directories
- Source tracking (user, project, bundled, plugin, managed, MCP)
- Agent definitions with tool allowlists/denylists

### utils/plugins/ — Full plugin ecosystem (43 files)
TS has a complete plugin system: manifest validation, marketplace management, dependency resolution, installation, caching, version management, LSP integration, hook registration, output style customization. This is a large feature area that Go doesn't implement. Not needed for basic CLI.

## What's NOT done (deferred)

### Plugin system
43 TS files covering the entire plugin lifecycle. Would be a dedicated multi-week implementation if needed.

### Background task framework
TS utils/task/framework.ts manages background shell/agent tasks with output persistence, formatting, and SDK progress reporting. Go's task tools are simpler (in-memory CRUD without background execution).

### Skill change detection
TS watches for skill file changes mid-session. Go loads skills once at startup.

## Patterns noticed

1. **Skill loading is well-ported**: Go's `pkg/skills/loader.go` faithfully ports the TS skill loading with full frontmatter parsing (description, allowed-tools, arguments, model, effort, paths, shell, context, agent, etc.). The `ParseSkillFromMarkdown` function handles all 15+ frontmatter fields.

2. **Task/Todo duality**: TS has two overlapping systems — the legacy TodoWriteTool (content/activeForm/status) and the newer TaskCreate/Update system (subject/description/status/owner/blocks). Go has both, which matches TS.

3. **Git operations are shell-based in Go**: Rather than dedicated git utility functions, Go runs git commands through the Bash tool. This is simpler but less structured for error handling.
