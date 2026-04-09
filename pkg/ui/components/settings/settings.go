// Package settings provides the settings dialog UI component.
//
// Source: components/Settings/Settings.tsx, Status.tsx, Config.tsx, Usage.tsx
//
// In TS, Settings is a tabbed Pane with Status/Config/Usage/Gates tabs.
// In Go, it's a bubbletea model with tab switching and content rendering.
package settings

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// Tab identifies a settings tab.
type Tab string

const (
	TabStatus Tab = "Status"
	TabConfig Tab = "Config"
	TabUsage  Tab = "Usage"
)

// AllTabs is the ordered list of tabs.
var AllTabs = []Tab{TabStatus, TabConfig, TabUsage}

// Property is a key-value pair displayed in the Status tab.
// Source: utils/status.ts — Property type
type Property struct {
	Label string
	Value string
}

// Diagnostic is a health check result shown in the Status tab.
// Source: utils/status.ts — Diagnostic type
type Diagnostic struct {
	Name    string
	Status  string // "ok", "warning", "error"
	Message string
}

// UsageInfo shows plan usage data for the Usage tab.
type UsageInfo struct {
	PlanName     string
	UsedPercent  float64
	ResetDate    string
	TotalTokens  int64
	SessionTokens int64
}

// ClosedMsg is sent when the user closes the settings dialog.
type ClosedMsg struct {
	Result string
}

// StatusData holds all data for the Status tab.
type StatusData struct {
	Primary     []Property
	Secondary   []Property
	Diagnostics []Diagnostic
}

// ConfigEntry is a single setting in the Config tab.
type ConfigEntry struct {
	Key         string
	Value       string
	Source      string // "user", "project", "default"
	Description string
}

// Model is the settings dialog bubbletea model.
type Model struct {
	tab         Tab
	tabIndex    int
	status      StatusData
	configs     []ConfigEntry
	usage       *UsageInfo
	width       int
	height      int
}

// New creates a settings model with the given default tab.
func New(defaultTab Tab, status StatusData, configs []ConfigEntry, usage *UsageInfo) Model {
	tabIndex := 0
	for i, t := range AllTabs {
		if t == defaultTab {
			tabIndex = i
			break
		}
	}
	return Model{
		tab:      defaultTab,
		tabIndex: tabIndex,
		status:   status,
		configs:  configs,
		usage:    usage,
		width:    80,
		height:   24,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyTab:
			m.tabIndex = (m.tabIndex + 1) % len(AllTabs)
			m.tab = AllTabs[m.tabIndex]
		case tea.KeyEscape, 'q':
			return m, func() tea.Msg {
				return ClosedMsg{Result: "Settings dialog dismissed"}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	dimStyle := lipgloss.NewStyle().Faint(true)
	tabActiveStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.TabActive))

	var b strings.Builder

	// Tab bar
	for i, t := range AllTabs {
		if i > 0 {
			b.WriteString("  ")
		}
		if i == m.tabIndex {
			b.WriteString(tabActiveStyle.Render("[" + string(t) + "]"))
		} else {
			b.WriteString(dimStyle.Render(" " + string(t) + " "))
		}
	}
	b.WriteString("\n\n")

	// Tab content
	switch m.tab {
	case TabStatus:
		b.WriteString(m.viewStatus(titleStyle, dimStyle))
	case TabConfig:
		b.WriteString(m.viewConfig(titleStyle, dimStyle))
	case TabUsage:
		b.WriteString(m.viewUsage(titleStyle, dimStyle))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Tab switch · Esc close"))

	return b.String()
}

func (m Model) viewStatus(titleStyle, dimStyle lipgloss.Style) string {
	colors := theme.Current().Colors()
	var b strings.Builder

	// Primary properties
	if len(m.status.Primary) > 0 {
		maxLabel := 0
		for _, p := range m.status.Primary {
			if len(p.Label) > maxLabel {
				maxLabel = len(p.Label)
			}
		}
		for _, p := range m.status.Primary {
			b.WriteString(fmt.Sprintf("  %-*s  %s\n", maxLabel, p.Label, p.Value))
		}
	}

	// Secondary properties
	if len(m.status.Secondary) > 0 {
		b.WriteString("\n")
		maxLabel := 0
		for _, p := range m.status.Secondary {
			if len(p.Label) > maxLabel {
				maxLabel = len(p.Label)
			}
		}
		for _, p := range m.status.Secondary {
			b.WriteString(fmt.Sprintf("  %-*s  %s\n", maxLabel, p.Label, p.Value))
		}
	}

	// Diagnostics
	if len(m.status.Diagnostics) > 0 {
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("  Diagnostics"))
		b.WriteString("\n")
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error))

		for _, d := range m.status.Diagnostics {
			var icon string
			switch d.Status {
			case "ok":
				icon = successStyle.Render("✓")
			case "warning":
				icon = warnStyle.Render("⚠")
			case "error":
				icon = errStyle.Render("✗")
			default:
				icon = "·"
			}
			b.WriteString(fmt.Sprintf("  %s %s", icon, d.Name))
			if d.Message != "" {
				b.WriteString(": " + dimStyle.Render(d.Message))
			}
			b.WriteString("\n")
		}
	}

	if len(m.status.Primary) == 0 && len(m.status.Diagnostics) == 0 {
		b.WriteString(dimStyle.Render("  No status data available"))
	}

	return b.String()
}

