package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	br "github.com/projectbarks/gopher-code/pkg/bridge"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// ---------------------------------------------------------------------------
// Fake transport for ReplBridgeHook tests
// ---------------------------------------------------------------------------

type fakeTransport struct {
	connectErr error
	status     BridgeStatus
	connected  bool
}

func (f *fakeTransport) Connect(_ context.Context) error {
	if f.connectErr != nil {
		return f.connectErr
	}
	f.connected = true
	f.status = StatusConnected
	return nil
}

func (f *fakeTransport) Disconnect() error {
	f.connected = false
	f.status = StatusDisconnected
	return nil
}

func (f *fakeTransport) Send(_ context.Context, _ br.BridgeEvent) error {
	return nil
}

func (f *fakeTransport) Status() BridgeStatus {
	return f.status
}

// ---------------------------------------------------------------------------
// ReplBridgeHook tests — status propagation, failure fuse
// ---------------------------------------------------------------------------

func TestReplBridgeHook_StatusPropagatesOnConnect(t *testing.T) {
	transport := &fakeTransport{}
	hook := NewReplBridgeHook(ReplBridgeHookConfig{
		Transport: transport,
		Enabled:   true,
	})

	// Init should return a connect command.
	cmd := hook.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd, expected connect command")
	}

	// Execute the command — it should produce a StatusConnected message.
	msg := cmd()
	statusMsg, ok := msg.(BridgeStatusMsg)
	if !ok {
		t.Fatalf("expected BridgeStatusMsg, got %T", msg)
	}
	if statusMsg.Status != StatusConnected {
		t.Errorf("status = %v, want StatusConnected", statusMsg.Status)
	}
	if statusMsg.Source != "repl" {
		t.Errorf("source = %q, want %q", statusMsg.Source, "repl")
	}

	// Feed the status message through Update.
	model, _ := hook.Update(statusMsg)
	updated := model.(*ReplBridgeHook)
	if updated.Status() != StatusConnected {
		t.Errorf("after Update, status = %v, want StatusConnected", updated.Status())
	}
}

func TestReplBridgeHook_FailureIncrements(t *testing.T) {
	transport := &fakeTransport{connectErr: fmt.Errorf("auth failed")}
	hook := NewReplBridgeHook(ReplBridgeHookConfig{
		Transport: transport,
		Enabled:   true,
	})

	cmd := hook.Init()
	msg := cmd()
	statusMsg := msg.(BridgeStatusMsg)
	if statusMsg.Status != StatusError {
		t.Errorf("status = %v, want StatusError", statusMsg.Status)
	}
	if statusMsg.Err == nil {
		t.Error("expected non-nil error")
	}
	if hook.ConsecutiveFailures() != 1 {
		t.Errorf("consecutiveFailures = %d, want 1", hook.ConsecutiveFailures())
	}
}

func TestReplBridgeHook_FuseBlowsAfterMaxFailures(t *testing.T) {
	transport := &fakeTransport{connectErr: fmt.Errorf("auth failed")}
	hook := NewReplBridgeHook(ReplBridgeHookConfig{
		Transport: transport,
		Enabled:   true,
	})

	// Blow through the fuse limit.
	for i := 0; i < MaxConsecutiveInitFailures; i++ {
		cmd := hook.SetEnabled(true)
		if cmd == nil {
			// Already disabled after last cycle.
			break
		}
		msg := cmd()
		_ = msg
	}

	// Now attempting to enable should return StatusDisabled.
	cmd := hook.SetEnabled(true)
	if cmd == nil {
		t.Fatal("expected cmd for disabled status")
	}
	msg := cmd()
	statusMsg := msg.(BridgeStatusMsg)
	if statusMsg.Status != StatusDisabled {
		t.Errorf("status = %v, want StatusDisabled", statusMsg.Status)
	}
}

func TestReplBridgeHook_DisabledReturnsNilInit(t *testing.T) {
	hook := NewReplBridgeHook(ReplBridgeHookConfig{
		Enabled: false,
	})
	cmd := hook.Init()
	if cmd != nil {
		t.Error("expected nil cmd when disabled")
	}
}

