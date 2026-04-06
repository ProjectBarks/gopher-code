// Package bridge provides bubbletea hook models for the CCR bridge protocol:
// REPL bridge mode, remote session lifecycle, and mailbox polling.
//
// Source: src/hooks/useReplBridge.tsx, src/hooks/useRemoteSession.ts,
//
//	src/hooks/useMailboxBridge.ts, src/hooks/useDirectConnect.ts
package bridge

import (
	"context"
	"fmt"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	br "github.com/projectbarks/gopher-code/pkg/bridge"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// ---------------------------------------------------------------------------
// Constants (verbatim match with TS source)
// ---------------------------------------------------------------------------

// BridgeFailureDismissMS is how long after a failure before replBridgeEnabled
// is auto-cleared, stopping retry attempts.
// Source: useReplBridge.tsx BRIDGE_FAILURE_DISMISS_MS
const BridgeFailureDismissMS = 10_000

// MaxConsecutiveInitFailures is the hard limit of init failures before the
// hook stops re-attempting for the session lifetime.
// Source: useReplBridge.tsx MAX_CONSECUTIVE_INIT_FAILURES
const MaxConsecutiveInitFailures = 3

// ResponseTimeoutMS is how long to wait for a response before showing a warning.
// Source: useRemoteSession.ts RESPONSE_TIMEOUT_MS
const ResponseTimeoutMS = 60_000

// CompactionTimeoutMS is the extended timeout during compaction.
// Source: useRemoteSession.ts COMPACTION_TIMEOUT_MS
const CompactionTimeoutMS = 180_000

// DefaultMailboxPollInterval is the default interval for mailbox polling.
const DefaultMailboxPollInterval = 2 * time.Second

// RemoteSessionBaseURL is the base URL for remote session display.
const RemoteSessionBaseURL = "https://claude.ai/code"

// ---------------------------------------------------------------------------
// BridgeStatus — shared status type for bridge connection state
// ---------------------------------------------------------------------------

// BridgeStatus represents the current state of a bridge connection.
type BridgeStatus int

const (
	// StatusDisconnected means no bridge connection is active.
	StatusDisconnected BridgeStatus = iota
	// StatusConnecting means a connection attempt is in progress.
	StatusConnecting
	// StatusConnected means the bridge is active and forwarding messages.
	StatusConnected
	// StatusError means the last connection attempt failed.
	StatusError
	// StatusDisabled means the hook has been permanently disabled (fuse blown).
	StatusDisabled
)

// String returns a human-readable status name.
func (s BridgeStatus) String() string {
	switch s {
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusError:
		return "error"
	case StatusDisabled:
		return "disabled"
	default:
		return fmt.Sprintf("BridgeStatus(%d)", int(s))
	}
}

// ---------------------------------------------------------------------------
// Tea messages — bridge hooks communicate via these
// ---------------------------------------------------------------------------

// BridgeStatusMsg is dispatched when bridge connection status changes.
type BridgeStatusMsg struct {
	Source string       // "repl", "remote", "mailbox"
	Status BridgeStatus
	Err    error // non-nil when Status == StatusError
}

// BridgeInboundMsg carries a message received from the bridge.
type BridgeInboundMsg struct {
	Source string // "repl", "remote", "mailbox"
	Text   string
	UUID   string
}

// MailboxPollMsg carries messages retrieved from a mailbox poll cycle.
type MailboxPollMsg struct {
	Messages []session.TeammateMessage
	Err      error
}

// RemoteSessionURLMsg carries the generated remote session URL for display.
type RemoteSessionURLMsg struct {
	SessionID string
	URL       string
}

// bridgeTickMsg is an internal tick for polling loops.
type bridgeTickMsg struct {
	source string
}

// ---------------------------------------------------------------------------
// ReplBridgeHook — integrates bridge transport into the TUI
// Source: src/hooks/useReplBridge.tsx (722 LOC)
// ---------------------------------------------------------------------------

// ReplBridgeTransport abstracts the bridge connection for testability.
type ReplBridgeTransport interface {
	// Connect initiates a bridge connection. Returns an error if auth fails.
	Connect(ctx context.Context) error
	// Disconnect tears down the bridge connection.
	Disconnect() error
	// Send forwards a message to the bridge session.
	Send(ctx context.Context, evt br.BridgeEvent) error
	// Status returns the current connection status.
	Status() BridgeStatus
}

// ReplBridgeHookConfig configures a ReplBridgeHook.
type ReplBridgeHookConfig struct {
	// Transport is the underlying bridge connection.
	Transport ReplBridgeTransport
	// Enabled controls whether the hook attempts to connect on Init.
	Enabled bool
	// OutboundOnly when true sends local actions but does not subscribe
	// to inbound messages.
	OutboundOnly bool
}

// ReplBridgeHook manages the REPL bridge lifecycle as a bubbletea model.
// It forwards messages, updates status, handles permission prompts from
// remote, and implements auto-disable after consecutive failures.
type ReplBridgeHook struct {
	cfg ReplBridgeHookConfig

	mu                  sync.Mutex
	status              BridgeStatus
	consecutiveFailures int
	lastError           error
	enabled             bool
	outboundOnly        bool
}

// NewReplBridgeHook creates a new REPL bridge hook.
func NewReplBridgeHook(cfg ReplBridgeHookConfig) *ReplBridgeHook {
	return &ReplBridgeHook{
		cfg:          cfg,
		status:       StatusDisconnected,
		enabled:      cfg.Enabled,
		outboundOnly: cfg.OutboundOnly,
	}
}

// Init implements tea.Model. If enabled, starts connecting.
func (h *ReplBridgeHook) Init() tea.Cmd {
	if !h.enabled {
		return nil
	}
	return h.connectCmd()
}

// Update implements tea.Model.
func (h *ReplBridgeHook) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case BridgeStatusMsg:
		if msg.Source != "repl" {
			return h, nil
		}
		h.mu.Lock()
		h.status = msg.Status
		if msg.Err != nil {
			h.lastError = msg.Err
		}
		h.mu.Unlock()
		return h, nil
	}
	return h, nil
}

