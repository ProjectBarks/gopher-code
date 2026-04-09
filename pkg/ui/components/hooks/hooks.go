// Package hooks provides the hooks configuration browser UI.
//
// Source: components/hooks/HooksConfigMenu.tsx, SelectHookMode.tsx,
//         SelectMatcherMode.tsx, ViewHookMode.tsx
//
// A read-only browser for configured hooks. Users drill down:
// event list → matcher list → hook list → hook detail.
// To modify hooks, they edit settings.json or ask Claude.
package hooks

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	pkghooks "github.com/projectbarks/gopher-code/pkg/hooks"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// DoneMsg is sent when the user closes the hooks browser.
type DoneMsg struct{}

// ViewLevel describes the current drill-down level.
type viewLevel int

const (
	levelEvents   viewLevel = iota // list of hook events
	levelMatchers                  // matchers for a selected event
	levelHooks                     // hooks for a selected event+matcher
	levelDetail                    // detail view for a single hook
)

// Model is the hooks config browser bubbletea model.
type Model struct {
	level     viewLevel
	cursor    int
	event     pkghooks.HookEvent   // selected event
	matcher   string               // selected matcher
	hook      *pkghooks.IndividualHookConfig // selected hook for detail view

	events    []pkghooks.HookEvent
	matchers  []string
	hookList  []pkghooks.IndividualHookConfig
	grouped   map[pkghooks.HookEvent]map[string][]pkghooks.IndividualHookConfig
	metadata  map[pkghooks.HookEvent]pkghooks.HookEventMetadata
}

// New creates a hooks config browser from the given hooks list.
func New(allHooks []pkghooks.IndividualHookConfig, toolNames []string) Model {
	grouped := pkghooks.GroupHooksByEventAndMatcher(allHooks, toolNames)
	metadata := pkghooks.GetHookEventMetadata(toolNames)

	// Events with at least one hook
	var events []pkghooks.HookEvent
	for _, ev := range pkghooks.AllHookEvents {
		if matchers, ok := grouped[ev]; ok && len(matchers) > 0 {
			for _, hooks := range matchers {
				if len(hooks) > 0 {
					events = append(events, ev)
					break
				}
			}
		}
	}

	return Model{
		level:    levelEvents,
		events:   events,
		grouped:  grouped,
		metadata: metadata,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown, 'j':
			if m.cursor < m.listLen()-1 {
				m.cursor++
			}
		case tea.KeyEnter:
			return m.enter()
		case tea.KeyEscape:
			return m.back()
		case 'q':
			if m.level == levelEvents {
				return m, func() tea.Msg { return DoneMsg{} }
			}
			return m.back()
		}
	}
	return m, nil
}

func (m Model) listLen() int {
	switch m.level {
	case levelEvents:
		return len(m.events)
	case levelMatchers:
		return len(m.matchers)
	case levelHooks:
		return len(m.hookList)
	default:
		return 0
	}
}

func (m Model) enter() (Model, tea.Cmd) {
	switch m.level {
	case levelEvents:
		if m.cursor < len(m.events) {
			m.event = m.events[m.cursor]
			matchers := sortedMatchers(m.grouped[m.event])
			if len(matchers) == 1 {
				// Skip matcher level if only one
				m.matcher = matchers[0]
				m.hookList = m.grouped[m.event][m.matcher]
				m.level = levelHooks
			} else {
				m.matchers = matchers
				m.level = levelMatchers
			}
			m.cursor = 0
		}
	case levelMatchers:
		if m.cursor < len(m.matchers) {
			m.matcher = m.matchers[m.cursor]
			m.hookList = m.grouped[m.event][m.matcher]
			m.level = levelHooks
			m.cursor = 0
		}
	case levelHooks:
		if m.cursor < len(m.hookList) {
			h := m.hookList[m.cursor]
			m.hook = &h
			m.level = levelDetail
		}
	}
	return m, nil
}