func TestReplBridgeHook_SetEnabledFalse(t *testing.T) {
	hook := NewReplBridgeHook(ReplBridgeHookConfig{
		Transport: &fakeTransport{},
		Enabled:   true,
	})

	cmd := hook.SetEnabled(false)
	if cmd == nil {
		t.Fatal("expected cmd for disconnect status")
	}
	msg := cmd()
	statusMsg := msg.(BridgeStatusMsg)
	if statusMsg.Status != StatusDisconnected {
		t.Errorf("status = %v, want StatusDisconnected", statusMsg.Status)
	}
}

func TestReplBridgeHook_IgnoresOtherSourceStatus(t *testing.T) {
	hook := NewReplBridgeHook(ReplBridgeHookConfig{
		Transport: &fakeTransport{},
		Enabled:   true,
	})

	// A "remote" source message should be ignored.
	remoteMsg := BridgeStatusMsg{Source: "remote", Status: StatusConnected}
	_, _ = hook.Update(remoteMsg)
	if hook.Status() != StatusDisconnected {
		t.Errorf("status should remain disconnected, got %v", hook.Status())
	}
}

func TestReplBridgeHook_OutboundOnly(t *testing.T) {
	hook := NewReplBridgeHook(ReplBridgeHookConfig{
		Transport:    &fakeTransport{},
		Enabled:      true,
		OutboundOnly: true,
	})
	if !hook.IsOutboundOnly() {
		t.Error("expected outbound-only mode")
	}
}

func TestReplBridgeHook_NoTransportErrors(t *testing.T) {
	hook := NewReplBridgeHook(ReplBridgeHookConfig{
		Transport: nil,
		Enabled:   true,
	})
	cmd := hook.Init()
	msg := cmd()
	statusMsg := msg.(BridgeStatusMsg)
	if statusMsg.Status != StatusError {
		t.Errorf("status = %v, want StatusError", statusMsg.Status)
	}
	if statusMsg.Err == nil {
		t.Error("expected error about no transport")
	}
}

// ---------------------------------------------------------------------------
// RemoteSessionHook tests — URL generation, echo dedup
// ---------------------------------------------------------------------------

func TestRemoteSessionHook_URLGeneration(t *testing.T) {
	hook := NewRemoteSessionHook(&RemoteSessionConfig{
		SessionID: "sess_abc123",
	})

	cmd := hook.Init()
	if cmd == nil {
		t.Fatal("expected URL generation cmd")
	}

	msg := cmd()
	urlMsg, ok := msg.(RemoteSessionURLMsg)
	if !ok {
		t.Fatalf("expected RemoteSessionURLMsg, got %T", msg)
	}

	if urlMsg.SessionID != "sess_abc123" {
		t.Errorf("sessionID = %q, want %q", urlMsg.SessionID, "sess_abc123")
	}
	// URL should contain the session ID or compat form.
	if urlMsg.URL == "" {
		t.Error("expected non-empty URL")
	}
	// Should start with https://claude.ai/code/
	expected := "https://claude.ai/code/"
	if len(urlMsg.URL) < len(expected) || urlMsg.URL[:len(expected)] != expected {
		t.Errorf("URL = %q, want prefix %q", urlMsg.URL, expected)
	}

	// Feed URL message through Update.
	model, _ := hook.Update(urlMsg)
	updated := model.(*RemoteSessionHook)
	if updated.URL() != urlMsg.URL {
		t.Errorf("URL() = %q, want %q", updated.URL(), urlMsg.URL)
	}
	if updated.Status() != StatusConnected {
		t.Errorf("status = %v, want StatusConnected", updated.Status())
	}
}

func TestRemoteSessionHook_CustomIngressURL(t *testing.T) {
	url := GetRemoteSessionURL("sess_xyz", "https://custom.ingress.example.com")
	expected := "https://custom.ingress.example.com/code/"
	if len(url) < len(expected) || url[:len(expected)] != expected {
		t.Errorf("URL = %q, want prefix %q", url, expected)
	}
}

func TestRemoteSessionHook_NilConfigIsNoop(t *testing.T) {
	hook := NewRemoteSessionHook(nil)
	if hook.IsRemoteMode() {
		t.Error("expected non-remote mode")
	}
	cmd := hook.Init()
	if cmd != nil {
		t.Error("expected nil cmd for nil config")
	}
	if hook.SessionID() != "" {
		t.Error("expected empty session ID")
	}
}

func TestRemoteSessionHook_EchoDedup(t *testing.T) {
	hook := NewRemoteSessionHook(&RemoteSessionConfig{
		SessionID: "sess_test",
	})

	uuid := "msg-12345"
	if hook.IsEcho(uuid) {
		t.Error("should not be echo before marking")
	}

	hook.MarkEchoSent(uuid)
	if !hook.IsEcho(uuid) {
		t.Error("should be echo after marking")
	}
}

