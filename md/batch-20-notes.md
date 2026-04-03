# Batch 20 Notes — Bridge & Remote

## What was done

All 4 directories reviewed. All are CCR/enterprise remote infrastructure — no Go changes needed.

### bridge/ — REPL Bridge for Remote Control (31 files)
The bridge connects the CLI to the Remote Control web UI:
- Polling-based state sync (pollConfigDefaults.ts)
- Session API (codeSessionApi.ts)
- WebSocket event streaming
- Permission request bridging
- UI state synchronization
This is the "Remote Control" feature that lets users control Claude Code from a web browser. Not applicable to standalone CLI.

### remote/ — Remote Session Management (4 files)
- SessionsWebSocket: WebSocket connection to remote sessions
- RemoteSessionManager: Manages remote session lifecycle
- remotePermissionBridge: Bridges permission requests to remote UI
- sdkMessageAdapter: Adapts SDK messages for remote protocol
Enterprise CCR feature.

### server/ — Direct Connect Server (3 files)
Local HTTP server for peer-to-peer connections:
- createDirectConnectSession: Initiates direct connections
- directConnectManager: Manages active connections
Used for direct IDE/tool integration. Not needed for CLI.

### upstreamproxy/ — CONNECT Relay Proxy (2 files)
Local CONNECT relay for CCR environments:
- Injects credentials for org-configured upstream proxies
- Gated on CLAUDE_CODE_REMOTE + GrowthBook
Enterprise network infrastructure.

## Summary

This batch completes the infrastructure layer review. All bridge/remote/server/proxy features are CCR (Claude Code Remote) enterprise infrastructure that doesn't apply to the standalone Go CLI.

## Audit progress note

With Batch 20 complete, we've finished:
- All tools (Batches 3-9)
- All services (Batches 10-13)
- All utils (Batches 14-18)
- All runtime systems (Batches 19-20)

Remaining batches (21-34) are:
- TUI components (21-25) — React/Ink vs Bubbletea
- Commands/slash commands (26-32) — interactive REPL commands
- Data/extras (33-34) — plugins, skills, migrations, etc.

The TUI batches (21-25) will be architectural comparisons since TS uses React/Ink and Go uses Bubbletea — different paradigms. The command batches (26-32) may have concrete behavioral differences worth checking.
