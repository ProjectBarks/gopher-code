package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/core"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// TreeNode represents a node in the tree hierarchy
type TreeNode struct {
	ID       string      // Unique identifier
	Label    string      // Display text
	Expanded bool        // Whether node is expanded
	Selected bool        // Whether node is selected
	Children []*TreeNode // Child nodes
	Depth    int         // Depth in tree (0 = root)
}

// TreeView displays a hierarchical tree structure with expand/collapse support
type TreeView struct {
	root         *TreeNode
	selectedID   string        // ID of selected node
	scrollOffset int           // Scroll position
	width        int           // View width
	height       int           // View height
	focused      bool          // Whether tree has focus
	th           theme.Theme   // Theme for styling
	nodeIndex    map[string]*TreeNode // Quick lookup by ID
}

// NewTreeView creates a new tree view
func NewTreeView(th theme.Theme) *TreeView {
	return &TreeView{
		root: &TreeNode{
			ID:       "root",
			Label:    "Root",
			Expanded: true,
			Children: make([]*TreeNode, 0),
			Depth:    0,
		},
		selectedID: "",
		scrollOffset: 0,
		width:      40,
		height:     20,
		focused:    false,
		th:         th,
		nodeIndex:  make(map[string]*TreeNode),
	}
}

// Init initializes the tree view
func (tv *TreeView) Init() tea.Cmd {
	return nil
}

// Update handles input and navigation
func (tv *TreeView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !tv.focused {
		return tv, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return tv.handleKeyPress(msg)
	}

	return tv, nil
}

// handleKeyPress processes keyboard navigation
func (tv *TreeView) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "up":
		tv.selectPrevious()

	case "down":
		tv.selectNext()

	case "right":
		// Expand current node
		if node := tv.getSelectedNode(); node != nil && len(node.Children) > 0 {
			node.Expanded = true
		}

	case "left":
		// Collapse current node
		if node := tv.getSelectedNode(); node != nil {
			node.Expanded = false
		}

	case "home":
		visibleNodes := tv.getVisibleNodes()
		if len(visibleNodes) > 0 {
			tv.selectedID = visibleNodes[0].ID
		}
		tv.scrollOffset = 0

	case "end":
		// Select last visible node
		visibleNodes := tv.getVisibleNodes()
		if len(visibleNodes) > 0 {
			tv.selectedID = visibleNodes[len(visibleNodes)-1].ID
		}
	}

	return tv, nil
}

// selectPrevious moves selection up
func (tv *TreeView) selectPrevious() {
	visibleNodes := tv.getVisibleNodes()
	if len(visibleNodes) == 0 {
		return
	}

	currentIdx := -1
	for i, node := range visibleNodes {
		if node.ID == tv.selectedID {
			currentIdx = i
			break
		}
	}

	if currentIdx > 0 {
		tv.selectedID = visibleNodes[currentIdx-1].ID
		tv.ensureVisible(currentIdx - 1)
	}
}

// selectNext moves selection down
func (tv *TreeView) selectNext() {
	visibleNodes := tv.getVisibleNodes()
	if len(visibleNodes) == 0 {
		return
	}

	currentIdx := -1
	for i, node := range visibleNodes {
		if node.ID == tv.selectedID {
			currentIdx = i
			break
		}
	}

	if currentIdx < len(visibleNodes)-1 {
		tv.selectedID = visibleNodes[currentIdx+1].ID
		tv.ensureVisible(currentIdx + 1)
	}
}

// ensureVisible scrolls to keep visible node in view
func (tv *TreeView) ensureVisible(visibleIdx int) {
	viewHeight := tv.height - 2
	if viewHeight <= 0 {
		return
	}

	if visibleIdx < tv.scrollOffset {
		tv.scrollOffset = visibleIdx
	}

	if visibleIdx >= tv.scrollOffset+viewHeight {
		tv.scrollOffset = visibleIdx - viewHeight + 1
	}
}

// getVisibleNodes returns all nodes in visible order (expanded = children shown)
func (tv *TreeView) getVisibleNodes() []*TreeNode {
	var nodes []*TreeNode
	tv.collectVisibleNodes(tv.root, &nodes)
	return nodes
}

// collectVisibleNodes recursively collects visible nodes
func (tv *TreeView) collectVisibleNodes(node *TreeNode, result *[]*TreeNode) {
	// Skip root node itself but process its children
	if node != tv.root {
		*result = append(*result, node)
	}

	if node.Expanded && len(node.Children) > 0 {
		for _, child := range node.Children {
			tv.collectVisibleNodes(child, result)
		}
	}
}

// getSelectedNode returns the currently selected node
func (tv *TreeView) getSelectedNode() *TreeNode {
	if tv.selectedID == "" || tv.selectedID == "root" {
		return tv.root
	}
	return tv.nodeIndex[tv.selectedID]
}

