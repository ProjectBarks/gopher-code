package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

// MCPClient manages a connection to an MCP server process.
type MCPClient struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Reader
	writeMu  sync.Mutex // guards stdin writes
	pendMu   sync.Mutex // guards pending map
	nextID   atomic.Int64
	pending  map[int64]chan *jsonRPCResponse
}

// ServerConfig describes how to start an MCP server.
type ServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// ToolInfo describes a tool provided by the MCP server.
type ToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// NewClient starts an MCP server process and initializes the connection.
func NewClient(ctx context.Context, cfg ServerConfig) (*MCPClient, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		env := os.Environ()
		for k, v := range cfg.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr // MCP servers log to stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start MCP server: %w", err)
	}

	c := &MCPClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		pending: make(map[int64]chan *jsonRPCResponse),
	}

	// Start reader goroutine
	go c.readLoop()

	// Send initialize
	_, err = c.call(ctx, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "gopher-code",
			"version": "0.1.0",
		},
	})
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	// Send initialized notification
	c.notify("notifications/initialized", nil)

	return c, nil
}

// newClientFromPipes creates an MCPClient from pre-wired pipes (for testing).
func newClientFromPipes(stdin io.WriteCloser, stdout io.Reader, cmd *exec.Cmd) *MCPClient {
	return &MCPClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		pending: make(map[int64]chan *jsonRPCResponse),
	}
}

// ListTools returns all tools provided by the server.
func (c *MCPClient) ListTools(ctx context.Context) ([]ToolInfo, error) {
	result, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Tools []ToolInfo `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal tools/list: %w", err)
	}
	return resp.Tools, nil
}

// CallTool invokes a tool on the server.
func (c *MCPClient) CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": json.RawMessage(args),
	}
	return c.call(ctx, "tools/call", params)
}

// Close shuts down the MCP server.
func (c *MCPClient) Close() error {
	c.stdin.Close()
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Wait()
	}
	return nil
}

// Internal JSON-RPC types

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
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

func (c *MCPClient) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
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
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	c.writeMu.Lock()
	_, writeErr := c.stdin.Write(data)
	c.writeMu.Unlock()
	if writeErr != nil {
		return nil, fmt.Errorf("write request: %w", writeErr)
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

func (c *MCPClient) notify(method string, params interface{}) {
	req := jsonRPCRequest{JSONRPC: "2.0", Method: method, Params: params}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	c.writeMu.Lock()
	c.stdin.Write(data)
	c.writeMu.Unlock()
}

func (c *MCPClient) readLoop() {
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			// Close all pending channels to unblock callers
			c.pendMu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pendMu.Unlock()
			return
		}

		var resp jsonRPCResponse
		if json.Unmarshal(line, &resp) != nil {
			continue
		}
		if resp.ID == nil {
			continue // notification, skip
		}

		c.pendMu.Lock()
		ch, ok := c.pending[*resp.ID]
		c.pendMu.Unlock()

		if ok {
			ch <- &resp
		}
	}
}
