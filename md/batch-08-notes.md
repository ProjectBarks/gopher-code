# Batch 8 Notes — Mode & Config Tools

## What was done

### AskUserQuestionTool: Full schema parity
Go only had a simple `question` string parameter. TS has a rich elicitation schema:
- `question` (required): The question text
- `header` (optional): Short chip label (max 12 chars)
- `options` (required, 2-4 items): Array of `{label, description, preview}` choices
- `multiSelect` (optional): Allow multiple selections
- Users can always type "Other" for custom input

Updated Go schema and Execute to match. The REPL/TUI layer would need to render this as an interactive selection UI.

### ConfigTool: Schema alignment (setting/value)
Go used an `action`/`key`/`value` pattern with get/set/list actions. TS uses a `setting`/`value` pattern where:
- Providing just `setting` = GET (read current value)
- Providing `setting` + `value` = SET (write new value)
- Value can be string, boolean, or number (not just string)

Changed Go to match. The TS ConfigTool also has a `supportedSettings` list that maps settings to their storage location (global config vs settings.json) and validates values. Go uses an in-memory map for now.

### PlanMode/Worktree tools: Already at parity
Go PlanState correctly implements:
- Enter/exit transitions with mode save/restore
- HasExitedPlanMode tracking
- NeedsPlanModeExitAttachment flag
- ExitPlanModeResult with plan content and approval fields
- HandlePlanModeTransition for mode change side effects

Worktree tools use git commands directly and handle branch creation with fallback.

## What's NOT done (deferred)

### ConfigTool: Persistent settings
TS ConfigTool reads/writes to actual global config (~/.claude/globalConfig.json) and settings.json. Go uses in-memory map. Supported settings include: theme, editorMode, verbose, model, autoCompactEnabled, autoMemoryEnabled, etc. Each has validation and formatting rules.

### AskUserQuestionTool: Interactive rendering
Go returns the question+options as formatted text. TS renders an interactive UI (using Ink/React) with option selection, keyboard navigation, preview panels, and "Other" text input. The REPL/TUI layer would need to intercept AskUserQuestion tool results and render them interactively.

### ConfigTool: Read-only detection
TS ConfigTool's `isReadOnly` depends on whether `value` is provided. Go always returns true. This affects the tool orchestrator's concurrency decisions.

### ExitPlanMode: Plan approval workflow
TS ExitPlanMode has a rich plan approval flow — in team mode, the leader can approve/reject the plan. Go just exits plan mode directly.

### EnterWorktree: Session tracking
TS tracks worktree sessions with detailed metadata (originalCwd, worktreePath, branch, etc.) for resumption. Go just runs `git worktree add`.

## Patterns noticed

1. **AskUserQuestion is an elicitation tool** — it's designed for structured multi-choice input, not free-form questions. The model should present concrete options whenever possible. The TS prompt explicitly says to add "(Recommended)" to the first option if one is preferred.

2. **ConfigTool is setting-oriented, not action-oriented** — the TS API is `{setting: "theme", value: "dark"}` not `{action: "set", key: "theme", value: "dark"}`. This is a simpler, more intuitive API. The Go change aligns with this.

3. **Plan mode has rich state** — the PlanState in Go already tracks enter/exit, mode restoration, attachment needs, and plan content. This is well-implemented and matches TS behavior for the core flow.
