package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	pkgdoctor "github.com/projectbarks/gopher-code/pkg/doctor"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/remote"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/keybindings"
	"github.com/projectbarks/gopher-code/pkg/ui/commands"
	"github.com/projectbarks/gopher-code/pkg/ui/components"
	"github.com/projectbarks/gopher-code/pkg/ui/core"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks"
	bridgehooks "github.com/projectbarks/gopher-code/pkg/ui/hooks/bridge"
	cmdhooks "github.com/projectbarks/gopher-code/pkg/ui/hooks/commands"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/ide"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/lifecycle"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/notifications"
	swarmhooks "github.com/projectbarks/gopher-code/pkg/ui/hooks/swarm"
	"github.com/projectbarks/gopher-code/pkg/ui/screens"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ---------------------------------------------------------------------------
// T164: Scroll activity tracking — Source: bootstrap/state.ts
// Background intervals check GetIsScrollDraining() before doing work so they
// don't compete with scroll frames for the event loop. Set by scroll events,
// cleared ScrollDrainIdleMs after the last scroll event.
// ---------------------------------------------------------------------------

const ScrollDrainIdleMs = 150 * time.Millisecond

// scrollTracker tracks scroll activity for drain suspension.
// Module-scope (not in STATE) — ephemeral hot-path flag, no test-reset needed
// since the debounce timer self-clears.
type scrollTracker struct {
	mu             sync.Mutex
	draining       bool
	timer          *time.Timer
	idleNotifyCh   chan struct{} // closed when draining becomes false
}

func newScrollTracker() *scrollTracker {
	return &scrollTracker{}
}

// MarkScrollActivity marks that a scroll event just happened.
// Source: bootstrap/state.ts — markScrollActivity
func (st *scrollTracker) MarkScrollActivity() {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.draining = true

	// Close any pending idle-notify channel — waiters re-check draining.
	if st.idleNotifyCh != nil {
		select {
		case <-st.idleNotifyCh:
			// already closed
		default:
		}
	}

	if st.timer != nil {
		st.timer.Stop()
	}
	st.timer = time.AfterFunc(ScrollDrainIdleMs, func() {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.draining = false
		st.timer = nil
		// Signal any waiters
		if st.idleNotifyCh != nil {
			close(st.idleNotifyCh)
			st.idleNotifyCh = nil
		}
	})
}

// GetIsScrollDraining returns true while scroll is actively draining
// (within 150ms of last event). Intervals should early-return when this
// is set — the work picks up next tick after scroll settles.
// Source: bootstrap/state.ts — getIsScrollDraining
func (st *scrollTracker) GetIsScrollDraining() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.draining
}

// WaitForScrollIdle blocks until scroll is no longer draining. Resolves
// immediately if not scrolling; otherwise waits until the debounce clears.
// Source: bootstrap/state.ts — waitForScrollIdle
func (st *scrollTracker) WaitForScrollIdle() {
	for {
		st.mu.Lock()
		if !st.draining {
			st.mu.Unlock()
			return
		}
		// Ensure there is an idle-notify channel
		if st.idleNotifyCh == nil {
			st.idleNotifyCh = make(chan struct{})
		}
		ch := st.idleNotifyCh
		st.mu.Unlock()

		// Wait for signal
		<-ch
	}
}

// --- Message types ---

// QueryEventMsg wraps a query.QueryEvent for the Bubbletea message loop.
type QueryEventMsg struct {
	Event query.QueryEvent
}

// TextDeltaMsg carries a streaming text chunk.
type TextDeltaMsg struct {
	Text string
}

// ToolUseStartMsg signals a tool invocation has begun.
type ToolUseStartMsg struct {
	ToolUseID string
	ToolName  string
}

// ToolResultMsg carries the result of a tool execution.
type ToolResultMsg struct {
	ToolUseID string
	Content   string
	IsError   bool
	Display   any // optional structured payload (e.g. tools.DiffDisplay)
}

// TurnCompleteMsg signals the model has finished its turn.
type TurnCompleteMsg struct {
	StopReason interface{}
}

// StatusUpdateMsg updates the status line display.
type StatusUpdateMsg struct {
	Model        string
	Tokens       int
	Mode         AppMode
	Cost         float64
	InputTokens  int
	OutputTokens int
}

// queryDoneMsg signals a query goroutine has finished.
type queryDoneMsg struct {
	err error
}

// AppMode describes the current application state.
type AppMode int

const (
	ModeIdle        AppMode = iota
	ModeStreaming
	ModeToolRunning
)

// QueryFunc is the function signature for executing a query against the model.
// It's injected by RunTUIV2 so the UI can trigger queries without depending
// on the full query package directly.
type QueryFunc func(ctx context.Context, sess *session.SessionState, onEvent query.EventCallback) error

// --- AppModel ---

