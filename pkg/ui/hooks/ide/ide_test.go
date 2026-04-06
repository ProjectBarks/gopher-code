package ide

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Connection state transitions
// ---------------------------------------------------------------------------

func TestIDEConnection_InitialState(t *testing.T) {
	c := NewIDEConnection()
	assert.Equal(t, Disconnected, c.State())
	assert.Empty(t, c.IDEName())
	assert.Empty(t, c.URL())
	assert.Equal(t, TransportNone, c.Transport())
}

func TestIDEConnection_ConnectingTransition(t *testing.T) {
	c := NewIDEConnection()
	c.SetConnecting("vscode", "ws://localhost:3000", "tok123")

	assert.Equal(t, Connecting, c.State())
	assert.Equal(t, "vscode", c.IDEName())
	assert.Equal(t, "ws://localhost:3000", c.URL())
	assert.Equal(t, TransportWS, c.Transport())
}

func TestIDEConnection_ConnectedTransition(t *testing.T) {
	c := NewIDEConnection()
	// Must go through Connecting first.
	c.SetConnecting("cursor", "http://localhost:8080", "abc")
	c.SetConnected()

	assert.Equal(t, Connected, c.State())
	assert.Equal(t, "cursor", c.IDEName())
	assert.Equal(t, TransportSSE, c.Transport())
}

func TestIDEConnection_ConnectedRequiresConnecting(t *testing.T) {
	c := NewIDEConnection()
	// SetConnected from Disconnected is a no-op.
	c.SetConnected()
	assert.Equal(t, Disconnected, c.State())
}

func TestIDEConnection_DisconnectedClearsMetadata(t *testing.T) {
	c := NewIDEConnection()
	c.SetConnecting("vscode", "ws://localhost:3000", "tok")
	c.SetConnected()
	c.SetDisconnected()

	assert.Equal(t, Disconnected, c.State())
	assert.Empty(t, c.IDEName())
	assert.Empty(t, c.URL())
	assert.Equal(t, TransportNone, c.Transport())
}

func TestIDEConnection_ReconnectCycle(t *testing.T) {
	c := NewIDEConnection()
	// Full cycle: disconnected → connecting → connected → disconnected → connecting → connected
	c.SetConnecting("vscode", "ws://localhost:1", "a")
	assert.Equal(t, Connecting, c.State())
	c.SetConnected()
	assert.Equal(t, Connected, c.State())
	c.SetDisconnected()
	assert.Equal(t, Disconnected, c.State())
	c.SetConnecting("cursor", "http://localhost:2", "b")
	assert.Equal(t, Connecting, c.State())
	assert.Equal(t, "cursor", c.IDEName())
	c.SetConnected()
	assert.Equal(t, Connected, c.State())
}

func TestConnState_String(t *testing.T) {
	assert.Equal(t, "disconnected", Disconnected.String())
	assert.Equal(t, "connecting", Connecting.String())
	assert.Equal(t, "connected", Connected.String())
	assert.Contains(t, ConnState(99).String(), "ConnState(99)")
}

// ---------------------------------------------------------------------------
// Transport detection
// ---------------------------------------------------------------------------

func TestTransportFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want TransportType
	}{
		{"ws://localhost:3000", TransportWS},
		{"wss://localhost:3000/path", TransportWS},
		{"http://localhost:8080", TransportSSE},
		{"https://ide.example.com", TransportSSE},
		{"", TransportSSE}, // empty defaults to SSE
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, TransportFromURL(tt.url), "url=%q", tt.url)
	}
}

// ---------------------------------------------------------------------------
// At-mention extraction from input text
// ---------------------------------------------------------------------------

func TestExtractAtMentions_NoMentions(t *testing.T) {
	result := ExtractAtMentions("just some normal text without at signs")
	assert.Nil(t, result)
}

func TestExtractAtMentions_SingleFile(t *testing.T) {
	result := ExtractAtMentions("look at @src/main.go please")
	require.Len(t, result, 1)
	assert.Equal(t, "src/main.go", result[0].FilePath)
	assert.Zero(t, result[0].LineStart)
	assert.Zero(t, result[0].LineEnd)
}

func TestExtractAtMentions_FileWithLine(t *testing.T) {
	result := ExtractAtMentions("check @pkg/foo.go:42 for the bug")
	require.Len(t, result, 1)
	assert.Equal(t, "pkg/foo.go", result[0].FilePath)
	assert.Equal(t, 42, result[0].LineStart)
	assert.Zero(t, result[0].LineEnd)
}

