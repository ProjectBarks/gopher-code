# Batch 12 Notes — Auxiliary Services

## What was done

All 6 services reviewed. None require Go code changes — they're all higher-level features depending on infrastructure not present in Go.

### AgentSummary
- **Purpose**: Periodic background summarization for coordinator mode sub-agents
- **Trigger**: Runs forked agent every ~30s (SUMMARY_INTERVAL_MS) to generate 3-5 word progress description
- **Why deferred**: Needs forked agent infrastructure and coordinator mode (Batch 5 deferred)

### lsp (services/lsp/)
- **Purpose**: LSP server lifecycle management — discovers, starts, and manages LSP servers loaded from plugins
- **Files**: LSPClient, LSPServerInstance, LSPServerManager, config, passiveFeedback
- **Why deferred**: Go LSPTool (Batch 4) uses shell-based heuristics (go vet, tsc, regex symbols) instead of real LSP protocol. Full LSP needs plugin infrastructure.

### MagicDocs
- **Purpose**: Automatically maintains markdown files with "# MAGIC DOC: [title]" headers. Runs forked agent to update docs with conversation learnings.
- **Trigger**: Post-sampling hook when tracked magic docs exist
- **Why deferred**: Needs forked agent, file read listeners, and post-sampling hooks

### plugins
- **Purpose**: Full plugin ecosystem — install/uninstall/enable/disable/update plugins from marketplace
- **Files**: PluginInstallationManager, pluginCliCommands, pluginOperations
- **Why deferred**: Entire plugin system (manifests, repositories, dependency resolution, marketplace)

### policyLimits
- **Purpose**: Fetches organization-level policy restrictions from API. Enterprise feature for Team/C4E subscribers.
- **Constants**: FETCH_TIMEOUT_MS=10s, MAX_RETRIES=5, POLLING_INTERVAL=1hr
- **Why deferred**: Enterprise API feature. Fails open when not available.

### PromptSuggestion
- **Purpose**: Generates follow-up prompt suggestions after assistant turns using forked agent
- **Also includes**: Speculation engine for speculative execution
- **Why deferred**: UI feature. Needs forked agent and AppState integration.

## Patterns noticed

1. **Forked agent pattern**: AgentSummary, MagicDocs, PromptSuggestion, and extractMemories (Batch 11) all use `runForkedAgent()` for background work. When Go implements agent forking, these features would all benefit.

2. **Post-sampling hooks**: Many background services register via `registerPostSamplingHook()` — they run after each model turn completes. Go's query loop would need a hook registration system for these.

3. **Enterprise features**: policyLimits and plugins are enterprise/managed features. They're gated behind API access and subscription checks. Go can operate without them.
