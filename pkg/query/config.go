package query

import (
	"os"
	"strings"
)

// QueryConfig is an immutable snapshot of runtime gates and session identity,
// captured once at Query() entry. Separating these from the per-iteration
// SessionState and mutable ToolUseContext makes future step() extraction
// tractable -- a pure reducer can take (state, event, config) where config is
// plain data.
//
// Intentionally excludes feature() bundle gates -- those are tree-shaking
// boundaries in the TS source and must stay inline at guarded blocks. In Go
// this distinction is less critical, but we preserve the separation for parity.
//
// Source: src/query/config.ts -- QueryConfig type
type QueryConfig struct {
	// SessionID is the unique identifier for this session, snapshotted once.
	SessionID string

	// Gates holds runtime gates (env / statsig). NOT feature() gates.
	Gates QueryGates
}

// QueryGates holds the runtime gate values snapshotted at query entry.
// Source: src/query/config.ts -- gates object
type QueryGates struct {
	// StreamingToolExecution mirrors the statsig gate
	// "tengu_streaming_tool_execution2". In Go we default to the env override
	// CLAUDE_CODE_STREAMING_TOOL_EXECUTION; when absent, defaults to true
	// (the gate is broadly rolled out).
	// T44: streamingToolExecution gate
	StreamingToolExecution bool

	// EmitToolUseSummaries is controlled by the
	// CLAUDE_CODE_EMIT_TOOL_USE_SUMMARIES environment variable.
	// T45: env flag
	EmitToolUseSummaries bool

	// IsAnt is true when USER_TYPE == "ant" (internal Anthropic users).
	IsAnt bool

	// FastModeEnabled is true unless CLAUDE_CODE_DISABLE_FAST_MODE is truthy.
	FastModeEnabled bool
}

// BuildQueryConfig creates an immutable QueryConfig snapshot. Call once at
// Query() entry -- the returned value should be threaded through all loop
// iterations without mutation.
//
// Source: src/query/config.ts -- buildQueryConfig()
func BuildQueryConfig(sessionID string) QueryConfig {
	return QueryConfig{
		SessionID: sessionID,
		Gates: QueryGates{
			StreamingToolExecution: streamingToolExecutionGate(),
			EmitToolUseSummaries:   isEnvTruthy(os.Getenv("CLAUDE_CODE_EMIT_TOOL_USE_SUMMARIES")),
			IsAnt:                  os.Getenv("USER_TYPE") == "ant",
			FastModeEnabled:        !isEnvTruthy(os.Getenv("CLAUDE_CODE_DISABLE_FAST_MODE")),
		},
	}
}

// streamingToolExecutionGate returns the value of the streaming tool execution
// gate. In the TS source this is a statsig gate
// ("tengu_streaming_tool_execution2") checked with CACHED_MAY_BE_STALE. In Go
// we use an env-var override (CLAUDE_CODE_STREAMING_TOOL_EXECUTION) and default
// to true since the gate is broadly enabled.
//
// T44: streamingToolExecution gate
func streamingToolExecutionGate() bool {
	v := os.Getenv("CLAUDE_CODE_STREAMING_TOOL_EXECUTION")
	if v == "" {
		return true // default: enabled (gate is broadly rolled out)
	}
	return isEnvTruthy(v)
}

// isEnvTruthy mirrors the TS isEnvTruthy helper -- returns true for "1",
// "true", "yes" (case-insensitive).
func isEnvTruthy(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "1" || v == "true" || v == "yes"
}
