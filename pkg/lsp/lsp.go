// Package lsp provides a Language Server Protocol client and server manager.
//
// Source: services/lsp/manager.ts, services/lsp/LSPClient.ts, services/lsp/LSPServerManager.ts
//
// The LSP subsystem manages one language server per file extension. The Manager
// is a singleton created during startup. Each server is a child process
// speaking JSON-RPC 2.0 over stdio, managed by a Client.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// ---------------------------------------------------------------------------
// Client — JSON-RPC 2.0 over stdio to a single LSP server process
// Source: services/lsp/LSPClient.ts
// ---------------------------------------------------------------------------

// Client communicates with a single LSP server process via JSON-RPC stdio.
type Client struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	writeMu   sync.Mutex
	pendMu    sync.Mutex
	nextID    atomic.Int64
	pending   map[int64]chan *jsonRPCResponse
	caps      json.RawMessage // server capabilities from initialize response
	initDone  bool
	stopping  bool
}

// NewClient spawns an LSP server process and initializes the connection.
func NewClient(ctx context.Context, command string, args []string, cwd string) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = os.Environ()
	if cwd != "" {
		cmd.Dir = cwd
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start LSP server %q: %w", command, err)
	}

	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		pending: make(map[int64]chan *jsonRPCResponse),
	}
	go c.readLoop()

	return c, nil
}

// Initialize sends the LSP initialize request and initialized notification.
func (c *Client) Initialize(ctx context.Context, rootURI string) error {
	params := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   rootURI,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"hover":          map[string]interface{}{"contentFormat": []string{"plaintext", "markdown"}},
				"definition":     map[string]interface{}{},
				"references":     map[string]interface{}{},
				"documentSymbol": map[string]interface{}{},
				"implementation": map[string]interface{}{},
			},
			"workspace": map[string]interface{}{
				"symbol": map[string]interface{}{},
			},
		},
		"clientInfo": map[string]interface{}{
			"name":    "gopher-code",
			"version": "0.1.0",
		},
	}

	result, err := c.call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	c.caps = result
	c.initDone = true

	// Send initialized notification
	c.notify("initialized", map[string]interface{}{})
	return nil
}

// SendRequest sends an LSP request and returns the raw JSON result.
func (c *Client) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if !c.initDone {
		return nil, fmt.Errorf("LSP server not initialized")
	}
	return c.call(ctx, method, params)
}

// SendNotification sends an LSP notification (no response expected).
func (c *Client) SendNotification(method string, params interface{}) {
	c.notify(method, params)
}

// IsInitialized returns true if the server has completed initialization.
func (c *Client) IsInitialized() bool { return c.initDone }

// Capabilities returns the raw server capabilities JSON.
func (c *Client) Capabilities() json.RawMessage { return c.caps }

// Stop shuts down the LSP server gracefully.
func (c *Client) Stop() error {
	c.stopping = true
	// Send shutdown request
	ctx := context.Background()
	c.call(ctx, "shutdown", nil) //nolint:errcheck
	c.notify("exit", nil)
	c.stdin.Close()
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Wait()
	}
	return nil
}

