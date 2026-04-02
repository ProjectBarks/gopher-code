package mcp

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// mockServer simulates an MCP server by reading JSON-RPC requests from
// serverIn and writing JSON-RPC responses to serverOut.
type mockServer struct {
	serverIn  io.Reader // server reads requests from here (client's stdin)
	serverOut io.Writer // server writes responses here (client's stdout)
	handler   func(method string, id int64, params json.RawMessage) json.RawMessage
}

func (m *mockServer) run() {
	buf := make([]byte, 4096)
	var leftover string
	for {
		n, err := m.serverIn.Read(buf)
		if err != nil {
			return
		}
		leftover += string(buf[:n])

		// Process complete lines
		for {
			idx := strings.Index(leftover, "\n")
			if idx == -1 {
				break
			}
			line := leftover[:idx]
			leftover = leftover[idx+1:]

			var req struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      *int64          `json:"id,omitempty"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
			}
			if json.Unmarshal([]byte(line), &req) != nil {
				continue
			}

			// Notifications have no ID - no response needed
			if req.ID == nil {
				continue
			}

			result := m.handler(req.Method, *req.ID, req.Params)
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"result":  json.RawMessage(result),
			}
			data, _ := json.Marshal(resp)
			data = append(data, '\n')
			m.serverOut.Write(data)
		}
	}
}

// setupMockClient creates an MCPClient connected to a mock server via pipes.
// The handler function processes requests and returns result payloads.
func setupMockClient(t *testing.T, handler func(method string, id int64, params json.RawMessage) json.RawMessage) *MCPClient {
	t.Helper()

	// clientWrite -> serverRead (client stdin pipe)
	serverRead, clientWrite := io.Pipe()
	// serverWrite -> clientRead (client stdout pipe)
	clientRead, serverWrite := io.Pipe()

	mock := &mockServer{
		serverIn:  serverRead,
		serverOut: serverWrite,
		handler:   handler,
	}
	go mock.run()

	client := newClientFromPipes(clientWrite, clientRead, nil)
	go client.readLoop()

	t.Cleanup(func() {
		clientWrite.Close()
		serverRead.Close()
		clientRead.Close()
		serverWrite.Close()
	})

	return client
}

func TestJSONRPCRequestSerialization(t *testing.T) {
	id := int64(1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "initialize",
		Params:  map[string]interface{}{"key": "value"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var decoded jsonRPCRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if decoded.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", decoded.JSONRPC, "2.0")
	}
	if decoded.Method != "initialize" {
		t.Errorf("method = %q, want %q", decoded.Method, "initialize")
	}
	if decoded.ID == nil || *decoded.ID != 1 {
		t.Errorf("id = %v, want 1", decoded.ID)
	}
}

func TestJSONRPCResponseSerialization(t *testing.T) {
	id := int64(42)
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      &id,
		Result:  json.RawMessage(`{"tools":[]}`),
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var decoded jsonRPCResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if decoded.ID == nil || *decoded.ID != 42 {
		t.Errorf("id = %v, want 42", decoded.ID)
	}
	if string(decoded.Result) != `{"tools":[]}` {
		t.Errorf("result = %s, want {\"tools\":[]}", decoded.Result)
	}
}

func TestJSONRPCErrorSerialization(t *testing.T) {
	id := int64(1)
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      &id,
		Error: &jsonRPCError{
			Code:    -32601,
			Message: "Method not found",
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error response: %v", err)
	}

	var decoded jsonRPCResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if decoded.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if decoded.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", decoded.Error.Code)
	}
	if decoded.Error.Message != "Method not found" {
		t.Errorf("error message = %q, want %q", decoded.Error.Message, "Method not found")
	}
}

func TestNotificationSerialization(t *testing.T) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}
	var decoded jsonRPCRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	if decoded.ID != nil {
		t.Errorf("notification should have nil ID, got %v", *decoded.ID)
	}
	if decoded.Method != "notifications/initialized" {
		t.Errorf("method = %q, want %q", decoded.Method, "notifications/initialized")
	}
}

func TestListTools(t *testing.T) {
	handler := func(method string, id int64, params json.RawMessage) json.RawMessage {
		switch method {
		case "tools/list":
			return json.RawMessage(`{"tools":[{"name":"echo","description":"Echoes input","inputSchema":{"type":"object","properties":{"message":{"type":"string"}}}}]}`)
		default:
			return json.RawMessage(`{}`)
		}
	}

	client := setupMockClient(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools))
	}
	if tools[0].Name != "echo" {
		t.Errorf("tool name = %q, want %q", tools[0].Name, "echo")
	}
	if tools[0].Description != "Echoes input" {
		t.Errorf("tool description = %q, want %q", tools[0].Description, "Echoes input")
	}
}

func TestCallTool(t *testing.T) {
	handler := func(method string, id int64, params json.RawMessage) json.RawMessage {
		switch method {
		case "tools/call":
			var p struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			json.Unmarshal(params, &p)
			var args struct {
				Message string `json:"message"`
			}
			json.Unmarshal(p.Arguments, &args)
			return json.RawMessage(`{"content":[{"type":"text","text":"` + args.Message + `"}],"isError":false}`)
		default:
			return json.RawMessage(`{}`)
		}
	}

	client := setupMockClient(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.CallTool(ctx, "echo", json.RawMessage(`{"message":"hello"}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &callResult); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(callResult.Content) != 1 {
		t.Fatalf("got %d content items, want 1", len(callResult.Content))
	}
	if callResult.Content[0].Text != "hello" {
		t.Errorf("text = %q, want %q", callResult.Content[0].Text, "hello")
	}
	if callResult.IsError {
		t.Error("unexpected isError=true")
	}
}

func TestCallToolError(t *testing.T) {
	// Custom pipe-based server that returns JSON-RPC error responses
	serverRead, clientWrite := io.Pipe()
	clientRead, serverWrite := io.Pipe()

	client := newClientFromPipes(clientWrite, clientRead, nil)
	go client.readLoop()

	t.Cleanup(func() {
		clientWrite.Close()
		serverRead.Close()
		clientRead.Close()
		serverWrite.Close()
	})

	// Server that returns an error for every request
	go func() {
		buf := make([]byte, 4096)
		var leftover string
		for {
			n, err := serverRead.Read(buf)
			if err != nil {
				return
			}
			leftover += string(buf[:n])
			for {
				idx := strings.Index(leftover, "\n")
				if idx == -1 {
					break
				}
				line := leftover[:idx]
				leftover = leftover[idx+1:]

				var req struct {
					ID *int64 `json:"id"`
				}
				if json.Unmarshal([]byte(line), &req) != nil || req.ID == nil {
					continue
				}

				errResp := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      *req.ID,
					"error": map[string]interface{}{
						"code":    -32601,
						"message": "tool not found",
					},
				}
				data, _ := json.Marshal(errResp)
				data = append(data, '\n')
				serverWrite.Write(data)
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.CallTool(ctx, "nonexistent", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tool not found") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "tool not found")
	}
}

func TestCallToolTimeout(t *testing.T) {
	// Server that reads requests but never responds
	serverRead, clientWrite := io.Pipe()
	clientRead, _ := io.Pipe()

	client := newClientFromPipes(clientWrite, clientRead, nil)
	go client.readLoop()

	// Drain serverRead so writes don't block
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := serverRead.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	t.Cleanup(func() {
		clientWrite.Close()
		serverRead.Close()
		clientRead.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.call(ctx, "tools/list", nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("error = %q, want context deadline exceeded", err.Error())
	}
}

func TestMultipleConcurrentCalls(t *testing.T) {
	handler := func(method string, id int64, params json.RawMessage) json.RawMessage {
		return json.RawMessage(`{"tools":[]}`)
	}

	client := setupMockClient(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make 5 concurrent calls
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := client.ListTools(ctx)
			errs <- err
		}()
	}

	for i := 0; i < 5; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent call %d failed: %v", i, err)
		}
	}
}