// View implements tea.Model (bridge hooks are invisible — status is read via accessors).
func (h *ReplBridgeHook) View() tea.View { return tea.NewView("") }

// Status returns the current bridge status (thread-safe).
func (h *ReplBridgeHook) Status() BridgeStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.status
}

// LastError returns the most recent connection error (thread-safe).
func (h *ReplBridgeHook) LastError() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastError
}

// ConsecutiveFailures returns the failure count (thread-safe).
func (h *ReplBridgeHook) ConsecutiveFailures() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.consecutiveFailures
}

// IsOutboundOnly returns whether the hook is in outbound-only mode.
func (h *ReplBridgeHook) IsOutboundOnly() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.outboundOnly
}

// SetEnabled toggles the bridge. When toggled off, sends disconnect status.
// When toggled on, returns a Cmd to initiate connection.
func (h *ReplBridgeHook) SetEnabled(enabled bool) tea.Cmd {
	h.mu.Lock()
	h.enabled = enabled

	if !enabled {
		h.status = StatusDisconnected
		h.mu.Unlock()
		return func() tea.Msg {
			return BridgeStatusMsg{Source: "repl", Status: StatusDisconnected}
		}
	}

	// Check fuse.
	if h.consecutiveFailures >= MaxConsecutiveInitFailures {
		h.status = StatusDisabled
		h.mu.Unlock()
		return func() tea.Msg {
			return BridgeStatusMsg{Source: "repl", Status: StatusDisabled}
		}
	}
	h.mu.Unlock()
	return h.connectCmd()
}

