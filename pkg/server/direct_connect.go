// Package server — createDirectConnectSession: HTTP session creator for
// direct-connect mode. POSTs to /sessions on a direct-connect server and
// returns a DirectConnectConfig ready for the session manager.
//
// Source: src/server/createDirectConnectSession.ts
// Tasks: T100 (session creator), T101 (DirectConnectError), T102 (POST shape + Bearer)
package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ---------------------------------------------------------------------------
// T101: DirectConnectError
// ---------------------------------------------------------------------------

// DirectConnectError is returned by CreateDirectConnectSession when the
// connection, HTTP request, or response parsing fails.
// Matches TS DirectConnectError (name = "DirectConnectError").
type DirectConnectError struct {
	Msg string
}

func (e *DirectConnectError) Error() string { return e.Msg }

// IsDirectConnectError returns true if err is a *DirectConnectError.
func IsDirectConnectError(err error) bool {
	_, ok := err.(*DirectConnectError)
	return ok
}

// ---------------------------------------------------------------------------
// T100 + T102: CreateDirectConnectSession
// ---------------------------------------------------------------------------

// CreateDirectConnectSessionOpts are the parameters for creating a session.
type CreateDirectConnectSessionOpts struct {
	// ServerURL is the base URL of the direct-connect server (e.g. "http://localhost:8080").
	ServerURL string
	// AuthToken is the optional Bearer token for authentication.
	AuthToken string
	// CWD is the working directory for the session.
	CWD string
	// DangerouslySkipPermissions, when true, tells the server to skip permission checks.
	DangerouslySkipPermissions bool
}

// CreateDirectConnectSessionResult is the return value on success.
type CreateDirectConnectSessionResult struct {
	Config  DirectConnectConfig
	WorkDir string // optional; may be empty
}

// CreateDirectConnectSession POSTs to ${serverUrl}/sessions to create a new
// direct-connect session. Returns a DirectConnectConfig on success or a
// *DirectConnectError on any failure (network, HTTP, or parse).
//
// Uses the provided http.Client (or http.DefaultClient if nil).
func CreateDirectConnectSession(client *http.Client, opts CreateDirectConnectSessionOpts) (*CreateDirectConnectSessionResult, error) {
	if client == nil {
		client = http.DefaultClient
	}

	// Build request body — T102
	body := map[string]any{
		"cwd": opts.CWD,
	}
	if opts.DangerouslySkipPermissions {
		body["dangerously_skip_permissions"] = true
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, &DirectConnectError{Msg: fmt.Sprintf("Failed to connect to server at %s: %s", opts.ServerURL, err)}
	}

	url := opts.ServerURL + "/sessions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &DirectConnectError{Msg: fmt.Sprintf("Failed to connect to server at %s: %s", opts.ServerURL, err)}
	}

	req.Header.Set("Content-Type", "application/json")

	// T102: Bearer auth
	if opts.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+opts.AuthToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, &DirectConnectError{Msg: fmt.Sprintf("Failed to connect to server at %s: %s", opts.ServerURL, err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &DirectConnectError{Msg: fmt.Sprintf("Failed to create session: %d %s", resp.StatusCode, resp.Status)}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &DirectConnectError{Msg: fmt.Sprintf("Failed to connect to server at %s: %s", opts.ServerURL, err)}
	}

	var cr ConnectResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return nil, &DirectConnectError{Msg: fmt.Sprintf("Invalid session response: %s", err)}
	}

	if err := ValidateConnectResponse(cr); err != nil {
		return nil, &DirectConnectError{Msg: fmt.Sprintf("Invalid session response: %s", err)}
	}

	return &CreateDirectConnectSessionResult{
		Config: DirectConnectConfig{
			ServerURL: opts.ServerURL,
			SessionID: cr.SessionID,
			WSURL:     cr.WSURL,
			AuthToken: opts.AuthToken,
		},
		WorkDir: cr.WorkDir,
	}, nil
}
