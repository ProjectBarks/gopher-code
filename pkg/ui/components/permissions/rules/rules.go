// Package rules provides the permission rule management UI.
// Source: components/permissions/rules/PermissionRuleList.tsx
//
// Tabbed view of permission rules: Recent denials, Allow, Ask, Deny.
// Users can view, add, and remove rules across scopes.
package rules

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Tab identifies which rule tab is active.
type Tab int

const (
	TabRecent Tab = iota
	TabAllow
	TabAsk
	TabDeny
)

// Rule represents a single permission rule.
type Rule struct {
	Pattern  string // e.g., "Bash(git:*)", "Edit(*)"
	Source   string // "userSettings", "projectSettings", etc.
	Behavior string // "allow", "ask", "deny"
}

// RuleDismissedMsg signals the rule list was closed.
type RuleDismissedMsg struct{}

// Model is the permission rule management screen.
type Model struct {
	tab      Tab
	rules    map[Tab][]Rule
	selected int
	scroll   int
	width    int
}

// New creates a permission rule list with categorized rules.
func New(allowRules, askRules, denyRules, recentDenials []Rule, width int) Model {
	return Model{
		tab: TabRecent,
		rules: map[Tab][]Rule{
			TabRecent: recentDenials,
			TabAllow:  allowRules,
			TabAsk:    askRules,
			TabDeny:   denyRules,
		},
		width: width,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEscape, 'q':
			return m, func() tea.Msg { return RuleDismissedMsg{} }
		case tea.KeyTab:
			m.tab = (m.tab + 1) % 4
			m.selected = 0
			m.scroll = 0
		case tea.KeyUp, 'k':
			if m.selected > 0 {
				m.selected--
			}
		case tea.KeyDown, 'j':
			rules := m.currentRules()
			if m.selected < len(rules)-1 {
				m.selected++
			}
		}
	}
	return m, nil
}

func (m Model) currentRules() []Rule {
	return m.rules[m.tab]
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	activeTab := lipgloss.NewStyle().Bold(true).Underline(true).Padding(0, 1)
	inactiveTab := lipgloss.NewStyle().Faint(true).Padding(0, 1)
	ruleStyle := lipgloss.NewStyle()
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	sourceStyle := lipgloss.NewStyle().Faint(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Permission Rules"))
	sb.WriteString("\n\n")

	// Tab bar
	tabs := []struct {
		label string
		tab   Tab
	}{
		{"Recent", TabRecent},
		{"Allow", TabAllow},
		{"Ask", TabAsk},
		{"Deny", TabDeny},
	}
	for _, t := range tabs {
		style := inactiveTab
		if t.tab == m.tab {
			style = activeTab
		}
		count := len(m.rules[t.tab])
		sb.WriteString(style.Render(fmt.Sprintf("%s (%d)", t.label, count)))
		sb.WriteString(" ")
	}
	sb.WriteString("\n\n")

	// Rules list
	rules := m.currentRules()
	if len(rules) == 0 {
		switch m.tab {
		case TabRecent:
			sb.WriteString(dimStyle.Render("No recent denials"))
		case TabAllow:
			sb.WriteString(dimStyle.Render("No allow rules configured"))
		case TabAsk:
			sb.WriteString(dimStyle.Render("No ask rules configured"))
		case TabDeny:
			sb.WriteString(dimStyle.Render("No deny rules configured"))
		}
	} else {
		maxShow := 10
		start := m.scroll
		end := start + maxShow
		if end > len(rules) {
			end = len(rules)
		}

		for i := start; i < end; i++ {
			rule := rules[i]
			cursor := "  "
			style := ruleStyle
			if i == m.selected {
				cursor = "> "
				style = selStyle
			}
			sb.WriteString(cursor + style.Render(rule.Pattern))
			sb.WriteString("  " + sourceStyle.Render("("+rule.Source+")"))
			sb.WriteString("\n")
		}

		if end < len(rules) {
			sb.WriteString(dimStyle.Render(fmt.Sprintf("  ... and %d more", len(rules)-end)))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("Tab to switch, ↑/↓ to navigate, Escape to close"))

	return sb.String()
}
