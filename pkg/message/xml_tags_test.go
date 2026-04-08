package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestXMLTagConstants(t *testing.T) {
	// Command tags
	assert.Equal(t, "command-name", CommandNameTag)
	assert.Equal(t, "command-message", CommandMessageTag)
	assert.Equal(t, "command-args", CommandArgsTag)

	// Bash/terminal tags
	assert.Equal(t, "bash-input", BashInputTag)
	assert.Equal(t, "bash-stdout", BashStdoutTag)
	assert.Equal(t, "bash-stderr", BashStderrTag)
	assert.Equal(t, "local-command-stdout", LocalCommandStdoutTag)
	assert.Equal(t, "local-command-stderr", LocalCommandStderrTag)
	assert.Equal(t, "local-command-caveat", LocalCommandCaveatTag)

	// Tick
	assert.Equal(t, "tick", TickTag)

	// Task notification tags
	assert.Equal(t, "task-notification", TaskNotificationTag)
	assert.Equal(t, "task-id", TaskIDTag)
	assert.Equal(t, "tool-use-id", ToolUseIDTag)
	assert.Equal(t, "task-type", TaskTypeTag)
	assert.Equal(t, "output-file", OutputFileTag)
	assert.Equal(t, "status", StatusTag)
	assert.Equal(t, "summary", SummaryTag)
	assert.Equal(t, "reason", ReasonTag)
	assert.Equal(t, "worktree", WorktreeTag)
	assert.Equal(t, "worktreePath", WorktreePathTag)
	assert.Equal(t, "worktreeBranch", WorktreeBranchTag)

	// Ultraplan
	assert.Equal(t, "ultraplan", UltraplanTag)

	// Review
	assert.Equal(t, "remote-review", RemoteReviewTag)
	assert.Equal(t, "remote-review-progress", RemoteReviewProgressTag)

	// Teammate / channel / cross-session
	assert.Equal(t, "teammate-message", TeammateMessageTag)
	assert.Equal(t, "channel-message", ChannelMessageTag)
	assert.Equal(t, "channel", ChannelTag)
	assert.Equal(t, "cross-session-message", CrossSessionMessageTag)

	// Fork
	assert.Equal(t, "fork-boilerplate", ForkBoilerplateTag)
	assert.Equal(t, "Your directive: ", ForkDirectivePrefix)
}

func TestTerminalOutputTags(t *testing.T) {
	expected := []string{
		"bash-input",
		"bash-stdout",
		"bash-stderr",
		"local-command-stdout",
		"local-command-stderr",
		"local-command-caveat",
	}
	assert.Equal(t, expected, TerminalOutputTags)
}

func TestCommonHelpArgs(t *testing.T) {
	expected := []string{"help", "-h", "--help"}
	assert.Equal(t, expected, CommonHelpArgs)
}

func TestCommonInfoArgs(t *testing.T) {
	expected := []string{
		"list", "show", "display", "current", "view",
		"get", "check", "describe", "print", "version",
		"about", "status", "?",
	}
	assert.Equal(t, expected, CommonInfoArgs)
}

func TestWrapTag(t *testing.T) {
	// WrapTag produces valid XML that ExtractTag can round-trip
	wrapped := WrapTag(BashStdoutTag, "hello world")
	assert.Equal(t, "<bash-stdout>hello world</bash-stdout>", wrapped)

	// ExtractTag can extract the content back
	extracted := ExtractTag(wrapped, BashStdoutTag)
	assert.Equal(t, "hello world", extracted)

	// Works with all tag families
	for _, tag := range []string{
		CommandNameTag, CommandMessageTag, CommandArgsTag,
		TaskNotificationTag, TaskIDTag, StatusTag, SummaryTag,
		UltraplanTag, RemoteReviewTag, TeammateMessageTag,
		ChannelMessageTag, CrossSessionMessageTag, ForkBoilerplateTag,
	} {
		content := "test-content-for-" + tag
		got := ExtractTag(WrapTag(tag, content), tag)
		assert.Equal(t, content, got, "round-trip failed for tag %q", tag)
	}
}

func TestIsTerminalOutputTag(t *testing.T) {
	// All tags in TerminalOutputTags should return true
	for _, tag := range TerminalOutputTags {
		assert.True(t, IsTerminalOutputTag(tag), "expected %q to be a terminal output tag", tag)
	}
	// Non-terminal tags should return false
	assert.False(t, IsTerminalOutputTag(CommandNameTag))
	assert.False(t, IsTerminalOutputTag(TaskNotificationTag))
	assert.False(t, IsTerminalOutputTag("unknown-tag"))
}

func TestWrapTag_TaskNotification_Integration(t *testing.T) {
	// Build a task notification the way the coordinator does
	inner := WrapTag(TaskIDTag, "agent-abc") + "\n" +
		WrapTag(StatusTag, "completed") + "\n" +
		WrapTag(SummaryTag, "task finished successfully")
	notification := WrapTag(TaskNotificationTag, "\n"+inner+"\n")

	// Verify we can extract each nested tag
	assert.Equal(t, "agent-abc", ExtractTag(notification, TaskIDTag))
	assert.Equal(t, "completed", ExtractTag(notification, StatusTag))
	assert.Equal(t, "task finished successfully", ExtractTag(notification, SummaryTag))
}
