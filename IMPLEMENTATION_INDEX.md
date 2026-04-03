# Gopher UI Redesign — Implementation Index

**6-phase Bubbletea UI redesign. Phases 1-5 complete. Phase 6 (Visual Parity) is active.**

---

## Current Status

| Phase | Focus | Tasks | Status |
|-------|-------|-------|--------|
| Phase 1 | Core framework, layout, components | 8/8 | ✅ Complete |
| Phase 2 | Content rendering, streaming, tools | 6/6 | ✅ Complete |
| Phase 3 | Dialogs, permissions, errors | 9/9 | ✅ Complete |
| Phase 4 | Slash commands, sessions, header | 5/5 | ✅ Complete |
| Phase 5 | Power features, responsive layout | 6/6 | ✅ Complete |
| **Phase 6** | **Visual parity with Claude Code** | **0/10** | **🔴 ACTIVE** |

---

## Phase 6: Visual Parity Tasks

These gaps were identified by comparing screenshots of Claude Code v2.1.91 against
Gopher's current output, then verified against Claude Code source code.

| # | Task | Files | Lines | Status |
|---|------|-------|-------|--------|
| 6.1 | Welcome Screen (bordered box, mascot, tips) | `welcome.go` | ~350 | [ ] |
| 6.2 | Prompt character "›" (U+203A) | `input.go`, `message_bubble.go` | ~15 | [ ] |
| 6.3 | Message connector "└" spacing | `message_bubble.go` | ~30 | [ ] |
| 6.4 | Spinner verb system (188 verbs, 6-frame glyph) | `spinner_verbs.go` (new) | ~300 | [ ] |
| 6.5 | User message styling (bold + background) | `message_bubble.go` | ~25 | [ ] |
| 6.6 | Divider line ━━━ + status bar overhaul | `app.go`, `statusline.go` | ~80 | [ ] |
| 6.7 | Welcome screen integration in AppModel | `app.go` | ~50 | [ ] |
| 6.8 | Spinner integration in conversation flow | `app.go`, `conversation.go` | ~60 | [ ] |
| 6.9 | Effort level display (○/◐/●/◉) | `spinner_verbs.go`, `app.go` | ~40 | [ ] |
| 6.10 | Tip line below spinner | `spinner_verbs.go` | ~30 | [ ] |

**Total Phase 6 effort**: ~980 lines

---

## Key Visual Gaps (Screenshot Evidence)

### Claude Code renders:
- **Bordered welcome box** with version title, mascot, tips, recent activity
- **"›"** prompt character (not ">")
- **"└"** response connector with 2-space indent
- **"✻ Verb…"** animated spinner during thinking (188 random verbs)
- **Bold white** user messages on dark background rows
- **━━━** heavy horizontal divider above input
- **"esc to interrupt"** during streaming
- **Effort icons** ○◐●◉ next to thinking state

### Gopher currently renders:
- **Flat header** "🐿 Gopher │ model │ path"
- **">"** ASCII prompt character
- **"⎿"** connector with minimal spacing
- **Blinking cursor only** during streaming (no verb, no spinner animation)
- **Dim gray** user messages (opposite of Claude Code)
- **No divider** between conversation and input
- **"Streaming"** text in status bar (no interrupt hint)
- **No effort icons**

---

## Implementation Order

**Quick wins first** (tasks 6.2, 6.3, 6.5, 6.6), then **medium features** (6.4, 6.8-6.10), then **big feature** (6.1, 6.7):

1. **Task 6.2** — Change `>` to `›` (5 min, huge visual impact)
2. **Task 6.3** — Fix connector to `└` with proper spacing (10 min)
3. **Task 6.5** — Bold user messages with background (15 min)
4. **Task 6.6** — Add divider + "esc to interrupt" (30 min)
5. **Task 6.4** — Spinner verb system (1 hour)
6. **Task 6.8** — Wire spinner into conversation (30 min)
7. **Task 6.9** — Effort level display (20 min)
8. **Task 6.10** — Tip line (15 min)
9. **Task 6.1** — Welcome screen component (1.5 hours)
10. **Task 6.7** — Welcome integration in AppModel (30 min)

---

## Documents

| Document | Purpose |
|----------|---------|
| [IMPLEMENTATION_PHASES.md](IMPLEMENTATION_PHASES.md) | Detailed task breakdown with checklists |
| [md/GOPHER_UI_REDESIGN_PROPOSAL.md](md/GOPHER_UI_REDESIGN_PROPOSAL.md) | Original design proposal |
| [md/UI_REDESIGN_COMPONENT_CATALOG.md](md/UI_REDESIGN_COMPONENT_CATALOG.md) | Component specifications |

## Reference Source

- **Claude Code source**: `/Users/alexgaribaldi/claude-code-v2/research/claude-code-source-build`
  - Spinner: `src/components/Spinner/`
  - Welcome: `src/components/LogoV2/WelcomeV2.tsx`
  - Messages: `src/components/messages/`
  - Figures: `src/constants/figures.ts`
  - Verbs: `src/constants/spinnerVerbs.ts`

## Commands

```bash
# Run tests
go test -race ./pkg/ui/...

# Build
go build -o gopher ./cmd/gopher-code

# Run new UI
GOPHER_NEW_UI=1 ./gopher

# Run old REPL (fallback)
./gopher
```