// AppModel is the top-level Bubbletea model that composes all UI components.
// It manages the layout, focus routing, and event dispatch.
type AppModel struct {
	// State
	session *session.SessionState
	bridge  *EventBridge
	mode    AppMode
	width   int
	height  int

	// Focus management
	focus *core.FocusManager

	// Child components
	header       *components.Header
	conversation *components.ConversationPane
	input        *components.InputPane
	slashInput   *components.SlashCommandInput
	statusLine   *components.StatusLine
	bubble       *components.MessageBubble
	streaming    *components.StreamingText

	// Streaming state — tracks the current assistant turn
	streamingText   strings.Builder
	activeToolCalls map[string]string // toolUseID → toolName
	spinner         *components.ThinkingSpinner

	// Query execution
	queryFunc   QueryFunc
	queryCtx    context.Context
	cancelQuery context.CancelFunc

	// Command dispatch
	dispatcher *commands.Dispatcher

	// T403: Command keybinding resolver + queue processor
	cmdKeybindings *cmdhooks.CommandKeybindings
	cmdQueue       *cmdhooks.Queue
	cmdProcessor   *cmdhooks.QueueProcessor

	// Welcome screen
	showWelcome bool
	welcome     *components.WelcomeScreen

	// Doctor screen (modal overlay)
	showDoctor bool
	doctorModel *screens.DoctorModel

	// Resume screen (modal overlay)
	showResume bool
	resumeModel *screens.ResumeModel

	// Permission prompt state
	// Source: interactiveHandler.ts — permission prompt overlay
	pendingPermission *permissions.PermissionRequestMsg

	// Remote permission bridge (for CCR synthetic message wrapping)
	// Source: remotePermissionBridge.ts
	remoteBridge *remote.PermissionBridge

	// Ctrl+C double-press tracking using lifecycle.DoublePress (T414).
	// Claude requires two Ctrl+C presses on empty idle input to quit.
	// First press shows "Press Ctrl-C again to exit" hint; 800ms timeout resets.
	ctrlCExit *lifecycle.DoublePress

	// T164: Scroll activity tracking
	scrollTracker *scrollTracker

	// T400: Notification hooks state
	notifs *notifState

	// T404: IDE integration hooks — connection, @-mentions, selection, logging.
	ideConn      *ide.IDEConnection
	ideSelection ide.Selection
	// T407: Swarm/task hooks — initialization, task watcher, permission poller
	// Source: useSwarmInitialization.ts, useTaskListWatcher.ts, useSwarmPermissionPoller.ts
	swarmInit    *swarmhooks.SwarmInit
	taskWatcher  *swarmhooks.TaskWatcher
	permPoller   *swarmhooks.PermissionPoller
	// T399: File/context suggestions for @-mention autocomplete.
	// Source: useInputSuggestion.tsx — file path autocomplete on @ prefix.
	fileSuggester     *hooks.FileSuggester
	fileSuggestions   []hooks.SuggestionItem
	fileSuggestActive bool

	// T406: Bridge/remote hooks
	replBridgeHook    *bridgehooks.ReplBridgeHook
	remoteSessionHook *bridgehooks.RemoteSessionHook
	mailboxHook       *bridgehooks.MailboxBridgeHook

	// T411: Display hooks — typeahead buffering, elapsed time, blink, status throttle.
	displayHooks *DisplayHooks

	// T415: Standalone hooks from pkg/ui/hooks (top-level).
	terminalSize       *hooks.TerminalSizeTracker
	globalKeys         *hooks.GlobalKeybindings
	mergedTools        *hooks.MergedTools
	apiKeyVerification *hooks.ApiKeyVerification
	interactionTracker *hooks.InteractionTracker
	updateNotification *hooks.UpdateNotification
}

// NewAppModel creates a new AppModel with the given session and bridge.
func NewAppModel(sess *session.SessionState, bridge *EventBridge) *AppModel {
	t := theme.Current()

	header := components.NewHeader(t)
	if sess != nil {
		header.SetModel(sess.Config.Model)
		header.SetCWD(sess.CWD)
	}

	inputPane := components.NewInputPane()
	inputPane.Focus()

	// Slash autocomplete: load built-ins + user/project commands + skills.
	slashInput := components.NewSlashCommandInput(t)
	sessCWD := ""
	if sess != nil {
		sessCWD = sess.CWD
	}
	slashInput.SetCommands(components.LoadSlashCommands(sessCWD))

	app := &AppModel{
		session:         sess,
		bridge:          bridge,
		mode:            ModeIdle,
		header:          header,
		conversation:    components.NewConversationPane(),
		input:           inputPane,
		slashInput:      slashInput,
		statusLine:      components.NewStatusLine(sess),
		bubble:          components.NewMessageBubble(t, 80),
		streaming:       components.NewStreamingText(t),
		spinner:         components.NewThinkingSpinner(t),
		activeToolCalls: make(map[string]string),
		scrollTracker:   newScrollTracker(),
		ctrlCExit:       lifecycle.NewDoublePress(),
	}

	// T399: File suggestion engine for @-mention autocomplete.
	app.initFileSuggester(sessCWD)

	// Remote permission bridge for CCR synthetic wrapping
	app.remoteBridge = remote.NewPermissionBridge()

	// Focus ring: input gets initial focus, conversation is focusable for scrolling
	app.focus = core.NewFocusManager(inputPane, app.conversation)

	// Command dispatcher for slash commands
	app.dispatcher = commands.NewDispatcher()

	// T403: Command keybinding resolver + queue processor.
	resolver := cmdhooks.NewResolver(keybindings.DefaultBindingMap())
	app.cmdKeybindings = cmdhooks.NewCommandKeybindings(resolver)
	app.cmdQueue = cmdhooks.NewQueue()
	app.cmdProcessor = cmdhooks.NewQueueProcessor(app.cmdQueue, func(cmd cmdhooks.QueuedCommand) tea.Cmd {
		dispatchCmd := app.dispatcher.Dispatch(cmd.Value)
		drainCmd := func() tea.Msg { return cmdhooks.QueueDrainedMsg{Completed: cmd} }
		if dispatchCmd == nil {
			return drainCmd
		}
		return tea.Batch(dispatchCmd, drainCmd)
	})

	// Welcome screen shown on startup
	modelName := ""
	cwd := ""
	if sess != nil {
		modelName = sess.Config.Model
		cwd = sess.CWD
	}
	app.showWelcome = true
	app.welcome = components.NewWelcomeScreen(t, modelName, cwd)

	// T400: Notification hooks state
	app.notifs = initNotifState()


	// T404: IDE connection tracker (Disconnected until IDE extension connects).
	app.ideConn = ide.NewIDEConnection()
	app.ideSelection = ide.EmptySelection
	// T407: Swarm/task hooks — disabled by default, enabled when session is
	// in swarm mode (TeammateContext is set).
	app.swarmInit = &swarmhooks.SwarmInit{Enabled: false}
	app.taskWatcher = &swarmhooks.TaskWatcher{}
	app.permPoller = &swarmhooks.PermissionPoller{}

	// T406: Bridge/remote hooks (disabled by default).
	app.replBridgeHook = bridgehooks.NewReplBridgeHook(bridgehooks.ReplBridgeHookConfig{})
	app.remoteSessionHook = bridgehooks.NewRemoteSessionHook(nil)
	app.mailboxHook = bridgehooks.NewMailboxBridgeHook(bridgehooks.MailboxBridgeConfig{})

	// T411: Display hooks — typeahead, elapsed time, blink, status throttle.
	app.displayHooks = NewDisplayHooks()

	// T415: Standalone hooks from pkg/ui/hooks (top-level).
	app.initStandaloneHooks()

	return app
}

