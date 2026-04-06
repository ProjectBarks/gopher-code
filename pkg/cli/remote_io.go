// RemoteIO — bidirectional stream-JSON I/O wrapper for remote sessions.
// Extends StructuredIO by adding a network transport (WebSocket v1 or
// SSE/CCR v2), session-token auth headers, environment-runner-version
// header, keep-alive timer, and session-state/metadata change listeners.
// Source: src/cli/remoteIO.ts
package cli

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/projectbarks/gopher-code/pkg/bridge"
)

// ---------------------------------------------------------------------------
// Transport abstraction (v1 WS / v2 SSE+POST)
// ---------------------------------------------------------------------------

// Transport is the interface that both WS and SSE transports implement.
type Transport interface {
	// Connect initiates the connection.
	Connect() error
	// Write sends a message to the remote.
	Write(msg any) error
	// SetOnData registers a callback for incoming data.
	SetOnData(fn func(data string))
	// SetOnClose registers a callback for connection close.
	SetOnClose(fn func())
	// Close tears down the connection.
	Close()
}

// TransportVersion discriminates v1 (WS) from v2 (SSE+POST).
type TransportVersion int

const (
	TransportV1 TransportVersion = iota // WebSocket
	TransportV2                         // SSE + POST (CCR v2)
)

// ---------------------------------------------------------------------------
// TransportFactory — pluggable constructor for tests
// ---------------------------------------------------------------------------

// TransportFactory creates a Transport for the given URL, headers, session ID,
// and a dynamic header refresh callback.
type TransportFactory func(
	u *url.URL,
	headers http.Header,
	sessionID string,
	refreshHeaders func() http.Header,
) Transport

// ---------------------------------------------------------------------------
// TokenSource — pluggable auth-token provider
// ---------------------------------------------------------------------------

// TokenSource returns the current session-ingress auth token, or "" if none.
type TokenSource func() string

// ---------------------------------------------------------------------------
// RemoteIO
// ---------------------------------------------------------------------------

// RemoteIO wraps a Transport with authentication headers, keep-alive, and
// debug logging for remote (bridge/CCR) sessions. It pipes incoming data
// into an io.Reader that callers can consume (the structured-IO input side).
type RemoteIO struct {
	mu sync.Mutex

	url       *url.URL
	transport Transport
	version   TransportVersion
	isBridge  bool
	isDebug   bool
	logger    *slog.Logger

	// inputReader is the read-half that callers consume.
	inputReader *io.PipeReader
	// inputWriter is the write-half that the transport pushes data into.
	inputWriter *io.PipeWriter

	keepAliveTicker *time.Ticker
	keepAliveDone   chan struct{}

	// SessionStateChangedFn is called when session state changes (v2 only).
	SessionStateChangedFn func(state string, details map[string]string)
	// SessionMetadataChangedFn is called when session metadata changes (v2 only).
	SessionMetadataChangedFn func(metadata map[string]string)

	closed bool
}

// RemoteIOConfig holds the constructor parameters for NewRemoteIO.
type RemoteIOConfig struct {
	// StreamURL is the session-ingress URL (ws:// or https://).
	StreamURL string
	// SessionID passed to the transport factory.
	SessionID string
	// TokenSource provides the session-ingress auth token.
	TokenSource TokenSource
	// TransportFactory builds the concrete Transport.
	TransportFactory TransportFactory
	// PollConfig supplies the keep-alive interval.
	PollConfig bridge.PollIntervalConfig
	// Logger for debug output (defaults to slog.Default).
	Logger *slog.Logger
}