// --- JSON-RPC internals ---

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int64      `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonRPCError) Error() string {
	return fmt.Sprintf("LSP error %d: %s", e.Code, e.Message)
}

func (c *Client) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	ch := make(chan *jsonRPCResponse, 1)

	c.pendMu.Lock()
	c.pending[id] = ch
	c.pendMu.Unlock()

	defer func() {
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
	}()

	req := jsonRPCRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params}
	if err := c.writeMessage(req); err != nil {
		return nil, err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("connection closed")
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Client) notify(method string, params interface{}) {
	req := jsonRPCRequest{JSONRPC: "2.0", Method: method, Params: params}
	c.writeMessage(req) //nolint:errcheck
}

func (c *Client) writeMessage(req jsonRPCRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	_, err = c.stdin.Write(data)
	return err
}

func (c *Client) readLoop() {
	for {
		// Read headers
		var contentLength int
		for {
			line, err := c.stdout.ReadString('\n')
			if err != nil {
				c.closeAllPending()
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break // end of headers
			}
			if strings.HasPrefix(line, "Content-Length:") {
				fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			}
		}
		if contentLength <= 0 {
			continue
		}

		// Read body
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(c.stdout, body); err != nil {
			c.closeAllPending()
			return
		}

		var resp jsonRPCResponse
		if json.Unmarshal(body, &resp) != nil {
			continue
		}
		if resp.ID == nil {
			continue // notification from server, skip
		}

		c.pendMu.Lock()
		ch, ok := c.pending[*resp.ID]
		c.pendMu.Unlock()
		if ok {
			ch <- &resp
		}
	}
}

func (c *Client) closeAllPending() {
	c.pendMu.Lock()
	for _, ch := range c.pending {
		close(ch)
	}
	c.pendMu.Unlock()
}

// ---------------------------------------------------------------------------
// Manager — manages multiple language server instances by file extension
// Source: services/lsp/manager.ts, services/lsp/LSPServerManager.ts
// ---------------------------------------------------------------------------

// InitStatus is the initialization state of the LSP manager.
type InitStatus string

const (
	InitNotStarted InitStatus = "not-started"
	InitPending    InitStatus = "pending"
	InitSuccess    InitStatus = "success"
	InitFailed     InitStatus = "failed"
)

// ServerState describes a language server's health.
type ServerState string

const (
	ServerReady ServerState = "ready"
	ServerError ServerState = "error"
)

// ServerEntry is a managed language server instance.
type ServerEntry struct {
	Client    *Client
	Command   string
	Args      []string
	State     ServerState
	Extension string // file extension this server handles
}

// Manager manages multiple LSP server instances, one per file extension.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*ServerEntry // extension → server
	rootURI string
	status  InitStatus
	initErr error
}

// NewManager creates a new LSP manager for the given project root.
func NewManager(rootDir string) *Manager {
	rootURI := "file://" + rootDir
	return &Manager{
		servers: make(map[string]*ServerEntry),
		rootURI: rootURI,
		status:  InitNotStarted,
	}
}

// Status returns the current initialization status.
func (m *Manager) Status() InitStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// IsConnected returns true if at least one healthy server is available.
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.servers {
		if s.State == ServerReady {
			return true
		}
	}
	return false
}

// RegisterServer registers a language server for a file extension.
// The server is started lazily on first request for that extension.
func (m *Manager) RegisterServer(ext, command string, args []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[ext] = &ServerEntry{
		Command:   command,
		Args:      args,
		Extension: ext,
		State:     ServerReady,
	}
}

// SendRequest sends an LSP request to the server handling the given file.
// Returns nil result if no server is available for the file type.
func (m *Manager) SendRequest(ctx context.Context, filePath, method string, params json.RawMessage) (json.RawMessage, error) {
	ext := filepath.Ext(filePath)
	m.mu.RLock()
	entry := m.servers[ext]
	m.mu.RUnlock()

	if entry == nil {
		return nil, nil // no server for this extension
	}

	// Lazy start: create client on first use
	if entry.Client == nil {
		m.mu.Lock()
		if entry.Client == nil { // double-check under write lock
			client, err := NewClient(ctx, entry.Command, entry.Args, "")
			if err != nil {
				entry.State = ServerError
				m.mu.Unlock()
				return nil, fmt.Errorf("start LSP server for %s: %w", ext, err)
			}
			if err := client.Initialize(ctx, m.rootURI); err != nil {
				client.Stop()
				entry.State = ServerError
				m.mu.Unlock()
				return nil, fmt.Errorf("initialize LSP server for %s: %w", ext, err)
			}
			entry.Client = client
		}
		m.mu.Unlock()
	}

	if entry.State == ServerError {
		return nil, nil
	}

	var p interface{}
	if params != nil {
		json.Unmarshal(params, &p)
	}
	return entry.Client.SendRequest(ctx, method, p)
}

// Shutdown stops all managed servers.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range m.servers {
		if entry.Client != nil {
			entry.Client.Stop()
		}
	}
}

// ServerCount returns the number of registered servers.
func (m *Manager) ServerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.servers)
}

// Extensions returns all registered file extensions.
func (m *Manager) Extensions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	exts := make([]string, 0, len(m.servers))
	for ext := range m.servers {
		exts = append(exts, ext)
	}
	return exts
}
