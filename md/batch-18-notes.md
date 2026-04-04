# Batch 18 Notes — Platform & Remote Utils

## What was done

All 8 directories reviewed. All are platform-specific — no Go code changes needed.

### utils/background/ — CCR remote sessions
Remote session management for Claude Code Remote. Checks preconditions (OAuth, subscription, policy) and manages remote CCR sessions.

### utils/claudeInChrome/ — Chrome extension integration
Chrome native messaging host for browser extension integration. MCP server for Chrome tab control. Not applicable to CLI.

### utils/computerUse/ — Computer Use automation (13 files)
Computer Use MCP server providing screenshot, click, type, drag, scroll automation. Uses platform-specific screenshot tools. This is the Chrome DevTools / Computer Use integration. Not applicable to CLI.

### utils/deepLink/ — Protocol handler
Registers and handles `claude://` protocol deep links. Opens the CLI from browser links. Desktop/Electron feature.

### utils/dxt/ — Desktop extensions
Desktop extension format handling (zip packaging). For the desktop app extension system.

### utils/nativeInstaller/ — Binary installer
Native binary installer/updater. Downloads and installs Claude Code binaries. Go is distributed as a standalone binary — no installer needed.

### utils/teleport/ — CCR environments
Manages CCR (Claude Code Remote) environments. API client for remote session creation and management. Enterprise/remote feature.

### utils/ultraplan/ — Advanced planning
Advanced planning mode with CCR sessions. Keyword detection for triggering ultraplan. Enterprise feature.

## Summary

This entire batch is platform-specific infrastructure that doesn't apply to a Go CLI tool:
- Chrome/browser integration (claudeInChrome, computerUse)
- Desktop app features (deepLink, dxt, nativeInstaller)
- CCR/remote infrastructure (background, teleport, ultraplan)

Go operates as a standalone terminal CLI — none of these desktop/browser/remote features are needed.
