package core

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// sizing describes how a child component is sized within a layout.
type sizing int

const (
	sizingDefault  sizing = iota // Equal share of remaining space
	sizingFixed                  // Exact pixel size
	sizingFlexible               // Weighted share of remaining space
)

// childEntry wraps a component with its layout sizing info.
type childEntry struct {
	component  Component
	sizing     sizing
	fixedSize  int // For sizingFixed: exact size in rows/cols
	flexWeight int // For sizingFlexible: relative weight
}

// VerticalStack lays out children top-to-bottom, distributing height.
// All children get the full width of the stack.
type VerticalStack struct {
	children []childEntry
	width    int
	height   int
}

// NewVerticalStack creates a vertical stack with the given children.
// Children added this way get equal flexible sizing (weight 1).
func NewVerticalStack(components ...Component) *VerticalStack {
	vs := &VerticalStack{}
	for _, c := range components {
		vs.children = append(vs.children, childEntry{
			component:  c,
			sizing:     sizingDefault,
			flexWeight: 1,
		})
	}
	return vs
}

// Add appends a component with default (equal flex) sizing.
func (vs *VerticalStack) Add(c Component) {
	vs.children = append(vs.children, childEntry{
		component:  c,
		sizing:     sizingDefault,
		flexWeight: 1,
	})
}

// AddFixed appends a component with a fixed height.
func (vs *VerticalStack) AddFixed(c Component, height int) {
	vs.children = append(vs.children, childEntry{
		component: c,
		sizing:    sizingFixed,
		fixedSize: height,
	})
}

// AddFlexible appends a component with a weighted flexible height.
func (vs *VerticalStack) AddFlexible(c Component, weight int) {
	vs.children = append(vs.children, childEntry{
		component:  c,
		sizing:     sizingFlexible,
		flexWeight: weight,
	})
}

// SetSize distributes width and height to children.
// Fixed children get their exact height; remaining space is split
// among flexible/default children by weight.
func (vs *VerticalStack) SetSize(width, height int) {
	vs.width = width
	vs.height = height

	if len(vs.children) == 0 {
		return
	}

	// Sum fixed heights and total flex weight.
	fixedTotal := 0
	flexTotal := 0
	for _, c := range vs.children {
		switch c.sizing {
		case sizingFixed:
			fixedTotal += c.fixedSize
		default:
			flexTotal += c.flexWeight
		}
	}

	remaining := height - fixedTotal
	if remaining < 0 {
		remaining = 0
	}

	for _, c := range vs.children {
		var h int
		switch c.sizing {
		case sizingFixed:
			h = c.fixedSize
		default:
			if flexTotal > 0 {
				h = remaining * c.flexWeight / flexTotal
			}
		}
		c.component.SetSize(width, h)
	}
}

// Init initializes all children, batching their commands.
func (vs *VerticalStack) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, c := range vs.children {
		if cmd := c.component.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// Update routes messages to all children.
func (vs *VerticalStack) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for i, c := range vs.children {
		model, cmd := c.component.Update(msg)
		if comp, ok := model.(Component); ok {
			vs.children[i].component = comp
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return vs, tea.Batch(cmds...)
}

// View renders children top-to-bottom separated by newlines.
func (vs *VerticalStack) View() tea.View {
	views := make([]string, 0, len(vs.children))
	for _, c := range vs.children {
		views = append(views, c.component.View().Content)
	}
	return tea.NewView(strings.Join(views, "\n"))
}

// HorizontalStack lays out children left-to-right, distributing width.
// All children get the full height of the stack.
type HorizontalStack struct {
	children []childEntry
	width    int
	height   int
}

// NewHorizontalStack creates a horizontal stack with the given children.
// Children added this way get equal flexible sizing (weight 1).
func NewHorizontalStack(components ...Component) *HorizontalStack {
	hs := &HorizontalStack{}
	for _, c := range components {
		hs.children = append(hs.children, childEntry{
			component:  c,
			sizing:     sizingDefault,
			flexWeight: 1,
		})
	}
	return hs
}

// Add appends a component with default (equal flex) sizing.
func (hs *HorizontalStack) Add(c Component) {
	hs.children = append(hs.children, childEntry{
		component:  c,
		sizing:     sizingDefault,
		flexWeight: 1,
	})
}

// AddFixed appends a component with a fixed width.
func (hs *HorizontalStack) AddFixed(c Component, width int) {
	hs.children = append(hs.children, childEntry{
		component: c,
		sizing:    sizingFixed,
		fixedSize: width,
	})
}

// AddFlexible appends a component with a weighted flexible width.
func (hs *HorizontalStack) AddFlexible(c Component, weight int) {
	hs.children = append(hs.children, childEntry{
		component:  c,
		sizing:     sizingFlexible,
		flexWeight: weight,
	})
}

// SetSize distributes width and height to children.
// Fixed children get their exact width; remaining space is split
// among flexible/default children by weight.
func (hs *HorizontalStack) SetSize(width, height int) {
	hs.width = width
	hs.height = height

	if len(hs.children) == 0 {
		return
	}

	fixedTotal := 0
	flexTotal := 0
	for _, c := range hs.children {
		switch c.sizing {
		case sizingFixed:
			fixedTotal += c.fixedSize
		default:
			flexTotal += c.flexWeight
		}
	}

	remaining := width - fixedTotal
	if remaining < 0 {
		remaining = 0
	}

	for _, c := range hs.children {
		var w int
		switch c.sizing {
		case sizingFixed:
			w = c.fixedSize
		default:
			if flexTotal > 0 {
				w = remaining * c.flexWeight / flexTotal
			}
		}
		c.component.SetSize(w, height)
	}
}

// Init initializes all children, batching their commands.
func (hs *HorizontalStack) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, c := range hs.children {
		if cmd := c.component.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// Update routes messages to all children.
func (hs *HorizontalStack) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for i, c := range hs.children {
		model, cmd := c.component.Update(msg)
		if comp, ok := model.(Component); ok {
			hs.children[i].component = comp
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return hs, tea.Batch(cmds...)
}

// View renders children left-to-right (concatenated).
func (hs *HorizontalStack) View() tea.View {
	views := make([]string, 0, len(hs.children))
	for _, c := range hs.children {
		views = append(views, c.component.View().Content)
	}
	return tea.NewView(strings.Join(views, ""))
}
