package components

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
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
	Source      string // "builtin", "user", "project", "skill"
}

// DefaultSlashCommands returns the built-in slash commands.
// Source: claude-code-source-build/source/src/commands/ — the full built-in set.
func DefaultSlashCommands() []SlashCommand {
	return []SlashCommand{
		{Name: "/help", Description: "Show available commands", Handler: "help", Source: "builtin"},
		{Name: "/clear", Description: "Clear conversation history", Handler: "clear", Source: "builtin"},
		{Name: "/compact", Description: "Compact conversation to free context", Handler: "compact", Source: "builtin"},
		{Name: "/model", Description: "Switch AI model", Handler: "model", Source: "builtin"},
		{Name: "/session", Description: "Switch or manage session", Handler: "session", Source: "builtin"},
		{Name: "/resume", Description: "Resume a previous conversation", Handler: "resume", Source: "builtin"},
		{Name: "/thinking", Description: "Toggle extended thinking", Handler: "thinking", Source: "builtin"},
		{Name: "/effort", Description: "Set thinking effort (low/medium/high/max)", Handler: "effort", Source: "builtin"},
		{Name: "/fast", Description: "Toggle fast output mode", Handler: "fast", Source: "builtin"},
		{Name: "/cost", Description: "Show session cost and token usage", Handler: "cost", Source: "builtin"},
		{Name: "/status", Description: "Show session status", Handler: "status", Source: "builtin"},
		{Name: "/context", Description: "Show context window usage", Handler: "context", Source: "builtin"},
		{Name: "/config", Description: "View or edit configuration", Handler: "config", Source: "builtin"},
		{Name: "/doctor", Description: "Diagnose environment issues", Handler: "doctor", Source: "builtin"},
		{Name: "/theme", Description: "Switch UI theme", Handler: "theme", Source: "builtin"},
		{Name: "/login", Description: "Authenticate with provider", Handler: "login", Source: "builtin"},
		{Name: "/logout", Description: "Sign out", Handler: "logout", Source: "builtin"},
		{Name: "/mcp", Description: "Manage MCP servers", Handler: "mcp", Source: "builtin"},
		{Name: "/hooks", Description: "Manage hooks", Handler: "hooks", Source: "builtin"},
		{Name: "/permissions", Description: "Manage tool permissions", Handler: "permissions", Source: "builtin"},
		{Name: "/skills", Description: "List available skills", Handler: "skills", Source: "builtin"},
		{Name: "/agents", Description: "List available agents", Handler: "agents", Source: "builtin"},
		{Name: "/memory", Description: "View or edit memory", Handler: "memory", Source: "builtin"},
		{Name: "/init", Description: "Initialize project", Handler: "init", Source: "builtin"},
		{Name: "/review", Description: "Review code changes", Handler: "review", Source: "builtin"},
		{Name: "/commit", Description: "Create a git commit", Handler: "commit", Source: "builtin"},
		{Name: "/diff", Description: "Show git diff", Handler: "diff", Source: "builtin"},
		{Name: "/plan", Description: "Enter planning mode", Handler: "plan", Source: "builtin"},
		{Name: "/rewind", Description: "Rewind to a previous turn", Handler: "rewind", Source: "builtin"},
		{Name: "/export", Description: "Export conversation", Handler: "export", Source: "builtin"},
		{Name: "/copy", Description: "Copy last response", Handler: "copy", Source: "builtin"},
		{Name: "/vim", Description: "Toggle vim input mode", Handler: "vim", Source: "builtin"},
		{Name: "/keybindings", Description: "Show keybindings", Handler: "keybindings", Source: "builtin"},
		{Name: "/release-notes", Description: "Show release notes", Handler: "release-notes", Source: "builtin"},
		{Name: "/version", Description: "Show version", Handler: "version", Source: "builtin"},
		{Name: "/quit", Description: "Exit gopher", Handler: "quit", Source: "builtin"},
	}
}

// LoadSlashCommands returns built-ins plus user commands, project commands,
// and skills discovered on disk (matching Claude Code's command sources).
//   - ~/.claude/commands/*.md           → user commands
//   - <cwd>/.claude/commands/*.md       → project commands
//   - ~/.claude/skills/<n>/SKILL.md     → user skills
//   - <cwd>/.claude/skills/<n>/SKILL.md → project skills
func LoadSlashCommands(cwd string) []SlashCommand {
	cmds := DefaultSlashCommands()
	seen := make(map[string]bool, len(cmds))
	for _, c := range cmds {
		seen[c.Name] = true
	}

	home, _ := os.UserHomeDir()
	cmdDirs := []struct{ path, source string }{
		{filepath.Join(home, ".claude", "commands"), "user"},
		{filepath.Join(cwd, ".claude", "commands"), "project"},
	}
	for _, d := range cmdDirs {
		for _, sc := range loadCommandsFromDir(d.path, d.source) {
			if seen[sc.Name] {
				continue
			}
			seen[sc.Name] = true
			cmds = append(cmds, sc)
		}
	}

	skillDirs := []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(cwd, ".claude", "skills"),
	}
	for _, dir := range skillDirs {
		for _, sc := range loadSkillsFromDir(dir) {
			if seen[sc.Name] {
				continue
			}
			seen[sc.Name] = true
			cmds = append(cmds, sc)
		}
	}

	// Keep built-ins first (declaration order); sort everything after.
	builtinCount := len(DefaultSlashCommands())
	tail := cmds[builtinCount:]
	sort.Slice(tail, func(i, j int) bool { return tail[i].Name < tail[j].Name })
	return cmds
}

// loadCommandsFromDir scans dir for *.md files, each of which becomes a
// slash command named after the filename (sans .md). The description is the
// first non-empty non-frontmatter line of the file.
func loadCommandsFromDir(dir, source string) []SlashCommand {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []SlashCommand
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		out = append(out, SlashCommand{
			Name:        "/" + name,
			Description: readFirstDescription(filepath.Join(dir, entry.Name())),
			Handler:     name,
			Source:      source,
		})
	}
	return out
}

// loadSkillsFromDir scans dir for <skill>/SKILL.md files, returning each as
// a slash command. Description comes from the YAML frontmatter's
// `description:` field (supports folded scalars).
func loadSkillsFromDir(dir string) []SlashCommand {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []SlashCommand
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			continue
		}
		out = append(out, SlashCommand{
			Name:        "/" + entry.Name(),
			Description: readSkillDescription(skillFile),
			Handler:     entry.Name(),
			Source:      "skill",
		})
	}
	return out
}

// readFirstDescription returns the first non-empty, non-frontmatter,
// non-heading line of a markdown file, truncated for display.
func readFirstDescription(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > 70 {
			line = line[:67] + "…"
		}
		return line
	}
	return ""
}

// readSkillDescription extracts the `description:` field from the YAML
// frontmatter of a SKILL.md file. Handles folded scalar continuations.
func readSkillDescription(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	collecting := false
	var desc strings.Builder
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "---" {
			if inFrontmatter {
				break
			}
			inFrontmatter = true
			continue
		}
		if !inFrontmatter {
			continue
		}
		if strings.HasPrefix(line, "description:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			if val == "" || val == ">" || val == ">-" || val == "|" {
				collecting = true
				continue
			}
			desc.WriteString(val)
			break
		}
		if collecting {
			if len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t') {
				if desc.Len() > 0 {
					desc.WriteString(" ")
				}
				desc.WriteString(line)
			} else {
				break
			}
		}
	}
	result := desc.String()
	if len(result) > 70 {
		result = result[:67] + "…"
	}
	return result
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
