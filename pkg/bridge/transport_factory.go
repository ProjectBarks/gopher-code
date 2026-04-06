// Package bridge — GetTransportForUrl: transport factory selecting SSE/Hybrid/WebSocket.
// Source: src/cli/transports/transportUtils.ts
package bridge

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// TransportKind identifies which transport type was selected by the factory.
type TransportKind int

const (
	// TransportKindSSE selects SSETransport (SSE reads + POST writes).
	TransportKindSSE TransportKind = iota
	// TransportKindHybrid selects HybridTransport (WS reads + POST writes).
	TransportKindHybrid
	// TransportKindWebSocket selects WebSocketTransport (WS reads + WS writes).
	TransportKindWebSocket
)

// TransportSelection is the result of GetTransportForUrl.
// It describes which transport to create and the resolved URL.
type TransportSelection struct {
	Kind      TransportKind
	URL       *url.URL
	Headers   map[string]string
	SessionID string
}

// GetTransportForUrl selects the appropriate transport based on the session URL
// scheme and environment variables.
//
// Transport selection priority:
//  1. SSETransport when CLAUDE_CODE_USE_CCR_V2 is set — derives the SSE stream
//     URL by swapping ws→http / wss→https and appending /worker/events/stream.
//  2. HybridTransport when CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2 is set and
//     the URL scheme is ws/wss.
//  3. WebSocketTransport (default) when the URL scheme is ws/wss.
//
// Returns an error for unsupported URL schemes when CCR_V2 is not active.
func GetTransportForUrl(
	rawURL string,
	headers map[string]string,
	sessionID string,
) (*TransportSelection, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	// Priority 1: SSE transport (CCR v2).
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_CCR_V2")) {
		sseURL := *u
		switch sseURL.Scheme {
		case "wss":
			sseURL.Scheme = "https"
		case "ws":
			sseURL.Scheme = "http"
		}
		sseURL.Path = strings.TrimRight(sseURL.Path, "/") + "/worker/events/stream"
		return &TransportSelection{
			Kind:      TransportKindSSE,
			URL:       &sseURL,
			Headers:   headers,
			SessionID: sessionID,
		}, nil
	}

	// Non-SSE path requires ws/wss scheme.
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("Unsupported protocol: %s:", u.Scheme)
	}

	// Priority 2: Hybrid transport (POST for session ingress v2).
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2")) {
		return &TransportSelection{
			Kind:      TransportKindHybrid,
			URL:       u,
			Headers:   headers,
			SessionID: sessionID,
		}, nil
	}

	// Priority 3: WebSocket transport (default).
	return &TransportSelection{
		Kind:      TransportKindWebSocket,
		URL:       u,
		Headers:   headers,
		SessionID: sessionID,
	}, nil
}
