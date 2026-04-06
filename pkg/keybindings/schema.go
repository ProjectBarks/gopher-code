// Package keybindings defines the schema for keybindings.json configuration,
// including context enums, action enums, and context descriptions.
package keybindings

// Context represents a UI context where keybindings can be applied.
type Context string

const (
	ContextGlobal          Context = "Global"
	ContextChat            Context = "Chat"
	ContextAutocomplete    Context = "Autocomplete"
	ContextConfirmation    Context = "Confirmation"
	ContextHelp            Context = "Help"
	ContextTranscript      Context = "Transcript"
	ContextHistorySearch   Context = "HistorySearch"
	ContextTask            Context = "Task"
	ContextThemePicker     Context = "ThemePicker"
	ContextSettings        Context = "Settings"
	ContextTabs            Context = "Tabs"
	ContextAttachments     Context = "Attachments"
	ContextFooter          Context = "Footer"
	ContextMessageSelector Context = "MessageSelector"
	ContextMessageActions  Context = "MessageActions"
	ContextScroll          Context = "Scroll"
	ContextDiffDialog      Context = "DiffDialog"
	ContextModelPicker     Context = "ModelPicker"
	ContextSelect          Context = "Select"
	ContextPlugin          Context = "Plugin"
)

// AllContexts is the canonical ordered list of keybinding contexts.
var AllContexts = []Context{
	ContextGlobal,
	ContextChat,
	ContextAutocomplete,
	ContextConfirmation,
	ContextHelp,
	ContextTranscript,
	ContextHistorySearch,
	ContextTask,
	ContextThemePicker,
	ContextSettings,
	ContextTabs,
	ContextAttachments,
	ContextFooter,
	ContextMessageSelector,
	ContextMessageActions,
	ContextScroll,
	ContextDiffDialog,
	ContextModelPicker,
	ContextSelect,
	ContextPlugin,
}

// ContextDescriptions maps each context to a human-readable description.
var ContextDescriptions = map[Context]string{
	ContextGlobal:          "Active everywhere, regardless of focus",
	ContextChat:            "When the chat input is focused",
	ContextAutocomplete:    "When autocomplete menu is visible",
	ContextConfirmation:    "When a confirmation/permission dialog is shown",
	ContextHelp:            "When the help overlay is open",
	ContextTranscript:      "When viewing the transcript",
	ContextHistorySearch:   "When searching command history (ctrl+r)",
	ContextTask:            "When a task/agent is running in the foreground",
	ContextThemePicker:     "When the theme picker is open",
	ContextSettings:        "When the settings menu is open",
	ContextTabs:            "When tab navigation is active",
	ContextAttachments:     "When navigating image attachments in a select dialog",
	ContextFooter:          "When footer indicators are focused",
	ContextMessageSelector: "When the message selector (rewind) is open",
	ContextMessageActions:  "When the message actions panel is open",
	ContextScroll:          "When scrollable content is focused",
	ContextDiffDialog:      "When the diff dialog is open",
	ContextModelPicker:     "When the model picker is open",
	ContextSelect:          "When a select/list component is focused",
	ContextPlugin:          "When the plugin dialog is open",
}

// Action represents a keybinding action identifier.
type Action string

