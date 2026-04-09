package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockLSPClient implements LSPClient for testing.
type mockLSPClient struct {
	connected bool
	result    json.RawMessage
	err       error
}

func (m *mockLSPClient) IsConnected() bool { return m.connected }
func (m *mockLSPClient) SendRequest(_ context.Context, _, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return m.result, m.err
}

func TestLSPTool_Metadata(t *testing.T) {
	tool := &LSPTool{}
	if tool.Name() != "LSP" {
		t.Errorf("name = %q", tool.Name())
	}
	if !tool.IsReadOnly() {
		t.Error("should be read-only")
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(tool.InputSchema(), &schema); err != nil {
		t.Fatalf("invalid schema: %v", err)
	}

	// Verify all 9 standard operations + 2 legacy are in the schema
	props := schema["properties"].(map[string]interface{})
	opProp := props["operation"].(map[string]interface{})
	ops := opProp["enum"].([]interface{})
	if len(ops) != 11 {
		t.Errorf("expected 11 operations in schema, got %d", len(ops))
	}
}

func TestLSPTool_AllOperationsValid(t *testing.T) {
	for _, op := range []string{
		"goToDefinition", "findReferences", "hover",
		"documentSymbol", "workspaceSymbol", "goToImplementation",
		"prepareCallHierarchy", "incomingCalls", "outgoingCalls",
		"diagnostics", "symbols",
	} {
		if !IsValidLSPOperation(op) {
			t.Errorf("%q should be valid", op)
		}
	}
	if IsValidLSPOperation("invalid") {
		t.Error("'invalid' should not be valid")
	}
}

func TestLSPTool_WithLSPClient(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	os.WriteFile(testFile, []byte("package main\nfunc main() {}\n"), 0644)

	client := &mockLSPClient{
		connected: true,
		result:    json.RawMessage(`[{"uri":"file:///test.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":10}}}]`),
	}
	tool := &LSPTool{Client: client}

	input, _ := json.Marshal(map[string]interface{}{
		"operation": "goToDefinition",
		"filePath":  testFile,
		"line":      2,
		"character": 6,
	})

	out, err := tool.Execute(context.Background(), &ToolContext{CWD: dir}, input)
	if err != nil {
		t.Fatal(err)
	}
	if out.IsError {
		t.Fatalf("unexpected error: %s", out.Content)
	}

	// Output should be JSON with operation and result
	var lspOut LSPOutput
	if err := json.Unmarshal([]byte(out.Content), &lspOut); err != nil {
		t.Fatalf("output is not valid LSPOutput JSON: %v\nraw: %s", err, out.Content)
	}
	if lspOut.Operation != "goToDefinition" {
		t.Errorf("operation = %q", lspOut.Operation)
	}
}

func TestLSPTool_NoLSPServer_FallbackSymbols(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "example.go")
	os.WriteFile(goFile, []byte("package example\n\ntype Foo struct{}\n\nfunc Bar() {}\n"), 0644)

	tool := &LSPTool{} // no client
	input, _ := json.Marshal(map[string]interface{}{
		"operation": "documentSymbol",
		"filePath":  goFile,
	})

	out, err := tool.Execute(context.Background(), &ToolContext{CWD: dir}, input)
	if err != nil {
		t.Fatal(err)
	}
	if out.IsError {
		t.Fatalf("unexpected error: %s", out.Content)
	}
	if !strings.Contains(out.Content, "type Foo") {
		t.Errorf("should contain Foo symbol: %s", out.Content)
	}
}

func TestLSPTool_NoLSPServer_RequiresServer(t *testing.T) {
	tool := &LSPTool{}
	input, _ := json.Marshal(map[string]interface{}{
		"operation": "goToDefinition",
		"filePath":  "test.go",
		"line":      1,
		"character": 1,
	})

	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatal(err)
	}
	// Should return structured output indicating no server
	var lspOut LSPOutput
	if err := json.Unmarshal([]byte(out.Content), &lspOut); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if !strings.Contains(lspOut.Result, "No LSP server") {
		t.Errorf("should mention no LSP server: %s", lspOut.Result)
	}
}

func TestLSPTool_LegacyFieldNames(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "test.go")
	os.WriteFile(goFile, []byte("package main\nfunc main() {}\n"), 0644)

	tool := &LSPTool{}
	// Use legacy field names: "command" and "file_path"
	input, _ := json.Marshal(map[string]interface{}{
		"command":   "symbols",
		"file_path": goFile,
	})

	out, err := tool.Execute(context.Background(), &ToolContext{CWD: dir}, input)
	if err != nil {
		t.Fatal(err)
	}
	if out.IsError {
		t.Fatalf("legacy field names should work: %s", out.Content)
	}
	if !strings.Contains(out.Content, "func main") {
		t.Errorf("should find symbols: %s", out.Content)
	}
}