// SetQueryFunc sets the function used to execute queries against the model.
func (a *AppModel) SetQueryFunc(fn QueryFunc) {
	a.queryFunc = fn
}

// Init initializes the AppModel and all child components.
func (a *AppModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, a.input.Init())
	// T400: Run startup notification checks.
	cmds = append(cmds, a.notifs.runStartupChecks(notifications.StartupOptions{})...)

	// T406: Initialize bridge/remote hooks.
	cmds = append(cmds, a.replBridgeHook.Init())
	cmds = append(cmds, a.remoteSessionHook.Init())
	cmds = append(cmds, a.mailboxHook.Init())

	// T407: Kick off swarm initialization if enabled.
	if initCmd := a.swarmInit.Init(); initCmd != nil {
		cmds = append(cmds, initCmd)
	}
	return tea.Batch(cmds...)
}

// Update handles messages and routes them to the appropriate handler.
func (a *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// T414: Forward messages to the double-press detector for timeout processing.
	a.ctrlCExit.Update(msg)

	// When doctor screen is active, delegate all messages to it.
	if a.showDoctor && a.doctorModel != nil {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			a.width = msg.Width
			a.height = msg.Height
			a.doctorModel.Update(msg)
			return a, nil
		case screens.DoctorDoneMsg:
			a.showDoctor = false
			a.doctorModel = nil
			return a, nil
		default:
			a.doctorModel.Update(msg)
			return a, nil
		}
	}

	// When resume screen is active, delegate all messages to it.
	if a.showResume && a.resumeModel != nil {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			a.width = msg.Width
			a.height = msg.Height
			a.resumeModel.Update(msg)
			return a, nil
		case screens.ResumeDoneMsg:
			a.showResume = false
			a.resumeModel = nil
			return a, nil
		case screens.ResumeSelectMsg:
			a.showResume = false
			a.resumeModel = nil
			return a.handleResumeSelect(msg.SessionID)
		default:
			a.resumeModel.Update(msg)
			return a, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return a.handleResize(msg)

	case tea.KeyPressMsg:
		return a.handleKey(msg)

	case components.SubmitMsg:
		return a.handleSubmit(msg)

	case components.SlashCommandSelectedMsg:
		// Fill the input with the chosen command; the user presses Enter
		// to submit. Matches Claude Code: autocomplete completes the name
		// but does not auto-submit.
		if a.input != nil {
			a.input.SetValue(msg.Command.Name + " ")
		}
		return a, nil

	case components.SpinnerTickMsg:
		a.spinner.Update(msg)
		if a.spinner.IsActive() {
			return a, a.spinner.Tick()
		}
		return a, nil

	case QueryEventMsg:
		return a.handleQueryEvent(msg)

	case TextDeltaMsg:
		return a.handleTextDelta(msg)

	case ToolUseStartMsg:
		return a.handleToolUseStart(msg)

	case ToolResultMsg:
		return a.handleToolResult(msg)

	case TurnCompleteMsg:
		return a.handleTurnComplete(msg)

	case queryDoneMsg:
		return a.handleQueryDone(msg)

	case StatusUpdateMsg:
		a.mode = msg.Mode
		return a, nil

	// T400: Route toast notifications.
	case components.ToastMsg:
		_, cmd := a.notifs.toast.Update(msg)
		return a, cmd

	case components.ToastDismissMsg:
		a.notifs.toast.Update(msg)
		return a, nil

	// Slash command results
	case commands.ClearConversationMsg:
		a.conversation.Update(components.ClearMessagesMsg{})
		if a.session != nil {
			a.session.Messages = a.session.Messages[:0]
			a.session.TurnCount = 0
		}
		return a, nil

	case commands.ModelSwitchMsg:
		if a.session != nil {
			a.session.Config.Model = msg.Model
			a.header.SetModel(msg.Model)
		}
		return a, nil

	case commands.QuitMsg:
		return a, tea.Quit

	case commands.ExitGoodbyeMsg:
		// Show goodbye message then quit
		a.session.Messages = append(a.session.Messages, message.Message{
			Role:    "assistant",
			Content: []message.ContentBlock{{Type: "text", Text: msg.Message}},
		})
		return a, tea.Quit

	case commands.ShowDoctorMsg:
		return a.handleShowDoctor()

	case commands.ShowResumeMsg:
		return a.handleShowResume()

	case screens.ResumeDoneMsg:
		a.showResume = false
		a.resumeModel = nil
		return a, nil

	case screens.ResumeSelectMsg:
		a.showResume = false
		a.resumeModel = nil
		return a.handleResumeSelect(msg.SessionID)

	case screens.DoctorDoneMsg:
		a.showDoctor = false
		a.doctorModel = nil
		return a, nil

	// --- Permission prompt integration ---
	// Source: interactiveHandler.ts — permission prompt overlay in REPL
	case permissions.PermissionRequestMsg:
		return a.handlePermissionRequest(msg)

	case permissions.PermissionResponseMsg:
		return a.handlePermissionResponse(msg)

	// T406: Bridge/remote hook message routing
	case bridgehooks.BridgeStatusMsg:
		var bcmds []tea.Cmd
		_, cmd1 := a.replBridgeHook.Update(msg)
		_, cmd2 := a.remoteSessionHook.Update(msg)
		if cmd1 != nil {
			bcmds = append(bcmds, cmd1)
		}
		if cmd2 != nil {
			bcmds = append(bcmds, cmd2)
		}
		return a, tea.Batch(bcmds...)

	case bridgehooks.RemoteSessionURLMsg:
		_, cmd := a.remoteSessionHook.Update(msg)
		return a, cmd

	case bridgehooks.MailboxPollMsg:
		_, cmd := a.mailboxHook.Update(msg)
		return a, cmd

	// --- Slash command: /compact ---
	// Source: REPL.tsx — /compact triggers context window compaction
	case commands.CompactMsg:
		return a.handleCompact()

	// --- Slash command: /thinking ---
	// Source: REPL.tsx — /thinking toggles extended thinking
	case commands.ThinkingToggleMsg:
		return a.handleThinkingToggle()

	case commands.ShowHelpMsg:
		helpText := a.dispatcher.HelpText()
		helpMsg := message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{{Type: message.ContentText, Text: helpText}},
		}
		a.conversation.AddMessage(helpMsg)
		return a, nil

	case commands.CommandResult:
		if msg.Error != nil {
			errMsg := message.Message{
				Role:    message.RoleAssistant,
				Content: []message.ContentBlock{{Type: message.ContentText, Text: "Error: " + msg.Error.Error()}},
			}
			a.conversation.AddMessage(errMsg)
		} else if msg.Output != "" {
			outMsg := message.Message{
				Role:    message.RoleAssistant,
				Content: []message.ContentBlock{{Type: message.ContentText, Text: msg.Output}},
			}
			a.conversation.AddMessage(outMsg)
		}
		return a, nil

	// --- T407: Swarm/task hook messages ---
	// Source: useSwarmInitialization.ts, useTaskListWatcher.ts, useSwarmPermissionPoller.ts
	case swarmhooks.SwarmInitMsg:
		return a.handleSwarmInit(msg)

	case swarmhooks.TaskWatchMsg:
		return a.handleTaskWatch(msg)

	case swarmhooks.PermissionPollMsg:
		return a.handlePermissionPoll(msg)

	// T403: Command keybinding -> queue -> processor pipeline.
	case cmdhooks.ExecuteCommandMsg:
		a.cmdQueue.Enqueue(cmdhooks.QueuedCommand{
			Value:    msg.Command,
			Priority: cmdhooks.PriorityNow,
			Mode:     "keybinding",
		})
		return a, a.cmdProcessor.Update(cmdhooks.ProcessQueueMsg{})

	case cmdhooks.ProcessQueueMsg:
		return a, a.cmdProcessor.Update(msg)

	case cmdhooks.QueueDrainedMsg:
		return a, a.cmdProcessor.Update(msg)

	default:
		// T411: Display hook tick messages (ElapsedTimeMsg, BlinkMsg, MinDisplayTimeMsg).
		if handled, dcmd := a.displayHooks.HandleDisplayMsg(msg); handled {
			return a, dcmd
		}
	}

	// Route unhandled messages to focused component
	cmd := a.focus.Route(msg)
	return a, cmd
}