func (m Model) back() (Model, tea.Cmd) {
	switch m.level {
	case levelEvents:
		return m, func() tea.Msg { return DoneMsg{} }
	case levelMatchers:
		m.level = levelEvents
		m.cursor = 0
	case levelHooks:
		if len(m.matchers) > 1 {
			m.level = levelMatchers
		} else {
			m.level = levelEvents
		}
		m.cursor = 0
	case levelDetail:
		m.level = levelHooks
		m.hook = nil
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)
	keyStyle := lipgloss.NewStyle().Bold(true)

	var b strings.Builder

	switch m.level {
	case levelEvents:
		b.WriteString(titleStyle.Render("Hook Events"))
		b.WriteString("\n\n")
		if len(m.events) == 0 {
			b.WriteString(dimStyle.Render("  No hooks configured.\n"))
			b.WriteString(dimStyle.Render("  Edit settings.json or ask Claude to add hooks.\n"))
		} else {
			for i, ev := range m.events {
				cursor := "  "
				style := lipgloss.NewStyle()
				if i == m.cursor {
					cursor = "> "
					style = selectedStyle
				}
				count := countHooksForEvent(m.grouped[ev])
				b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, style.Render(string(ev)), dimStyle.Render(fmt.Sprintf("(%d hook%s)", count, plural(count)))))
			}
		}

	case levelMatchers:
		b.WriteString(titleStyle.Render(fmt.Sprintf("%s — Matchers", m.event)))
		b.WriteString("\n\n")
		for i, matcher := range m.matchers {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}
			label := matcher
			if label == "" {
				label = "(all)"
			}
			count := len(m.grouped[m.event][matcher])
			b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, style.Render(label), dimStyle.Render(fmt.Sprintf("(%d)", count))))
		}

	case levelHooks:
		title := string(m.event)
		if m.matcher != "" {
			title += " — " + m.matcher
		}
		b.WriteString(titleStyle.Render(title))
		b.WriteString("\n\n")
		if len(m.hookList) == 0 {
			b.WriteString(dimStyle.Render("  No hooks for this matcher.\n"))
		} else {
			for i, hook := range m.hookList {
				cursor := "  "
				style := lipgloss.NewStyle()
				if i == m.cursor {
					cursor = "> "
					style = selectedStyle
				}
				display := hookDisplayText(hook)
				b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, style.Render(display), dimStyle.Render("["+string(hook.Source)+"]")))
			}
		}

	case levelDetail:
		b.WriteString(titleStyle.Render("Hook Detail"))
		b.WriteString("\n\n")
		if m.hook != nil {
			b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Event:"), m.hook.Event))
			if m.hook.Matcher != "" {
				b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Matcher:"), m.hook.Matcher))
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Type:"), m.hook.Config.Type))
			if m.hook.Config.Command != "" {
				b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Command:"), m.hook.Config.Command))
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Source:"), m.hook.Source))
			if m.hook.PluginName != "" {
				b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Plugin:"), m.hook.PluginName))
			}
		}
	}

	b.WriteString("\n")
	if m.level == levelEvents {
		b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter drill down · Esc/q close"))
	} else {
		b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter select · Esc back"))
	}

	return b.String()
}

func hookDisplayText(h pkghooks.IndividualHookConfig) string {
	if h.Config.Command != "" {
		return h.Config.Command
	}
	if h.Config.Type != "" {
		return string(h.Config.Type) + " hook"
	}
	return "hook"
}

func countHooksForEvent(matchers map[string][]pkghooks.IndividualHookConfig) int {
	count := 0
	for _, hooks := range matchers {
		count += len(hooks)
	}
	return count
}

func sortedMatchers(matchers map[string][]pkghooks.IndividualHookConfig) []string {
	result := make([]string, 0, len(matchers))
	for k := range matchers {
		result = append(result, k)
	}
	return result
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
