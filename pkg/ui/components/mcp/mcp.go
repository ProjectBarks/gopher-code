// Package mcp provides MCP server UI components (tool browser, dialogs).
//
// Source: components/mcp/MCPToolListView.tsx, MCPToolDetailView.tsx,
//         MCPReconnect.tsx, CapabilitiesSection.tsx
//
// These are the dialogs shown when browsing MCP server tools via /mcp.
// Tool list → tool detail drill-down, reconnect confirmation, and
// capabilities display.
package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ServerStatus describes an MCP server's connection state.
type ServerStatus string

const (
	StatusConnected    ServerStatus = "connected"
	StatusDisconnected ServerStatus = "disconnected"
	StatusError        ServerStatus = "error"
)

// ServerInfo describes an MCP server for display.
type ServerInfo struct {
	Name         string
	Status       ServerStatus
	ToolCount    int
	Transport    string // "stdio", "sse", "http"
	Command      string // for stdio servers
	URL          string // for remote servers
	Capabilities []string
}

// ToolInfo describes an MCP tool for display.
type ToolInfo struct {
	Name         string
	DisplayName  string
	Description  string
	IsReadOnly   bool
	IsDestructive bool
	InputSchema  json.RawMessage
}

// ToolSelectedMsg is sent when the user selects a tool to view details.
type ToolSelectedMsg struct {
	Index int
}

// BackMsg is sent when the user navigates back.
type BackMsg struct{}

// ReconnectMsg is sent when the user requests a server reconnection.
type ReconnectMsg struct {
	ServerName string
}

// DoneMsg is sent when the user closes the dialog.
type DoneMsg struct{}

// ---------------------------------------------------------------------------
// Tool List View — Source: MCPToolListView.tsx
// ---------------------------------------------------------------------------

// ToolListModel shows the tools provided by an MCP server.
type ToolListModel struct {
	Server ServerInfo
	Tools  []ToolInfo
	cursor int
}

// NewToolListModel creates a tool list for a server.
func NewToolListModel(server ServerInfo, tools []ToolInfo) ToolListModel {
	return ToolListModel{Server: server, Tools: tools}
}

func (m ToolListModel) Init() tea.Cmd { return nil }

func (m ToolListModel) Update(msg tea.Msg) (ToolListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown, 'j':
			if m.cursor < len(m.Tools)-1 {
				m.cursor++
			}
		case tea.KeyEnter:
			if m.cursor < len(m.Tools) {
				idx := m.cursor
				return m, func() tea.Msg { return ToolSelectedMsg{Index: idx} }
			}
		case tea.KeyEscape:
			return m, func() tea.Msg { return BackMsg{} }
		}
	}
	return m, nil
}

