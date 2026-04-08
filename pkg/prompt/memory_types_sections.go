package prompt

// Source: utils/memdir/memoryTypeSections.ts

// MemoryTypeSection returns the system prompt section describing memory types.
// This guides the LLM on how to categorize and store memories.
const MemoryTypeSection = `## Types of memory

There are several discrete types of memory that you can store:

- **user**: Information about the user's role, goals, and preferences
- **feedback**: Guidance on how to approach work (corrections and confirmations)
- **project**: Ongoing work, goals, and initiatives within the project
- **reference**: Pointers to information in external systems`

// MemoryFormatSection describes the file format for memory files.
const MemoryFormatSection = `## How to save memories

Write the memory to its own file using this frontmatter format:

` + "```markdown" + `
---
name: {{memory name}}
description: {{one-line description}}
type: {{user, feedback, project, reference}}
---

{{memory content}}
` + "```"
