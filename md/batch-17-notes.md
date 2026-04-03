# Batch 17 Notes — Storage & Swarm Utils

## What was done

All 6 directories reviewed. No code changes needed.

### utils/secureStorage/ → pkg/auth/auth.go
Go's auth system matches TS's 3-tier secure storage exactly:
1. ANTHROPIC_API_KEY environment variable (highest priority)
2. OS keyring via go-keyring (macOS Keychain, Linux secret-service, Windows credential manager)
3. ~/.claude/auth.json plaintext fallback

TS has the same chain: macOS Keychain → plainTextStorage fallback. The keyring service name matches ("claude-code").

### utils/swarm/ → pkg/session/team.go + teammate.go + mailbox.go
Go has 1032 lines of team/teammate infrastructure:
- **team.go**: SpawnTeam, ReadTeamFile, AddTeamMember, CleanupTeam — file-based team storage
- **teammate.go**: TeammateContext, TeammateColorManager with round-robin color assignment
- **mailbox.go**: ReadMailbox, WriteToMailbox, ReadUnreadMessages, MarkAllRead — file-based inter-agent messaging

TS utils/swarm has 21 files covering: spawn backends (tmux, in-process), leader permission bridging, pane management, teammate mode snapshots. Go covers the core data structures; the spawn backends are more complex.

### utils/filePersistence/ — Enterprise feature
TS uploads modified files to Files API at end of each turn. BYOC/cloud mode feature. Go doesn't need this for standalone CLI.

### utils/processUserInput/ — UI/REPL feature
TS processes user text input: slash commands, image pastes, IDE selections, agent mentions, effort level changes. Go handles this at the REPL/TUI level directly.

### utils/suggestions/ — TUI autocomplete
TS has 5 files: command suggestions, directory completion, shell history completion, skill usage tracking, Slack channel suggestions. Go's TUI has basic input history. Not needed for headless/pipe mode.

### utils/telemetry/ — Platform infrastructure
TS has 9 files: OpenTelemetry instrumentation, BigQuery exporter, session tracing, Perfetto tracing, skill load events, logger. Platform-specific observability. Go doesn't have this but could add via standard Go telemetry libraries.

## Patterns noticed

1. **Auth is the most important parity point**: Go's 3-tier auth (env → keyring → plaintext) matches TS exactly. This is critical for API access.

2. **Swarm is partially ported**: Go has the data layer (team files, teammate context, mailbox messaging) but not the spawn layer (tmux pane management, in-process teammates, leader permission bridging). The data layer is the foundation — spawn could be added on top.

3. **Remaining batches (18-34) are increasingly UI/command-specific**: Batches 18-20 cover platform utils and runtime systems. Batches 21-25 are TUI components (React/Ink in TS, Bubbletea in Go). Batches 26-32 are slash commands. Batches 33-34 are data/extras. These are progressively less likely to have concrete Go parity gaps.