func TestExtractAtMentions_FileWithLineRange(t *testing.T) {
	result := ExtractAtMentions("see @cmd/server.go:10-20")
	require.Len(t, result, 1)
	assert.Equal(t, "cmd/server.go", result[0].FilePath)
	assert.Equal(t, 10, result[0].LineStart)
	assert.Equal(t, 20, result[0].LineEnd)
}

func TestExtractAtMentions_Multiple(t *testing.T) {
	result := ExtractAtMentions("compare @a.go and @b.go:5")
	require.Len(t, result, 2)
	assert.Equal(t, "a.go", result[0].FilePath)
	assert.Equal(t, "b.go", result[1].FilePath)
	assert.Equal(t, 5, result[1].LineStart)
}

func TestExtractAtMentions_UnderscoreAndHyphen(t *testing.T) {
	result := ExtractAtMentions("@my_dir/some-file.ts:1-100")
	require.Len(t, result, 1)
	assert.Equal(t, "my_dir/some-file.ts", result[0].FilePath)
	assert.Equal(t, 1, result[0].LineStart)
	assert.Equal(t, 100, result[0].LineEnd)
}

func TestExtractAtMentions_EmailNotMatched(t *testing.T) {
	// An email like user@example.com should still match the regex since
	// example.com looks like a path. This is acceptable — upstream TS
	// also does not filter emails at the extraction stage.
	result := ExtractAtMentions("email user@example.com here")
	// The regex will capture "example.com" as a file path — acceptable.
	require.Len(t, result, 1)
	assert.Equal(t, "example.com", result[0].FilePath)
}

// ---------------------------------------------------------------------------
// AtMention from IDE notification (0-based → 1-based)
// ---------------------------------------------------------------------------

func TestAtMentionFromNotification_WithLines(t *testing.T) {
	am := AtMentionFromNotification("/home/user/file.go", 9, 19, true, true)
	assert.Equal(t, "/home/user/file.go", am.FilePath)
	assert.Equal(t, 10, am.LineStart) // 9+1
	assert.Equal(t, 20, am.LineEnd)   // 19+1
}

func TestAtMentionFromNotification_NoLines(t *testing.T) {
	am := AtMentionFromNotification("foo.py", 0, 0, false, false)
	assert.Equal(t, "foo.py", am.FilePath)
	assert.Zero(t, am.LineStart)
	assert.Zero(t, am.LineEnd)
}

// ---------------------------------------------------------------------------
// Selection forwarding
// ---------------------------------------------------------------------------

func TestSelectionFromRange_Basic(t *testing.T) {
	s := SelectionFromRange(
		SelectionPoint{Line: 5, Character: 0},
		SelectionPoint{Line: 10, Character: 8},
		"selected text",
		"/tmp/file.go",
	)
	assert.Equal(t, 6, s.LineCount) // 10-5+1
	assert.Equal(t, 5, s.LineStart)
	assert.Equal(t, "selected text", s.Text)
	assert.Equal(t, "/tmp/file.go", s.FilePath)
}

func TestSelectionFromRange_EndCharZeroSubtractsLine(t *testing.T) {
	// TS: if end.character === 0, lineCount--
	s := SelectionFromRange(
		SelectionPoint{Line: 3, Character: 5},
		SelectionPoint{Line: 7, Character: 0},
		"",
		"f.go",
	)
	assert.Equal(t, 4, s.LineCount) // 7-3+1 = 5, minus 1 for char=0 → 4
}

func TestSelectionFromRange_SingleLine(t *testing.T) {
	s := SelectionFromRange(
		SelectionPoint{Line: 0, Character: 2},
		SelectionPoint{Line: 0, Character: 10},
		"hello",
		"a.go",
	)
	assert.Equal(t, 1, s.LineCount)
	assert.Equal(t, 0, s.LineStart)
}

func TestEmptySelection(t *testing.T) {
	assert.Equal(t, 0, EmptySelection.LineCount)
	assert.Equal(t, -1, EmptySelection.LineStart)
	assert.Empty(t, EmptySelection.Text)
}

// ---------------------------------------------------------------------------
// LogEvent
// ---------------------------------------------------------------------------

func TestLogEvent_PrefixedName(t *testing.T) {
	e := LogEvent{EventName: "file_opened", EventData: map[string]any{"ext": "go"}}
	assert.Equal(t, "tengu_ide_file_opened", e.PrefixedName())
}

func TestLogEvent_EmptyName(t *testing.T) {
	e := LogEvent{}
	assert.Equal(t, "tengu_ide_", e.PrefixedName())
}