func TestRemoteSessionHook_IgnoresOtherSourceStatus(t *testing.T) {
	hook := NewRemoteSessionHook(&RemoteSessionConfig{
		SessionID: "sess_test",
	})

	replMsg := BridgeStatusMsg{Source: "repl", Status: StatusConnected}
	_, _ = hook.Update(replMsg)
	if hook.Status() != StatusDisconnected {
		t.Errorf("status should remain disconnected, got %v", hook.Status())
	}
}

// ---------------------------------------------------------------------------
// MailboxBridgeHook tests — polling, loading suppression
// ---------------------------------------------------------------------------

func TestMailboxBridgeHook_PollReadsUnreadMessages(t *testing.T) {
	// Set up a temp mailbox with unread messages.
	tmpDir := t.TempDir()
	mailbox := session.NewMailbox(tmpDir)

	err := mailbox.WriteToMailbox("agent-a", "team1", "agent-b", "hello from b")
	if err != nil {
		t.Fatalf("WriteToMailbox: %v", err)
	}

	hook := NewMailboxBridgeHook(MailboxBridgeConfig{
		Mailbox:      mailbox,
		AgentName:    "agent-a",
		TeamName:     "team1",
		PollInterval: 100 * time.Millisecond,
	})

	// Execute poll command directly.
	cmd := hook.pollCmd()
	msg := cmd()
	pollMsg, ok := msg.(MailboxPollMsg)
	if !ok {
		t.Fatalf("expected MailboxPollMsg, got %T", msg)
	}
	if pollMsg.Err != nil {
		t.Fatalf("unexpected error: %v", pollMsg.Err)
	}
	if len(pollMsg.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(pollMsg.Messages))
	}
	if pollMsg.Messages[0].From != "agent-b" {
		t.Errorf("from = %q, want %q", pollMsg.Messages[0].From, "agent-b")
	}
	if pollMsg.Messages[0].Text != "hello from b" {
		t.Errorf("text = %q, want %q", pollMsg.Messages[0].Text, "hello from b")
	}
}

func TestMailboxBridgeHook_SkipsWhenLoading(t *testing.T) {
	tmpDir := t.TempDir()
	mailbox := session.NewMailbox(tmpDir)
	_ = mailbox.WriteToMailbox("agent-a", "team1", "agent-b", "pending msg")

	hook := NewMailboxBridgeHook(MailboxBridgeConfig{
		Mailbox:      mailbox,
		AgentName:    "agent-a",
		TeamName:     "team1",
		PollInterval: 100 * time.Millisecond,
	})
	hook.SetLoading(true)

	cmd := hook.pollCmd()
	msg := cmd()
	pollMsg := msg.(MailboxPollMsg)
	// Should return empty when loading.
	if len(pollMsg.Messages) != 0 {
		t.Errorf("expected 0 messages while loading, got %d", len(pollMsg.Messages))
	}
}

func TestMailboxBridgeHook_NilMailboxIsNoop(t *testing.T) {
	hook := NewMailboxBridgeHook(MailboxBridgeConfig{
		Mailbox: nil,
	})
	cmd := hook.Init()
	if cmd != nil {
		t.Error("expected nil cmd for nil mailbox")
	}
}

func TestMailboxBridgeHook_EmptyInboxReturnsNoMessages(t *testing.T) {
	tmpDir := t.TempDir()
	mailbox := session.NewMailbox(tmpDir)

	hook := NewMailboxBridgeHook(MailboxBridgeConfig{
		Mailbox:   mailbox,
		AgentName: "agent-a",
		TeamName:  "team1",
	})

	cmd := hook.pollCmd()
	msg := cmd()
	pollMsg := msg.(MailboxPollMsg)
	if pollMsg.Err != nil {
		t.Fatalf("unexpected error: %v", pollMsg.Err)
	}
	if len(pollMsg.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(pollMsg.Messages))
	}
}