// View renders the tree view
func (tv *TreeView) View() tea.View {
	cs := tv.th.Colors()

	// Container styling
	containerStyle := lipgloss.NewStyle().
		Width(tv.width).
		Height(tv.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(cs.TextSecondary)).
		Padding(0, 1)

	var content strings.Builder

	visibleNodes := tv.getVisibleNodes()
	viewHeight := tv.height - 3

	for i := tv.scrollOffset; i < len(visibleNodes) && i < tv.scrollOffset+viewHeight; i++ {
		node := visibleNodes[i]
		isSelected := node.ID == tv.selectedID

		// Build tree prefix
		var prefix strings.Builder
		for j := 0; j < node.Depth; j++ {
			prefix.WriteString("  ")
		}

		// Add expand/collapse indicator if has children
		if len(node.Children) > 0 {
			if node.Expanded {
				prefix.WriteString("▼ ")
			} else {
				prefix.WriteString("▶ ")
			}
		} else {
			prefix.WriteString("  ")
		}

		// Line styling
		style := lipgloss.NewStyle().
			Width(tv.width - 4).
			Foreground(lipgloss.Color(cs.TextSecondary))

		if isSelected {
			style = style.
				Background(lipgloss.Color(cs.Accent)).
				Foreground(lipgloss.Color(cs.Surface))
		}

		line := prefix.String() + truncateField(node.Label, tv.width-4-prefix.Len())
		content.WriteString(style.Render(line))

		if i < len(visibleNodes)-1 && i < tv.scrollOffset+viewHeight-1 {
			content.WriteString("\n")
		}
	}

	// Render empty state if no nodes
	if len(visibleNodes) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextSecondary))
		content.WriteString(emptyStyle.Render("(no items)"))
	}

	return tea.NewView(containerStyle.Render(content.String()))
}

// SetSize sets the tree view dimensions
func (tv *TreeView) SetSize(width, height int) {
	tv.width = width
	tv.height = height
}

// Focus gives focus to the tree view
func (tv *TreeView) Focus() {
	tv.focused = true
}

// Blur removes focus from the tree view
func (tv *TreeView) Blur() {
	tv.focused = false
}

// Focused returns whether the tree view is focused
func (tv *TreeView) Focused() bool {
	return tv.focused
}

// AddNode adds a child node to a parent
func (tv *TreeView) AddNode(parentID, nodeID, label string) bool {
	parent := tv.nodeIndex[parentID]
	if parent == nil && parentID != "root" {
		return false
	}

	if parent == nil {
		parent = tv.root
	}

	newNode := &TreeNode{
		ID:       nodeID,
		Label:    label,
		Expanded: false,
		Selected: false,
		Children: make([]*TreeNode, 0),
		Depth:    parent.Depth + 1,
	}

	parent.Children = append(parent.Children, newNode)
	tv.nodeIndex[nodeID] = newNode

	return true
}

// RemoveNode removes a node and its children
func (tv *TreeView) RemoveNode(nodeID string) bool {
	node := tv.nodeIndex[nodeID]
	if node == nil || nodeID == "root" {
		return false
	}

	// Find parent and remove from children
	var parent *TreeNode
	for _, n := range tv.nodeIndex {
		for i, child := range n.Children {
			if child.ID == nodeID {
				parent = n
				n.Children = append(n.Children[:i], n.Children[i+1:]...)
				break
			}
		}
		if parent != nil {
			break
		}
	}

	// Remove from index
	delete(tv.nodeIndex, nodeID)

	// Remove children from index recursively
	var removeChildren func(*TreeNode)
	removeChildren = func(n *TreeNode) {
		delete(tv.nodeIndex, n.ID)
		for _, child := range n.Children {
			removeChildren(child)
		}
	}
	removeChildren(node)

	// Adjust selection if needed
	if tv.selectedID == nodeID {
		visibleNodes := tv.getVisibleNodes()
		if len(visibleNodes) > 0 {
			tv.selectedID = visibleNodes[0].ID
		} else {
			tv.selectedID = ""
		}
	}

	return true
}

// SetNodeLabel updates a node's label
func (tv *TreeView) SetNodeLabel(nodeID, newLabel string) bool {
	node := tv.nodeIndex[nodeID]
	if node == nil {
		return false
	}
	node.Label = newLabel
	return true
}

// GetSelectedNode returns the selected node
func (tv *TreeView) GetSelectedNodeID() string {
	return tv.selectedID
}

// GetNode returns a node by ID
func (tv *TreeView) GetNode(nodeID string) *TreeNode {
	return tv.nodeIndex[nodeID]
}

// ToggleNode expands/collapses a node
func (tv *TreeView) ToggleNode(nodeID string) bool {
	node := tv.nodeIndex[nodeID]
	if node == nil || len(node.Children) == 0 {
		return false
	}
	node.Expanded = !node.Expanded
	return true
}

// IsNodeExpanded checks if a node is expanded
func (tv *TreeView) IsNodeExpanded(nodeID string) bool {
	node := tv.nodeIndex[nodeID]
	if node == nil {
		return false
	}
	return node.Expanded
}

// NodeCount returns total number of nodes
func (tv *TreeView) NodeCount() int {
	return len(tv.nodeIndex)
}

// Ensure TreeView implements core.Component and core.Focusable
var _ core.Component = (*TreeView)(nil)
var _ core.Focusable = (*TreeView)(nil)