// NewRemoteIO constructs a RemoteIO. Transport version is selected by the
// CLAUDE_CODE_USE_CCR_V2 environment variable. Headers are built from
// CLAUDE_CODE_ENVIRONMENT_RUNNER_VERSION and the TokenSource.
func NewRemoteIO(cfg RemoteIOConfig) (*RemoteIO, error) {
	u, err := url.Parse(cfg.StreamURL)
	if err != nil {
		return nil, fmt.Errorf("parse stream URL: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Determine transport version from env.
	version := resolveTransportVersion()

	// Build initial headers.
	headers := buildHeaders(cfg.TokenSource, logger)

	// For v2, rewrite the URL to the SSE stream endpoint.
	transportURL := u
	if version == TransportV2 {
		transportURL = deriveSSEURL(u)
	}

	// Header-refresh closure (re-reads token + runner version dynamically).
	refreshHeaders := func() http.Header {
		return buildHeaders(cfg.TokenSource, logger)
	}

	// Create the transport.
	transport := cfg.TransportFactory(transportURL, headers, cfg.SessionID, refreshHeaders)

	pr, pw := io.Pipe()

	isBridgeEnv := os.Getenv("CLAUDE_CODE_ENVIRONMENT_KIND") == "bridge"

	rio := &RemoteIO{
		url:         u,
		transport:   transport,
		version:     version,
		isBridge:    isBridgeEnv,
		isDebug:     os.Getenv("CLAUDE_CODE_DEBUG") != "",
		logger:      logger,
		inputReader: pr,
		inputWriter: pw,
	}

	// Wire data callback → pipe writer.
	transport.SetOnData(func(data string) {
		// Ensure trailing newline for NDJSON framing.
		if !strings.HasSuffix(data, "\n") {
			data += "\n"
		}
		_, _ = io.WriteString(pw, data)
	})

	// Wire close callback → end input pipe.
	transport.SetOnClose(func() {
		_ = pw.Close()
	})

	// Start keep-alive timer (bridge-only, interval > 0).
	keepAliveInterval := cfg.PollConfig.SessionKeepaliveInterval
	if isBridgeEnv && keepAliveInterval > 0 {
		rio.startKeepAlive(keepAliveInterval)
	}

	return rio, nil
}

// Reader returns the read-half of the input pipe. Callers (e.g. StructuredIO)
// read NDJSON lines from this.
func (r *RemoteIO) Reader() io.Reader {
	return r.inputReader
}

// Write sends a message through the transport (or CCR client for v2).
func (r *RemoteIO) Write(msg any) error {
	return r.transport.Write(msg)
}

// Connect initiates the underlying transport connection.
func (r *RemoteIO) Connect() error {
	return r.transport.Connect()
}

// Version returns the selected transport version.
func (r *RemoteIO) Version() TransportVersion {
	return r.version
}

// Close tears down the keep-alive timer, transport, and input pipe.
func (r *RemoteIO) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true

	r.stopKeepAlive()
	r.transport.Close()
	_ = r.inputWriter.Close()
}

// ---------------------------------------------------------------------------
// Keep-alive
// ---------------------------------------------------------------------------

func (r *RemoteIO) startKeepAlive(interval time.Duration) {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	r.mu.Lock()
	r.keepAliveTicker = ticker
	r.keepAliveDone = done
	r.mu.Unlock()

	go func() {
		for {
			select {
			case <-ticker.C:
				r.logger.Debug("[remote-io] keep_alive sent")
				if err := r.transport.Write(map[string]string{"type": "keep_alive"}); err != nil {
					r.logger.Debug("[remote-io] keep_alive write failed", "err", err)
				}
			case <-done:
				return
			}
		}
	}()
}

func (r *RemoteIO) stopKeepAlive() {
	if r.keepAliveTicker != nil {
		r.keepAliveTicker.Stop()
		close(r.keepAliveDone)
		r.keepAliveTicker = nil
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resolveTransportVersion reads CLAUDE_CODE_USE_CCR_V2 to pick v1 or v2.
func resolveTransportVersion() TransportVersion {
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_CCR_V2")) {
		return TransportV2
	}
	return TransportV1
}

// buildHeaders constructs the initial/refresh header set.
func buildHeaders(tokenSrc TokenSource, logger *slog.Logger) http.Header {
	h := make(http.Header)

	if tokenSrc != nil {
		tok := tokenSrc()
		if tok != "" {
			h.Set("Authorization", "Bearer "+tok)
		} else if logger != nil {
			logger.Debug("[remote-io] No session ingress token available")
		}
	}

	if v := os.Getenv("CLAUDE_CODE_ENVIRONMENT_RUNNER_VERSION"); v != "" {
		h.Set("x-environment-runner-version", v)
	}

	return h
}

// deriveSSEURL converts a session URL to the v2 SSE stream endpoint.
// wss:→https:, ws:→http:, appends /worker/events/stream.
func deriveSSEURL(u *url.URL) *url.URL {
	cp := *u
	switch cp.Scheme {
	case "wss":
		cp.Scheme = "https"
	case "ws":
		cp.Scheme = "http"
	}
	cp.Path = strings.TrimRight(cp.Path, "/") + "/worker/events/stream"
	return &cp
}

// isEnvTruthy returns true for "1", "true", "yes" (case-insensitive).
func isEnvTruthy(val string) bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "true", "yes":
		return true
	}
	return false
}