// connectCmd returns a tea.Cmd that attempts to connect the transport.
func (h *ReplBridgeHook) connectCmd() tea.Cmd {
	return func() tea.Msg {
		h.mu.Lock()
		if h.consecutiveFailures >= MaxConsecutiveInitFailures {
			h.status = StatusDisabled
			h.mu.Unlock()
			return BridgeStatusMsg{Source: "repl", Status: StatusDisabled}
		}
		h.status = StatusConnecting
		h.mu.Unlock()

		if h.cfg.Transport == nil {
			h.mu.Lock()
			h.consecutiveFailures++
			h.status = StatusError
			h.lastError = fmt.Errorf("no transport configured")
			h.mu.Unlock()
			return BridgeStatusMsg{
				Source: "repl",
				Status: StatusError,
				Err:    fmt.Errorf("no transport configured"),
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := h.cfg.Transport.Connect(ctx)
		if err != nil {
			h.mu.Lock()
			h.consecutiveFailures++
			h.status = StatusError
			h.lastError = err
			failures := h.consecutiveFailures
			h.mu.Unlock()

			status := StatusError
			if failures >= MaxConsecutiveInitFailures {
				status = StatusDisabled
			}
			return BridgeStatusMsg{Source: "repl", Status: status, Err: err}
		}

		h.mu.Lock()
		h.consecutiveFailures = 0
		h.status = StatusConnected
		h.mu.Unlock()
		return BridgeStatusMsg{Source: "repl", Status: StatusConnected}
	}
}

// ---------------------------------------------------------------------------
// RemoteSessionHook — manages remote session lifecycle, displays URL
// Source: src/hooks/useRemoteSession.ts (605 LOC)
// ---------------------------------------------------------------------------

// RemoteSessionConfig configures a remote session.
type RemoteSessionConfig struct {
	// SessionID is the server-assigned session identifier.
	SessionID string
	// IngressURL is the optional custom ingress URL (empty = default claude.ai).
	IngressURL string
}

// RemoteSessionHook manages the remote session lifecycle as a bubbletea model.
// It generates the session URL for display (e.g., QR code) and tracks connection
// status.
type RemoteSessionHook struct {
	cfg    *RemoteSessionConfig
	mu     sync.Mutex
	status BridgeStatus
	url    string
	echoDedup *br.BoundedUUIDSet
}

// NewRemoteSessionHook creates a remote session hook. Pass nil config for
// non-remote mode (the hook becomes a no-op).
func NewRemoteSessionHook(cfg *RemoteSessionConfig) *RemoteSessionHook {
	return &RemoteSessionHook{
		cfg:       cfg,
		status:    StatusDisconnected,
		echoDedup: br.NewBoundedUUIDSet(256),
	}
}

// Init implements tea.Model. Generates the session URL if configured.
func (h *RemoteSessionHook) Init() tea.Cmd {
	if h.cfg == nil {
		return nil
	}
	return h.generateURLCmd()
}

// Update implements tea.Model.
func (h *RemoteSessionHook) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case BridgeStatusMsg:
		if msg.Source != "remote" {
			return h, nil
		}
		h.mu.Lock()
		h.status = msg.Status
		h.mu.Unlock()
		return h, nil

	case RemoteSessionURLMsg:
		h.mu.Lock()
		h.url = msg.URL
		h.status = StatusConnected
		h.mu.Unlock()
		return h, nil
	}
	return h, nil
}

// View implements tea.Model (invisible).
func (h *RemoteSessionHook) View() tea.View { return tea.NewView("") }

// IsRemoteMode returns true if the hook is configured for remote mode.
func (h *RemoteSessionHook) IsRemoteMode() bool {
	return h.cfg != nil
}

// URL returns the remote session URL (thread-safe). Empty if not yet generated.
func (h *RemoteSessionHook) URL() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.url
}

// Status returns the remote session status (thread-safe).
func (h *RemoteSessionHook) Status() BridgeStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.status
}

// SessionID returns the session ID, or empty if not configured.
func (h *RemoteSessionHook) SessionID() string {
	if h.cfg == nil {
		return ""
	}
	return h.cfg.SessionID
}

// MarkEchoSent records a UUID as sent by this client, so inbound echoes are dropped.
func (h *RemoteSessionHook) MarkEchoSent(uuid string) {
	h.echoDedup.Add(uuid)
}

// IsEcho returns true if the UUID was sent by this client.
func (h *RemoteSessionHook) IsEcho(uuid string) bool {
	return h.echoDedup.Has(uuid)
}

// GetRemoteSessionURL constructs the remote session URL from a session ID and
// optional ingress URL. Matches TS getRemoteSessionUrl from constants/product.ts.
func GetRemoteSessionURL(sessionID string, ingressURL string) string {
	compatID := br.ToCompatSessionID(sessionID)
	base := "https://claude.ai"
	if ingressURL != "" {
		base = ingressURL
	}
	return fmt.Sprintf("%s/code/%s", base, compatID)
}