// View renders the full application UI.
func (a *AppModel) View() tea.View {
	if a.width == 0 || a.height == 0 {
		return tea.NewView("Initializing...")
	}

	// Doctor screen overlay
	if a.showDoctor && a.doctorModel != nil {
		return a.doctorModel.View()
	}

	// Resume screen overlay
	if a.showResume && a.resumeModel != nil {
		return a.resumeModel.View()
	}

	t := theme.Current()
	cs := t.Colors()
	var sections []string

	if a.showWelcome {
		// Welcome screen with input and status below
		sections = append(sections, a.welcome.View().Content)
	} else {
		// Normal mode: header + conversation
		sections = append(sections, a.header.View().Content)
		sections = append(sections, a.conversation.View().Content)
	}

	// Heavy divider line ━━━ separating content from input
	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.BorderSubtle))
	sections = append(sections, dividerStyle.Render(strings.Repeat(components.DividerChar, a.width)))

	// Input pane (always visible)
	sections = append(sections, a.input.View().Content)

	// Slash-command autocomplete, rendered directly below the input when active.
	if a.slashInput != nil && a.slashInput.IsActive() {
		sections = append(sections, a.slashInput.View().Content)
	}

	// T399: File suggestion autocomplete, rendered below input when @-mention active.
	if fileSuggestView := a.renderFileSuggestions(); fileSuggestView != "" {
		sections = append(sections, fileSuggestView)
	}

	// Second divider below input (Claude has dividers above AND below prompt)
	sections = append(sections, dividerStyle.Render(strings.Repeat(components.DividerChar, a.width)))

	// T400: Toast notifications (above status line)
	if a.notifs.toast.HasToasts() {
		sections = append(sections, a.notifs.toast.View().Content)
	}

	// Status line (always visible)
	sections = append(sections, a.statusLine.View().Content)

	v := tea.NewView(strings.Join(sections, "\n"))
	// Enable alternate screen buffer so the user's terminal history is
	// preserved when the TUI exits.
	// Source: ink/ink.tsx — TS Ink uses alternate screen
	v.AltScreen = true
	return v
}

// --- Message handlers ---

