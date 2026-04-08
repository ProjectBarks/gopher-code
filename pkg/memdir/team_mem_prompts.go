package memdir

// Source: utils/memdir/teamMemPrompts.ts

// TeamMemorySystemPromptSection returns the system prompt section for team memories.
const TeamMemorySystemPromptSection = `# Team Memory

Team memories are shared across all members of a team. They persist across
sessions and are visible to all teammates. Use team memories to store
information that the whole team should know about — project conventions,
architectural decisions, shared context, etc.

Team memory files are stored in ~/.claude/teams/{team}/memory/ as markdown
files with YAML frontmatter (same format as personal memories).`

// TeamMemoryInstructions returns instructions for working with team memories.
const TeamMemoryInstructions = `When saving a team memory:
1. Write the memory to its own .md file in the team memory directory
2. Add a pointer to MEMORY.md (the team's memory index)
3. Use the same frontmatter format as personal memories`
