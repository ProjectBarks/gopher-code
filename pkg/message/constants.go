package message

import "fmt"

// Message constants — verbatim strings from src/utils/messages.ts:207-247.

const (
	// InterruptMessage is sent when the user interrupts a request.
	InterruptMessage = "[Request interrupted by user]"

	// InterruptMessageForToolUse is sent when the user interrupts during tool use.
	InterruptMessageForToolUse = "[Request interrupted by user for tool use]"

	// CancelMessage is sent when the user cancels an action.
	CancelMessage = "The user doesn't want to take this action right now. STOP what you are doing and wait for the user to tell you how to proceed."

	// RejectMessage is sent when the user rejects a tool use.
	RejectMessage = "The user doesn't want to proceed with this tool use. The tool use was rejected (eg. if it was a file edit, the new_string was NOT written to the file). STOP what you are doing and wait for the user to tell you how to proceed."

	// RejectMessageWithReasonPrefix is the prefix for rejections with a reason.
	RejectMessageWithReasonPrefix = "The user doesn't want to proceed with this tool use. The tool use was rejected (eg. if it was a file edit, the new_string was NOT written to the file). To tell you how to proceed, the user said:\n"

	// SubagentRejectMessage is sent when a subagent tool use is denied.
	SubagentRejectMessage = "Permission for this tool use was denied. The tool use was rejected (eg. if it was a file edit, the new_string was NOT written to the file). Try a different approach or report the limitation to complete your task."

	// SubagentRejectMessageWithReasonPrefix is the prefix for subagent rejections with a reason.
	SubagentRejectMessageWithReasonPrefix = "Permission for this tool use was denied. The tool use was rejected (eg. if it was a file edit, the new_string was NOT written to the file). The user said:\n"

	// PlanRejectionPrefix is the prefix for plan rejections.
	PlanRejectionPrefix = "The agent proposed a plan that was rejected by the user. The user chose to stay in plan mode rather than proceed with implementation.\n\nRejected plan:\n"

	// DenialWorkaroundGuidance is shared guidance for permission denials.
	DenialWorkaroundGuidance = `IMPORTANT: You *may* attempt to accomplish this action using other tools that might naturally be used to accomplish this goal, ` +
		`e.g. using head instead of cat. But you *should not* attempt to work around this denial in malicious ways, ` +
		`e.g. do not use your ability to run tests to execute non-test actions. ` +
		`You should only try to work around this restriction in reasonable ways that do not attempt to bypass the intent behind this denial. ` +
		`If you believe this capability is essential to complete the user's request, STOP and explain to the user ` +
		`what you were trying to do and why you need this permission. Let the user decide how to proceed.`

	// NoResponseRequested is the text for messages that don't need a response.
	NoResponseRequested = "No response requested."

	// SyntheticModel is the model name for synthetic messages.
	SyntheticModel = "<synthetic>"

	// AutoModeRejectionPrefix is the prefix used by UI to detect classifier denials.
	AutoModeRejectionPrefix = "Permission for this action has been denied. Reason: "
)

// SyntheticMessages is the set of message texts that are synthetic (not from the model).
// Source: utils/messages.ts:302-308
var SyntheticMessages = map[string]bool{
	InterruptMessage:        true,
	InterruptMessageForToolUse: true,
	CancelMessage:           true,
	RejectMessage:           true,
	NoResponseRequested:     true,
}

// AutoRejectMessage builds a rejection message for auto-mode denials.
// Source: utils/messages.ts:234
func AutoRejectMessage(toolName string) string {
	return fmt.Sprintf("Permission to use %s has been denied. %s", toolName, DenialWorkaroundGuidance)
}

// DontAskRejectMessage builds a rejection message for don't-ask mode denials.
// Source: utils/messages.ts:237
func DontAskRejectMessage(toolName string) string {
	return fmt.Sprintf("Permission to use %s has been denied because Claude Code is running in don't ask mode. %s", toolName, DenialWorkaroundGuidance)
}

// IsClassifierDenial checks if a tool result message is a classifier denial.
// Source: utils/messages.ts:257
func IsClassifierDenial(content string) bool {
	return len(content) >= len(AutoModeRejectionPrefix) &&
		content[:len(AutoModeRejectionPrefix)] == AutoModeRejectionPrefix
}

// BuildClassifierUnavailableMessage builds a message for when the classifier is unavailable.
// Source: utils/messages.ts:288
func BuildClassifierUnavailableMessage(toolName, classifierModel string) string {
	return fmt.Sprintf(
		"%s is temporarily unavailable, so auto mode cannot determine the safety of %s right now. "+
			"Wait briefly and then try this action again. "+
			"If it keeps failing, continue with other tasks that don't require this action and come back to it later. "+
			"Note: reading files, searching code, and other read-only operations do not require the classifier and can still be used.",
		classifierModel, toolName,
	)
}