const (
	// App-level actions (Global context)
	ActionAppInterrupt              Action = "app:interrupt"
	ActionAppExit                   Action = "app:exit"
	ActionAppToggleTodos            Action = "app:toggleTodos"
	ActionAppToggleTranscript       Action = "app:toggleTranscript"
	ActionAppToggleBrief            Action = "app:toggleBrief"
	ActionAppToggleTeammatePreview  Action = "app:toggleTeammatePreview"
	ActionAppToggleTerminal         Action = "app:toggleTerminal"
	ActionAppRedraw                 Action = "app:redraw"
	ActionAppGlobalSearch           Action = "app:globalSearch"
	ActionAppQuickOpen              Action = "app:quickOpen"

	// History navigation
	ActionHistorySearch   Action = "history:search"
	ActionHistoryPrevious Action = "history:previous"
	ActionHistoryNext     Action = "history:next"

	// Chat input actions
	ActionChatCancel         Action = "chat:cancel"
	ActionChatKillAgents     Action = "chat:killAgents"
	ActionChatCycleMode      Action = "chat:cycleMode"
	ActionChatModelPicker    Action = "chat:modelPicker"
	ActionChatFastMode       Action = "chat:fastMode"
	ActionChatThinkingToggle Action = "chat:thinkingToggle"
	ActionChatSubmit         Action = "chat:submit"
	ActionChatNewline        Action = "chat:newline"
	ActionChatUndo           Action = "chat:undo"
	ActionChatExternalEditor Action = "chat:externalEditor"
	ActionChatStash          Action = "chat:stash"
	ActionChatImagePaste     Action = "chat:imagePaste"
	ActionChatMessageActions Action = "chat:messageActions"

	// Autocomplete menu actions
	ActionAutocompleteAccept   Action = "autocomplete:accept"
	ActionAutocompleteDismiss  Action = "autocomplete:dismiss"
	ActionAutocompletePrevious Action = "autocomplete:previous"
	ActionAutocompleteNext     Action = "autocomplete:next"

	// Confirmation dialog actions
	ActionConfirmYes               Action = "confirm:yes"
	ActionConfirmNo                Action = "confirm:no"
	ActionConfirmPrevious          Action = "confirm:previous"
	ActionConfirmNext              Action = "confirm:next"
	ActionConfirmNextField         Action = "confirm:nextField"
	ActionConfirmPreviousField     Action = "confirm:previousField"
	ActionConfirmCycleMode         Action = "confirm:cycleMode"
	ActionConfirmToggle            Action = "confirm:toggle"
	ActionConfirmToggleExplanation Action = "confirm:toggleExplanation"

	// Tabs navigation actions
	ActionTabsNext     Action = "tabs:next"
	ActionTabsPrevious Action = "tabs:previous"

	// Transcript viewer actions
	ActionTranscriptToggleShowAll Action = "transcript:toggleShowAll"
	ActionTranscriptExit          Action = "transcript:exit"

	// History search actions
	ActionHistorySearchNext    Action = "historySearch:next"
	ActionHistorySearchAccept  Action = "historySearch:accept"
	ActionHistorySearchCancel  Action = "historySearch:cancel"
	ActionHistorySearchExecute Action = "historySearch:execute"

	// Task/agent actions
	ActionTaskBackground Action = "task:background"

	// Theme picker actions
	ActionThemeToggleSyntaxHighlighting Action = "theme:toggleSyntaxHighlighting"

	// Help menu actions
	ActionHelpDismiss Action = "help:dismiss"

	// Attachment navigation
	ActionAttachmentsNext     Action = "attachments:next"
	ActionAttachmentsPrevious Action = "attachments:previous"
	ActionAttachmentsRemove   Action = "attachments:remove"
	ActionAttachmentsExit     Action = "attachments:exit"

	// Footer indicator actions
	ActionFooterUp             Action = "footer:up"
	ActionFooterDown           Action = "footer:down"
	ActionFooterNext           Action = "footer:next"
	ActionFooterPrevious       Action = "footer:previous"
	ActionFooterOpenSelected   Action = "footer:openSelected"
	ActionFooterClearSelection Action = "footer:clearSelection"
	ActionFooterClose          Action = "footer:close"

	// Scroll actions
	ActionScrollPageUp   Action = "scroll:pageUp"
	ActionScrollPageDown Action = "scroll:pageDown"
	ActionScrollLineUp   Action = "scroll:lineUp"
	ActionScrollLineDown Action = "scroll:lineDown"
	ActionScrollTop      Action = "scroll:top"
	ActionScrollBottom   Action = "scroll:bottom"

	// Selection actions
	ActionSelectionCopy Action = "selection:copy"

	// Message selector (rewind) actions
	ActionMessageSelectorUp     Action = "messageSelector:up"
	ActionMessageSelectorDown   Action = "messageSelector:down"
	ActionMessageSelectorTop    Action = "messageSelector:top"
	ActionMessageSelectorBottom Action = "messageSelector:bottom"
	ActionMessageSelectorSelect Action = "messageSelector:select"

	// Message actions panel
	ActionMessageActionsPrev     Action = "messageActions:prev"
	ActionMessageActionsNext     Action = "messageActions:next"
	ActionMessageActionsTop      Action = "messageActions:top"
	ActionMessageActionsBottom   Action = "messageActions:bottom"
	ActionMessageActionsPrevUser Action = "messageActions:prevUser"
	ActionMessageActionsNextUser Action = "messageActions:nextUser"
	ActionMessageActionsEscape   Action = "messageActions:escape"
	ActionMessageActionsCtrlc    Action = "messageActions:ctrlc"
	ActionMessageActionsEnter    Action = "messageActions:enter"
	ActionMessageActionsC        Action = "messageActions:c"
	ActionMessageActionsP        Action = "messageActions:p"

	// Diff dialog actions
	ActionDiffDismiss        Action = "diff:dismiss"
	ActionDiffPreviousSource Action = "diff:previousSource"
	ActionDiffNextSource     Action = "diff:nextSource"
	ActionDiffBack           Action = "diff:back"
	ActionDiffViewDetails    Action = "diff:viewDetails"
	ActionDiffPreviousFile   Action = "diff:previousFile"
	ActionDiffNextFile       Action = "diff:nextFile"

	// Model picker actions
	ActionModelPickerDecreaseEffort Action = "modelPicker:decreaseEffort"
	ActionModelPickerIncreaseEffort Action = "modelPicker:increaseEffort"

	// Select component actions
	ActionSelectNext     Action = "select:next"
	ActionSelectPrevious Action = "select:previous"
	ActionSelectAccept   Action = "select:accept"
	ActionSelectCancel   Action = "select:cancel"

	// Plugin dialog actions
	ActionPluginToggle  Action = "plugin:toggle"
	ActionPluginInstall Action = "plugin:install"

	// Permission dialog actions
	ActionPermissionToggleDebug Action = "permission:toggleDebug"

	// Settings config panel actions
	ActionSettingsSearch Action = "settings:search"
	ActionSettingsRetry  Action = "settings:retry"
	ActionSettingsClose  Action = "settings:close"

	// Voice actions
	ActionVoicePushToTalk Action = "voice:pushToTalk"
)

