# Batch 13 Notes — Remaining Services

## What was done

All 4 services reviewed. None require Go code changes.

### remoteManagedSettings
- **Purpose**: Fetches organization-level managed settings from Anthropic API for enterprise customers
- **Behavior**: Fail-open — if fetch fails, continues without remote settings. ETag-based caching, background polling.
- **Why not needed in Go**: Go works correctly without it. When remote settings aren't available, local settings apply. Enterprise-only feature requiring OAuth.

### settingsSync
- **Purpose**: Syncs user settings and memory files across Claude Code environments (CLI ↔ CCR)
- **Behavior**: Interactive CLI uploads local settings; CCR downloads before plugin install
- **Why not needed in Go**: OAuth-dependent, cloud-only. Go uses local settings files directly.

### tips
- **Purpose**: Contextual tip system shown on spinner during model calls
- **Behavior**: Selects tips based on history (least-recently-shown), context, and settings
- **Why not needed in Go**: Go already has `SpinnerTips` in `pkg/ui/components/spinner_verbs.go` with a static tip list. TS has a more sophisticated system with tip history tracking and contextual filtering, but the basic tip display is covered.

### toolUseSummary
- **Purpose**: Generates human-readable one-line summaries of tool batches using Haiku
- **Behavior**: SDK feature for mobile app progress display. Calls Haiku with tool inputs/outputs and gets ~30-char summary label.
- **Why not needed in Go**: SDK/mobile-only feature. CLI doesn't need these summaries.

## Patterns noticed

1. **Cloud-dependent services cluster**: Batches 11-13 have revealed a pattern — many TS services (remoteManagedSettings, settingsSync, policyLimits, teamMemorySync) depend on the Anthropic cloud API with OAuth. Go operates standalone with local files, which is correct for a CLI tool.

2. **Tip system could be enhanced**: Go's static `SpinnerTips` list works but TS's tip system is more sophisticated with: (a) history tracking across sessions to avoid repetition, (b) contextual filtering based on current state, (c) user ability to disable tips. Low priority enhancement.

3. **toolUseSummary is interesting architecturally**: It uses a secondary Haiku call to generate progress labels. This is the same pattern as WebFetch's `applyPromptToMarkdown` — using a cheap model call for UX enhancement. If Go ever needs this, it would be straightforward to add.
