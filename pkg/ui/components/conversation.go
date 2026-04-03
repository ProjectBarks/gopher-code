package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// AddMessageMsg triggers adding a message to the conversation pane.
type AddMessageMsg struct {
	Message message.Message
}

// ClearMessagesMsg clears all messages from the conversation pane.
type ClearMessagesMsg struct{}

// ConversationPane displays scrollable message history.
// It uses MessageBubble for rendering individual messages with Glamour
// markdown, tool call styling, and role-based formatting.
type ConversationPane struct {
	messages []message.Message
	rendered []string // Pre-rendered message strings (via MessageBubble)
	bubble   *MessageBubble
	width    int
	height   int
	focused  bool

	// Scroll state
	scrollOffset int // Lines scrolled from bottom (0 = at bottom)
	autoScroll   bool

	// Streaming state
	streamingText string
}

// NewConversationPane creates a new empty conversation pane.
func NewConversationPane() *ConversationPane {
	t := theme.Current()
	return &ConversationPane{
		messages:   make([]message.Message, 0),
		rendered:   make([]string, 0),
		bubble:     NewMessageBubble(t, 80),
		autoScroll: true,
	}
}

// Init initializes the conversation pane.
func (cp *ConversationPane) Init() tea.Cmd {
	return nil
}

// Update handles messages for the conversation pane.
func (cp *ConversationPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case AddMessageMsg:
		cp.AddMessage(msg.Message)
		return cp, nil

	case ClearMessagesMsg:
		cp.messages = cp.messages[:0]
		cp.rendered = cp.rendered[:0]
		cp.scrollOffset = 0
		return cp, nil

	case tea.KeyPressMsg:
		return cp.handleKey(msg)

	case tea.WindowSizeMsg:
		cp.SetSize(msg.Width, msg.Height)
		return cp, nil
	}
	return cp, nil
}

// View renders the conversation pane.
func (cp *ConversationPane) View() tea.View {
	if cp.width == 0 || cp.height == 0 {
		return tea.NewView("")
	}

	if len(cp.rendered) == 0 && cp.streamingText == "" {
		t := theme.Current()
		placeholder := t.TextSecondary().Render("No messages yet.")
		return tea.NewView(placeholder)
	}

	// Collect all rendered lines
	var allLines []string
	for _, r := range cp.rendered {
		allLines = append(allLines, strings.Split(r, "\n")...)
	}

	// Add streaming text if present
	if cp.streamingText != "" {
		allLines = append(allLines, strings.Split(cp.streamingText, "\n")...)
	}

	// Apply viewport: show last `height` lines (with scroll offset)
	totalLines := len(allLines)
	if totalLines == 0 {
		return tea.NewView("")
	}

	viewStart := totalLines - cp.height - cp.scrollOffset
	if viewStart < 0 {
		viewStart = 0
	}
	viewEnd := viewStart + cp.height
	if viewEnd > totalLines {
		viewEnd = totalLines
	}

	visible := allLines[viewStart:viewEnd]

	// Pad to fill height
	for len(visible) < cp.height {
		visible = append(visible, "")
	}

	return tea.NewView(strings.Join(visible, "\n"))
}

// SetSize sets the dimensions of the conversation pane.
func (cp *ConversationPane) SetSize(width, height int) {
	cp.width = width
	cp.height = height
	cp.bubble.SetWidth(width)
	// Re-render all messages with new width
	cp.rerenderAll()
}

// Focus gives focus to this pane.
func (cp *ConversationPane) Focus() {
	cp.focused = true
}

// Blur removes focus from this pane.
func (cp *ConversationPane) Blur() {
	cp.focused = false
}

// Focused returns whether this pane has focus.
func (cp *ConversationPane) Focused() bool {
	return cp.focused
}

// AddMessage adds a message to the conversation and renders it via MessageBubble.
func (cp *ConversationPane) AddMessage(msg message.Message) {
	cp.messages = append(cp.messages, msg)
	cp.rendered = append(cp.rendered, cp.bubble.Render(&msg))
	if cp.autoScroll {
		cp.scrollOffset = 0
	}
}

// SetStreamingText sets the current streaming text buffer.
func (cp *ConversationPane) SetStreamingText(text string) {
	cp.streamingText = text
}

// ClearStreamingText clears the streaming text buffer.
func (cp *ConversationPane) ClearStreamingText() {
	cp.streamingText = ""
}

// MessageCount returns the number of messages.
func (cp *ConversationPane) MessageCount() int {
	return len(cp.messages)
}

// --- Internal ---

func (cp *ConversationPane) handleKey(msg tea.KeyPressMsg) (*ConversationPane, tea.Cmd) {
	switch msg.Code {
	case tea.KeyUp:
		cp.scrollUp()
	case tea.KeyDown:
		cp.scrollDown()
	case tea.KeyPgUp:
		cp.scrollOffset += cp.height
	case tea.KeyPgDown:
		cp.scrollOffset -= cp.height
		if cp.scrollOffset < 0 {
			cp.scrollOffset = 0
		}
	}
	return cp, nil
}

func (cp *ConversationPane) scrollUp() {
	cp.scrollOffset++
	cp.autoScroll = false
}

func (cp *ConversationPane) scrollDown() {
	if cp.scrollOffset > 0 {
		cp.scrollOffset--
	}
	if cp.scrollOffset == 0 {
		cp.autoScroll = true
	}
}

func (cp *ConversationPane) rerenderAll() {
	cp.rendered = make([]string, len(cp.messages))
	for i := range cp.messages {
		cp.rendered[i] = cp.bubble.Render(&cp.messages[i])
	}
}
