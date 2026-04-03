package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestTreeViewCreation(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	if tv == nil {
		t.Fatal("TreeView should not be nil")
	}

	if tv.root == nil {
		t.Error("Root node should exist")
	}

	if tv.selectedID != "" {
		t.Error("Should start with no selection")
	}

	if tv.focused {
		t.Error("Should start unfocused")
	}
}

func TestTreeViewAddNode(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	success := tv.AddNode("root", "file-1", "main.go")
	if !success {
		t.Error("AddNode should return true")
	}

	if tv.NodeCount() != 1 {
		t.Errorf("Expected 1 node, got %d", tv.NodeCount())
	}

	node := tv.GetNode("file-1")
	if node == nil || node.Label != "main.go" {
		t.Error("Node should be added with correct label")
	}

	if node.Depth != 1 {
		t.Errorf("Node depth should be 1, got %d", node.Depth)
	}
}

func TestTreeViewAddNestedNodes(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "dir-1", "src")
	tv.AddNode("dir-1", "file-1", "main.go")
	tv.AddNode("dir-1", "file-2", "utils.go")

	if tv.NodeCount() != 3 {
		t.Errorf("Expected 3 nodes, got %d", tv.NodeCount())
	}

	parent := tv.GetNode("dir-1")
	if len(parent.Children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(parent.Children))
	}

	child := tv.GetNode("file-1")
	if child.Depth != 2 {
		t.Errorf("Child depth should be 2, got %d", child.Depth)
	}
}

func TestTreeViewRemoveNode(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "main.go")
	tv.AddNode("root", "file-2", "utils.go")

	success := tv.RemoveNode("file-1")
	if !success {
		t.Error("RemoveNode should return true")
	}

	if tv.NodeCount() != 1 {
		t.Errorf("Expected 1 node, got %d", tv.NodeCount())
	}

	if tv.GetNode("file-1") != nil {
		t.Error("Removed node should not exist")
	}
}

func TestTreeViewRemoveNodeWithChildren(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "dir-1", "src")
	tv.AddNode("dir-1", "file-1", "main.go")

	success := tv.RemoveNode("dir-1")
	if !success {
		t.Error("RemoveNode should return true")
	}

	if tv.NodeCount() != 0 {
		t.Errorf("Expected 0 nodes, got %d", tv.NodeCount())
	}

	if tv.GetNode("dir-1") != nil || tv.GetNode("file-1") != nil {
		t.Error("Removed nodes should not exist")
	}
}

func TestTreeViewSetNodeLabel(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "original.go")

	success := tv.SetNodeLabel("file-1", "updated.go")
	if !success {
		t.Error("SetNodeLabel should return true")
	}

	node := tv.GetNode("file-1")
	if node.Label != "updated.go" {
		t.Errorf("Label should be updated, got %s", node.Label)
	}
}

func TestTreeViewToggleExpand(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "dir-1", "src")
	tv.AddNode("dir-1", "file-1", "main.go")

	if tv.IsNodeExpanded("dir-1") {
		t.Error("Node should not be expanded initially")
	}

	success := tv.ToggleNode("dir-1")
	if !success {
		t.Error("ToggleNode should return true")
	}

	if !tv.IsNodeExpanded("dir-1") {
		t.Error("Node should be expanded after toggle")
	}

	tv.ToggleNode("dir-1")
	if tv.IsNodeExpanded("dir-1") {
		t.Error("Node should be collapsed after second toggle")
	}
}

func TestTreeViewToggleLeafNode(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "main.go")

	success := tv.ToggleNode("file-1")
	if success {
		t.Error("ToggleNode should return false for leaf node")
	}
}

func TestTreeViewNavigateDown(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "a.go")
	tv.AddNode("root", "file-2", "b.go")
	tv.AddNode("root", "file-3", "c.go")

	tv.selectedID = "file-1"
	tv.Focus()

	msg := tea.KeyPressMsg{Code: tea.KeyDown}
	tv.Update(msg)

	if tv.selectedID != "file-2" {
		t.Errorf("Expected file-2, got %s", tv.selectedID)
	}
}

func TestTreeViewNavigateUp(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "a.go")
	tv.AddNode("root", "file-2", "b.go")
	tv.AddNode("root", "file-3", "c.go")

	tv.selectedID = "file-2"
	tv.Focus()

	msg := tea.KeyPressMsg{Code: tea.KeyUp}
	tv.Update(msg)

	if tv.selectedID != "file-1" {
		t.Errorf("Expected file-1, got %s", tv.selectedID)
	}
}