// AllActions is the canonical ordered list of keybinding actions.
var AllActions = []Action{
	ActionAppInterrupt,
	ActionAppExit,
	ActionAppToggleTodos,
	ActionAppToggleTranscript,
	ActionAppToggleBrief,
	ActionAppToggleTeammatePreview,
	ActionAppToggleTerminal,
	ActionAppRedraw,
	ActionAppGlobalSearch,
	ActionAppQuickOpen,
	ActionHistorySearch,
	ActionHistoryPrevious,
	ActionHistoryNext,
	ActionChatCancel,
	ActionChatKillAgents,
	ActionChatCycleMode,
	ActionChatModelPicker,
	ActionChatFastMode,
	ActionChatThinkingToggle,
	ActionChatSubmit,
	ActionChatNewline,
	ActionChatUndo,
	ActionChatExternalEditor,
	ActionChatStash,
	ActionChatImagePaste,
	ActionChatMessageActions,
	ActionAutocompleteAccept,
	ActionAutocompleteDismiss,
	ActionAutocompletePrevious,
	ActionAutocompleteNext,
	ActionConfirmYes,
	ActionConfirmNo,
	ActionConfirmPrevious,
	ActionConfirmNext,
	ActionConfirmNextField,
	ActionConfirmPreviousField,
	ActionConfirmCycleMode,
	ActionConfirmToggle,
	ActionConfirmToggleExplanation,
	ActionTabsNext,
	ActionTabsPrevious,
	ActionTranscriptToggleShowAll,
	ActionTranscriptExit,
	ActionHistorySearchNext,
	ActionHistorySearchAccept,
	ActionHistorySearchCancel,
	ActionHistorySearchExecute,
	ActionTaskBackground,
	ActionThemeToggleSyntaxHighlighting,
	ActionHelpDismiss,
	ActionAttachmentsNext,
	ActionAttachmentsPrevious,
	ActionAttachmentsRemove,
	ActionAttachmentsExit,
	ActionFooterUp,
	ActionFooterDown,
	ActionFooterNext,
	ActionFooterPrevious,
	ActionFooterOpenSelected,
	ActionFooterClearSelection,
	ActionFooterClose,
	ActionScrollPageUp,
	ActionScrollPageDown,
	ActionScrollLineUp,
	ActionScrollLineDown,
	ActionScrollTop,
	ActionScrollBottom,
	ActionSelectionCopy,
	ActionMessageSelectorUp,
	ActionMessageSelectorDown,
	ActionMessageSelectorTop,
	ActionMessageSelectorBottom,
	ActionMessageSelectorSelect,
	ActionMessageActionsPrev,
	ActionMessageActionsNext,
	ActionMessageActionsTop,
	ActionMessageActionsBottom,
	ActionMessageActionsPrevUser,
	ActionMessageActionsNextUser,
	ActionMessageActionsEscape,
	ActionMessageActionsCtrlc,
	ActionMessageActionsEnter,
	ActionMessageActionsC,
	ActionMessageActionsP,
	ActionDiffDismiss,
	ActionDiffPreviousSource,
	ActionDiffNextSource,
	ActionDiffBack,
	ActionDiffViewDetails,
	ActionDiffPreviousFile,
	ActionDiffNextFile,
	ActionModelPickerDecreaseEffort,
	ActionModelPickerIncreaseEffort,
	ActionSelectNext,
	ActionSelectPrevious,
	ActionSelectAccept,
	ActionSelectCancel,
	ActionPluginToggle,
	ActionPluginInstall,
	ActionPermissionToggleDebug,
	ActionSettingsSearch,
	ActionSettingsRetry,
	ActionSettingsClose,
	ActionVoicePushToTalk,
}

// KeybindingBlock represents a single context block in keybindings.json.
type KeybindingBlock struct {
	Context  Context           `json:"context"`
	Bindings map[string]string `json:"bindings"`
}

// KeybindingsFile represents the top-level keybindings.json structure.
type KeybindingsFile struct {
	Schema   string            `json:"$schema,omitempty"`
	Docs     string            `json:"$docs,omitempty"`
	Bindings []KeybindingBlock `json:"bindings"`
}

// ValidContext returns true if c is a known keybinding context.
func ValidContext(c Context) bool {
	_, ok := ContextDescriptions[c]
	return ok
}

// ValidAction returns true if a is a known keybinding action.
func ValidAction(a Action) bool {
	for _, v := range AllActions {
		if v == a {
			return true
		}
	}
	return false
}
