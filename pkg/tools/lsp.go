package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LSPTool provides access to Language Server Protocol features for code intelligence.
type LSPTool struct{}

func (t *LSPTool) Name() string        { return "LSP" }
func (t *LSPTool) Description() string { return "Access Language Server Protocol features for code intelligence" }
func (t *LSPTool) IsReadOnly() bool    { return true }

func (t *LSPTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "enum": ["diagnostics", "hover", "definition", "references", "symbols"], "description": "LSP command to execute"},
			"file_path": {"type": "string", "description": "File path for the LSP query"},
			"line": {"type": "integer", "description": "Line number (0-based)"},
			"character": {"type": "integer", "description": "Character offset (0-based)"}
		},
		"required": ["command", "file_path"],
		"additionalProperties": false
	}`)
}

func (t *LSPTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Command  string `json:"command"`
		FilePath string `json:"file_path"`
		Line     int    `json:"line"`
		Char     int    `json:"character"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Command == "" {
		return ErrorOutput("command is required"), nil
	}
	if params.FilePath == "" {
		return ErrorOutput("file_path is required"), nil
	}

	cwd := ""
	if tc != nil {
		cwd = tc.CWD
	}

	// For now, use basic heuristics instead of a real LSP server.
	// This matches the TS behavior where LSP is optional.
	switch params.Command {
	case "diagnostics":
		return runDiagnostics(ctx, cwd, params.FilePath)
	case "symbols":
		return extractSymbols(cwd, params.FilePath)
	default:
		return ErrorOutput(fmt.Sprintf("LSP command '%s' requires a running language server (not yet connected)", params.Command)), nil
	}
}

func runDiagnostics(ctx context.Context, cwd, filePath string) (*ToolOutput, error) {
	// Resolve file path
	absPath := filePath
	if !filepath.IsAbs(absPath) && cwd != "" {
		absPath = filepath.Join(cwd, absPath)
	}

	// Detect language and run appropriate checker
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

func extractSymbols(cwd, filePath string) (*ToolOutput, error) {
	absPath := filePath
	if !filepath.IsAbs(absPath) && cwd != "" {
		absPath = filepath.Join(cwd, absPath)
	}
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