func (m ToolListModel) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s — Tools (%d)", m.Server.Name, len(m.Tools))))
	b.WriteString("\n\n")

	if len(m.Tools) == 0 {
		b.WriteString(dimStyle.Render("  No tools available"))
		b.WriteString("\n")
	} else {
		for i, tool := range m.Tools {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}
			name := tool.DisplayName
			if name == "" {
				name = tool.Name
			}

			b.WriteString(cursor)
			b.WriteString(style.Render(name))

			annotations := toolAnnotations(tool)
			if annotations != "" {
				b.WriteString(" ")
				b.WriteString(dimStyle.Render(annotations))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter view details · Esc back"))
	return b.String()
}

func toolAnnotations(t ToolInfo) string {
	var parts []string
	if t.IsReadOnly {
		parts = append(parts, "read-only")
	}
	if t.IsDestructive {
		parts = append(parts, "destructive")
	}
	if len(parts) == 0 {
		return ""
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// ---------------------------------------------------------------------------
// Tool Detail View — Source: MCPToolDetailView.tsx
// ---------------------------------------------------------------------------

// ToolDetailModel shows details of a single MCP tool.
type ToolDetailModel struct {
	Tool   ToolInfo
	Server ServerInfo
}

func (m ToolDetailModel) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	keyStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error))

	name := m.Tool.DisplayName
	if name == "" {
		name = m.Tool.Name
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(name))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Server:"), m.Server.Name))
	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Tool:"), m.Tool.Name))

	if m.Tool.IsReadOnly {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Access:"), successStyle.Render("read-only")))
	} else if m.Tool.IsDestructive {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Access:"), errStyle.Render("destructive")))
	}

	if m.Tool.Description != "" {
		b.WriteString(fmt.Sprintf("\n  %s\n", keyStyle.Render("Description:")))
		for _, line := range strings.Split(m.Tool.Description, "\n") {
			b.WriteString("  " + dimStyle.Render(line) + "\n")
		}
	}

	if len(m.Tool.InputSchema) > 0 {
		b.WriteString(fmt.Sprintf("\n  %s\n", keyStyle.Render("Input Schema:")))
		// Pretty-print JSON schema
		var pretty json.RawMessage
		if json.Unmarshal(m.Tool.InputSchema, &pretty) == nil {
			formatted, err := json.MarshalIndent(pretty, "  ", "  ")
			if err == nil {
				b.WriteString("  " + dimStyle.Render(string(formatted)) + "\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Esc back"))
	return b.String()
}

// ---------------------------------------------------------------------------
// Reconnect Dialog — Source: MCPReconnect.tsx
// ---------------------------------------------------------------------------

// ReconnectModel shows a reconnection prompt for a disconnected server.
type ReconnectModel struct {
	Server ServerInfo
	cursor int // 0=reconnect, 1=cancel
}

func NewReconnectModel(server ServerInfo) ReconnectModel {
	return ReconnectModel{Server: server}
}

func (m ReconnectModel) Update(msg tea.Msg) (ReconnectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			m.cursor = 0
		case tea.KeyDown, 'j':
			m.cursor = 1
		case tea.KeyEnter:
			if m.cursor == 0 {
				name := m.Server.Name
				return m, func() tea.Msg { return ReconnectMsg{ServerName: name} }
			}
			return m, func() tea.Msg { return BackMsg{} }
		case tea.KeyEscape:
			return m, func() tea.Msg { return BackMsg{} }
		}
	}
	return m, nil
}

func (m ReconnectModel) View() string {
	colors := theme.Current().Colors()
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(warnStyle.Render(fmt.Sprintf("⚠ %s is disconnected", m.Server.Name)))
	b.WriteString("\n\n")

	if m.Server.Transport != "" {
		b.WriteString(fmt.Sprintf("  Transport: %s\n", m.Server.Transport))
	}
	if m.Server.Command != "" {
		b.WriteString(fmt.Sprintf("  Command: %s\n", m.Server.Command))
	}
	if m.Server.URL != "" {
		b.WriteString(fmt.Sprintf("  URL: %s\n", m.Server.URL))
	}
	b.WriteString("\n")

	options := []string{"Reconnect", "Cancel"}
	for i, opt := range options {
		if i == m.cursor {
			b.WriteString("  > " + selectedStyle.Render(opt) + "\n")
		} else {
			b.WriteString("    " + dimStyle.Render(opt) + "\n")
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Capabilities Section — Source: CapabilitiesSection.tsx
// ---------------------------------------------------------------------------

// RenderCapabilities returns a formatted string of server capabilities.
func RenderCapabilities(caps []string) string {
	if len(caps) == 0 {
		return ""
	}
	colors := theme.Current().Colors()
	keyStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)
	checkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))

	var b strings.Builder
	b.WriteString(keyStyle.Render("Capabilities:"))
	b.WriteString("\n")
	for _, cap := range caps {
		b.WriteString(fmt.Sprintf("  %s %s\n", checkStyle.Render("✓"), dimStyle.Render(cap)))
	}
	return b.String()
}
