package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageConstants(t *testing.T) {
	// Source: utils/messages.ts:207-247 — verbatim string parity
	assert.Equal(t, "[Request interrupted by user]", InterruptMessage)
	assert.Equal(t, "[Request interrupted by user for tool use]", InterruptMessageForToolUse)
	assert.Contains(t, CancelMessage, "doesn't want to take this action")
	assert.Contains(t, RejectMessage, "doesn't want to proceed with this tool use")
	assert.Contains(t, RejectMessageWithReasonPrefix, "the user said:")
	assert.Contains(t, SubagentRejectMessage, "Permission for this tool use was denied")
	assert.Contains(t, PlanRejectionPrefix, "plan that was rejected")
	assert.Contains(t, DenialWorkaroundGuidance, "IMPORTANT: You *may*")
	assert.Equal(t, "No response requested.", NoResponseRequested)
	assert.Equal(t, "<synthetic>", SyntheticModel)
}

func TestSyntheticMessages(t *testing.T) {
	assert.True(t, SyntheticMessages[InterruptMessage])
	assert.True(t, SyntheticMessages[InterruptMessageForToolUse])
	assert.True(t, SyntheticMessages[CancelMessage])
	assert.True(t, SyntheticMessages[RejectMessage])
	assert.True(t, SyntheticMessages[NoResponseRequested])
	assert.False(t, SyntheticMessages["hello"])
}

func TestAutoRejectMessage(t *testing.T) {
	msg := AutoRejectMessage("bash")
	assert.Contains(t, msg, "Permission to use bash has been denied")
	assert.Contains(t, msg, DenialWorkaroundGuidance)
}

func TestDontAskRejectMessage(t *testing.T) {
	msg := DontAskRejectMessage("bash")
	assert.Contains(t, msg, "don't ask mode")
	assert.Contains(t, msg, DenialWorkaroundGuidance)
}

func TestIsClassifierDenial(t *testing.T) {
	assert.True(t, IsClassifierDenial(AutoModeRejectionPrefix+"too dangerous"))
	assert.False(t, IsClassifierDenial("some other message"))
	assert.False(t, IsClassifierDenial(""))
}

func TestBuildClassifierUnavailableMessage(t *testing.T) {
	msg := BuildClassifierUnavailableMessage("bash", "claude-3")
	assert.Contains(t, msg, "claude-3 is temporarily unavailable")
	assert.Contains(t, msg, "bash")
	assert.Contains(t, msg, "read-only operations")
}