func TestMailboxBridgeHook_TickThenPollCycle(t *testing.T) {
	tmpDir := t.TempDir()
	mailbox := session.NewMailbox(tmpDir)

	hook := NewMailboxBridgeHook(MailboxBridgeConfig{
		Mailbox:      mailbox,
		AgentName:    "agent-a",
		TeamName:     "team1",
		PollInterval: 50 * time.Millisecond,
	})

	// Init returns a tick command.
	cmd := hook.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}

	// Simulate tick → poll → tick cycle.
	// Update with a tick message should return a poll command.
	tick := bridgeTickMsg{source: "mailbox"}
	_, pollCmd := hook.Update(tick)
	if pollCmd == nil {
		t.Fatal("expected poll cmd from tick")
	}

	// Execute poll.
	pollResult := pollCmd()
	pollMsg, ok := pollResult.(MailboxPollMsg)
	if !ok {
		t.Fatalf("expected MailboxPollMsg, got %T", pollResult)
	}

	// Update with poll result should schedule next tick.
	_, nextCmd := hook.Update(pollMsg)
	if nextCmd == nil {
		t.Fatal("expected next tick cmd after poll")
	}
}

func TestMailboxBridgeHook_PollError(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a mailbox pointing to a non-writable path.
	badPath := filepath.Join(tmpDir, "nonexistent", "deeply", "nested")
	mailbox := session.NewMailbox(badPath)

	// Write a corrupt file at the inbox path.
	inboxPath := mailbox.GetInboxPath("agent-a", "team1")
	if err := os.MkdirAll(filepath.Dir(inboxPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inboxPath, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	hook := NewMailboxBridgeHook(MailboxBridgeConfig{
		Mailbox:   mailbox,
		AgentName: "agent-a",
		TeamName:  "team1",
	})

	cmd := hook.pollCmd()
	msg := cmd()
	pollMsg := msg.(MailboxPollMsg)
	if pollMsg.Err == nil {
		t.Error("expected error for corrupt mailbox file")
	}

	// Feed error through Update to verify lastErr is set.
	_, _ = hook.Update(pollMsg)
	if hook.LastError() == nil {
		t.Error("expected LastError to be set after poll error")
	}
}

// ---------------------------------------------------------------------------
// GetRemoteSessionURL tests
// ---------------------------------------------------------------------------

func TestGetRemoteSessionURL_DefaultBase(t *testing.T) {
	url := GetRemoteSessionURL("test-session-id", "")
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	// Should use claude.ai as default.
	if len(url) < 21 || url[:21] != "https://claude.ai/cod" {
		t.Errorf("URL = %q, want prefix https://claude.ai/code/", url)
	}
}

func TestGetRemoteSessionURL_CustomIngress(t *testing.T) {
	url := GetRemoteSessionURL("session-123", "https://my-ingress.example.com")
	expected := "https://my-ingress.example.com/code/"
	if len(url) < len(expected) || url[:len(expected)] != expected {
		t.Errorf("URL = %q, want prefix %q", url, expected)
	}
}

// ---------------------------------------------------------------------------
// BridgeStatus.String tests
// ---------------------------------------------------------------------------

func TestBridgeStatus_String(t *testing.T) {
	tests := []struct {
		status BridgeStatus
		want   string
	}{
		{StatusDisconnected, "disconnected"},
		{StatusConnecting, "connecting"},
		{StatusConnected, "connected"},
		{StatusError, "error"},
		{StatusDisabled, "disabled"},
		{BridgeStatus(99), "BridgeStatus(99)"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("BridgeStatus(%d).String() = %q, want %q", int(tt.status), got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Constants sanity checks
// ---------------------------------------------------------------------------

func TestConstants(t *testing.T) {
	if BridgeFailureDismissMS != 10_000 {
		t.Errorf("BridgeFailureDismissMS = %d, want 10000", BridgeFailureDismissMS)
	}
	if MaxConsecutiveInitFailures != 3 {
		t.Errorf("MaxConsecutiveInitFailures = %d, want 3", MaxConsecutiveInitFailures)
	}
	if ResponseTimeoutMS != 60_000 {
		t.Errorf("ResponseTimeoutMS = %d, want 60000", ResponseTimeoutMS)
	}
	if CompactionTimeoutMS != 180_000 {
		t.Errorf("CompactionTimeoutMS = %d, want 180000", CompactionTimeoutMS)
	}
}

// ---------------------------------------------------------------------------
// Verify tea.Model interface compliance
// ---------------------------------------------------------------------------

var (
	_ tea.Model = (*ReplBridgeHook)(nil)
	_ tea.Model = (*RemoteSessionHook)(nil)
	_ tea.Model = (*MailboxBridgeHook)(nil)
)