func (a *AppModel) handleShowDoctor() (*AppModel, tea.Cmd) {
	// Collect diagnostic data using the T69 aggregator.
	// Source: Doctor.tsx — gathers diagnostic data on mount via getDoctorDiagnostic.
	diag := pkgdoctor.Collect(pkgdoctor.CollectOptions{})
	cfg := screens.NewDoctorConfigFromDiagnostic(diag)
	a.doctorModel = screens.NewDoctorModel(cfg)
	a.showDoctor = true
	// Send a resize so doctor knows terminal dimensions.
	if a.width > 0 && a.height > 0 {
		a.doctorModel.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
	}
	return a, nil
}

func (a *AppModel) handleResize(msg tea.WindowSizeMsg) (*AppModel, tea.Cmd) {
	a.width = msg.Width
	a.height = msg.Height

	// Layout: header(1) + conversation(flex) + divider(1) + input(3) + status(1)
	headerHeight := 1
	dividerHeight := 1
	inputHeight := 3
	statusHeight := 1
	convHeight := a.height - headerHeight - dividerHeight - inputHeight - statusHeight
	if convHeight < 1 {
		convHeight = 1
	}

	a.header.SetSize(a.width, headerHeight)
	a.conversation.SetSize(a.width, convHeight)
	a.input.SetSize(a.width, inputHeight)
	a.statusLine.SetSize(a.width, statusHeight)
	a.bubble.SetWidth(a.width)
	a.streaming.SetSize(a.width, 0)
	a.welcome.SetSize(a.width, a.height)

	// T415: Update terminal size tracker on resize.
	if a.terminalSize != nil {
		a.terminalSize.Update(a.width, a.height)
	}

	return a, nil
}

func (a *AppModel) handleKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	// T411: Typeahead buffering — swallow keystrokes while streaming/tool-running.
	isCancel := (msg.Code == 'c' && msg.Mod == tea.ModCtrl) || msg.Code == tea.KeyEscape
	if !isCancel {
		if !a.displayHooks.PushKey(msg) {
			return a, nil
		}
	}

	// Reset Ctrl+C pending on any non-Ctrl+C key (T414: delegate to lifecycle.DoublePress).
	if !(msg.Code == 'c' && msg.Mod == tea.ModCtrl) && a.ctrlCExit.Pending() {
		a.ctrlCExit.Reset()
		a.statusLine.Update(components.ModeChangeMsg{Mode: components.ModeIdle})
	}

	// Dismiss welcome screen on any printable key
	if a.showWelcome && msg.Text != "" {
		a.showWelcome = false
	}

	switch {
	// Ctrl+C behavior (matching Claude Code):
	// 1. During streaming → cancel the query
	// 2. With text in input → clear input (stash behavior)
	// 3. Empty input, first press → show "Press Ctrl-C again to exit" hint
	// 4. Empty input, second press → quit
	// Source: data/claude/area-06-status/status-ctrlc-first snapshot
	case msg.Code == 'c' && msg.Mod == tea.ModCtrl:
		if a.mode != ModeIdle && a.cancelQuery != nil {
			a.cancelQuery()
			a.ctrlCExit.Reset()
			return a, nil
		}
		if a.input.HasText() {
			a.input.Clear()
			a.ctrlCExit.Reset()
			return a, nil
		}
		// Empty input: double-press to quit (T414: lifecycle.DoublePress with 800ms timeout)
		fired, cmd := a.ctrlCExit.Press()
		if fired {
			return a, tea.Quit
		}
		a.statusLine.Update(components.CtrlCHintMsg{})
		return a, cmd

	// Focus cycling
	case msg.Code == tea.KeyTab && msg.Mod == 0:
		a.focus.Next()
		return a, nil
	case msg.Code == tea.KeyTab && msg.Mod == tea.ModShift:
		a.focus.Prev()
		return a, nil

	// Escape: cancel running query OR close modal
	// Source: screens/REPL.tsx — Escape cancels running queries
	case msg.Code == tea.KeyEscape:
		if a.focus.ModalActive() {
			a.focus.PopModal()
			return a, nil
		}
		if a.mode != ModeIdle && a.cancelQuery != nil {
			a.cancelQuery()
			return a, nil
		}
	}

	// T403: Check command keybindings.
	if a.cmdKeybindings != nil {
		if cmd := a.cmdKeybindings.Update(msg); cmd != nil {
			return a, cmd
		}
	}

	// Slash autocomplete: when active, arrow keys / Enter / Tab / Escape
	// are routed to the autocomplete. Other keys fall through so the user
	// can keep typing to filter suggestions.
	if a.slashInput != nil && a.slashInput.IsActive() {
		switch msg.Code {
		case tea.KeyUp, tea.KeyDown, tea.KeyTab, tea.KeyEscape, tea.KeyEnter:
			_, cmd := a.slashInput.Update(msg)
			return a, cmd
		}
	}

	// Route to focused component (input pane) then refresh autocomplete
	// state from the resulting input-buffer contents.
	cmd := a.focus.Route(msg)
	a.refreshSlashAutocomplete()
	a.refreshFileAutocomplete()
	return a, cmd
}

// refreshSlashAutocomplete activates/deactivates and refilters the slash
// autocomplete based on the current input buffer.
func (a *AppModel) refreshSlashAutocomplete() {
	if a.slashInput == nil || a.input == nil {
		return
	}
	text := a.input.Value()
	if strings.HasPrefix(text, "/") && !strings.Contains(text, " ") {
		a.slashInput.Activate(text)
	} else if a.slashInput.IsActive() {
		a.slashInput.Deactivate()
	}
}