func (m Model) viewConfig(titleStyle, dimStyle lipgloss.Style) string {
	var b strings.Builder

	if len(m.configs) == 0 {
		b.WriteString(dimStyle.Render("  No configuration settings"))
		return b.String()
	}

	colors := theme.Current().Colors()
	keyStyle := lipgloss.NewStyle().Bold(true)
	sourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Info))

	for _, cfg := range m.configs {
		b.WriteString("  ")
		b.WriteString(keyStyle.Render(cfg.Key))
		b.WriteString(": ")
		b.WriteString(cfg.Value)
		if cfg.Source != "" && cfg.Source != "default" {
			b.WriteString(" ")
			b.WriteString(sourceStyle.Render("[" + cfg.Source + "]"))
		}
		b.WriteString("\n")
		if cfg.Description != "" {
			b.WriteString("    ")
			b.WriteString(dimStyle.Render(cfg.Description))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) viewUsage(titleStyle, dimStyle lipgloss.Style) string {
	var b strings.Builder

	if m.usage == nil {
		b.WriteString(dimStyle.Render("  No usage data available"))
		return b.String()
	}

	colors := theme.Current().Colors()

	b.WriteString(titleStyle.Render("  Plan Usage"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  Plan:           %s\n", m.usage.PlanName))
	b.WriteString(fmt.Sprintf("  Usage:          %.1f%%\n", m.usage.UsedPercent))
	if m.usage.ResetDate != "" {
		b.WriteString(fmt.Sprintf("  Resets:         %s\n", m.usage.ResetDate))
	}

	// Usage bar
	barWidth := 30
	filled := int(m.usage.UsedPercent / 100.0 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	barColor := colors.Success
	if m.usage.UsedPercent > 80 {
		barColor = colors.Warning
	}
	if m.usage.UsedPercent > 95 {
		barColor = colors.Error
	}
	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(barColor))
	bar := barStyle.Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", barWidth-filled))
	b.WriteString(fmt.Sprintf("\n  [%s] %.1f%%\n", bar, m.usage.UsedPercent))

	if m.usage.SessionTokens > 0 {
		b.WriteString(fmt.Sprintf("\n  Session tokens: %s\n", formatTokens(m.usage.SessionTokens)))
	}
	if m.usage.TotalTokens > 0 {
		b.WriteString(fmt.Sprintf("  Total tokens:   %s\n", formatTokens(m.usage.TotalTokens)))
	}

	return b.String()
}

// formatTokens formats a token count with K/M suffixes.
func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// CurrentTab returns the active tab.
func (m *Model) CurrentTab() Tab { return m.tab }
