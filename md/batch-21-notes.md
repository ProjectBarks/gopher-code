# Batch 21 Notes — TUI Core

## What was done

### ink/ functional transfer
Key functionality transferred from TS ink/ to Go:
1. **Alternate screen** (View().AltScreen = true) — preserves terminal history on exit
2. **Platform-specific spinner glyphs** — macOS uses ✽, Ghostty uses *, Linux uses * instead of ✳

### Component coverage mapping

| TS Component | Go Equivalent | Status |
|---|---|---|
| **ink/** (90 files) | Bubbletea framework | ✅ Different framework, equivalent capabilities |
| **PromptInput/** (21 files) | input.go + input_with_history.go + slash_input.go + command_palette.go | ✅ Core input covered |
| **messages/** (41 files) | message_bubble.go + conversation.go + streaming_text.go + agent_message.go | ✅ Message rendering covered |
| **Spinner/** (12 files) | spinner_verbs.go (188 verbs, 8 completion verbs, platform glyphs) | ✅ Spinner covered |
| **LogoV2/** (15 files) | welcome.go | ✅ Welcome screen covered |

## Deferred
- PromptInput: shimmer animation, voice indicator, swarm banner, prompt suggestions
- Messages: grouped tool use, highlighted thinking, plan approval UI, image rendering
- LogoV2: animated mascot, feed columns, upsells
