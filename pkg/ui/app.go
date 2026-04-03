package ui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
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
	statusLine   *components.StatusLine
	bubble       *components.MessageBubble
	streaming    *components.StreamingText

	// Streaming state — tracks the current assistant turn
	streamingText   strings.Builder
	activeToolCalls map[string]string // toolUseID → toolName

	// Query execution
	queryFunc QueryFunc
	queryCtx  context.Context
	cancelQuery context.CancelFunc
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

	app := &AppModel{
		session:         sess,
		bridge:          bridge,
		mode:            ModeIdle,
		header:          header,
		conversation:    components.NewConversationPane(),
		input:           inputPane,
		statusLine:      components.NewStatusLine(sess),
		bubble:          components.NewMessageBubble(t, 80),
		streaming:       components.NewStreamingText(t),
		activeToolCalls: make(map[string]string),
	}

	// Focus ring: input gets initial focus, conversation is focusable for scrolling
	app.focus = core.NewFocusManager(inputPane, app.conversation)

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

	var sections []string

	// Header (1 line)
	sections = append(sections, a.header.View().Content)

	// Conversation area (fills remaining space)
	sections = append(sections, a.conversation.View().Content)

	// Input pane (1-3 lines)
	sections = append(sections, a.input.View().Content)

	// Status line (1 line)
	sections = append(sections, a.statusLine.View().Content)

	return tea.NewView(strings.Join(sections, "\n"))
}

// --- Message handlers ---

func (a *AppModel) handleResize(msg tea.WindowSizeMsg) (*AppModel, tea.Cmd) {
	a.width = msg.Width
	a.height = msg.Height

	// Layout: header(1) + conversation(flex) + input(3) + status(1)
	headerHeight := 1
	inputHeight := 3
	statusHeight := 1
	convHeight := a.height - headerHeight - inputHeight - statusHeight
	if convHeight < 1 {
		convHeight = 1
	}

	a.header.SetSize(a.width, headerHeight)
	a.conversation.SetSize(a.width, convHeight)
	a.input.SetSize(a.width, inputHeight)
	a.statusLine.SetSize(a.width, statusHeight)
	a.bubble.SetWidth(a.width)
	a.streaming.SetSize(a.width, 0)

	return a, nil
}

func (a *AppModel) handleKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	switch {
	// Quit: Ctrl+C
	case msg.Code == 'c' && msg.Mod == tea.ModCtrl:
		if a.mode != ModeIdle && a.cancelQuery != nil {
			// Cancel the running query, don't quit
			a.cancelQuery()
			return a, nil
		}
		return a, tea.Quit

	// Focus cycling
	case msg.Code == tea.KeyTab && msg.Mod == 0:
		a.focus.Next()
		return a, nil
	case msg.Code == tea.KeyTab && msg.Mod == tea.ModShift:
		a.focus.Prev()
		return a, nil

	// Escape closes modal
	case msg.Code == tea.KeyEscape:
		if a.focus.ModalActive() {
			a.focus.PopModal()
			return a, nil
		}
	}

	// Route to focused component
	cmd := a.focus.Route(msg)
	return a, cmd
}

func (a *AppModel) handleSubmit(msg components.SubmitMsg) (*AppModel, tea.Cmd) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return a, nil
	}

	// Add user message to session and conversation
	userMsg := message.UserMessage(text)
	if a.session != nil {
		a.session.PushMessage(userMsg)
	}
	a.conversation.AddMessage(userMsg)

	// Add to input history
	a.input.AddToHistory(text)

	// Dispatch query in background goroutine
	if a.queryFunc != nil {
		a.mode = ModeStreaming
		a.statusLine.Update(components.ModeChangeMsg{Mode: components.ModeStreaming})

		ctx, cancel := context.WithCancel(context.Background())
		a.queryCtx = ctx
		a.cancelQuery = cancel

		queryFunc := a.queryFunc
		sess := a.session

		return a, func() tea.Msg {
			var onEvent query.EventCallback
			if a.bridge != nil {
				onEvent = a.bridge.BridgeCallback()
			}
			err := queryFunc(ctx, sess, onEvent)
			return queryDoneMsg{err: err}
		}
	}

	return a, nil
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
	a.streaming.AppendDelta(msg.Text)
	a.conversation.SetStreamingText(a.streamingText.String())

	a.statusLine.Update(components.ModeChangeMsg{Mode: components.ModeStreaming})

	return a, nil
}

func (a *AppModel) handleToolUseStart(msg ToolUseStartMsg) (*AppModel, tea.Cmd) {
	a.mode = ModeToolRunning
	a.activeToolCalls[msg.ToolUseID] = msg.ToolName

	a.statusLine.Update(components.ModeChangeMsg{Mode: components.ModeToolRunning})

	return a, nil
}

func (a *AppModel) handleToolResult(msg ToolResultMsg) (*AppModel, tea.Cmd) {
	delete(a.activeToolCalls, msg.ToolUseID)
	return a, nil
}

func (a *AppModel) handleTurnComplete(msg TurnCompleteMsg) (*AppModel, tea.Cmd) {
	a.mode = ModeIdle

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
