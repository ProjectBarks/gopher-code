package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
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
	Model       string
	Tokens      int
	Mode        AppMode
	Cost        float64
	InputTokens int
	OutputTokens int
}

// AppMode describes the current application state.
type AppMode int

const (
	ModeIdle       AppMode = iota
	ModeStreaming
	ModeToolRunning
)

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

	// Streaming state
	streamingText strings.Builder
}

// NewAppModel creates a new AppModel with the given session and bridge.
func NewAppModel(sess *session.SessionState, bridge *EventBridge) *AppModel {
	app := &AppModel{
		session: sess,
		bridge:  bridge,
		mode:    ModeIdle,
	}

	// Initialize focus manager (will add children as components are created)
	app.focus = core.NewFocusManager()

	return app
}

// Init initializes the AppModel and all child components.
func (a *AppModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and routes them to the appropriate handler.
func (a *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return a.handleResize(msg)

	case tea.KeyPressMsg:
		return a.handleKey(msg)

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

	t := theme.Current()
	var sections []string

	// Header
	header := a.renderHeader(t)
	sections = append(sections, header)

	// Conversation area
	conversation := a.renderConversation(t)
	sections = append(sections, conversation)

	// Status line
	status := a.renderStatusLine(t)
	sections = append(sections, status)

	return tea.NewView(strings.Join(sections, "\n"))
}

// --- Message handlers ---

func (a *AppModel) handleResize(msg tea.WindowSizeMsg) (*AppModel, tea.Cmd) {
	a.width = msg.Width
	a.height = msg.Height
	return a, nil
}

func (a *AppModel) handleKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyTab && msg.Mod == 0:
		a.focus.Next()
		return a, nil
	case msg.Code == tea.KeyTab && msg.Mod == tea.ModShift:
		a.focus.Prev()
		return a, nil
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
		}
		return a, nil
	}
	return a, nil
}

func (a *AppModel) handleTextDelta(msg TextDeltaMsg) (*AppModel, tea.Cmd) {
	a.mode = ModeStreaming
	a.streamingText.WriteString(msg.Text)
	return a, nil
}

func (a *AppModel) handleToolUseStart(msg ToolUseStartMsg) (*AppModel, tea.Cmd) {
	a.mode = ModeToolRunning
	return a, nil
}

func (a *AppModel) handleToolResult(msg ToolResultMsg) (*AppModel, tea.Cmd) {
	return a, nil
}

func (a *AppModel) handleTurnComplete(msg TurnCompleteMsg) (*AppModel, tea.Cmd) {
	a.mode = ModeIdle
	a.streamingText.Reset()
	return a, nil
}

// --- Render helpers ---

func (a *AppModel) renderHeader(t theme.Theme) string {
	modelName := ""
	if a.session != nil {
		modelName = a.session.Config.Model
	}
	style := t.TextPrimary()
	return style.Render("Gopher — " + modelName)
}

func (a *AppModel) renderConversation(t theme.Theme) string {
	if a.streamingText.Len() > 0 {
		return a.streamingText.String()
	}
	return t.TextSecondary().Render("No messages yet. Type below to start a conversation.")
}

func (a *AppModel) renderStatusLine(t theme.Theme) string {
	var parts []string

	switch a.mode {
	case ModeIdle:
		parts = append(parts, "Idle")
	case ModeStreaming:
		parts = append(parts, "Streaming...")
	case ModeToolRunning:
		parts = append(parts, "Running tool...")
	}

	if a.session != nil {
		parts = append(parts, a.session.Config.Model)
	}

	style := t.TextSecondary()
	return style.Render(strings.Join(parts, " | "))
}
