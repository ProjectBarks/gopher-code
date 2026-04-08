package message

// XML tag constants for embedding metadata in messages.
// Ported from src/constants/xml.ts.

const (
	// Command tags
	CommandNameTag    = "command-name"
	CommandMessageTag = "command-message"
	CommandArgsTag    = "command-args"

	// Bash/terminal tags
	BashInputTag          = "bash-input"
	BashStdoutTag         = "bash-stdout"
	BashStderrTag         = "bash-stderr"
	LocalCommandStdoutTag = "local-command-stdout"
	LocalCommandStderrTag = "local-command-stderr"
	LocalCommandCaveatTag = "local-command-caveat"

	TickTag = "tick"

	// Task notification tags
	TaskNotificationTag = "task-notification"
	TaskIDTag           = "task-id"
	ToolUseIDTag        = "tool-use-id"
	TaskTypeTag         = "task-type"
	OutputFileTag       = "output-file"
	StatusTag           = "status"
	SummaryTag          = "summary"
	ReasonTag           = "reason"
	WorktreeTag         = "worktree"
	WorktreePathTag     = "worktreePath"
	WorktreeBranchTag   = "worktreeBranch"

	// Ultraplan
	UltraplanTag = "ultraplan"

	// Remote review
	RemoteReviewTag         = "remote-review"
	RemoteReviewProgressTag = "remote-review-progress"

	// Teammate / channel / cross-session
	TeammateMessageTag     = "teammate-message"
	ChannelMessageTag      = "channel-message"
	ChannelTag             = "channel"
	CrossSessionMessageTag = "cross-session-message"

	// Fork
	ForkBoilerplateTag  = "fork-boilerplate"
	ForkDirectivePrefix = "Your directive: "
)

// TerminalOutputTags lists tags that indicate terminal output, not user prompts.
var TerminalOutputTags = []string{
	BashInputTag,
	BashStdoutTag,
	BashStderrTag,
	LocalCommandStdoutTag,
	LocalCommandStderrTag,
	LocalCommandCaveatTag,
}

// CommonHelpArgs are argument patterns requesting help from a slash command.
var CommonHelpArgs = []string{"help", "-h", "--help"}

// CommonInfoArgs are argument patterns requesting current state/info from a slash command.
var CommonInfoArgs = []string{
	"list", "show", "display", "current", "view",
	"get", "check", "describe", "print", "version",
	"about", "status", "?",
}

// WrapTag wraps content in the named XML tag: <tag>content</tag>.
// Source: used throughout TS codebase to embed metadata in messages.
func WrapTag(tag, content string) string {
	return "<" + tag + ">" + content + "</" + tag + ">"
}

// IsTerminalOutputTag returns true if the tag name identifies terminal output
// (bash stdout/stderr, local command output) rather than user prompts.
func IsTerminalOutputTag(tag string) bool {
	for _, t := range TerminalOutputTags {
		if t == tag {
			return true
		}
	}
	return false
}