func (a *AppModel) handleSubmit(msg components.SubmitMsg) (*AppModel, tea.Cmd) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		// Claude returns early on empty submit — welcome stays visible
		// Source: REPL.tsx line 1368: if (inputValue.trim().length === 0) return;
		return a, nil
	}

	// Dismiss welcome screen only when there's actual content to submit
	a.showWelcome = false

	// Add to input history
	a.input.AddToHistory(text)

	// Check for slash commands first
	if commands.IsCommand(text) {
		cmd := a.dispatcher.Dispatch(text)
		return a, cmd
	}

	// Regular user input — add to session and dispatch query
	userMsg := message.UserMessage(text)
	if a.session != nil {
		a.session.PushMessage(userMsg)
	}
	a.conversation.AddMessage(userMsg)

	// Start spinner and dispatch query in background goroutine
	a.mode = ModeStreaming
	a.statusLine.Update(components.ModeChangeMsg{Mode: components.ModeStreaming})
	a.spinner.Start()

	// Set spinner effort from thinking config if available
	if a.session != nil && a.session.Config.ThinkingEnabled {
		budget := a.session.Config.ThinkingBudget
		switch {
		case budget >= 30000:
			a.spinner.SetEffort("max")
		case budget >= 15000:
			a.spinner.SetEffort("high")
		case budget >= 5000:
			a.spinner.SetEffort("medium")
		default:
			a.spinner.SetEffort("low")
		}
	}

	// Show spinner in conversation
	a.conversation.SetStreamingText(a.spinner.View() + "\n" + a.spinner.TipView())

	// T411: Block typeahead, start elapsed timer, and enable blink.
	displayCmd := a.displayHooks.BlockForQuery()

	if a.queryFunc != nil {
		ctx, cancel := context.WithCancel(context.Background())
		a.queryCtx = ctx
		a.cancelQuery = cancel

		queryFunc := a.queryFunc
		sess := a.session

		return a, tea.Batch(a.spinner.Tick(), displayCmd, func() tea.Msg {
			var onEvent query.EventCallback
			if a.bridge != nil {
				onEvent = a.bridge.BridgeCallback()
			}
			err := queryFunc(ctx, sess, onEvent)
			return queryDoneMsg{err: err}
		})
	}

	return a, tea.Batch(a.spinner.Tick(), displayCmd)
}

func (a *AppModel) handleQueryEvent(msg QueryEventMsg) (*AppModel, tea.Cmd) {
	evt := msg.Event
	switch evt.Type {
	case query.QEventTextDelta:
		return a.handleTextDelta(TextDeltaMsg{Text: evt.Text})
	case query.QEventToolUseStart:
		return a.handleToolUseStart(ToolUseStartMsg{
			ToolUseID: evt.ToolUseID,
			ToolName:  evt.ToolName,
		})
	case query.QEventToolResult:
		return a.handleToolResult(ToolResultMsg{
			ToolUseID: evt.ToolUseID,
			Content:   evt.Content,
			IsError:   evt.IsError,
			Display:   evt.Display,
		})
	case query.QEventTurnComplete:
		return a.handleTurnComplete(TurnCompleteMsg{StopReason: evt.StopReason})
	case query.QEventUsage:
		if a.session != nil {
			a.session.TotalInputTokens += evt.InputTokens
			a.session.TotalOutputTokens += evt.OutputTokens
			a.statusLine.Update(components.TokenUpdateMsg{
				InputTokens:  a.session.TotalInputTokens,
				OutputTokens: a.session.TotalOutputTokens,
			})
		}
		return a, nil
	}
	return a, nil
}

func (a *AppModel) handleTextDelta(msg TextDeltaMsg) (*AppModel, tea.Cmd) {
	a.mode = ModeStreaming
	a.streamingText.WriteString(msg.Text)

	// Update streaming text component and conversation pane
	// Show spinner line above the streaming text
	a.streaming.AppendDelta(msg.Text)
	streamContent := a.streamingText.String()
	if a.spinner.IsActive() {
		streamContent = a.spinner.View() + "\n" + streamContent
	}
	a.conversation.SetStreamingText(streamContent)

	a.statusLine.Update(components.ModeChangeMsg{Mode: components.ModeStreaming})

	return a, nil
}

func (a *AppModel) handleToolUseStart(msg ToolUseStartMsg) (*AppModel, tea.Cmd) {
	a.mode = ModeToolRunning
	a.activeToolCalls[msg.ToolUseID] = msg.ToolName

	// Show tool name in streaming area (matching TS inline tool display).
	// Source: components/messages/AssistantToolUseMessage.tsx
	toolLine := fmt.Sprintf("\n⏺ %s", msg.ToolName)
	a.streamingText.WriteString(toolLine)
	streamContent := a.streamingText.String()
	if a.spinner.IsActive() {
		streamContent = a.spinner.View() + "\n" + streamContent
	}
	a.conversation.SetStreamingText(streamContent)

	a.statusLine.Update(components.ModeChangeMsg{Mode: components.ModeToolRunning})

	return a, nil
}

func (a *AppModel) handleToolResult(msg ToolResultMsg) (*AppModel, tea.Cmd) {
	toolName := a.activeToolCalls[msg.ToolUseID]
	delete(a.activeToolCalls, msg.ToolUseID)

	// If the tool attached a structured Display payload (e.g. a diff from
	// Edit/Write), finalize any in-flight streaming text and add the
	// tool_result as its own conversation message so the MessageBubble
	// renderer can draw the rich block.
	if !msg.IsError && msg.Display != nil {
		if a.streamingText.Len() > 0 {
			assistantMsg := message.Message{
				Role: message.RoleAssistant,
				Content: []message.ContentBlock{
					{Type: message.ContentText, Text: a.streamingText.String()},
				},
			}
			a.conversation.AddMessage(assistantMsg)
			a.streamingText.Reset()
			a.streaming.Reset()
		}
		resultMsg := message.Message{
			Role: message.RoleUser,
			Content: []message.ContentBlock{{
				Type:      message.ContentToolResult,
				ToolUseID: msg.ToolUseID,
				Content:   msg.Content,
				IsError:   false,
				Display:   msg.Display,
			}},
		}
		a.conversation.AddMessage(resultMsg)
		a.conversation.ClearStreamingText()
		return a, nil
	}

	// Show brief result indicator in streaming area.
	// Source: components/messages/UserToolResultMessage — shows ✓/✗ with truncated content
	if msg.IsError {
		a.streamingText.WriteString(fmt.Sprintf("\n  ✗ %s error", toolName))
	} else {
		content := msg.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		if content != "" {
			a.streamingText.WriteString(fmt.Sprintf("\n  ✓ %s", toolName))
		}
	}
	streamContent := a.streamingText.String()
	if a.spinner.IsActive() {
		streamContent = a.spinner.View() + "\n" + streamContent
	}
	a.conversation.SetStreamingText(streamContent)

	return a, nil
}

