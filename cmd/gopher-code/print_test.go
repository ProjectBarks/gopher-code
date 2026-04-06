package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// --- helpers ----------------------------------------------------------------

// stubProvider returns a ModelProvider whose Stream yields a single text response.
type stubProvider struct {
	text string
}

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) Stream(_ context.Context, _ provider.ModelRequest) (<-chan provider.StreamResult, error) {
	ch := make(chan provider.StreamResult, 3)
	ch <- provider.StreamResult{Event: &provider.StreamEvent{
		Type: provider.EventTextDelta,
		Text: s.text,
	}}
	ch <- provider.StreamResult{Event: &provider.StreamEvent{
		Type: provider.EventMessageDone,
		Response: &provider.ModelResponse{
			StopReason: ptrStop(provider.StopReasonEndTurn),
			Usage:      provider.Usage{InputTokens: 10, OutputTokens: 5},
		},
	}}
	close(ch)
	return ch, nil
}

func ptrStop(r provider.StopReason) *provider.StopReason { return &r }

func newTestSession() *session.SessionState {
	return session.New(session.SessionConfig{
		Model:        "test-model",
		SystemPrompt: "You are a test.",
		MaxTurns:     10,
	}, "/tmp")
}

// captureHeadless runs runHeadless with captured exit/stdout/stderr.
func captureHeadless(
	ctx context.Context,
	sess *session.SessionState,
	prov provider.ModelProvider,
	registry *tools.ToolRegistry,
	orch *tools.ToolOrchestrator,
	cfg HeadlessConfig,
	args []string,
) (exitCode int, stdoutStr, stderrStr string) {
	origExit := exitFunc
	defer func() { exitFunc = origExit }()

	exitCode = -1
	exitFunc = func(code int) { exitCode = code }

	var outBuf, errBuf bytes.Buffer
	cfg.Stdout = &outBuf
	cfg.Stderr = &errBuf

	runHeadless(ctx, sess, prov, registry, orch, cfg, args)
	return exitCode, outBuf.String(), errBuf.String()
}

// --- OutputFormat tests -----------------------------------------------------

func TestParseOutputFormat_Valid(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  OutputFormat
	}{
		{"text", OutputText},
		{"json", OutputJSON},
		{"stream-json", OutputStreamJSON},
		{"", OutputText},
	} {
		got, err := ParseOutputFormat(tc.input)
		if err != nil {
			t.Errorf("ParseOutputFormat(%q): unexpected error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseOutputFormat(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseOutputFormat_Invalid(t *testing.T) {
	_, err := ParseOutputFormat("xml")
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
}

// --- Flag validation tests --------------------------------------------------

func TestValidateHeadlessFlags_AllErrors(t *testing.T) {
	tests := []struct {
		name             string
		cfg              HeadlessConfig
		hasResume        bool
		resumeSessionAt  string
		rewindFiles      string
		prompt           string
		wantErr          string
	}{
		{
			name:            "resume-session-at without resume",
			cfg:             HeadlessConfig{OutputFormat: OutputText, Verbose: true},
			hasResume:       false,
			resumeSessionAt: "3",
			wantErr:         "Error: --resume-session-at requires --resume\n",
		},
		{
			name:        "rewind-files without resume",
			cfg:         HeadlessConfig{OutputFormat: OutputText, Verbose: true},
			hasResume:   false,
			rewindFiles: "foo.go",
			wantErr:     "Error: --rewind-files requires --resume\n",
		},
		{
			name:        "rewind-files with prompt",
			cfg:         HeadlessConfig{OutputFormat: OutputText, Verbose: true},
			hasResume:   true,
			rewindFiles: "foo.go",
			prompt:      "hello",
			wantErr:     "Error: --rewind-files is a standalone operation and cannot be used with a prompt\n",
		},
		{
			name:    "stream-json without verbose",
			cfg:     HeadlessConfig{OutputFormat: OutputStreamJSON, Verbose: false},
			wantErr: "Error: When using --print, --output-format=stream-json requires --verbose\n",
		},
		{
			name:      "valid combination",
			cfg:       HeadlessConfig{OutputFormat: OutputText, Verbose: true},
			hasResume: true,
			wantErr:   "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateHeadlessFlags(tc.cfg, tc.hasResume, tc.resumeSessionAt, tc.rewindFiles, tc.prompt)
			if got != tc.wantErr {
				t.Errorf("ValidateHeadlessFlags() = %q, want %q", got, tc.wantErr)
			}
		})
	}
}

// --- Text output format test ------------------------------------------------

func TestRunHeadless_TextOutput(t *testing.T) {
	ctx := context.Background()
	sess := newTestSession()
	prov := &stubProvider{text: "Hello, world!"}
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)

	code, out, errOut := captureHeadless(ctx, sess, prov, reg, orch, HeadlessConfig{
		OutputFormat: OutputText,
		Stdin:        strings.NewReader(""),
	}, []string{"test", "prompt"})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, errOut)
	}
	// Text output should contain the model's text followed by a trailing newline.
	if !strings.Contains(out, "Hello, world!") {
		t.Errorf("expected output to contain %q, got %q", "Hello, world!", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("expected trailing newline in text output")
	}
}

// --- JSON output format test ------------------------------------------------

