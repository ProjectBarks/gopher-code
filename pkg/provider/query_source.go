package provider

// QuerySource identifies the origin of an API query for analytics and retry
// policy decisions. Mirrors the TS QuerySource type used throughout
// services/api/claude.ts and query.ts.
// Source: constants/querySource (inferred from usage in claude.ts:1066-1070)
type QuerySource string

const (
	// QuerySourceREPLMainThread is the primary interactive conversation.
	QuerySourceREPLMainThread QuerySource = "repl_main_thread"

	// QuerySourceSDK is used for SDK-driven queries (headless mode).
	QuerySourceSDK QuerySource = "sdk"

	// QuerySourceHookAgent is used for hook-triggered agent queries.
	QuerySourceHookAgent QuerySource = "hook_agent"

	// QuerySourceVerificationAgent is used by the verification agent.
	QuerySourceVerificationAgent QuerySource = "verification_agent"

	// QuerySourceCompact is used during context compaction.
	QuerySourceCompact QuerySource = "compact"

	// QuerySourceMicroCompact is used during micro-compaction.
	QuerySourceMicroCompact QuerySource = "micro_compact"

	// QuerySourceTitle is used for conversation title generation.
	QuerySourceTitle QuerySource = "title"

	// QuerySourceMemory is used for memory relevance queries.
	QuerySourceMemory QuerySource = "memory"
)

// IsAgenticQuerySource returns true if the query source represents an agentic
// (foreground, user-facing) query that should receive full retry treatment.
// Background/auxiliary queries bail immediately on 529 to avoid retry
// amplification during capacity cascades.
// Source: claude.ts:1066-1070
func IsAgenticQuerySource(qs QuerySource) bool {
	switch qs {
	case QuerySourceREPLMainThread, QuerySourceSDK,
		QuerySourceHookAgent, QuerySourceVerificationAgent:
		return true
	}
	// Agent sub-queries start with "agent:"
	if len(qs) > 6 && qs[:6] == "agent:" {
		return true
	}
	return false
}

// ShouldRetry529 determines whether a 529 (overloaded) error should be
// retried for the given query source. Non-foreground sources bail immediately
// to avoid retry amplification during capacity cascades.
// Source: withRetry.ts:316-324
func ShouldRetry529(qs QuerySource) bool {
	return IsAgenticQuerySource(qs)
}
