package ui

import "github.com/projectbarks/gopher-code/pkg/ui/hooks/ide"

// IDEConnection returns the IDE connection tracker.
// Source: useIdeConnectionStatus.ts — exposes connection state to the TUI.
func (a *AppModel) IDEConnection() *ide.IDEConnection {
	return a.ideConn
}

// IDESelection returns the current IDE text selection.
// Source: useIdeSelection.ts — forwarded from the IDE extension.
func (a *AppModel) IDESelection() ide.Selection {
	return a.ideSelection
}

// SetIDESelection updates the IDE selection state (called on IDE notifications).
func (a *AppModel) SetIDESelection(sel ide.Selection) {
	a.ideSelection = sel
}
