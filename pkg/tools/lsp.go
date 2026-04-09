package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Source: tools/LSPTool/LSPTool.ts, tools/LSPTool/schemas.ts

// MaxLSPFileSizeBytes is the maximum file size for LSP analysis.
const MaxLSPFileSizeBytes = 10_000_000

// LSP operation constants matching the TS enum.
const (
	LSPOpGoToDefinition       = "goToDefinition"
	LSPOpFindReferences       = "findReferences"
	LSPOpHover                = "hover"
	LSPOpDocumentSymbol       = "documentSymbol"
	LSPOpWorkspaceSymbol      = "workspaceSymbol"
	LSPOpGoToImplementation   = "goToImplementation"
	LSPOpPrepareCallHierarchy = "prepareCallHierarchy"
	LSPOpIncomingCalls        = "incomingCalls"
	LSPOpOutgoingCalls        = "outgoingCalls"
	// Legacy operations (backward compat with existing stub)
	LSPOpDiagnostics = "diagnostics"
	LSPOpSymbols     = "symbols"
)

// ValidLSPOperations is the set of valid LSP operations.
var ValidLSPOperations = map[string]bool{
	LSPOpGoToDefinition: true, LSPOpFindReferences: true,
	LSPOpHover: true, LSPOpDocumentSymbol: true,
	LSPOpWorkspaceSymbol: true, LSPOpGoToImplementation: true,
	LSPOpPrepareCallHierarchy: true, LSPOpIncomingCalls: true,
	LSPOpOutgoingCalls: true,
	LSPOpDiagnostics: true, LSPOpSymbols: true,
}

// LSPOutput is the structured output from LSP operations.
// Source: LSPTool.ts outputSchema
type LSPOutput struct {
	Operation   string `json:"operation"`
	Result      string `json:"result"`
	FilePath    string `json:"filePath"`
	ResultCount int    `json:"resultCount,omitempty"`
	FileCount   int    `json:"fileCount,omitempty"`
}

// LSPClient abstracts the LSP server communication for testability.
type LSPClient interface {
	// IsConnected returns true if an LSP server is available.
	IsConnected() bool
	// SendRequest sends an LSP request and returns the raw JSON result.
	SendRequest(ctx context.Context, filePath, method string, params json.RawMessage) (json.RawMessage, error)
}

// LSPTool provides access to Language Server Protocol features for code intelligence.
type LSPTool struct {
	Client LSPClient // nil means no LSP server, use fallback
}

func (t *LSPTool) Name() string { return "LSP" }
func (t *LSPTool) Description() string {
	return "Access Language Server Protocol features for code intelligence (definitions, references, symbols, hover, implementations, call hierarchy)"
}
func (t *LSPTool) IsReadOnly() bool { return true }

func (t *LSPTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"operation": {
				"type": "string",
				"enum": ["goToDefinition", "findReferences", "hover", "documentSymbol", "workspaceSymbol", "goToImplementation", "prepareCallHierarchy", "incomingCalls", "outgoingCalls", "diagnostics", "symbols"],
				"description": "The LSP operation to perform"
			},
			"filePath": {"type": "string", "description": "The absolute or relative path to the file"},
			"line": {"type": "integer", "description": "The line number (1-based, as shown in editors)"},
			"character": {"type": "integer", "description": "The character offset (1-based, as shown in editors)"}
		},
		"required": ["operation", "filePath"],
		"additionalProperties": false
	}`)
}

// lspInput is the parsed input for LSP operations.
type lspInput struct {
	Operation string `json:"operation"`
	FilePath  string `json:"filePath"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
	// Legacy field names (backward compat)
	Command   string `json:"command"`
	LegacyFP  string `json:"file_path"`
}

func (t *LSPTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params lspInput
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}

	// Handle legacy field names
	if params.Operation == "" && params.Command != "" {
		params.Operation = params.Command
	}
	if params.FilePath == "" && params.LegacyFP != "" {
		params.FilePath = params.LegacyFP
	}

	if params.Operation == "" {
		return ErrorOutput("operation is required"), nil
	}
	if params.FilePath == "" {
		return ErrorOutput("filePath is required"), nil
	}
	if !ValidLSPOperations[params.Operation] {
		return ErrorOutput(fmt.Sprintf("invalid operation %q", params.Operation)), nil
	}

	cwd := ""
	if tc != nil {
		cwd = tc.CWD
	}

	absPath := params.FilePath
	if !filepath.IsAbs(absPath) && cwd != "" {
		absPath = filepath.Join(cwd, absPath)
	}

	// If LSP client is connected, delegate to it
	if t.Client != nil && t.Client.IsConnected() {
		return t.executeLSP(ctx, params, absPath, cwd)
	}

	// Fallback: handle legacy operations without LSP server
	switch params.Operation {
	case LSPOpDiagnostics:
		return runDiagnostics(ctx, cwd, absPath)
	case LSPOpSymbols, LSPOpDocumentSymbol:
		return extractSymbols(cwd, absPath)
	default:
		out := LSPOutput{
			Operation: params.Operation,
			Result:    fmt.Sprintf("No LSP server connected. Operation %q requires a running language server.", params.Operation),
			FilePath:  params.FilePath,
		}
		return t.formatOutput(out), nil
	}
}