func (a *AppModel) handleTurnComplete(msg TurnCompleteMsg) (*AppModel, tea.Cmd) {
	a.mode = ModeIdle
	a.spinner.Stop()

	// Finalize the assistant message — add accumulated text to conversation
	if a.streamingText.Len() > 0 {
		assistantMsg := message.Message{
			Role: message.RoleAssistant,
			Content: []message.ContentBlock{
				{Type: message.ContentText, Text: a.streamingText.String()},
			},
		}
		a.conversation.AddMessage(assistantMsg)
	}

	// Reset streaming state
	a.streamingText.Reset()
	a.streaming.Reset()
	a.conversation.ClearStreamingText()
	a.activeToolCalls = make(map[string]string)

	a.statusLine.Update(components.ModeChangeMsg{Mode: components.ModeIdle})

	// Persist session after each turn.
	// Source: REPL.tsx — session is saved after each assistant turn.
	if a.session != nil {
		a.session.TurnCount++
		_ = a.session.Save() // fire-and-forget; errors are non-fatal
	}

	// T411: Unblock typeahead, stop elapsed timer, disable blink, replay buffered keys.
	replayCmds := a.displayHooks.UnblockAfterQuery()
	if len(replayCmds) > 0 {
		return a, tea.Batch(replayCmds...)
	}
	return a, nil
}

func (a *AppModel) handleQueryDone(msg queryDoneMsg) (*AppModel, tea.Cmd) {
	a.cancelQuery = nil
	a.queryCtx = nil

	// Stop spinner and clean up streaming state.
	// Source: screens/REPL.tsx — error paths must reset UI state
	a.spinner.Stop()
	if a.streamingText.Len() > 0 {
		// Finalize any partial streaming text before showing error
		assistantMsg := message.Message{
			Role: message.RoleAssistant,
			Content: []message.ContentBlock{
				{Type: message.ContentText, Text: a.streamingText.String()},
			},
		}
		a.conversation.AddMessage(assistantMsg)
	}
	a.streamingText.Reset()
	a.streaming.Reset()
	a.conversation.ClearStreamingText()
	a.activeToolCalls = make(map[string]string)

	if msg.err != nil {
		// Show error as a system message in conversation
		errMsg := message.Message{
			Role: message.RoleAssistant,
			Content: []message.ContentBlock{
				{Type: message.ContentText, Text: "Error: " + msg.err.Error()},
			},
		}
		a.conversation.AddMessage(errMsg)
	}

	// Ensure mode is idle after query completes
	a.mode = ModeIdle
	a.statusLine.Update(components.ModeChangeMsg{Mode: components.ModeIdle})

	return a, nil
}

func (a *AppModel) handleShowResume() (*AppModel, tea.Cmd) {
	// Load session list for the resume picker.
	// Source: ResumeConversation.tsx — loads sessions on mount
	sessions, err := session.ListSessions()
	if err != nil {
		errMsg := message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{{Type: message.ContentText, Text: "Error loading sessions: " + err.Error()}},
		}
		a.conversation.AddMessage(errMsg)
		return a, nil
	}

	a.resumeModel = screens.NewResumeModel(sessions)
	a.showResume = true
	if a.width > 0 && a.height > 0 {
		a.resumeModel.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
	}
	return a, nil
}

func (a *AppModel) handleResumeSelect(sessionID string) (*AppModel, tea.Cmd) {
	// Load the selected session.
	// Source: ResumeConversation.tsx — onSelect callback
	loaded, err := session.Load(sessionID)
	if err != nil {
		errMsg := message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{{Type: message.ContentText, Text: "Error loading session: " + err.Error()}},
		}
		a.conversation.AddMessage(errMsg)
		return a, nil
	}

	// Replace current session with the loaded one
	if a.session != nil {
		loaded.CWD = a.session.CWD
		loaded.Config.SystemPrompt = a.session.Config.SystemPrompt
	}
	a.session = loaded

	// Reload conversation from restored session messages
	a.conversation.Update(components.ClearMessagesMsg{})
	for _, msg := range loaded.Messages {
		a.conversation.AddMessage(msg)
	}

	// Update header
	a.header.SetModel(loaded.Config.Model)

	// Update status line
	a.statusLine = components.NewStatusLine(loaded)

	infoMsg := message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: fmt.Sprintf("Resumed session %s (%d turns)", shortSessionID(loaded.ID), loaded.TurnCount)}},
	}
	a.conversation.AddMessage(infoMsg)

	// Cross-project resume warning
	// Source: sessionRestore.ts:439-440
	if warning := session.ValidateCrossProjectResume(loaded.CWD, a.session.CWD); warning != "" {
		warnMsg := message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{{Type: message.ContentText, Text: warning}},
		}
		a.conversation.AddMessage(warnMsg)
	}

	return a, nil
}

// MarkScrollActivity records a scroll event for drain tracking.
// Source: bootstrap/state.ts — markScrollActivity
func (a *AppModel) MarkScrollActivity() {
	a.scrollTracker.MarkScrollActivity()
}