func TestRunHeadless_JSONOutput(t *testing.T) {
	ctx := context.Background()
	sess := newTestSession()
	prov := &stubProvider{text: "JSON response"}
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)

	code, out, errOut := captureHeadless(ctx, sess, prov, reg, orch, HeadlessConfig{
		OutputFormat: OutputJSON,
		Stdin:        strings.NewReader(""),
	}, []string{"test"})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, errOut)
	}

	// Parse as JSON envelope
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, out)
	}
	if envelope["type"] != "result" {
		t.Errorf("expected type=result, got %v", envelope["type"])
	}
	if envelope["role"] != "assistant" {
		t.Errorf("expected role=assistant, got %v", envelope["role"])
	}
	// Check content contains the text
	content, ok := envelope["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("expected content array, got %v", envelope["content"])
	}
	block, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected content block map, got %T", content[0])
	}
	if block["text"] != "JSON response" {
		t.Errorf("expected text=%q, got %v", "JSON response", block["text"])
	}
}

// --- Stream-JSON output format test -----------------------------------------

func TestRunHeadless_StreamJSONOutput(t *testing.T) {
	ctx := context.Background()
	sess := newTestSession()
	prov := &stubProvider{text: "streamed"}
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)

	code, out, errOut := captureHeadless(ctx, sess, prov, reg, orch, HeadlessConfig{
		OutputFormat: OutputStreamJSON,
		Verbose:      true, // required for stream-json
		Stdin:        strings.NewReader(""),
	}, []string{"go"})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, errOut)
	}

	// Each line should be valid JSON
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one NDJSON line")
	}
	foundText := false
	for _, line := range lines {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line is not valid JSON: %q", line)
		}
		if obj["type"] == "text_delta" && obj["text"] == "streamed" {
			foundText = true
		}
	}
	if !foundText {
		t.Errorf("expected a text_delta line with text=streamed in output:\n%s", out)
	}
}

// --- Stdin piping detection test --------------------------------------------

func TestRunHeadless_StdinPiping(t *testing.T) {
	ctx := context.Background()
	sess := newTestSession()
	prov := &stubProvider{text: "piped-reply"}
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)

	// Simulate piped stdin by providing a reader with content
	code, out, _ := captureHeadless(ctx, sess, prov, reg, orch, HeadlessConfig{
		OutputFormat: OutputText,
		Stdin:        strings.NewReader("piped prompt\n"),
	}, nil) // no args — should read from stdin

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "piped-reply") {
		t.Errorf("expected output to contain piped-reply, got %q", out)
	}

	// Verify the session received the piped prompt
	found := false
	for _, m := range sess.Messages {
		if m.Role == message.RoleUser {
			for _, b := range m.Content {
				if strings.Contains(b.Text, "piped prompt") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("expected session to contain 'piped prompt' from stdin")
	}
}

// --- Error message tests ----------------------------------------------------

func TestRunHeadless_NoPromptError(t *testing.T) {
	ctx := context.Background()
	sess := newTestSession()
	prov := &stubProvider{text: "unused"}
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)

	// Empty stdin + no args => error
	code, _, errOut := captureHeadless(ctx, sess, prov, reg, orch, HeadlessConfig{
		OutputFormat: OutputText,
		Stdin:        strings.NewReader(""),
	}, nil)

	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "no prompt provided") {
		t.Errorf("expected 'no prompt provided' error, got %q", errOut)
	}
}

func TestRunHeadless_StreamJSONRequiresVerbose(t *testing.T) {
	// Directly test the validator for this specific error
	errMsg := ValidateHeadlessFlags(
		HeadlessConfig{OutputFormat: OutputStreamJSON, Verbose: false},
		false, "", "", "",
	)
	want := "Error: When using --print, --output-format=stream-json requires --verbose\n"
	if errMsg != want {
		t.Errorf("got %q, want %q", errMsg, want)
	}
}

func TestRunHeadless_StreamJSON_InputFormat(t *testing.T) {
	ctx := context.Background()
	sess := newTestSession()
	prov := &stubProvider{text: "stream-reply"}
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)

	// Provide stream-json input lines
	input := `{"type":"user","text":"hello from stream"}` + "\n"
	code, out, _ := captureHeadless(ctx, sess, prov, reg, orch, HeadlessConfig{
		OutputFormat: OutputStreamJSON,
		InputFormat:  "stream-json",
		Verbose:      true,
		Stdin:        strings.NewReader(input),
	}, nil)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "stream-reply") {
		t.Errorf("expected output to contain 'stream-reply', got %q", out)
	}
}

// --- emitError routing test -------------------------------------------------

func TestEmitError_TextGoesToStderr(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	cfg := HeadlessConfig{
		OutputFormat: OutputText,
		Stdout:       &outBuf,
		Stderr:       &errBuf,
	}
	emitError(cfg, fmt.Errorf("something broke"))
	if !strings.Contains(errBuf.String(), "Error: something broke") {
		t.Errorf("expected stderr error, got %q", errBuf.String())
	}
	if outBuf.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", outBuf.String())
	}
}

func TestEmitError_StreamJSONGoesToStdout(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	cfg := HeadlessConfig{
		OutputFormat: OutputStreamJSON,
		Stdout:       &outBuf,
		Stderr:       &errBuf,
	}
	emitError(cfg, fmt.Errorf("stream error"))
	if errBuf.Len() != 0 {
		t.Errorf("expected empty stderr, got %q", errBuf.String())
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(outBuf.String())), &obj); err != nil {
		t.Fatalf("expected valid JSON on stdout, got %q", outBuf.String())
	}
	if obj["type"] != "error" {
		t.Errorf("expected type=error, got %v", obj["type"])
	}
}
