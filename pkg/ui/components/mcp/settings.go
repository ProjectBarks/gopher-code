package mcp

// Source: components/mcp/MCPSettings.tsx, MCPListPanel.tsx
//
// MCPSettings is the main /mcp command view. Shows servers grouped by scope
// (project/user/local/enterprise), with drill-down to server detail, tool
// list, and tool detail. Manages view state transitions.

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ConfigScope identifies where an MCP server config comes from.
type ConfigScope string

const (
	ScopeProject    ConfigScope = "project"
	ScopeUser       ConfigScope = "user"
	ScopeLocal      ConfigScope = "local"
	ScopeEnterprise ConfigScope = "enterprise"
	ScopeDynamic    ConfigScope = "dynamic"
)

// ScopeLabel returns a human-readable label for a config scope.
func ScopeLabel(scope ConfigScope) string {
	switch scope {
	case ScopeProject:
		return "Project MCPs"
	case ScopeUser:
		return "User MCPs"
	case ScopeLocal:
		return "Local MCPs"
	case ScopeEnterprise:
		return "Enterprise MCPs"
	case ScopeDynamic:
		return "Built-in MCPs"
	default:
		return string(scope)
	}
}

// viewLevel describes the current drill-down level in MCPSettings.
type settingsLevel int

const (
	settingsLevelList       settingsLevel = iota // server list
	settingsLevelServer                          // server detail
	settingsLevelToolList                        // tool list for a server
	settingsLevelToolDetail                      // tool detail
)

// SettingsClosedMsg is sent when the user closes the MCP settings.
type SettingsClosedMsg struct{}

// SettingsModel is the main /mcp command UI model.
type SettingsModel struct {
	level          settingsLevel
	servers        []ServerInfo
	tools          []ToolInfo // tools for the selected server
	cursor         int
	selectedServer *ServerInfo
	selectedTool   *ToolInfo
	width          int
}

// NewSettingsModel creates the MCP settings view from a server list.
func NewSettingsModel(servers []ServerInfo) SettingsModel {
	return SettingsModel{
		level:   settingsLevelList,
		servers: servers,
		width:   80,
	}
}

func (m SettingsModel) Init() tea.Cmd { return nil }

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch m.level {
		case settingsLevelList:
			return m.updateList(msg)
		case settingsLevelServer:
			return m.updateServerDetail(msg)
		case settingsLevelToolList:
			return m.updateToolList(msg)
		case settingsLevelToolDetail:
			return m.updateToolDetail(msg)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, nil
}

func (m SettingsModel) updateList(msg tea.KeyPressMsg) (SettingsModel, tea.Cmd) {
	switch msg.Code {
	case tea.KeyUp, 'k':
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown, 'j':
		if m.cursor < len(m.servers)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		if m.cursor < len(m.servers) {
			srv := m.servers[m.cursor]
			m.selectedServer = &srv
			m.level = settingsLevelServer
			m.cursor = 0
		}
	case tea.KeyEscape, 'q':
		return m, func() tea.Msg { return SettingsClosedMsg{} }
	}
	return m, nil
}

func (m SettingsModel) updateServerDetail(msg tea.KeyPressMsg) (SettingsModel, tea.Cmd) {
	switch msg.Code {
	case 't': // view tools
		if m.selectedServer != nil {
			m.level = settingsLevelToolList
			m.cursor = 0
		}
	case tea.KeyEscape, 'q':
		m.level = settingsLevelList
		m.cursor = 0
		m.selectedServer = nil
	}
	return m, nil
}

func (m SettingsModel) updateToolList(msg tea.KeyPressMsg) (SettingsModel, tea.Cmd) {
	switch msg.Code {
	case tea.KeyUp, 'k':
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown, 'j':
		if m.cursor < len(m.tools)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		if m.cursor < len(m.tools) {
			tool := m.tools[m.cursor]
			m.selectedTool = &tool
			m.level = settingsLevelToolDetail
		}
	case tea.KeyEscape:
		m.level = settingsLevelServer
		m.cursor = 0
	}
	return m, nil
}

func (m SettingsModel) updateToolDetail(msg tea.KeyPressMsg) (SettingsModel, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		m.level = settingsLevelToolList
		m.selectedTool = nil
	}
	return m, nil
}

// SetTools sets the tool list for the selected server.
func (m *SettingsModel) SetTools(tools []ToolInfo) {
	m.tools = tools
}

