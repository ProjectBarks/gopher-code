package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// SlashCommand defines an available slash command.
type SlashCommand struct {
	Name        string
	Description string
	Handler     string // Handler key for dispatch
}

// DefaultSlashCommands returns the built-in slash commands.
func DefaultSlashCommands() []SlashCommand {
	return []SlashCommand{
		{Name: "/model", Description: "Switch AI model", Handler: "model"},
		{Name: "/session", Description: "Switch session", Handler: "session"},
		{Name: "/clear", Description: "Clear conversation", Handler: "clear"},
		{Name: "/help", Description: "Show commands", Handler: "help"},
		{Name: "/compact", Description: "Compact conversation", Handler: "compact"},
		{Name: "/quit", Description: "Exit gopher", Handler: "quit"},
		{Name: "/thinking", Description: "Toggle thinking mode", Handler: "thinking"},
	}
}

// SlashCommandSelectedMsg is sent when a slash command is selected.
type SlashCommandSelectedMsg struct {
	Command SlashCommand
}

// SlashCommandInput provides autocomplete for slash commands.
type SlashCommandInput struct {
	commands    []SlashCommand
	suggestions []SlashCommand
	selected    int
	active      bool // True when showing suggestions
	prefix      string
	theme       theme.Theme
	width       int
	height      int
	focused     bool
}

// NewSlashCommandInput creates a new slash command input.
func NewSlashCommandInput(t theme.Theme) *SlashCommandInput {
	return &SlashCommandInput{
		commands: DefaultSlashCommands(),
		theme:    t,
		width:    80,
		selected: 0,
	}
}

// SetCommands sets the available slash commands.
func (sci *SlashCommandInput) SetCommands(cmds []SlashCommand) {
	sci.commands = cmds
}

// Activate starts showing suggestions for the given prefix.
func (sci *SlashCommandInput) Activate(prefix string) {
	sci.prefix = prefix
	sci.active = true
	sci.selected = 0
	sci.filterSuggestions()
}

// Deactivate hides the suggestion list.
func (sci *SlashCommandInput) Deactivate() {
	sci.active = false
	sci.suggestions = nil
	sci.prefix = ""
}

// IsActive returns whether suggestions are being shown.
func (sci *SlashCommandInput) IsActive() bool {
	return sci.active
}

// Init initializes the component.
func (sci *SlashCommandInput) Init() tea.Cmd { return nil }

// Update handles key presses for navigation.
func (sci *SlashCommandInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !sci.active {
		return sci, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp:
			if sci.selected > 0 {
				sci.selected--
			}
		case tea.KeyDown:
			if sci.selected < len(sci.suggestions)-1 {
				sci.selected++
			}
		case tea.KeyEnter, tea.KeyTab:
			if len(sci.suggestions) > 0 && sci.selected < len(sci.suggestions) {
				cmd := sci.suggestions[sci.selected]
				sci.Deactivate()
				return sci, func() tea.Msg {
					return SlashCommandSelectedMsg{Command: cmd}
				}
			}
		case tea.KeyEscape:
			sci.Deactivate()
		}
	}
	return sci, nil
}

// View renders the suggestion list.
func (sci *SlashCommandInput) View() tea.View {
	if !sci.active || len(sci.suggestions) == 0 {
		return tea.NewView("")
	}

	cs := sci.theme.Colors()
	var lines []string

	for i, cmd := range sci.suggestions {
		nameStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Accent)).Bold(true)
		descStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextSecondary))

		line := nameStyle.Render(cmd.Name) + " " + descStyle.Render(cmd.Description)

		if i == sci.selected {
			selStyle := lipgloss.NewStyle().
				Background(lipgloss.Color(cs.Selection))
			line = selStyle.Render(line)
		}

		lines = append(lines, line)
	}

	return tea.NewView(strings.Join(lines, "\n"))
}

func (sci *SlashCommandInput) filterSuggestions() {
	prefix := strings.ToLower(sci.prefix)
	sci.suggestions = make([]SlashCommand, 0)

	for _, cmd := range sci.commands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), prefix) || FuzzyMatch(prefix, strings.ToLower(cmd.Name)) {
			sci.suggestions = append(sci.suggestions, cmd)
		}
	}
}

// Suggestions returns the current filtered suggestions.
func (sci *SlashCommandInput) Suggestions() []SlashCommand {
	return sci.suggestions
}

// SetSize sets the dimensions.
func (sci *SlashCommandInput) SetSize(width, height int) {
	sci.width = width
	sci.height = height
}

func (sci *SlashCommandInput) Focus()        { sci.focused = true }
func (sci *SlashCommandInput) Blur()         { sci.focused = false }
func (sci *SlashCommandInput) Focused() bool { return sci.focused }

var _ tea.Model = (*SlashCommandInput)(nil)