// executeLSP delegates an operation to the LSP client.
func (t *LSPTool) executeLSP(ctx context.Context, params lspInput, absPath, cwd string) (*ToolOutput, error) {
	// Check file size
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrorOutput(fmt.Sprintf("File does not exist: %s", params.FilePath)), nil
		}
		return ErrorOutput(fmt.Sprintf("Cannot access file: %s", err)), nil
	}
	if !info.Mode().IsRegular() {
		return ErrorOutput(fmt.Sprintf("Path is not a file: %s", params.FilePath)), nil
	}
	if info.Size() > MaxLSPFileSizeBytes {
		out := LSPOutput{
			Operation: params.Operation,
			Result:    fmt.Sprintf("File too large for LSP analysis (%dMB exceeds 10MB limit)", info.Size()/1_000_000),
			FilePath:  params.FilePath,
		}
		return t.formatOutput(out), nil
	}

	// Map operation to LSP method and params
	method, reqParams := mapOperationToLSP(params, absPath)

	reqJSON, err := json.Marshal(reqParams)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to marshal LSP params: %s", err)), nil
	}

	result, err := t.Client.SendRequest(ctx, absPath, method, reqJSON)
	if err != nil {
		out := LSPOutput{
			Operation: params.Operation,
			Result:    fmt.Sprintf("Error performing %s: %s", params.Operation, err),
			FilePath:  params.FilePath,
		}
		return t.formatOutput(out), nil
	}

	if result == nil {
		out := LSPOutput{
			Operation: params.Operation,
			Result:    fmt.Sprintf("No LSP server available for file type: %s", filepath.Ext(absPath)),
			FilePath:  params.FilePath,
		}
		return t.formatOutput(out), nil
	}

	out := LSPOutput{
		Operation: params.Operation,
		Result:    string(result),
		FilePath:  params.FilePath,
	}
	return t.formatOutput(out), nil
}

func (t *LSPTool) formatOutput(out LSPOutput) *ToolOutput {
	data, _ := json.Marshal(out)
	return SuccessOutput(string(data))
}

// mapOperationToLSP maps an operation name to the LSP method and request params.
// Source: LSPTool.ts getMethodAndParams
func mapOperationToLSP(params lspInput, absPath string) (method string, reqParams map[string]interface{}) {
	uri := pathToFileURI(absPath)
	// Convert from 1-based (user-friendly) to 0-based (LSP protocol)
	position := map[string]int{
		"line":      max(0, params.Line-1),
		"character": max(0, params.Character-1),
	}
	textDoc := map[string]string{"uri": uri}

	switch params.Operation {
	case LSPOpGoToDefinition:
		return "textDocument/definition", map[string]interface{}{
			"textDocument": textDoc, "position": position,
		}
	case LSPOpFindReferences:
		return "textDocument/references", map[string]interface{}{
			"textDocument": textDoc, "position": position,
			"context": map[string]bool{"includeDeclaration": true},
		}
	case LSPOpHover:
		return "textDocument/hover", map[string]interface{}{
			"textDocument": textDoc, "position": position,
		}
	case LSPOpDocumentSymbol:
		return "textDocument/documentSymbol", map[string]interface{}{
			"textDocument": textDoc,
		}
	case LSPOpWorkspaceSymbol:
		return "workspace/symbol", map[string]interface{}{
			"query": "",
		}
	case LSPOpGoToImplementation:
		return "textDocument/implementation", map[string]interface{}{
			"textDocument": textDoc, "position": position,
		}
	case LSPOpPrepareCallHierarchy, LSPOpIncomingCalls, LSPOpOutgoingCalls:
		// All three start with prepareCallHierarchy; incoming/outgoing
		// do a two-step call (handled by the LSP client layer).
		return "textDocument/prepareCallHierarchy", map[string]interface{}{
			"textDocument": textDoc, "position": position,
		}
	default:
		return "textDocument/hover", map[string]interface{}{
			"textDocument": textDoc, "position": position,
		}
	}
}

// pathToFileURI converts an absolute path to a file:// URI.
func pathToFileURI(absPath string) string {
	// url.URL escapes special characters correctly
	u := url.URL{Scheme: "file", Path: absPath}
	return u.String()
}

// IsValidLSPOperation returns true if the operation name is valid.
func IsValidLSPOperation(operation string) bool {
	return ValidLSPOperations[operation]
}

// ---------------------------------------------------------------------------
// Fallback implementations (used when no LSP server is connected)
// ---------------------------------------------------------------------------

func runDiagnostics(ctx context.Context, cwd, absPath string) (*ToolOutput, error) {
	ext := filepath.Ext(absPath)
	var cmd *exec.Cmd
	switch ext {
	case ".go":
		cmd = exec.CommandContext(ctx, "go", "vet", absPath)
	case ".py":
		cmd = exec.CommandContext(ctx, "python3", "-m", "py_compile", absPath)
	case ".ts", ".js":
		cmd = exec.CommandContext(ctx, "npx", "tsc", "--noEmit", absPath)
	default:
		return SuccessOutput("No diagnostics available for " + ext + " files"), nil
	}
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return SuccessOutput(string(out)), nil // Diagnostics found
	}
	return SuccessOutput("No issues found"), nil
}

func extractSymbols(cwd, absPath string) (*ToolOutput, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ErrorOutput("cannot read file: " + err.Error()), nil
	}

	var symbols []string
	for i, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "type ") ||
			strings.HasPrefix(trimmed, "class ") || strings.HasPrefix(trimmed, "def ") ||
			strings.HasPrefix(trimmed, "function ") || strings.HasPrefix(trimmed, "const ") ||
			strings.HasPrefix(trimmed, "export ") {
			symbols = append(symbols, fmt.Sprintf("  %d: %s", i+1, trimmed))
		}
	}
	if len(symbols) == 0 {
		return SuccessOutput("No symbols found"), nil
	}
	return SuccessOutput(strings.Join(symbols, "\n")), nil
}