func (m SettingsModel) View() string {
	switch m.level {
	case settingsLevelServer:
		return m.viewServerDetail()
	case settingsLevelToolList:
		tl := NewToolListModel(*m.selectedServer, m.tools)
		tl.cursor = m.cursor
		return tl.View()
	case settingsLevelToolDetail:
		if m.selectedTool != nil && m.selectedServer != nil {
			td := ToolDetailModel{Tool: *m.selectedTool, Server: *m.selectedServer}
			return td.View()
		}
		return ""
	default:
		return m.viewServerList()
	}
}

func (m SettingsModel) viewServerList() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)
	scopeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Info))

	var b strings.Builder
	b.WriteString(titleStyle.Render("MCP Servers"))
	b.WriteString(dimStyle.Render(fmt.Sprintf(" (%d servers)", len(m.servers))))
	b.WriteString("\n\n")

	if len(m.servers) == 0 {
		b.WriteString(dimStyle.Render("  No MCP servers configured.\n"))
		b.WriteString(dimStyle.Render("  Add servers to .claude/settings.json or ~/.claude/settings.json\n"))
	} else {
		// Group by scope
		scopes := []ConfigScope{ScopeProject, ScopeLocal, ScopeUser, ScopeEnterprise, ScopeDynamic}
		globalIdx := 0
		for _, scope := range scopes {
			var scopeServers []int
			for i, srv := range m.servers {
				if srv.Transport == string(scope) || (scope == ScopeDynamic && srv.Transport == "built-in") {
					scopeServers = append(scopeServers, i)
				}
			}

			// If no scope-based grouping, just list them all flat
			if len(scopeServers) == 0 && scope == ScopeProject {
				// Flat list for simplicity
				for i, srv := range m.servers {
					m.renderServerLine(&b, i, srv, i == m.cursor, selectedStyle, dimStyle, colors)
					globalIdx++
				}
				break
			}

			if len(scopeServers) > 0 {
				b.WriteString(scopeStyle.Render("  " + ScopeLabel(scope)))
				b.WriteString("\n")
				for _, i := range scopeServers {
					m.renderServerLine(&b, globalIdx, m.servers[i], globalIdx == m.cursor, selectedStyle, dimStyle, colors)
					globalIdx++
				}
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter view server · Esc/q close"))
	return b.String()
}

func (m SettingsModel) renderServerLine(b *strings.Builder, _ int, srv ServerInfo, isCursor bool, selectedStyle, dimStyle lipgloss.Style, colors theme.ColorScheme) {
	cursor := "    "
	style := lipgloss.NewStyle()
	if isCursor {
		cursor = "  > "
		style = selectedStyle
	}

	// Status icon
	var statusIcon string
	switch srv.Status {
	case StatusConnected:
		statusIcon = lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success)).Render("●")
	case StatusDisconnected:
		statusIcon = lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning)).Render("○")
	case StatusError:
		statusIcon = lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error)).Render("✗")
	default:
		statusIcon = "·"
	}

	b.WriteString(fmt.Sprintf("%s%s %s", cursor, statusIcon, style.Render(srv.Name)))
	if srv.ToolCount > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf(" (%d tools)", srv.ToolCount)))
	}
	b.WriteString("\n")
}

func (m SettingsModel) viewServerDetail() string {
	if m.selectedServer == nil {
		return ""
	}
	srv := m.selectedServer

	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	keyStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render(srv.Name))
	b.WriteString("\n\n")

	// Status
	var statusStr string
	switch srv.Status {
	case StatusConnected:
		statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success)).Render("connected")
	case StatusDisconnected:
		statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning)).Render("disconnected")
	case StatusError:
		statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error)).Render("error")
	}
	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Status:"), statusStr))

	if srv.Transport != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Transport:"), srv.Transport))
	}
	if srv.Command != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Command:"), srv.Command))
	}
	if srv.URL != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("URL:"), srv.URL))
	}
	b.WriteString(fmt.Sprintf("  %s %d\n", keyStyle.Render("Tools:"), srv.ToolCount))

	// Capabilities
	if len(srv.Capabilities) > 0 {
		b.WriteString("\n")
		b.WriteString(RenderCapabilities(srv.Capabilities))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  t view tools · Esc back"))
	return b.String()
}
