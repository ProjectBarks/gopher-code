package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/projectbarks/gopher-code/internal/cli"
	pkgcli "github.com/projectbarks/gopher-code/pkg/cli"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// OutputFormat controls how headless mode emits results.
type OutputFormat string

const (
	OutputText       OutputFormat = "text"
	OutputJSON       OutputFormat = "json"
	OutputStreamJSON OutputFormat = "stream-json"
)

// ParseOutputFormat validates and returns an OutputFormat.
// Returns an error string on invalid input.
func ParseOutputFormat(s string) (OutputFormat, error) {
	switch s {
	case "text", "":
		return OutputText, nil
	case "json":
		return OutputJSON, nil
	case "stream-json":
		return OutputStreamJSON, nil
	default:
		return "", fmt.Errorf("unknown output format: %s (use text, json, stream-json)", s)
	}
}

// HeadlessConfig holds the parameters for a headless (--print) invocation.
type HeadlessConfig struct {
	OutputFormat OutputFormat
	InputFormat  string // "text" or "stream-json"
	Verbose      bool

	// Stdin/Stdout/Stderr are injectable for testing.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// headlessDefaults fills in zero-value fields with production defaults.
func (c *HeadlessConfig) headlessDefaults() {
	if c.Stdin == nil {
		c.Stdin = os.Stdin
	}
	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
	if c.Stderr == nil {
		c.Stderr = os.Stderr
	}
	if c.OutputFormat == "" {
		c.OutputFormat = OutputText
	}
	if c.InputFormat == "" {
		c.InputFormat = "text"
	}
}

// ValidateHeadlessFlags checks flag combinations that are invalid in --print mode.
// Returns an error string to write to stderr, or "" if valid.
// Source: cli/print.ts — flag validation at entry
func ValidateHeadlessFlags(cfg HeadlessConfig, hasResume bool, resumeSessionAt string, rewindFiles string, prompt string) string {
	if resumeSessionAt != "" && !hasResume {
		return "Error: --resume-session-at requires --resume\n"
	}
	if rewindFiles != "" && !hasResume {
		return "Error: --rewind-files requires --resume\n"
	}
	if rewindFiles != "" && prompt != "" {
		return "Error: --rewind-files is a standalone operation and cannot be used with a prompt\n"
	}
	if cfg.OutputFormat == OutputStreamJSON && !cfg.Verbose {
		return "Error: When using --print, --output-format=stream-json requires --verbose\n"
	}
	return ""
}

// isStdinPiped returns true when stdin is not a terminal (data is piped in).
func isStdinPiped(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		stat, err := f.Stat()
		if err != nil {
			return false
		}
		return (stat.Mode() & os.ModeCharDevice) == 0
	}
	// For non-file readers (e.g. *bytes.Reader in tests), treat as piped.
	return true
}

// readPrompt determines the user prompt from args or stdin.
// Returns the prompt and any error string to emit.
func readPrompt(args []string, stdin io.Reader) (string, string) {
	prompt := strings.Join(args, " ")
	if prompt != "" {
		return prompt, ""
	}
	// Fall back to reading from stdin when piped
	if isStdinPiped(stdin) {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Sprintf("Error: %v\n", err)
		}
		prompt = strings.TrimSpace(string(data))
	}
	if prompt == "" {
		return "", "Error: no prompt provided\n"
	}
	return prompt, ""
}

// runHeadless is the orchestrator for --print mode.
// Single query -> get response -> print -> exit.
// Source: cli/print.ts — runHeadless
func runHeadless(
	ctx context.Context,
	sess *session.SessionState,
	prov provider.ModelProvider,
	registry *tools.ToolRegistry,
	orchestrator *tools.ToolOrchestrator,
	cfg HeadlessConfig,
	args []string,
) {
	cfg.headlessDefaults()

	// Validate flag combinations
	// (resumeSessionAt and rewindFiles are not yet wired — pass empty for now)
	if errMsg := ValidateHeadlessFlags(cfg, false, "", "", ""); errMsg != "" {
		fmt.Fprint(cfg.Stderr, errMsg)
		cliError("")
		return
	}

	// Ingest prompt(s)
	if cfg.InputFormat == "stream-json" {
		scanner := bufio.NewScanner(cfg.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			var msg struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal([]byte(line), &msg) == nil && msg.Text != "" {
				sess.PushMessage(message.UserMessage(msg.Text))
			}
		}
	} else {
		prompt, errMsg := readPrompt(args, cfg.Stdin)
		if errMsg != "" {
			fmt.Fprint(cfg.Stderr, errMsg)
			cliError("")
			return
		}
		sess.PushMessage(message.UserMessage(prompt))
	}

	// Run the query loop with format-appropriate callback, then exit.
	switch cfg.OutputFormat {
	case OutputJSON:
		collector := cli.NewJSONCollector()
		err := query.Query(ctx, sess, prov, registry, orchestrator, collector.Callback)
		// Always emit the envelope, even on error (matches TS behavior).
		collector.EmitTo(cfg.Stdout)
		if err != nil {
			emitError(cfg, err)
			cliError("")
			return
		}

	case OutputStreamJSON:
		cb := cli.NewStreamJSONWriter(cfg.Stdout)
		err := query.Query(ctx, sess, prov, registry, orchestrator, cb)
		if err != nil {
			emitError(cfg, err)
			cliError("")
			return
		}

	default: // OutputText
		cb := cli.NewPlainTextWriter(cfg.Stdout)
		err := query.Query(ctx, sess, prov, registry, orchestrator, cb)
		if err != nil {
			emitError(cfg, err)
			cliError("")
			return
		}
		fmt.Fprintln(cfg.Stdout) // trailing newline
	}

	cliOk("")
}

// emitError writes an error to stderr (or as stream-json to stdout).
// Source: cli/print.ts — emitLoadError
func emitError(cfg HeadlessConfig, err error) {
	if cfg.OutputFormat == OutputStreamJSON {
		fmt.Fprintln(cfg.Stdout, pkgcli.NdjsonSafeStringify(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		}))
	} else {
		fmt.Fprintf(cfg.Stderr, "Error: %v\n", err)
	}
}
