package keybindings

import "runtime"

// Keystroke is a single key or chord string (e.g. "ctrl+c", "ctrl+x ctrl+k").
type Keystroke = string

// Binding maps keystroke strings to actions within a context.
type Binding = map[Keystroke]Action

// BindingMap is the full set of keybindings across all contexts.
type BindingMap = map[Context]Binding

// Short aliases for context constants, used in DefaultBindings and tests.
const (
	Global          = ContextGlobal
	Chat            = ContextChat
	Autocomplete    = ContextAutocomplete
	Settings        = ContextSettings
	Confirmation    = ContextConfirmation
	Tabs            = ContextTabs
	Transcript      = ContextTranscript
	HistorySearch   = ContextHistorySearch
	Task            = ContextTask
	ThemePicker     = ContextThemePicker
	Scroll          = ContextScroll
	Help            = ContextHelp
	Attachments     = ContextAttachments
	Footer          = ContextFooter
	MessageSelector = ContextMessageSelector
	MessageActions  = ContextMessageActions
	DiffDialog      = ContextDiffDialog
	ModelPicker     = ContextModelPicker
	Select          = ContextSelect
	Plugin          = ContextPlugin
)

// imagePasteKey returns the platform-specific image paste shortcut.
func imagePasteKey() string {
	if runtime.GOOS == "windows" {
		return "alt+v"
	}
	return "ctrl+v"
}

// modeCycleKey returns the platform-specific mode cycle shortcut.
func modeCycleKey() string {
	if runtime.GOOS == "windows" {
		return "meta+m"
	}
	return "shift+tab"
}

