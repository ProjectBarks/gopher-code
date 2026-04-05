package ui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui/commands"
	"github.com/projectbarks/gopher-code/pkg/ui/components"
	"github.com/projectbarks/gopher-code/pkg/ui/core"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

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

	// Welcome screen
	showWelcome bool
	welcome     *components.WelcomeScreen

	// Ctrl+C double-press tracking
	// Claude requires two Ctrl+C presses on empty idle input to quit.
	// First press shows "Press Ctrl-C again to exit" hint.
	ctrlCPending bool
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
	}

	// Focus ring: input gets initial focus, conversation is focusable for scrolling
	app.focus = core.NewFocusManager(inputPane, app.conversation)

	// Command dispatcher for slash commands
	app.dispatcher = commands.NewDispatcher()

	// Welcome screen shown on startup
	modelName := ""
	cwd := ""
	if sess != nil {
		modelName = sess.Config.Model
		cwd = sess.CWD
	}
	app.showWelcome = true
	app.welcome = components.NewWelcomeScreen(t, modelName, cwd)

	return app
}

// SetQueryFunc sets the function used to execute queries against the model.
func (a *AppModel) SetQueryFunc(fn QueryFunc) {
	a.queryFunc = fn
}

// Init initializes the AppModel and all child components.
func (a *AppModel) Init() tea.Cmd {
	return a.input.Init()
}

// Update handles messages and routes them to the appropriate handler.
func (a *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case commands.ShowHelpMsg:
		helpText := "Commands: /help /clear /model <name> /session /quit /compact /thinking"
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

	// Second divider below input (Claude has dividers above AND below prompt)
	sections = append(sections, dividerStyle.Render(strings.Repeat(components.DividerChar, a.width)))

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

	return a, nil
}

func (a *AppModel) handleKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	// Reset Ctrl+C pending on any non-Ctrl+C key
	if !(msg.Code == 'c' && msg.Mod == tea.ModCtrl) && a.ctrlCPending {
		a.ctrlCPending = false
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
			a.ctrlCPending = false
			return a, nil
		}
		if a.input.HasText() {
			a.input.Clear()
			a.ctrlCPending = false
			return a, nil
		}
		// Empty input: double-press to quit
		if a.ctrlCPending {
			return a, tea.Quit
		}
		a.ctrlCPending = true
		a.statusLine.Update(components.CtrlCHintMsg{})
		return a, nil

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

	if a.queryFunc != nil {
		ctx, cancel := context.WithCancel(context.Background())
		a.queryCtx = ctx
		a.cancelQuery = cancel

		queryFunc := a.queryFunc
		sess := a.session

		return a, tea.Batch(a.spinner.Tick(), func() tea.Msg {
			var onEvent query.EventCallback
			if a.bridge != nil {
				onEvent = a.bridge.BridgeCallback()
			}
			err := queryFunc(ctx, sess, onEvent)
			return queryDoneMsg{err: err}
		})
	}

	return a, a.spinner.Tick()
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

	// If the result carries a unified diff (Edit/Write tools), finalize any
	// in-flight streaming text and add the tool_result as its own message so
	// renderToolResultBlock can render it as a colored diff.
	if !msg.IsError && strings.Contains(msg.Content, "--- a/") &&
		strings.Contains(msg.Content, "@@") {
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
