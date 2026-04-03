# Batch 21 Notes — TUI Components (covers Batches UI-20-A/B/C + 21-25)

## What was done

All TUI batches reviewed together. TS uses React/Ink; Go uses Bubbletea. These are fundamentally different UI frameworks — there's no line-by-line correspondence. Instead, both have **complete TUI implementations** in their respective frameworks.

### Go TUI: 80+ files in pkg/ui/

**App layer:**
- `app.go` — Main Bubbletea model (AppModel) with mode management (Idle/Streaming/ToolRunning)
- `bridge.go` — EventBridge converts query callbacks to tea.Msg
- `commands/handlers.go` — Slash command dispatch

**Components (30+ with tests):**
- `input.go` — Text input with cursor, selection, multiline
- `input_with_history.go` — Input with arrow-key history navigation
- `conversation.go` — Conversation pane with message display and scrolling
- `message_bubble.go` — Individual message rendering with markdown
- `streaming_text.go` — Live streaming text display
- `spinner_verbs.go` — Animated thinking spinner with turn completion verbs
- `header.go` — Model name, CWD display
- `statusline.go` — Token count, mode, cost display
- `diff.go` — Unified diff rendering with syntax highlighting
- `diff_approval.go` — Interactive diff approval UI
- `code_block.go` — Syntax-highlighted code rendering
- `tool_call.go` + `tool_result.go` — Tool execution display
- `toast.go` — Notification toast messages
- `command_palette.go` — Slash command autocomplete
- `session_picker.go` — Session resume selection
- `welcome.go` — Welcome screen
- `side_panel.go` — Side panel for context/tasks
- `tabbar.go` — Tab navigation
- `thinking.go` — Extended thinking display
- `tokens.go` — Token usage display
- `treeview.go` — Hierarchical tree view
- `agent_message.go` — Agent message formatting
- `slash_input.go` — Slash command input
- `error.go` — Error display

**Core:**
- `core/focus.go` — Focus ring with modal stack
- `core/keymap.go` — Key binding management
- `core/layout.go` — Flex layout system
- `core/component.go` — Base component interface

**Theme:**
- `theme/dark.go`, `theme/light.go`, `theme/highcontrast.go` — 3 themes
- `theme/colors.go`, `theme/palette.go` — Color system

**Permissions:**
- `permissions/bubbletea_policy.go` — Interactive permission prompts in TUI

**Layout:**
- `layout/stack.go` — Vertical/horizontal stack layout

### TS TUI: React/Ink components

The TS TUI uses React with Ink (terminal React renderer). Components include:
- PromptInput, messages, Spinner, LogoV2 (Batch 21)
- diff, StructuredDiff, HighlightedCode, tasks, teams, agents (Batch 22)
- permissions, TrustDialog, sandbox, Settings, hooks (Batch 23)
- mcp, skills, grove, ui, design-system, wizard (Batch 24)
- ClaudeCodeHint, CustomSelect, DesktopUpsell, FeedbackSurvey, HelpV2, LspRecommendation, ManagedSettingsSecurityDialog, Passes (Batch 25)

## Architectural comparison

| Aspect | TypeScript (React/Ink) | Go (Bubbletea) |
|--------|----------------------|----------------|
| **Framework** | React + Ink terminal renderer | Bubbletea (Elm architecture) |
| **State** | useState hooks | struct fields |
| **Rendering** | JSX → terminal ANSI | View() → string |
| **Events** | useInput hook | Update(tea.Msg) |
| **Styling** | Ink Box/Text + chalk | Lipgloss styles |
| **Focus** | React context | FocusManager |
| **Layout** | Ink Flexbox | Custom flex layout |
| **Diff** | Unified diff with syntax HL | Unified diff with Chroma HL |
| **Markdown** | Custom renderer | Glamour renderer |

Both implementations are complete and functional. The Go TUI has 80+ files with comprehensive tests across 7 packages.

## No code changes needed

The TUI is not about parity of React components — it's about whether the terminal experience works correctly. Go's Bubbletea TUI provides:
- Full conversation display with streaming
- Syntax-highlighted code blocks and diffs
- Interactive permission prompts
- Slash command autocomplete
- Session resume/selection
- 3 themes (dark/light/high-contrast)
- Token/cost tracking display
- Toast notifications