// DefaultBindings returns the default keybinding blocks for all 20 contexts.
func DefaultBindings() []KeybindingBlock {
	return []KeybindingBlock{
		{Context: ContextGlobal, Bindings: map[string]string{
			"ctrl+c":        string(ActionAppInterrupt),
			"ctrl+d":        string(ActionAppExit),
			"ctrl+l":        string(ActionAppRedraw),
			"ctrl+t":        string(ActionAppToggleTodos),
			"ctrl+o":        string(ActionAppToggleTranscript),
			"ctrl+shift+b":  string(ActionAppToggleBrief),
			"ctrl+shift+o":  string(ActionAppToggleTeammatePreview),
			"ctrl+r":        string(ActionHistorySearch),
			"ctrl+shift+f":  string(ActionAppGlobalSearch),
			"cmd+shift+f":   string(ActionAppGlobalSearch),
			"ctrl+shift+p":  string(ActionAppQuickOpen),
			"cmd+shift+p":   string(ActionAppQuickOpen),
			"meta+j":        string(ActionAppToggleTerminal),
		}},
		{Context: ContextChat, Bindings: map[string]string{
			"escape":         string(ActionChatCancel),
			"ctrl+x ctrl+k": string(ActionChatKillAgents),
			modeCycleKey():   string(ActionChatCycleMode),
			"meta+p":         string(ActionChatModelPicker),
			"meta+o":         string(ActionChatFastMode),
			"meta+t":         string(ActionChatThinkingToggle),
			"enter":          string(ActionChatSubmit),
			"up":             string(ActionHistoryPrevious),
			"down":           string(ActionHistoryNext),
			"ctrl+_":         string(ActionChatUndo),
			"ctrl+shift+-":   string(ActionChatUndo),
			"ctrl+x ctrl+e": string(ActionChatExternalEditor),
			"ctrl+g":         string(ActionChatExternalEditor),
			"ctrl+s":         string(ActionChatStash),
			imagePasteKey():  string(ActionChatImagePaste),
			"shift+up":       string(ActionChatMessageActions),
			"space":          string(ActionVoicePushToTalk),
		}},
		{Context: ContextAutocomplete, Bindings: map[string]string{
			"tab":    string(ActionAutocompleteAccept),
			"escape": string(ActionAutocompleteDismiss),
			"up":     string(ActionAutocompletePrevious),
			"down":   string(ActionAutocompleteNext),
		}},
		{Context: ContextSettings, Bindings: map[string]string{
			"escape": string(ActionConfirmNo),
			"up":     string(ActionSelectPrevious),
			"down":   string(ActionSelectNext),
			"k":      string(ActionSelectPrevious),
			"j":      string(ActionSelectNext),
			"ctrl+p": string(ActionSelectPrevious),
			"ctrl+n": string(ActionSelectNext),
			"space":  string(ActionSelectAccept),
			"enter":  string(ActionSettingsClose),
			"/":      string(ActionSettingsSearch),
			"r":      string(ActionSettingsRetry),
		}},
		{Context: ContextConfirmation, Bindings: map[string]string{
			"y":         string(ActionConfirmYes),
			"n":         string(ActionConfirmNo),
			"enter":     string(ActionConfirmYes),
			"escape":    string(ActionConfirmNo),
			"up":        string(ActionConfirmPrevious),
			"down":      string(ActionConfirmNext),
			"tab":       string(ActionConfirmNextField),
			"space":     string(ActionConfirmToggle),
			"shift+tab": string(ActionConfirmCycleMode),
			"ctrl+e":    string(ActionConfirmToggleExplanation),
			"ctrl+d":    string(ActionPermissionToggleDebug),
		}},
		{Context: ContextTabs, Bindings: map[string]string{
			"tab":       string(ActionTabsNext),
			"shift+tab": string(ActionTabsPrevious),
			"right":     string(ActionTabsNext),
			"left":      string(ActionTabsPrevious),
		}},
		{Context: ContextTranscript, Bindings: map[string]string{
			"ctrl+e": string(ActionTranscriptToggleShowAll),
			"ctrl+c": string(ActionTranscriptExit),
			"escape": string(ActionTranscriptExit),
			"q":      string(ActionTranscriptExit),
		}},
		{Context: ContextHistorySearch, Bindings: map[string]string{
			"ctrl+r": string(ActionHistorySearchNext),
			"escape": string(ActionHistorySearchAccept),
			"tab":    string(ActionHistorySearchAccept),
			"ctrl+c": string(ActionHistorySearchCancel),
			"enter":  string(ActionHistorySearchExecute),
		}},
		{Context: ContextTask, Bindings: map[string]string{
			"ctrl+b": string(ActionTaskBackground),
		}},
		{Context: ContextThemePicker, Bindings: map[string]string{
			"ctrl+t": string(ActionThemeToggleSyntaxHighlighting),
		}},
		{Context: ContextScroll, Bindings: map[string]string{
			"pageup":       string(ActionScrollPageUp),
			"pagedown":     string(ActionScrollPageDown),
			"wheelup":      string(ActionScrollLineUp),
			"wheeldown":    string(ActionScrollLineDown),
			"ctrl+home":    string(ActionScrollTop),
			"ctrl+end":     string(ActionScrollBottom),
			"ctrl+shift+c": string(ActionSelectionCopy),
			"cmd+c":        string(ActionSelectionCopy),
		}},
		{Context: ContextHelp, Bindings: map[string]string{
			"escape": string(ActionHelpDismiss),
		}},
		{Context: ContextAttachments, Bindings: map[string]string{
			"right":     string(ActionAttachmentsNext),
			"left":      string(ActionAttachmentsPrevious),
			"backspace": string(ActionAttachmentsRemove),
			"delete":    string(ActionAttachmentsRemove),
			"down":      string(ActionAttachmentsExit),
			"escape":    string(ActionAttachmentsExit),
		}},
		{Context: ContextFooter, Bindings: map[string]string{
			"up":     string(ActionFooterUp),
			"ctrl+p": string(ActionFooterUp),
			"down":   string(ActionFooterDown),
			"ctrl+n": string(ActionFooterDown),
			"right":  string(ActionFooterNext),
			"left":   string(ActionFooterPrevious),
			"enter":  string(ActionFooterOpenSelected),
			"escape": string(ActionFooterClearSelection),
		}},
		{Context: ContextMessageSelector, Bindings: map[string]string{
			"up":         string(ActionMessageSelectorUp),
			"down":       string(ActionMessageSelectorDown),
			"k":          string(ActionMessageSelectorUp),
			"j":          string(ActionMessageSelectorDown),
			"ctrl+p":     string(ActionMessageSelectorUp),
			"ctrl+n":     string(ActionMessageSelectorDown),
			"ctrl+up":    string(ActionMessageSelectorTop),
			"shift+up":   string(ActionMessageSelectorTop),
			"meta+up":    string(ActionMessageSelectorTop),
			"shift+k":    string(ActionMessageSelectorTop),
			"ctrl+down":  string(ActionMessageSelectorBottom),
			"shift+down": string(ActionMessageSelectorBottom),
			"meta+down":  string(ActionMessageSelectorBottom),
			"shift+j":    string(ActionMessageSelectorBottom),
			"enter":      string(ActionMessageSelectorSelect),
		}},
		{Context: ContextMessageActions, Bindings: map[string]string{
			"up":         string(ActionMessageActionsPrev),
			"down":       string(ActionMessageActionsNext),
			"k":          string(ActionMessageActionsPrev),
			"j":          string(ActionMessageActionsNext),
			"meta+up":    string(ActionMessageActionsTop),
			"meta+down":  string(ActionMessageActionsBottom),
			"super+up":   string(ActionMessageActionsTop),
			"super+down": string(ActionMessageActionsBottom),
			"shift+up":   string(ActionMessageActionsPrevUser),
			"shift+down": string(ActionMessageActionsNextUser),
			"escape":     string(ActionMessageActionsEscape),
			"ctrl+c":     string(ActionMessageActionsCtrlc),
			"enter":      string(ActionMessageActionsEnter),
			"c":          string(ActionMessageActionsC),
			"p":          string(ActionMessageActionsP),
		}},
		{Context: ContextDiffDialog, Bindings: map[string]string{
			"escape": string(ActionDiffDismiss),
			"left":   string(ActionDiffPreviousSource),
			"right":  string(ActionDiffNextSource),
			"up":     string(ActionDiffPreviousFile),
			"down":   string(ActionDiffNextFile),
			"enter":  string(ActionDiffViewDetails),
		}},
		{Context: ContextModelPicker, Bindings: map[string]string{
			"left":  string(ActionModelPickerDecreaseEffort),
			"right": string(ActionModelPickerIncreaseEffort),
		}},
		{Context: ContextSelect, Bindings: map[string]string{
			"up":     string(ActionSelectPrevious),
			"down":   string(ActionSelectNext),
			"j":      string(ActionSelectNext),
			"k":      string(ActionSelectPrevious),
			"ctrl+n": string(ActionSelectNext),
			"ctrl+p": string(ActionSelectPrevious),
			"enter":  string(ActionSelectAccept),
			"escape": string(ActionSelectCancel),
		}},
		{Context: ContextPlugin, Bindings: map[string]string{
			"space": string(ActionPluginToggle),
			"i":     string(ActionPluginInstall),
		}},
	}
}

// DefaultBindingMap returns the default bindings as a BindingMap (context -> keystroke -> action).
func DefaultBindingMap() BindingMap {
	blocks := DefaultBindings()
	m := make(BindingMap, len(blocks))
	for _, block := range blocks {
		binding := make(Binding, len(block.Bindings))
		for k, v := range block.Bindings {
			binding[k] = Action(v)
		}
		m[block.Context] = binding
	}
	return m
}