func TestTreeViewExpandCollapse(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "dir-1", "src")
	tv.AddNode("dir-1", "file-1", "main.go")

	tv.selectedID = "dir-1"
	tv.Focus()

	// Right to expand
	msg := tea.KeyPressMsg{Code: tea.KeyRight}
	tv.Update(msg)

	if !tv.IsNodeExpanded("dir-1") {
		t.Error("Node should be expanded")
	}

	// Left to collapse
	msg = tea.KeyPressMsg{Code: tea.KeyLeft}
	tv.Update(msg)

	if tv.IsNodeExpanded("dir-1") {
		t.Error("Node should be collapsed")
	}
}

func TestTreeViewNavigateHome(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "a.go")
	tv.AddNode("root", "file-2", "b.go")
	tv.AddNode("root", "file-3", "c.go")

	tv.selectedID = "file-3"
	tv.Focus()

	msg := tea.KeyPressMsg{Code: tea.KeyHome}
	tv.Update(msg)

	if tv.selectedID != "file-1" {
		t.Errorf("Expected file-1, got %s", tv.selectedID)
	}
}

func TestTreeViewNavigateEnd(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "a.go")
	tv.AddNode("root", "file-2", "b.go")
	tv.AddNode("root", "file-3", "c.go")

	tv.selectedID = "file-1"
	tv.Focus()

	msg := tea.KeyPressMsg{Code: tea.KeyEnd}
	tv.Update(msg)

	if tv.selectedID != "file-3" {
		t.Errorf("Expected file-3, got %s", tv.selectedID)
	}
}

func TestTreeViewNavigationBoundaries(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "a.go")
	tv.AddNode("root", "file-2", "b.go")

	tv.selectedID = "file-1"
	tv.Focus()

	// Try to go up from first
	msg := tea.KeyPressMsg{Code: tea.KeyUp}
	tv.Update(msg)

	if tv.selectedID != "file-1" {
		t.Error("Should not go up from first")
	}

	// Go down to last
	tv.selectedID = "file-2"
	msg = tea.KeyPressMsg{Code: tea.KeyDown}
	tv.Update(msg)

	if tv.selectedID != "file-2" {
		t.Error("Should not go down past last")
	}
}

func TestTreeViewVisibleNodes(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "dir-1", "src")
	tv.AddNode("dir-1", "file-1", "main.go")
	tv.AddNode("dir-1", "file-2", "utils.go")
	tv.AddNode("root", "file-3", "README.md")

	// Initially collapsed
	visible := tv.getVisibleNodes()
	if len(visible) != 2 {
		t.Errorf("Expected 2 visible nodes (dir-1 and file-3), got %d", len(visible))
	}

	// Expand dir-1
	tv.ToggleNode("dir-1")
	visible = tv.getVisibleNodes()
	if len(visible) != 4 {
		t.Errorf("Expected 4 visible nodes, got %d", len(visible))
	}
}

func TestTreeViewFocusBlur(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	if tv.Focused() {
		t.Error("Should not be focused initially")
	}

	tv.Focus()
	if !tv.Focused() {
		t.Error("Should be focused after Focus()")
	}

	tv.Blur()
	if tv.Focused() {
		t.Error("Should not be focused after Blur()")
	}
}

func TestTreeViewView(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "main.go")
	tv.AddNode("root", "dir-1", "src")
	tv.selectedID = "file-1"
	tv.SetSize(40, 10)

	view := tv.View()

	if view.Content == "" {
		t.Error("View should not be empty")
	}

	if !strings.Contains(view.Content, "main.go") {
		t.Error("View should contain node label")
	}
}

func TestTreeViewViewEmpty(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.SetSize(40, 10)
	view := tv.View()

	if view.Content == "" {
		t.Error("View should not be empty")
	}

	if !strings.Contains(view.Content, "(no items)") {
		t.Error("View should show empty state")
	}
}

func TestTreeViewSetSize(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.SetSize(50, 15)

	if tv.width != 50 {
		t.Errorf("Width should be 50, got %d", tv.width)
	}

	if tv.height != 15 {
		t.Errorf("Height should be 15, got %d", tv.height)
	}
}

func TestTreeViewInit(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	cmd := tv.Init()

	if cmd != nil {
		t.Error("Init should return nil")
	}
}

func TestTreeViewUpdateWhenNotFocused(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "a.go")
	tv.AddNode("root", "file-2", "b.go")

	tv.selectedID = "file-1"

	// Update without focus
	msg := tea.KeyPressMsg{Code: tea.KeyDown}
	tv.Update(msg)

	if tv.selectedID != "file-1" {
		t.Error("Should not process input when not focused")
	}
}