func TestLSPTool_FileTooLarge(t *testing.T) {
	dir := t.TempDir()
	bigFile := filepath.Join(dir, "big.go")
	// Create a file just over the limit
	f, _ := os.Create(bigFile)
	f.Truncate(MaxLSPFileSizeBytes + 1)
	f.Close()

	client := &mockLSPClient{connected: true}
	tool := &LSPTool{Client: client}

	input, _ := json.Marshal(map[string]interface{}{
		"operation": "hover",
		"filePath":  bigFile,
		"line":      1,
		"character": 1,
	})

	out, err := tool.Execute(context.Background(), &ToolContext{CWD: dir}, input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Content, "too large") {
		t.Errorf("should mention too large: %s", out.Content)
	}
}

func TestLSPTool_FileNotExist(t *testing.T) {
	client := &mockLSPClient{connected: true}
	tool := &LSPTool{Client: client}

	input, _ := json.Marshal(map[string]interface{}{
		"operation": "hover",
		"filePath":  "/nonexistent/file.go",
		"line":      1,
		"character": 1,
	})

	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatal(err)
	}
	if !out.IsError {
		t.Error("should error for nonexistent file")
	}
}

func TestLSPTool_MissingParams(t *testing.T) {
	tool := &LSPTool{}

	// Missing operation
	input, _ := json.Marshal(map[string]interface{}{
		"filePath": "test.go",
	})
	out, _ := tool.Execute(context.Background(), nil, input)
	if !out.IsError {
		t.Error("expected error for missing operation")
	}

	// Missing filePath
	input, _ = json.Marshal(map[string]interface{}{
		"operation": "hover",
	})
	out, _ = tool.Execute(context.Background(), nil, input)
	if !out.IsError {
		t.Error("expected error for missing filePath")
	}
}

func TestLSPTool_InvalidOperation(t *testing.T) {
	tool := &LSPTool{}
	input, _ := json.Marshal(map[string]interface{}{
		"operation": "invalid",
		"filePath":  "test.go",
	})
	out, _ := tool.Execute(context.Background(), nil, input)
	if !out.IsError {
		t.Error("expected error for invalid operation")
	}
}

func TestPathToFileURI(t *testing.T) {
	uri := pathToFileURI("/home/user/file.go")
	if !strings.HasPrefix(uri, "file:///") {
		t.Errorf("URI should start with file:///: %s", uri)
	}
	if !strings.Contains(uri, "file.go") {
		t.Errorf("URI should contain filename: %s", uri)
	}
}

func TestMapOperationToLSP(t *testing.T) {
	tests := []struct {
		op     string
		method string
	}{
		{LSPOpGoToDefinition, "textDocument/definition"},
		{LSPOpFindReferences, "textDocument/references"},
		{LSPOpHover, "textDocument/hover"},
		{LSPOpDocumentSymbol, "textDocument/documentSymbol"},
		{LSPOpWorkspaceSymbol, "workspace/symbol"},
		{LSPOpGoToImplementation, "textDocument/implementation"},
		{LSPOpPrepareCallHierarchy, "textDocument/prepareCallHierarchy"},
		{LSPOpIncomingCalls, "textDocument/prepareCallHierarchy"},
		{LSPOpOutgoingCalls, "textDocument/prepareCallHierarchy"},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			method, _ := mapOperationToLSP(lspInput{
				Operation: tt.op,
				FilePath:  "/test.go",
				Line:      10,
				Character: 5,
			}, "/test.go")
			if method != tt.method {
				t.Errorf("method = %q, want %q", method, tt.method)
			}
		})
	}
}

func TestLSPTool_DiagnosticsUnsupportedExt(t *testing.T) {
	tool := &LSPTool{}
	input, _ := json.Marshal(map[string]interface{}{
		"operation": "diagnostics",
		"filePath":  "test.rs",
	})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatal(err)
	}
	if out.IsError {
		t.Fatalf("unexpected error: %s", out.Content)
	}
	if out.Content != "No diagnostics available for .rs files" {
		t.Errorf("unexpected: %q", out.Content)
	}
}

func TestLSPTool_ExtractSymbols_GoFile(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "example.go")
	content := `package example

type Foo struct {
	Name string
}

func Bar() string {
	return "bar"
}

const MaxSize = 100
`
	os.WriteFile(goFile, []byte(content), 0644)

	tool := &LSPTool{}
	input, _ := json.Marshal(map[string]interface{}{
		"operation": "symbols",
		"filePath":  goFile,
	})
	out, err := tool.Execute(context.Background(), &ToolContext{CWD: dir}, input)
	if err != nil {
		t.Fatal(err)
	}
	if out.IsError {
		t.Fatalf("unexpected error: %s", out.Content)
	}

	for _, expected := range []string{"type Foo struct", "func Bar()", "const MaxSize"} {
		if !strings.Contains(out.Content, expected) {
			t.Errorf("missing symbol %q in output:\n%s", expected, out.Content)
		}
	}
}