// generateURLCmd returns a Cmd that computes the remote session URL.
func (h *RemoteSessionHook) generateURLCmd() tea.Cmd {
	return func() tea.Msg {
		url := GetRemoteSessionURL(h.cfg.SessionID, h.cfg.IngressURL)
		return RemoteSessionURLMsg{
			SessionID: h.cfg.SessionID,
			URL:       url,
		}
	}
}

// ---------------------------------------------------------------------------
// MailboxBridgeHook — polls the mailbox for incoming teammate messages
// Source: src/hooks/useMailboxBridge.ts (21 LOC)
// ---------------------------------------------------------------------------

// MailboxBridgeConfig configures a MailboxBridgeHook.
type MailboxBridgeConfig struct {
	// Mailbox is the underlying file-based mailbox.
	Mailbox *session.Mailbox
	// AgentName is this agent's name for inbox lookups.
	AgentName string
	// TeamName is the team to poll (empty = "default").
	TeamName string
	// PollInterval overrides DefaultMailboxPollInterval (zero = use default).
	PollInterval time.Duration
	// OnMessage is called when a new message is received. If it returns false,
	// the message is re-queued (the hook does not poll while loading).
	OnMessage func(msg session.TeammateMessage) bool
}

func (c *MailboxBridgeConfig) pollInterval() time.Duration {
	if c.PollInterval > 0 {
		return c.PollInterval
	}
	return DefaultMailboxPollInterval
}

// MailboxBridgeHook polls the teammate mailbox and dispatches new messages
// as bubbletea Msgs.
type MailboxBridgeHook struct {
	cfg MailboxBridgeConfig

	mu        sync.Mutex
	isLoading bool
	lastPoll  time.Time
	lastErr   error
}

// NewMailboxBridgeHook creates a mailbox bridge hook.
func NewMailboxBridgeHook(cfg MailboxBridgeConfig) *MailboxBridgeHook {
	return &MailboxBridgeHook{cfg: cfg}
}

// Init implements tea.Model. Starts the first poll tick.
func (h *MailboxBridgeHook) Init() tea.Cmd {
	if h.cfg.Mailbox == nil {
		return nil
	}
	return h.tickCmd()
}

// Update implements tea.Model.
func (h *MailboxBridgeHook) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bridgeTickMsg:
		if msg.source != "mailbox" {
			return h, nil
		}
		return h, h.pollCmd()

	case MailboxPollMsg:
		h.mu.Lock()
		h.lastPoll = time.Now()
		h.lastErr = msg.Err
		h.mu.Unlock()

		// Schedule next tick regardless of error.
		return h, h.tickCmd()
	}
	return h, nil
}

// View implements tea.Model (invisible).
func (h *MailboxBridgeHook) View() tea.View { return tea.NewView("") }

// SetLoading sets whether the main loop is busy (suppresses poll dispatch).
func (h *MailboxBridgeHook) SetLoading(loading bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.isLoading = loading
}

// LastError returns the error from the most recent poll, if any (thread-safe).
func (h *MailboxBridgeHook) LastError() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastErr
}

// LastPollTime returns the time of the most recent poll (thread-safe).
func (h *MailboxBridgeHook) LastPollTime() time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastPoll
}

// tickCmd returns a tea.Cmd that waits for the poll interval then sends a tick.
func (h *MailboxBridgeHook) tickCmd() tea.Cmd {
	interval := h.cfg.pollInterval()
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return bridgeTickMsg{source: "mailbox"}
	})
}

// pollCmd returns a tea.Cmd that reads unread messages from the mailbox.
func (h *MailboxBridgeHook) pollCmd() tea.Cmd {
	return func() tea.Msg {
		h.mu.Lock()
		loading := h.isLoading
		h.mu.Unlock()

		// Don't poll while main loop is busy (matches TS: if (isLoading) return).
		if loading {
			return MailboxPollMsg{}
		}

		if h.cfg.Mailbox == nil {
			return MailboxPollMsg{}
		}

		messages, err := h.cfg.Mailbox.ReadUnreadMessages(h.cfg.AgentName, h.cfg.TeamName)
		if err != nil {
			return MailboxPollMsg{Err: err}
		}

		return MailboxPollMsg{Messages: messages}
	}
}