func TestTreeViewComplexHierarchy(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	// Build a complex tree
	tv.AddNode("root", "src", "src")
	tv.AddNode("src", "components", "components")
	tv.AddNode("components", "button.go", "button.go")
	tv.AddNode("components", "input.go", "input.go")
	tv.AddNode("src", "main.go", "main.go")

	if tv.NodeCount() != 5 {
		t.Errorf("Expected 5 nodes, got %d", tv.NodeCount())
	}

	// Check depths
	if tv.GetNode("src").Depth != 1 {
		t.Error("src should be depth 1")
	}

	if tv.GetNode("components").Depth != 2 {
		t.Error("components should be depth 2")
	}

	if tv.GetNode("button.go").Depth != 3 {
		t.Error("button.go should be depth 3")
	}
}

func TestTreeViewNavigationInComplexHierarchy(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "dir-1", "src")
	tv.AddNode("dir-1", "dir-2", "components")
	tv.AddNode("dir-2", "file-1", "button.go")
	tv.AddNode("dir-1", "file-2", "main.go")

	// Expand dirs
	tv.ToggleNode("dir-1")
	tv.ToggleNode("dir-2")

	visible := tv.getVisibleNodes()
	if len(visible) != 4 {
		t.Errorf("Expected 4 visible nodes, got %d", len(visible))
	}

	// Navigate through all nodes
	tv.selectedID = visible[0].ID
	tv.Focus()

	for i := 1; i < len(visible); i++ {
		msg := tea.KeyPressMsg{Code: tea.KeyDown}
		tv.Update(msg)

		if tv.selectedID != visible[i].ID {
			t.Errorf("Expected node %s, got %s", visible[i].ID, tv.selectedID)
		}
	}
}

func TestTreeViewAddNodeToNonexistent(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	success := tv.AddNode("nonexistent", "file-1", "main.go")
	if success {
		t.Error("AddNode should return false for nonexistent parent")
	}
}

func TestTreeViewRemoveRoot(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	success := tv.RemoveNode("root")
	if success {
		t.Error("RemoveNode should return false for root")
	}
}

func TestTreeViewSelectedNodeID(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "main.go")

	if tv.GetSelectedNodeID() != "" {
		t.Error("Should start with no selection")
	}

	tv.selectedID = "file-1"
	if tv.GetSelectedNodeID() != "file-1" {
		t.Error("Should return selected node ID")
	}
}

func TestTreeViewGetNode(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "main.go")

	node := tv.GetNode("file-1")
	if node == nil || node.Label != "main.go" {
		t.Error("GetNode should return correct node")
	}

	node = tv.GetNode("nonexistent")
	if node != nil {
		t.Error("GetNode should return nil for nonexistent")
	}
}

func TestTreeViewNodeCount(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	if tv.NodeCount() != 0 {
		t.Errorf("Expected 0 nodes initially, got %d", tv.NodeCount())
	}

	tv.AddNode("root", "file-1", "main.go")
	if tv.NodeCount() != 1 {
		t.Errorf("Expected 1 node, got %d", tv.NodeCount())
	}

	tv.AddNode("root", "file-2", "utils.go")
	if tv.NodeCount() != 2 {
		t.Errorf("Expected 2 nodes, got %d", tv.NodeCount())
	}
}

func TestTreeViewLargeTree(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	// Add many nodes
	for i := 0; i < 100; i++ {
		id := "file-" + string(rune('0'+(i%10)))
		tv.AddNode("root", id+"-"+string(rune('0'+(i/10))), "file"+string(rune('0'+(i%10)))+".go")
	}

	if tv.NodeCount() != 100 {
		t.Errorf("Expected 100 nodes, got %d", tv.NodeCount())
	}

	visible := tv.getVisibleNodes()
	if len(visible) == 0 {
		t.Error("Should have visible nodes")
	}
}

func TestTreeViewSelectionAfterRemoval(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	tv.AddNode("root", "file-1", "a.go")
	tv.AddNode("root", "file-2", "b.go")
	tv.AddNode("root", "file-3", "c.go")

	tv.selectedID = "file-2"
	tv.RemoveNode("file-2")

	// Should adjust selection
	if tv.selectedID == "" {
		t.Error("Should have adjusted selection")
	}

	// Selection should be valid
	if tv.GetNode(tv.selectedID) == nil {
		t.Error("Selected node should exist")
	}
}

func TestTreeViewSetLabelNonexistent(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	success := tv.SetNodeLabel("nonexistent", "new")
	if success {
		t.Error("SetNodeLabel should return false for nonexistent")
	}
}

func TestTreeViewIsNodeExpandedNonexistent(t *testing.T) {
	th := theme.Current()
	tv := NewTreeView(th)

	expanded := tv.IsNodeExpanded("nonexistent")
	if expanded {
		t.Error("Should return false for nonexistent node")
	}
}