// GetIsScrollDraining returns true while scroll is actively draining.
// Source: bootstrap/state.ts — getIsScrollDraining
func (a *AppModel) GetIsScrollDraining() bool {
	return a.scrollTracker.GetIsScrollDraining()
}

// WaitForScrollIdle blocks until scroll is no longer draining.
// Source: bootstrap/state.ts — waitForScrollIdle
func (a *AppModel) WaitForScrollIdle() {
	a.scrollTracker.WaitForScrollIdle()
}

// shortSessionID returns first 8 chars of a session ID.
func shortSessionID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// --- T73: Permission prompt flow ---

// handlePermissionRequest shows a permission prompt in the conversation area.
// Source: REPL.tsx + interactiveHandler.ts — permission prompt display
func (a *AppModel) handlePermissionRequest(msg permissions.PermissionRequestMsg) (*AppModel, tea.Cmd) {
	a.pendingPermission = &msg

	// Format the permission prompt as a system message
	inputDesc := remote.FormatToolInput(msg.Input, 3)
	promptText := fmt.Sprintf("Permission required: %s\n  %s\n  %s\n\n  [y]es / [n]o / [a]lways allow",
		msg.ToolName, msg.Description, inputDesc)
	promptMsg := message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: promptText}},
	}
	a.conversation.AddMessage(promptMsg)

	return a, nil
}

// handlePermissionResponse processes user response to a permission prompt.
func (a *AppModel) handlePermissionResponse(msg permissions.PermissionResponseMsg) (*AppModel, tea.Cmd) {
	a.pendingPermission = nil
	return a, nil
}

// handleCompact triggers context window compaction.
// Source: REPL.tsx — /compact handler
func (a *AppModel) handleCompact() (*AppModel, tea.Cmd) {
	if a.session == nil {
		return a, nil
	}
	infoMsg := message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "Compacting conversation context..."}},
	}
	a.conversation.AddMessage(infoMsg)
	return a, nil
}

// handleThinkingToggle toggles extended thinking mode.
// Source: REPL.tsx — /thinking handler
func (a *AppModel) handleThinkingToggle() (*AppModel, tea.Cmd) {
	if a.session == nil {
		return a, nil
	}
	a.session.Config.ThinkingEnabled = !a.session.Config.ThinkingEnabled
	state := "disabled"
	if a.session.Config.ThinkingEnabled {
		state = "enabled"
		if a.session.Config.ThinkingBudget == 0 {
			a.session.Config.ThinkingBudget = 10000 // default budget
		}
	}
	infoMsg := message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: fmt.Sprintf("Extended thinking %s (budget: %d tokens)", state, a.session.Config.ThinkingBudget)}},
	}
	a.conversation.AddMessage(infoMsg)
	return a, nil
}

// ---------------------------------------------------------------------------
// T407: Swarm/task hook handlers
// Source: useSwarmInitialization.ts, useTaskListWatcher.ts, useSwarmPermissionPoller.ts
// ---------------------------------------------------------------------------

// handleSwarmInit processes the result of swarm initialization and starts
// the task watcher and permission poller ticks.
func (a *AppModel) handleSwarmInit(msg swarmhooks.SwarmInitMsg) (*AppModel, tea.Cmd) {
	if msg.Err != nil {
		errMsg := message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{{Type: message.ContentText, Text: "Swarm init error: " + msg.Err.Error()}},
		}
		a.conversation.AddMessage(errMsg)
		return a, nil
	}

	// Start task watcher and permission poller ticks.
	var cmds []tea.Cmd
	if a.taskWatcher.AgentID != "" {
		cmds = append(cmds, a.taskWatcher.Tick())
	}
	if a.permPoller.AgentName != "" {
		cmds = append(cmds, a.permPoller.Tick())
	}
	return a, tea.Batch(cmds...)
}

// handleTaskWatch processes a task-watcher tick result and reschedules the next tick.
func (a *AppModel) handleTaskWatch(msg swarmhooks.TaskWatchMsg) (*AppModel, tea.Cmd) {
	if msg.Err != nil {
		errMsg := message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{{Type: message.ContentText, Text: "Task watcher error: " + msg.Err.Error()}},
		}
		a.conversation.AddMessage(errMsg)
	}
	// Reschedule next tick.
	return a, a.taskWatcher.Tick()
}

// handlePermissionPoll processes a permission-poll tick result and reschedules.
func (a *AppModel) handlePermissionPoll(msg swarmhooks.PermissionPollMsg) (*AppModel, tea.Cmd) {
	if msg.Err != nil {
		errMsg := message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{{Type: message.ContentText, Text: "Permission poll error: " + msg.Err.Error()}},
		}
		a.conversation.AddMessage(errMsg)
	}
	// Reschedule next tick.
	return a, a.permPoller.Tick()
}

// SwarmInit returns the swarm initialization hook (for testing/integration).
func (a *AppModel) SwarmInit() *swarmhooks.SwarmInit {
	return a.swarmInit
}

// TaskWatcher returns the task watcher hook (for testing/integration).
func (a *AppModel) TaskWatcher() *swarmhooks.TaskWatcher {
	return a.taskWatcher
}

// PermissionPoller returns the permission poller hook (for testing/integration).
func (a *AppModel) PermissionPoller() *swarmhooks.PermissionPoller {
	return a.permPoller
}

// T406: Bridge hook accessors

func (a *AppModel) ReplBridgeHook() *bridgehooks.ReplBridgeHook {
	return a.replBridgeHook
}

func (a *AppModel) RemoteSessionHook() *bridgehooks.RemoteSessionHook {
	return a.remoteSessionHook
}

func (a *AppModel) MailboxHook() *bridgehooks.MailboxBridgeHook {
	return a.mailboxHook
}

// GetDisplayHooks returns the display hooks for testing/integration.
func (a *AppModel) GetDisplayHooks() *DisplayHooks {
	return a.displayHooks
}
