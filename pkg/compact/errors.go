package compact

// Source: services/compact/compact.ts:225-297

// Error message constants — verbatim from TS source.
const (
	// ErrorMessageNotEnoughMessages is displayed when too few messages to compact.
	// Source: compact.ts:225-226
	ErrorMessageNotEnoughMessages = "Not enough messages to compact."

	// ErrorMessagePromptTooLong is displayed when the compact request itself hits PTL.
	// Source: compact.ts:293-294
	ErrorMessagePromptTooLong = "Conversation too long. Press esc twice to go up a few messages and try again."

	// ErrorMessageUserAbort is the error text when the user aborts a compact request.
	// Source: compact.ts:295
	ErrorMessageUserAbort = "API Error: Request was aborted."

	// ErrorMessageIncompleteResponse is shown when compaction is interrupted (e.g. network).
	// Source: compact.ts:296-297
	ErrorMessageIncompleteResponse = "Compaction interrupted \u00b7 This may be due to network issues \u2014 please try again."
)

// MaxPTLRetries is the maximum number of prompt-too-long retries during compaction.
// Source: compact.ts:227
const MaxPTLRetries = 3

// PTLRetryMarker is prepended when the oldest groups are dropped and the first
// remaining message would be assistant-role (API requires user-first).
// Source: compact.ts:228
const PTLRetryMarker = "[earlier conversation truncated for compaction retry]"

// MaxCompactStreamingRetries is the retry limit for streaming failures.
// Source: compact.ts:131
const MaxCompactStreamingRetries = 2
