# Batch 15 Notes — Shell & Model Utils

## What was done

All 6 directories reviewed. Go implementations are comprehensive — no code changes needed.

### utils/model/ → pkg/provider/model.go
One of the most complete Go subsystems:
- **11 model configs** (Haiku 3.5/4.5, Sonnet 3.5/3.7/4/4.5/4.6, Opus 4/4.1/4.5/4.6) each with 4-provider variants (FirstParty, Bedrock, Vertex, Foundry)
- **Provider detection** via env vars (CLAUDE_CODE_USE_BEDROCK/VERTEX/FOUNDRY)
- **Model aliases**: sonnet, opus, haiku, best, opusplan, [1m] suffix support
- **Canonical name resolution**: FirstPartyNameToCanonical strips dates/versions
- **Legacy Opus remap**: claude-opus-4/4-1 → opus-4-6 for firstParty
- **Display names**: GetPublicModelDisplayName, GetMarketingNameForModel
- **Env overrides**: ANTHROPIC_DEFAULT_SONNET/OPUS/HAIKU_MODEL

### utils/shell/ + utils/bash/ → pkg/tools/shellparse.go
- **AST-based shell parsing** via mvdan.cc/sh library
- **70+ read-only commands** in readOnlyCommands map (ls, cat, grep, find, etc.)
- **Git read-only subcommands**: status, log, diff, show, branch (without -d), etc.
- **IsReadOnlyCommand**: Parses shell AST, checks all simple commands against allowlist
- TS has finer per-flag analysis (FlagArgType system), but Go's command-level checking covers the core concurrency-safety use case

### utils/sandbox/ → pkg/tools/sandbox.go
- macOS seatbelt (sandbox-exec) support
- Linux bwrap (bubblewrap) support
- Sandbox profile generation for file/network restrictions

### utils/messages/ → pkg/message/message.go
- **NormalizeForAPI**: Smooshes consecutive same-role messages, deduplicates, filters whitespace-only assistant content, ensures tool_use/tool_result pairing
- **NormalizeAttachment**: Wraps attachment content in system-reminder tags with 10K truncation

### utils/powershell/ — platform-specific
TS has parser.ts, staticPrefix.ts, dangerousCmdlets.ts for PowerShell command analysis. Go has basic PowerShell tool execution. Platform-specific.

## What's NOT done (deferred)

### Shell snapshot system
TS has `ShellSnapshot.ts` that captures the user's shell environment (PATH, aliases, functions) for reproducibility. Go directly calls the shell binary without snapshotting.

### Per-flag safety analysis
TS `readOnlyCommandValidation.ts` has 1500+ lines of per-flag safety analysis (which flags are safe for git, docker, etc.). Go uses command-level checking which is conservative but less granular — it may ask for permission on safe git commands that TS would auto-approve.

### Shell completion
TS has `shellCompletion.ts` for tab completion in the TUI. Go handles this at the CLI level (bash/zsh/fish completions via separate command).

## Patterns noticed

1. **Model system is one of the best-ported subsystems**: 11 model configs, 4 providers, alias resolution, canonical names, legacy remapping — all faithfully ported from TS.

2. **Shell parsing trade-off**: Go uses mvdan.cc/sh for real shell AST parsing (stronger than regex) but TS has per-flag analysis (more granular). Go's approach is safer (fewer false positives for concurrency-safe) but may be more conservative.

3. **Message normalization is important for API compatibility**: NormalizeForAPI ensures messages meet Anthropic API constraints (alternating roles, non-empty content, tool_use/tool_result pairing). Go's implementation matches TS behavior.
