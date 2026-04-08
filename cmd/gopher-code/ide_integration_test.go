package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/ide"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIDEIntegration_AppModelWiring verifies that IDE integration hooks are
// reachable from the binary through AppModel.
func TestIDEIntegration_AppModelWiring(t *testing.T) {
	sess := &session.SessionState{
		ID:  "test-session",
		CWD: t.TempDir(),
	}
	app := ui.NewAppModel(sess, nil)
	require.NotNil(t, app, "NewAppModel should return non-nil")

	conn := app.IDEConnection()
	require.NotNil(t, conn, "IDEConnection should be initialised by NewAppModel")
	assert.Equal(t, ide.Disconnected, conn.State(), "initial state should be Disconnected")
	assert.Empty(t, conn.IDEName(), "no IDE name initially")

	conn.SetConnecting("vscode", "ws://localhost:3000", "tok-abc")
	assert.Equal(t, ide.Connecting, conn.State())
	assert.Equal(t, "vscode", conn.IDEName())
	assert.Equal(t, ide.TransportWS, conn.Transport())

	conn.SetConnected()
	assert.Equal(t, ide.Connected, conn.State())

	sel := app.IDESelection()
	assert.Equal(t, ide.EmptySelection, sel, "initial selection should be empty")

	newSel := ide.SelectionFromRange(
		ide.SelectionPoint{Line: 10, Character: 0},
		ide.SelectionPoint{Line: 15, Character: 5},
		"selected code",
		"/tmp/file.go",
	)
	app.SetIDESelection(newSel)
	assert.Equal(t, newSel, app.IDESelection())
	assert.Equal(t, 6, app.IDESelection().LineCount)

	conn.SetDisconnected()
	assert.Equal(t, ide.Disconnected, conn.State())
	assert.Empty(t, conn.IDEName())
}

// TestIDEIntegration_AtMentionExtraction verifies @-mention extraction.
func TestIDEIntegration_AtMentionExtraction(t *testing.T) {
	mentions := ide.ExtractAtMentions("look at @src/main.go:42 and @pkg/foo.go")
	require.Len(t, mentions, 2)
	assert.Equal(t, "src/main.go", mentions[0].FilePath)
	assert.Equal(t, 42, mentions[0].LineStart)
	assert.Equal(t, "pkg/foo.go", mentions[1].FilePath)
	assert.Zero(t, mentions[1].LineStart)
}

// TestIDEIntegration_LogEventPrefixed verifies the log event prefix convention.
func TestIDEIntegration_LogEventPrefixed(t *testing.T) {
	e := ide.LogEvent{EventName: "file_changed", EventData: map[string]any{"path": "/x"}}
	assert.Equal(t, "tengu_ide_file_changed", e.PrefixedName())
}

// TestIDEIntegration_AtMentionFromNotification verifies 0-based to 1-based conversion.
func TestIDEIntegration_AtMentionFromNotification(t *testing.T) {
	am := ide.AtMentionFromNotification("/home/user/app.go", 9, 19, true, true)
	assert.Equal(t, 10, am.LineStart)
	assert.Equal(t, 20, am.LineEnd)
}
