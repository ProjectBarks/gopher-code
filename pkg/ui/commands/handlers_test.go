package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/projectbarks/gopher-code/pkg/hooks"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/session"
)

func TestDispatcherCreation(t *testing.T) {
	d := NewDispatcher()
	if d == nil {
		t.Fatal("Dispatcher should not be nil")
	}
}

func TestDispatcherHasDefaultCommands(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/model") {
		t.Error("Should have /model handler")
	}
	if !d.HasHandler("/clear") {
		t.Error("Should have /clear handler")
	}
	if !d.HasHandler("/help") {
		t.Error("Should have /help handler")
	}
	if !d.HasHandler("/session") {
		t.Error("Should have /session handler")
	}
	if !d.HasHandler("/quit") {
		t.Error("Should have /quit handler")
	}
}

func TestDispatcherModelCommand(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/model opus")
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	msg := cmd()
	if switchMsg, ok := msg.(ModelSwitchMsg); !ok {
		t.Errorf("Expected ModelSwitchMsg, got %T", msg)
	} else if switchMsg.Model != "opus" {
		t.Errorf("Expected model 'opus', got %q", switchMsg.Model)
	}
}

func TestDispatcherClearCommand(t *testing.T) {
	// Default dispatcher returns ClearConversationMsg
	d := NewDispatcher()
	cmd := d.Dispatch("/clear")
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	msg := cmd()
	if _, ok := msg.(ClearConversationMsg); !ok {
		t.Errorf("Expected ClearConversationMsg, got %T", msg)
	}
}

func TestDispatcherHelpCommand(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/help")
	msg := cmd()
	if _, ok := msg.(ShowHelpMsg); !ok {
		t.Errorf("Expected ShowHelpMsg, got %T", msg)
	}
}

// T253: HelpV2 screen integration tests
func TestHelpV2Text(t *testing.T) {
	d := NewDispatcher()
	helpText := d.HelpText()

	// Must contain section headers
	if !strings.Contains(helpText, "Available slash commands:") {
		t.Error("HelpText should contain 'Available slash commands:' header")
	}
	if !strings.Contains(helpText, "Keybindings:") {
		t.Error("HelpText should contain 'Keybindings:' section")
	}
	if !strings.Contains(helpText, "Tips:") {
		t.Error("HelpText should contain 'Tips:' section")
	}

	// Must list known commands
	for _, cmd := range []string{"/help", "/clear", "/compact", "/exit", "/model", "/cost", "/diff"} {
		if !strings.Contains(helpText, cmd) {
			t.Errorf("HelpText should list %s", cmd)
		}
	}

	// Must NOT list hidden commands
	if strings.Contains(helpText, "/heapdump") {
		t.Error("HelpText should not list hidden command /heapdump")
	}
	if strings.Contains(helpText, "/bridge-kick") {
		t.Error("HelpText should not list hidden command /bridge-kick")
	}
}

func TestHelpV2Alias(t *testing.T) {
	d := NewDispatcher()
	// /? should be an alias for /help
	cmd := d.Dispatch("/?")
	if cmd == nil {
		t.Fatal("/? should be dispatched as /help alias")
	}
	msg := cmd()
	if _, ok := msg.(ShowHelpMsg); !ok {
		t.Errorf("/? should produce ShowHelpMsg, got %T", msg)
	}
}

func TestHelpV2Registration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/help")
	if reg == nil {
		t.Fatal("/help should have a full registration")
	}
	if reg.Description == "" {
		t.Error("/help registration should have a description")
	}
	if reg.Type != CommandTypeLocal {
		t.Errorf("Expected CommandTypeLocal, got %v", reg.Type)
	}
}

func TestDispatcherUnknownCommand(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/unknown")
	if cmd == nil {
		t.Fatal("Should return error command for unknown")
	}
	msg := cmd()
	result, ok := msg.(CommandResult)
	if !ok {
		t.Fatalf("Expected CommandResult, got %T", msg)
	}
	if result.Error == nil {
		t.Error("Expected error for unknown command")
	}
}

func TestDispatcherNotACommand(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("not a command")
	if cmd != nil {
		t.Error("Non-command input should return nil")
	}
}

func TestDispatcherModelWithoutArgs(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/model")
	msg := cmd()
	result, ok := msg.(CommandResult)
	if !ok {
		t.Fatalf("Expected CommandResult, got %T", msg)
	}
	if result.Error == nil {
		t.Error("Expected error for /model without args")
	}
}

func TestIsCommand(t *testing.T) {
	if !IsCommand("/help") {
		t.Error("/help should be a command")
	}
	if !IsCommand("  /help") {
		t.Error("  /help should be a command")
	}
	if IsCommand("not a command") {
		t.Error("Regular text should not be a command")
	}
}

func TestDispatcherCustomHandler(t *testing.T) {
	d := NewDispatcher()
	d.Register("/custom", func(args string) tea.Cmd {
		return func() tea.Msg {
			return CommandResult{Command: "/custom", Output: "custom: " + args}
		}
	})
	if !d.HasHandler("/custom") {
		t.Error("Should have custom handler")
	}
}

func TestDispatcherCommands(t *testing.T) {
	d := NewDispatcher()
	cmds := d.Commands()
	if len(cmds) == 0 {
		t.Error("Should have registered commands")
	}
}

// ---------------------------------------------------------------------------
// T223: Command type system tests
// ---------------------------------------------------------------------------

func TestCommandTypeString(t *testing.T) {
	tests := []struct {
		ct   CommandType
		want string
	}{
		{CommandTypeLocal, "local"},
		{CommandTypeLocalJSX, "local-jsx"},
		{CommandTypePrompt, "prompt"},
		{CommandType(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.want {
			t.Errorf("CommandType(%d).String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestRegisterCommand_TypedRegistration(t *testing.T) {
	d := NewDispatcher()
	called := false
	d.RegisterCommand(CommandRegistration{
		Name:         "test-cmd",
		Description:  "A test command",
		Type:         CommandTypeLocal,
		ArgumentHint: "<arg>",
		Source:       "builtin",
		Handler: func(args string) tea.Cmd {
			called = true
			return func() tea.Msg {
				return CommandResult{Command: "/test-cmd", Output: "ok: " + args}
			}
		},
	})

	if !d.HasHandler("/test-cmd") {
		t.Fatal("Should have /test-cmd handler")
	}

	reg := d.GetRegistration("/test-cmd")
	if reg == nil {
		t.Fatal("Should have registration for /test-cmd")
	}
	if reg.Type != CommandTypeLocal {
		t.Errorf("Expected CommandTypeLocal, got %v", reg.Type)
	}
	if reg.Description != "A test command" {
		t.Errorf("Expected description 'A test command', got %q", reg.Description)
	}
	if reg.ArgumentHint != "<arg>" {
		t.Errorf("Expected argument hint '<arg>', got %q", reg.ArgumentHint)
	}

	cmd := d.Dispatch("/test-cmd hello")
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	msg := cmd()
	result, ok := msg.(CommandResult)
	if !ok {
		t.Fatalf("Expected CommandResult, got %T", msg)
	}
	if result.Output != "ok: hello" {
		t.Errorf("Expected 'ok: hello', got %q", result.Output)
	}
	if !called {
		t.Error("Handler should have been called")
	}
}

func TestRegisterCommand_Aliases(t *testing.T) {
	d := NewDispatcher()
	d.RegisterCommand(CommandRegistration{
		Name:    "quit",
		Aliases: []string{"q", "exit"},
		Type:    CommandTypeLocal,
		Handler: func(args string) tea.Cmd {
			return func() tea.Msg { return QuitMsg{} }
		},
	})

	for _, name := range []string{"/quit", "/q", "/exit"} {
		if !d.HasHandler(name) {
			t.Errorf("Should have handler for %s", name)
		}
		cmd := d.Dispatch(name)
		if cmd == nil {
			t.Fatalf("Expected non-nil command for %s", name)
		}
		msg := cmd()
		if _, ok := msg.(QuitMsg); !ok {
			t.Errorf("Expected QuitMsg for %s, got %T", name, msg)
		}
	}

	// Alias should resolve to canonical registration
	reg := d.GetRegistration("/q")
	if reg == nil {
		t.Fatal("Should resolve alias /q to registration")
	}
	if reg.Name != "quit" {
		t.Errorf("Alias should resolve to canonical name 'quit', got %q", reg.Name)
	}
}

func TestRegisterCommand_IsEnabled(t *testing.T) {
	enabled := false
	d := NewDispatcher()
	d.RegisterCommand(CommandRegistration{
		Name:      "gated",
		Type:      CommandTypeLocal,
		IsEnabled: func() bool { return enabled },
		Handler: func(args string) tea.Cmd {
			return func() tea.Msg { return CommandResult{Output: "ran"} }
		},
	})

	// Disabled: should return error
	cmd := d.Dispatch("/gated")
	msg := cmd()
	result, ok := msg.(CommandResult)
	if !ok {
		t.Fatalf("Expected CommandResult, got %T", msg)
	}
	if result.Error == nil {
		t.Error("Expected error when command is disabled")
	}

	// Enable it
	enabled = true
	cmd = d.Dispatch("/gated")
	msg = cmd()
	result, ok = msg.(CommandResult)
	if !ok {
		t.Fatalf("Expected CommandResult, got %T", msg)
	}
	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}
	if result.Output != "ran" {
		t.Errorf("Expected 'ran', got %q", result.Output)
	}
}

func TestRegistrations(t *testing.T) {
	d := NewDispatcher()
	regs := d.Registrations()
	// Should include the defaults registered via RegisterCommand
	found := make(map[string]bool)
	for _, r := range regs {
		found[r.Name] = true
	}
	for _, name := range []string{"add-dir", "advisor", "agents", "branch", "remote-control", "bridge-kick", "brief", "btw", "chrome", "clear", "color", "commit", "commit-push-pr", "compact"} {
		if !found[name] {
			t.Errorf("Expected registration for %q in Registrations()", name)
		}
	}
}

// ---------------------------------------------------------------------------
// T224: createMovedToPluginCommand tests
// ---------------------------------------------------------------------------

func TestCreateMovedToPluginCommand(t *testing.T) {
	reg := CreateMovedToPluginCommand(MovedToPluginOptions{
		Name:            "old-cmd",
		Description:     "This was moved",
		ProgressMessage: "redirecting...",
		PluginName:      "my-plugin",
		PluginCommand:   "new-cmd",
	})

	if reg.Name != "old-cmd" {
		t.Errorf("Expected name 'old-cmd', got %q", reg.Name)
	}
	if reg.Type != CommandTypePrompt {
		t.Errorf("Expected CommandTypePrompt, got %v", reg.Type)
	}
	if reg.Source != "builtin" {
		t.Errorf("Expected source 'builtin', got %q", reg.Source)
	}

	// Dispatch and check message content
	d := NewDispatcher()
	d.RegisterCommand(reg)

	cmd := d.Dispatch("/old-cmd")
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	msg := cmd()
	movedMsg, ok := msg.(MovedToPluginMsg)
	if !ok {
		t.Fatalf("Expected MovedToPluginMsg, got %T", msg)
	}
	if movedMsg.PluginName != "my-plugin" {
		t.Errorf("Expected plugin name 'my-plugin', got %q", movedMsg.PluginName)
	}
	if !strings.Contains(movedMsg.Message, "claude plugin install my-plugin@claude-code-marketplace") {
		t.Errorf("Message should contain install command, got: %s", movedMsg.Message)
	}
	if !strings.Contains(movedMsg.Message, "/my-plugin:new-cmd") {
		t.Errorf("Message should contain plugin command, got: %s", movedMsg.Message)
	}
	if !strings.Contains(movedMsg.Message, "claude-code-marketplace/blob/main/my-plugin/README.md") {
		t.Errorf("Message should contain README link, got: %s", movedMsg.Message)
	}
}

// ---------------------------------------------------------------------------
// T225: /add-dir tests
// ---------------------------------------------------------------------------

func TestAddDir_EmptyPath(t *testing.T) {
	result := ValidateDirectoryForWorkspace("", nil)
	if result.ResultType != AddDirEmptyPath {
		t.Errorf("Expected emptyPath, got %s", result.ResultType)
	}
	msg := AddDirHelpMessage(result)
	if msg != "Please provide a directory path." {
		t.Errorf("Unexpected message: %s", msg)
	}
}

func TestAddDir_ValidDirectory(t *testing.T) {
	dir := t.TempDir()
	result := ValidateDirectoryForWorkspace(dir, nil)
	if result.ResultType != AddDirSuccess {
		t.Errorf("Expected success, got %s", result.ResultType)
	}
	if result.AbsolutePath != filepath.Clean(dir) {
		t.Errorf("Expected abs path %q, got %q", filepath.Clean(dir), result.AbsolutePath)
	}
	msg := AddDirHelpMessage(result)
	if !strings.Contains(msg, "Added") {
		t.Errorf("Success message should contain 'Added', got: %s", msg)
	}
}

func TestAddDir_PathNotFound(t *testing.T) {
	result := ValidateDirectoryForWorkspace("/nonexistent/path/xyz", nil)
	if result.ResultType != AddDirPathNotFound {
		t.Errorf("Expected pathNotFound, got %s", result.ResultType)
	}
	msg := AddDirHelpMessage(result)
	if !strings.Contains(msg, "was not found") {
		t.Errorf("Not-found message should contain 'was not found', got: %s", msg)
	}
}

func TestAddDir_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "afile.txt")
	os.WriteFile(f, []byte("hello"), 0644)

	result := ValidateDirectoryForWorkspace(f, nil)
	if result.ResultType != AddDirNotADirectory {
		t.Errorf("Expected notADirectory, got %s", result.ResultType)
	}
	msg := AddDirHelpMessage(result)
	if !strings.Contains(msg, "is not a directory") {
		t.Errorf("Message should mention 'is not a directory', got: %s", msg)
	}
	if !strings.Contains(msg, "parent directory") {
		t.Errorf("Message should suggest parent directory, got: %s", msg)
	}
}

func TestAddDir_AlreadyInWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)

	result := ValidateDirectoryForWorkspace(sub, []string{dir})
	if result.ResultType != AddDirAlreadyInWorkingDir {
		t.Errorf("Expected alreadyInWorkingDirectory, got %s", result.ResultType)
	}
	msg := AddDirHelpMessage(result)
	if !strings.Contains(msg, "already accessible") {
		t.Errorf("Message should mention 'already accessible', got: %s", msg)
	}
}

func TestAddDir_TildeExpansion(t *testing.T) {
	// expandPath should handle ~ prefix
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot determine home directory")
	}
	expanded := expandPath("~/testdir")
	expected := filepath.Join(home, "testdir")
	if expanded != expected {
		t.Errorf("expandPath(\"~/testdir\") = %q, want %q", expanded, expected)
	}

	// Just "~" should expand to home
	expanded = expandPath("~")
	if expanded != filepath.Join(home, "") {
		t.Errorf("expandPath(\"~\") = %q, want home dir", expanded)
	}

	// No tilde: unchanged
	expanded = expandPath("/absolute/path")
	if expanded != "/absolute/path" {
		t.Errorf("expandPath(\"/absolute/path\") should be unchanged, got %q", expanded)
	}
}

func TestAddDir_DispatchIntegration(t *testing.T) {
	d := NewDispatcher()
	// /add-dir is registered in defaults

	// No args: should get error (emptyPath)
	cmd := d.Dispatch("/add-dir")
	msg := cmd()
	addMsg, ok := msg.(AddDirMsg)
	if !ok {
		t.Fatalf("Expected AddDirMsg, got %T", msg)
	}
	if addMsg.Error == nil {
		t.Error("Expected error for empty path")
	}

	// Valid directory
	dir := t.TempDir()
	cmd = d.Dispatch("/add-dir " + dir)
	msg = cmd()
	addMsg, ok = msg.(AddDirMsg)
	if !ok {
		t.Fatalf("Expected AddDirMsg, got %T", msg)
	}
	if addMsg.Error != nil {
		t.Errorf("Unexpected error: %v", addMsg.Error)
	}
	if addMsg.Path == "" {
		t.Error("Expected non-empty path for success")
	}
}

func TestPathInWorkingDir(t *testing.T) {
	tests := []struct {
		absPath    string
		workingDir string
		want       bool
	}{
		{"/a/b/c", "/a/b", true},
		{"/a/b", "/a/b", true},
		{"/a/b", "/a/b/c", false},
		{"/a/x", "/a/b", false},
	}
	for _, tt := range tests {
		got := pathInWorkingDir(tt.absPath, tt.workingDir)
		if got != tt.want {
			t.Errorf("pathInWorkingDir(%q, %q) = %v, want %v", tt.absPath, tt.workingDir, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// T226: /advisor tests
// ---------------------------------------------------------------------------

func TestAdvisor_NoArgs_Unset(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/advisor")
	msg := cmd()
	adv, ok := msg.(AdvisorMsg)
	if !ok {
		t.Fatalf("Expected AdvisorMsg, got %T", msg)
	}
	if !strings.Contains(adv.Message, "not set") {
		t.Errorf("Expected 'not set' message, got: %s", adv.Message)
	}
}

func TestAdvisor_SetModel(t *testing.T) {
	var currentModel string
	d := NewDispatcher()
	// Re-register with stateful callbacks
	d.Register("/advisor", newAdvisorHandler(
		func() AdvisorState { return AdvisorState{Model: currentModel} },
		func(model string) { currentModel = model },
	))

	// Set opus
	cmd := d.Dispatch("/advisor opus")
	msg := cmd()
	adv, ok := msg.(AdvisorMsg)
	if !ok {
		t.Fatalf("Expected AdvisorMsg, got %T", msg)
	}
	if !strings.Contains(adv.Message, "Advisor set to opus") {
		t.Errorf("Expected set message, got: %s", adv.Message)
	}
	if currentModel != "opus" {
		t.Errorf("Expected model to be 'opus', got %q", currentModel)
	}

	// Show current
	cmd = d.Dispatch("/advisor")
	msg = cmd()
	adv = msg.(AdvisorMsg)
	if !strings.Contains(adv.Message, "Advisor: opus") {
		t.Errorf("Expected current model in message, got: %s", adv.Message)
	}

	// Unset
	cmd = d.Dispatch("/advisor unset")
	msg = cmd()
	adv = msg.(AdvisorMsg)
	if !strings.Contains(adv.Message, "Advisor disabled (was opus)") {
		t.Errorf("Expected disabled message, got: %s", adv.Message)
	}
	if currentModel != "" {
		t.Errorf("Expected model to be empty, got %q", currentModel)
	}

	// Unset again
	cmd = d.Dispatch("/advisor off")
	msg = cmd()
	adv = msg.(AdvisorMsg)
	if !strings.Contains(adv.Message, "already unset") {
		t.Errorf("Expected 'already unset' message, got: %s", adv.Message)
	}
}

// ---------------------------------------------------------------------------
// T227: /agents tests
// ---------------------------------------------------------------------------

func TestAgents_DefaultList(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/agents")
	msg := cmd()
	agentsMsg, ok := msg.(AgentsMsg)
	if !ok {
		t.Fatalf("Expected AgentsMsg, got %T", msg)
	}
	if !strings.Contains(agentsMsg.Message, "Available agents") {
		t.Errorf("Expected 'Available agents' header, got: %s", agentsMsg.Message)
	}
	if !strings.Contains(agentsMsg.Message, "general-purpose") {
		t.Errorf("Expected 'general-purpose' agent, got: %s", agentsMsg.Message)
	}
	if !strings.Contains(agentsMsg.Message, "bash") {
		t.Errorf("Expected 'bash' agent, got: %s", agentsMsg.Message)
	}
}

func TestAgents_WithExtraAgents(t *testing.T) {
	d := NewDispatcher()
	d.Register("/agents", newAgentsHandler(func() []AgentConfig {
		return []AgentConfig{
			{Name: "custom-agent", Description: "A custom agent"},
		}
	}))

	cmd := d.Dispatch("/agents")
	msg := cmd()
	agentsMsg := msg.(AgentsMsg)
	if !strings.Contains(agentsMsg.Message, "custom-agent") {
		t.Errorf("Expected 'custom-agent' in list, got: %s", agentsMsg.Message)
	}
}

func TestAgents_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/agents")
	if reg == nil {
		t.Fatal("Expected registration for /agents")
	}
	if reg.Type != CommandTypeLocalJSX {
		t.Errorf("Expected CommandTypeLocalJSX, got %v", reg.Type)
	}
	if reg.Description != "Manage agent configurations" {
		t.Errorf("Expected 'Manage agent configurations', got %q", reg.Description)
	}
}

// ---------------------------------------------------------------------------
// T228: /branch tests
// ---------------------------------------------------------------------------

func TestBranch_ForksConversation(t *testing.T) {
	dir := t.TempDir()
	srcID := "sess-abc123"
	os.WriteFile(filepath.Join(dir, srcID+".jsonl"), []byte("{\"role\":\"user\"}\n"), 0644)

	var switchedTo string
	d := NewDispatcher()
	d.Register("/branch", newBranchHandler(BranchOptions{
		SessionID:     func() string { return srcID },
		SessionName:   func() string { return "My Chat" },
		TranscriptDir: func() string { return dir },
		SwitchSession: func(id string) { switchedTo = id },
	}))

	cmd := d.Dispatch("/branch")
	msg := cmd()
	bm, ok := msg.(BranchMsg)
	if !ok {
		t.Fatalf("Expected BranchMsg, got %T", msg)
	}
	if bm.Error != nil {
		t.Fatalf("Unexpected error: %v", bm.Error)
	}
	if !strings.Contains(bm.ForkName, " (Branch)") {
		t.Errorf("Fork name should contain ' (Branch)' suffix, got %q", bm.ForkName)
	}
	if !strings.Contains(bm.Message, "Forked conversation") {
		t.Errorf("Message should contain 'Forked conversation', got %q", bm.Message)
	}
	if switchedTo == "" {
		t.Error("SwitchSession should have been called")
	}

	// Verify the fork file was created
	forkPath := filepath.Join(dir, switchedTo+".jsonl")
	data, err := os.ReadFile(forkPath)
	if err != nil {
		t.Fatalf("Fork file should exist: %v", err)
	}
	if string(data) != "{\"role\":\"user\"}\n" {
		t.Errorf("Fork should be a copy of original, got %q", string(data))
	}
}

func TestBranch_MissingTranscript(t *testing.T) {
	dir := t.TempDir()
	d := NewDispatcher()
	d.Register("/branch", newBranchHandler(BranchOptions{
		SessionID:     func() string { return "nonexistent" },
		SessionName:   func() string { return "Gone" },
		TranscriptDir: func() string { return dir },
		SwitchSession: func(id string) {},
	}))

	cmd := d.Dispatch("/branch")
	msg := cmd()
	bm := msg.(BranchMsg)
	if bm.Error == nil {
		t.Error("Expected error for missing transcript")
	}
	if !strings.Contains(bm.Error.Error(), "cannot read transcript") {
		t.Errorf("Error should mention 'cannot read transcript', got: %v", bm.Error)
	}
}

func TestBranch_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/branch") {
		t.Fatal("Should have /branch handler")
	}
	reg := d.GetRegistration("/branch")
	if reg == nil {
		t.Fatal("Should have registration for /branch")
	}
	if reg.Description != "Fork the current conversation" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
}

// ---------------------------------------------------------------------------
// T229: /remote-control + /bridge-kick tests
// ---------------------------------------------------------------------------

func TestRemoteControl_StartsBridge(t *testing.T) {
	started := false
	d := NewDispatcher()
	d.Register("/remote-control", newRemoteControlHandler(
		func() bool { return false },
		func() error { started = true; return nil },
	))

	cmd := d.Dispatch("/remote-control")
	msg := cmd()
	rc, ok := msg.(RemoteControlMsg)
	if !ok {
		t.Fatalf("Expected RemoteControlMsg, got %T", msg)
	}
	if rc.Error != nil {
		t.Fatalf("Unexpected error: %v", rc.Error)
	}
	if !started {
		t.Error("Bridge should have been started")
	}
	if !strings.Contains(rc.Message, "Bridge started") {
		t.Errorf("Expected 'Bridge started' message, got %q", rc.Message)
	}
}

func TestRemoteControl_AlreadyConnected(t *testing.T) {
	d := NewDispatcher()
	d.Register("/remote-control", newRemoteControlHandler(
		func() bool { return true },
		func() error { return nil },
	))

	cmd := d.Dispatch("/remote-control")
	msg := cmd()
	rc := msg.(RemoteControlMsg)
	if !strings.Contains(rc.Message, "already connected") {
		t.Errorf("Expected 'already connected' message, got %q", rc.Message)
	}
}

func TestBridgeKick_NonAnt(t *testing.T) {
	d := NewDispatcher()
	d.Register("/bridge-kick", newBridgeKickHandler(func() bool { return false }))

	cmd := d.Dispatch("/bridge-kick")
	msg := cmd()
	bk, ok := msg.(BridgeKickMsg)
	if !ok {
		t.Fatalf("Expected BridgeKickMsg, got %T", msg)
	}
	if bk.Error == nil {
		t.Error("Expected error for non-ant user")
	}
	if !strings.Contains(bk.Error.Error(), "internal-only") {
		t.Errorf("Error should mention 'internal-only', got: %v", bk.Error)
	}
}

func TestBridgeKick_AntUser(t *testing.T) {
	d := NewDispatcher()
	d.Register("/bridge-kick", newBridgeKickHandler(func() bool { return true }))

	cmd := d.Dispatch("/bridge-kick")
	msg := cmd()
	bk := msg.(BridgeKickMsg)
	if bk.Error != nil {
		t.Fatalf("Unexpected error: %v", bk.Error)
	}
	if !strings.Contains(bk.Message, "diagnostics stub") {
		t.Errorf("Expected diagnostics stub message, got %q", bk.Message)
	}
}

func TestRemoteControl_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/remote-control") {
		t.Fatal("Should have /remote-control handler")
	}
	if !d.HasHandler("/bridge-kick") {
		t.Fatal("Should have /bridge-kick handler")
	}
	reg := d.GetRegistration("/bridge-kick")
	if reg == nil {
		t.Fatal("Should have registration for /bridge-kick")
	}
	if !reg.IsHidden {
		t.Error("/bridge-kick should be hidden")
	}
}

// ---------------------------------------------------------------------------
// T230: /brief tests
// ---------------------------------------------------------------------------

func TestBrief_Toggle(t *testing.T) {
	active := false
	d := NewDispatcher()
	d.Register("/brief", newBriefHandler(
		func() bool { return active },
		func(b bool) { active = b },
	))

	// Enable
	cmd := d.Dispatch("/brief")
	msg := cmd()
	bm, ok := msg.(BriefMsg)
	if !ok {
		t.Fatalf("Expected BriefMsg, got %T", msg)
	}
	if !bm.Active {
		t.Error("Expected Active=true after first toggle")
	}
	if bm.Message != "Brief mode enabled" {
		t.Errorf("Expected 'Brief mode enabled', got %q", bm.Message)
	}
	if !active {
		t.Error("Callback should have set active=true")
	}

	// Disable
	cmd = d.Dispatch("/brief")
	msg = cmd()
	bm = msg.(BriefMsg)
	if bm.Active {
		t.Error("Expected Active=false after second toggle")
	}
	if bm.Message != "Brief mode disabled" {
		t.Errorf("Expected 'Brief mode disabled', got %q", bm.Message)
	}
	if active {
		t.Error("Callback should have set active=false")
	}
}

func TestBrief_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/brief") {
		t.Fatal("Should have /brief handler")
	}
	reg := d.GetRegistration("/brief")
	if reg == nil {
		t.Fatal("Should have registration for /brief")
	}
	if reg.Description != "Toggle brief response mode" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
}

// ---------------------------------------------------------------------------
// T231: /btw tests
// ---------------------------------------------------------------------------

func TestBtw_SideQuestion(t *testing.T) {
	d := NewDispatcher()
	d.Register("/btw", newBtwHandler(func(q string) (string, error) {
		return "The answer is 42 for: " + q, nil
	}))

	cmd := d.Dispatch("/btw what is the meaning of life")
	msg := cmd()
	bm, ok := msg.(BtwMsg)
	if !ok {
		t.Fatalf("Expected BtwMsg, got %T", msg)
	}
	if bm.Error != nil {
		t.Fatalf("Unexpected error: %v", bm.Error)
	}
	if bm.Question != "what is the meaning of life" {
		t.Errorf("Expected question preserved, got %q", bm.Question)
	}
	if !strings.Contains(bm.Answer, "The answer is 42") {
		t.Errorf("Expected answer content, got %q", bm.Answer)
	}
}

func TestBtw_NoQuestion(t *testing.T) {
	d := NewDispatcher()
	d.Register("/btw", newBtwHandler(func(q string) (string, error) {
		return "", nil
	}))

	cmd := d.Dispatch("/btw")
	msg := cmd()
	bm := msg.(BtwMsg)
	if bm.Error == nil {
		t.Error("Expected error for empty question")
	}
	if !strings.Contains(bm.Error.Error(), "usage:") {
		t.Errorf("Error should contain usage hint, got: %v", bm.Error)
	}
}

func TestBtw_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/btw") {
		t.Fatal("Should have /btw handler")
	}
	reg := d.GetRegistration("/btw")
	if reg == nil {
		t.Fatal("Should have registration for /btw")
	}
	if reg.ArgumentHint != "<question>" {
		t.Errorf("Expected argument hint '<question>', got %q", reg.ArgumentHint)
	}
}

// ---------------------------------------------------------------------------
// T232: /chrome tests
// ---------------------------------------------------------------------------

func TestChrome_NoArgs_ShowsMenu(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/chrome")
	msg := cmd()
	cm, ok := msg.(ChromeMsg)
	if !ok {
		t.Fatalf("Expected ChromeMsg, got %T", msg)
	}
	if cm.Error != nil {
		t.Fatalf("Unexpected error: %v", cm.Error)
	}
	for _, action := range []string{"install", "reconnect", "manage-permissions", "toggle-default"} {
		if !strings.Contains(cm.Message, action) {
			t.Errorf("Menu should contain %q, got: %s", action, cm.Message)
		}
	}
}

func TestChrome_ValidAction(t *testing.T) {
	d := NewDispatcher()
	for _, action := range []string{"install", "reconnect", "manage-permissions", "toggle-default"} {
		cmd := d.Dispatch("/chrome " + action)
		msg := cmd()
		cm := msg.(ChromeMsg)
		if cm.Error != nil {
			t.Errorf("Unexpected error for %s: %v", action, cm.Error)
		}
		if cm.Action != ChromeAction(action) {
			t.Errorf("Expected action %q, got %q", action, cm.Action)
		}
		if !strings.Contains(cm.Message, "not yet implemented") {
			t.Errorf("Stub should say 'not yet implemented', got %q", cm.Message)
		}
	}
}

func TestChrome_UnknownAction(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/chrome bogus")
	msg := cmd()
	cm := msg.(ChromeMsg)
	if cm.Error == nil {
		t.Error("Expected error for unknown action")
	}
	if !strings.Contains(cm.Error.Error(), "unknown chrome action") {
		t.Errorf("Error should mention 'unknown chrome action', got: %v", cm.Error)
	}
}

func TestChrome_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/chrome") {
		t.Fatal("Should have /chrome handler")
	}
	reg := d.GetRegistration("/chrome")
	if reg == nil {
		t.Fatal("Should have registration for /chrome")
	}
	if reg.Description != "Chrome browser integration" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
}

// ---------------------------------------------------------------------------
// T228-T232: All new commands appear in registerDefaults
// ---------------------------------------------------------------------------

func TestNewCommandsRegisteredInDefaults(t *testing.T) {
	d := NewDispatcher()
	for _, name := range []string{"/branch", "/remote-control", "/bridge-kick", "/brief", "/btw", "/chrome", "/clear", "/color", "/commit", "/commit-push-pr", "/compact", "/hooks"} {
		if !d.HasHandler(name) {
			t.Errorf("Default dispatcher should have handler for %s", name)
		}
	}
}

// ---------------------------------------------------------------------------
// T233: /clear full clearing chain tests
// ---------------------------------------------------------------------------

func TestClear_FullChain(t *testing.T) {
	s := &session.SessionState{
		ID:              "old-id",
		Messages:        []message.Message{message.UserMessage("hello")},
		TurnCount:       5,
		TotalCostUSD:    1.23,
		TotalInputTokens: 1000,
		HasExitedPlanMode: true,
		PlanSlugCache:   map[string]string{"a": "b"},
		InvokedSkills:   map[string]bool{"x": true},
		CWD:             "/some/path",
	}
	planCleaned := false
	postClearCalled := false

	d := NewDispatcher()
	d.Register("/clear", newClearHandler(ClearState{
		Session:        func() *session.SessionState { return s },
		OriginalCWD:    func() string { return "/original" },
		ClearPlanSlugs: func() { planCleaned = true },
		OnPostClear:    func() { postClearCalled = true },
	}))

	cmd := d.Dispatch("/clear")
	msg := cmd()
	if _, ok := msg.(ClearConversationMsg); !ok {
		t.Fatalf("Expected ClearConversationMsg, got %T", msg)
	}

	// Session ID should have been regenerated.
	if s.ID == "old-id" {
		t.Error("Session ID should have been regenerated")
	}
	// Parent should be set to old ID.
	if s.ParentSessionID != "old-id" {
		t.Errorf("ParentSessionID = %q, want 'old-id'", s.ParentSessionID)
	}
	// Messages should be cleared.
	if len(s.Messages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(s.Messages))
	}
	// Turn count should be reset.
	if s.TurnCount != 0 {
		t.Errorf("TurnCount = %d, want 0", s.TurnCount)
	}
	// Costs reset.
	if s.TotalCostUSD != 0 {
		t.Errorf("TotalCostUSD = %f, want 0", s.TotalCostUSD)
	}
	if s.TotalInputTokens != 0 {
		t.Errorf("TotalInputTokens = %d, want 0", s.TotalInputTokens)
	}
	// Plan mode reset.
	if s.HasExitedPlanMode {
		t.Error("HasExitedPlanMode should be false")
	}
	// Plan slug cache cleared.
	if s.PlanSlugCache != nil {
		t.Error("PlanSlugCache should be nil")
	}
	// Invoked skills cleared.
	if s.InvokedSkills != nil {
		t.Error("InvokedSkills should be nil")
	}
	// CWD reset to original.
	if s.CWD != "/original" {
		t.Errorf("CWD = %q, want '/original'", s.CWD)
	}
	// Callbacks invoked.
	if !planCleaned {
		t.Error("ClearPlanSlugs should have been called")
	}
	if !postClearCalled {
		t.Error("OnPostClear should have been called")
	}
}

func TestClear_SetsEnvVar(t *testing.T) {
	s := &session.SessionState{ID: "before"}
	d := NewDispatcher()
	d.Register("/clear", newClearHandler(ClearState{
		Session: func() *session.SessionState { return s },
	}))

	cmd := d.Dispatch("/clear")
	cmd()

	envID := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if envID != s.ID {
		t.Errorf("CLAUDE_CODE_SESSION_ID = %q, want %q", envID, s.ID)
	}
}

func TestClear_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/clear")
	if reg == nil {
		t.Fatal("Should have registration for /clear")
	}
	if reg.Description != "Clear conversation and reset session" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
	if !reg.Immediate {
		t.Error("/clear should be immediate")
	}
}

// ---------------------------------------------------------------------------
// T234: /color tests
// ---------------------------------------------------------------------------

func TestColor_NoArgs_ShowsColors(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/color")
	msg := cmd()
	cm, ok := msg.(ColorMsg)
	if !ok {
		t.Fatalf("Expected ColorMsg, got %T", msg)
	}
	if cm.Error != nil {
		t.Fatalf("Unexpected error: %v", cm.Error)
	}
	if !strings.Contains(cm.Message, "Available colors") {
		t.Errorf("Expected color list, got: %s", cm.Message)
	}
	for _, c := range []string{"red", "blue", "green", "yellow", "purple", "orange", "pink", "cyan"} {
		if !strings.Contains(cm.Message, c) {
			t.Errorf("Missing color %q in message: %s", c, cm.Message)
		}
	}
}

func TestColor_SetValidColor(t *testing.T) {
	var current string
	d := NewDispatcher()
	d.Register("/color", newColorHandler(
		func() string { return current },
		func(c string) { current = c },
	))

	cmd := d.Dispatch("/color blue")
	msg := cmd()
	cm := msg.(ColorMsg)
	if cm.Error != nil {
		t.Fatalf("Unexpected error: %v", cm.Error)
	}
	if cm.Color != "blue" {
		t.Errorf("Expected color 'blue', got %q", cm.Color)
	}
	if current != "blue" {
		t.Errorf("Setter should have been called with 'blue', got %q", current)
	}
	if cm.Message != "Session color set to: blue" {
		t.Errorf("Unexpected message: %s", cm.Message)
	}
}

func TestColor_ResetAliases(t *testing.T) {
	for _, alias := range []string{"default", "reset", "none", "gray", "grey"} {
		var current string
		d := NewDispatcher()
		d.Register("/color", newColorHandler(
			func() string { return current },
			func(c string) { current = c },
		))

		cmd := d.Dispatch("/color " + alias)
		msg := cmd()
		cm := msg.(ColorMsg)
		if cm.Error != nil {
			t.Errorf("%s: unexpected error: %v", alias, cm.Error)
		}
		if cm.Color != "" {
			t.Errorf("%s: expected empty color for reset, got %q", alias, cm.Color)
		}
		if cm.Message != "Session color reset to default" {
			t.Errorf("%s: unexpected message: %s", alias, cm.Message)
		}
	}
}

func TestColor_InvalidColor(t *testing.T) {
	d := NewDispatcher()
	d.Register("/color", newColorHandler(
		func() string { return "" },
		func(c string) {},
	))

	cmd := d.Dispatch("/color magenta")
	msg := cmd()
	cm := msg.(ColorMsg)
	if cm.Error == nil {
		t.Fatal("Expected error for invalid color")
	}
	if !strings.Contains(cm.Error.Error(), "Invalid color") {
		t.Errorf("Error should mention 'Invalid color', got: %v", cm.Error)
	}
}

func TestColor_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/color")
	if reg == nil {
		t.Fatal("Should have registration for /color")
	}
	if reg.Description != "Set session prompt bar color" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
	if reg.ArgumentHint != "<color|default>" {
		t.Errorf("Unexpected argument hint: %q", reg.ArgumentHint)
	}
}

// ---------------------------------------------------------------------------
// T235: /commit tests
// ---------------------------------------------------------------------------

func TestCommit_ReturnsPromptMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/commit")
	msg := cmd()
	pm, ok := msg.(PromptMsg)
	if !ok {
		t.Fatalf("Expected PromptMsg, got %T", msg)
	}
	if pm.Command != "/commit" {
		t.Errorf("Expected command '/commit', got %q", pm.Command)
	}
	// Check key sections of the prompt template.
	for _, want := range []string{
		"## Context",
		"## Git Safety Protocol",
		"## Your task",
		"NEVER update the git config",
		"ALWAYS create NEW commits",
		"git commit -m",
		"HEREDOC syntax",
	} {
		if !strings.Contains(pm.Text, want) {
			t.Errorf("Prompt should contain %q", want)
		}
	}
}

func TestCommit_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/commit")
	if reg == nil {
		t.Fatal("Should have registration for /commit")
	}
	if reg.Type != CommandTypePrompt {
		t.Errorf("Expected CommandTypePrompt, got %v", reg.Type)
	}
	if reg.Description != "Create a git commit" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
}

// ---------------------------------------------------------------------------
// T236: /commit-push-pr tests
// ---------------------------------------------------------------------------

func TestCommitPushPR_ReturnsPromptMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/commit-push-pr")
	msg := cmd()
	pm, ok := msg.(PromptMsg)
	if !ok {
		t.Fatalf("Expected PromptMsg, got %T", msg)
	}
	if pm.Command != "/commit-push-pr" {
		t.Errorf("Expected command '/commit-push-pr', got %q", pm.Command)
	}
	for _, want := range []string{
		"## Context",
		"## Git Safety Protocol",
		"## Your task",
		"gh pr create",
		"gh pr edit",
		"NEVER run force push to main/master",
		"Keep PR titles short",
		"Return the PR URL",
	} {
		if !strings.Contains(pm.Text, want) {
			t.Errorf("Prompt should contain %q", want)
		}
	}
}

func TestCommitPushPR_WithArgs(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/commit-push-pr fix the tests first")
	msg := cmd()
	pm := msg.(PromptMsg)
	if !strings.Contains(pm.Text, "## Additional instructions from user") {
		t.Error("Should contain additional instructions section")
	}
	if !strings.Contains(pm.Text, "fix the tests first") {
		t.Error("Should contain the user's additional instructions")
	}
}

func TestCommitPushPR_NoArgsNoAdditionalSection(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/commit-push-pr")
	msg := cmd()
	pm := msg.(PromptMsg)
	if strings.Contains(pm.Text, "## Additional instructions from user") {
		t.Error("Should not contain additional instructions section when no args")
	}
}

func TestCommitPushPR_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/commit-push-pr")
	if reg == nil {
		t.Fatal("Should have registration for /commit-push-pr")
	}
	if reg.Type != CommandTypePrompt {
		t.Errorf("Expected CommandTypePrompt, got %v", reg.Type)
	}
	if reg.Description != "Commit, push, and open a PR" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
}

// ---------------------------------------------------------------------------
// T237: /compact tests
// ---------------------------------------------------------------------------

func TestCompact_Success(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{{Type: message.ContentText, Text: "hi there"}}},
	}
	var completedMsgs []message.Message

	d := NewDispatcher()
	d.Register("/compact", newCompactHandler(CompactDeps{
		GetMessages: func() []message.Message { return msgs },
		Summarize: func(ctx context.Context, m []message.Message, prompt string) (string, error) {
			return "Summary of conversation about greetings.", nil
		},
		TranscriptPath: func() string { return "/tmp/test.jsonl" },
		OnComplete:     func(m []message.Message) { completedMsgs = m },
	}))

	cmd := d.Dispatch("/compact")
	msg := cmd()
	cr, ok := msg.(CompactResultMsg)
	if !ok {
		t.Fatalf("Expected CompactResultMsg, got %T", msg)
	}
	if cr.Error != nil {
		t.Fatalf("Unexpected error: %v", cr.Error)
	}
	if cr.Result == nil {
		t.Fatal("Expected non-nil result")
	}
	if cr.Message != "Compacted conversation" {
		t.Errorf("Unexpected message: %s", cr.Message)
	}
	if len(completedMsgs) == 0 {
		t.Error("OnComplete should have been called with new messages")
	}
}

func TestCompact_NoMessages(t *testing.T) {
	d := NewDispatcher()
	d.Register("/compact", newCompactHandler(CompactDeps{
		GetMessages: func() []message.Message { return nil },
		Summarize: func(ctx context.Context, m []message.Message, prompt string) (string, error) {
			return "", nil
		},
	}))

	cmd := d.Dispatch("/compact")
	msg := cmd()
	cr := msg.(CompactResultMsg)
	if cr.Error == nil {
		t.Fatal("Expected error for no messages")
	}
	if !strings.Contains(cr.Error.Error(), "Not enough messages") {
		t.Errorf("Error should mention 'Not enough messages', got: %v", cr.Error)
	}
}

func TestCompact_SummarizerError(t *testing.T) {
	msgs := []message.Message{message.UserMessage("hi")}

	d := NewDispatcher()
	d.Register("/compact", newCompactHandler(CompactDeps{
		GetMessages: func() []message.Message { return msgs },
		Summarize: func(ctx context.Context, m []message.Message, prompt string) (string, error) {
			return "", fmt.Errorf("API error: model overloaded")
		},
	}))

	cmd := d.Dispatch("/compact")
	msg := cmd()
	cr := msg.(CompactResultMsg)
	if cr.Error == nil {
		t.Fatal("Expected error from summarizer")
	}
	if !strings.Contains(cr.Error.Error(), "Error during compaction") {
		t.Errorf("Error should wrap as compaction error, got: %v", cr.Error)
	}
}

func TestCompact_EmptySummary(t *testing.T) {
	msgs := []message.Message{message.UserMessage("hi")}

	d := NewDispatcher()
	d.Register("/compact", newCompactHandler(CompactDeps{
		GetMessages: func() []message.Message { return msgs },
		Summarize: func(ctx context.Context, m []message.Message, prompt string) (string, error) {
			return "   ", nil // whitespace-only = incomplete
		},
	}))

	cmd := d.Dispatch("/compact")
	msg := cmd()
	cr := msg.(CompactResultMsg)
	if cr.Error == nil {
		t.Fatal("Expected error for empty summary")
	}
	if !strings.Contains(cr.Error.Error(), "interrupted") {
		t.Errorf("Error should mention 'interrupted', got: %v", cr.Error)
	}
}

func TestCompact_UserAbort(t *testing.T) {
	msgs := []message.Message{message.UserMessage("hi")}

	d := NewDispatcher()
	d.Register("/compact", newCompactHandler(CompactDeps{
		GetMessages: func() []message.Message { return msgs },
		Summarize: func(ctx context.Context, m []message.Message, prompt string) (string, error) {
			return "", context.Canceled
		},
	}))

	cmd := d.Dispatch("/compact")
	msg := cmd()
	cr := msg.(CompactResultMsg)
	if cr.Error == nil {
		t.Fatal("Expected error for user abort")
	}
	if !strings.Contains(cr.Error.Error(), "canceled") && !strings.Contains(cr.Error.Error(), "Canceled") {
		t.Errorf("Error should mention cancellation, got: %v", cr.Error)
	}
}

func TestCompact_WithCustomInstructions(t *testing.T) {
	msgs := []message.Message{message.UserMessage("hi")}
	var receivedPrompt string

	d := NewDispatcher()
	d.Register("/compact", newCompactHandler(CompactDeps{
		GetMessages: func() []message.Message { return msgs },
		Summarize: func(ctx context.Context, m []message.Message, prompt string) (string, error) {
			receivedPrompt = prompt
			return "Summary with focus on testing.", nil
		},
		OnComplete: func(m []message.Message) {},
	}))

	cmd := d.Dispatch("/compact focus on the testing changes")
	msg := cmd()
	cr := msg.(CompactResultMsg)
	if cr.Error != nil {
		t.Fatalf("Unexpected error: %v", cr.Error)
	}
	// The custom instructions are passed through to the compact prompt builder,
	// which incorporates them into the system prompt. Verify the prompt is non-empty.
	if receivedPrompt == "" {
		t.Error("Summarizer should have received a non-empty prompt")
	}
}

func TestCompact_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/compact")
	if reg == nil {
		t.Fatal("Should have registration for /compact")
	}
	if reg.Description != "Compact conversation history" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
	if reg.ArgumentHint != "[custom instructions]" {
		t.Errorf("Unexpected argument hint: %q", reg.ArgumentHint)
	}
}

// ---------------------------------------------------------------------------
// T238: /config dispatch test
// ---------------------------------------------------------------------------

func TestConfig_DispatchReturnsShowSettingsMsg(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/config") {
		t.Fatal("Should have /config handler")
	}
	cmd := d.Dispatch("/config")
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	msg := cmd()
	if _, ok := msg.(ShowSettingsMsg); !ok {
		t.Errorf("Expected ShowSettingsMsg, got %T", msg)
	}
}

func TestConfig_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/config")
	if reg == nil {
		t.Fatal("Should have registration for /config")
	}
	if reg.Description != "Open settings" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
	if reg.Type != CommandTypeLocalJSX {
		t.Errorf("Expected CommandTypeLocalJSX, got %v", reg.Type)
	}
}

// ---------------------------------------------------------------------------
// T239: /context dispatch test
// ---------------------------------------------------------------------------

func TestContext_DispatchReturnsContextAnalysisMsg(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/context") {
		t.Fatal("Should have /context handler")
	}
	cmd := d.Dispatch("/context")
	msg := cmd()
	result, ok := msg.(ContextAnalysisMsg)
	if !ok {
		t.Fatalf("Expected ContextAnalysisMsg, got %T", msg)
	}
	if result.Stats == nil {
		t.Fatal("Stats should not be nil")
	}
	if !strings.Contains(result.Output, "Context Window Usage") {
		t.Errorf("Output should contain 'Context Window Usage', got %q", result.Output)
	}
}

func TestContext_WithMessages(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Hello, this is a test message with some tokens"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Here is my response with some content"},
		}},
	}
	handler := newContextHandler(ContextDeps{
		GetMessages:       func() []message.Message { return msgs },
		ContextWindowSize: func() int { return 200000 },
	})
	cmd := handler("")
	msg := cmd()
	result := msg.(ContextAnalysisMsg)
	if result.Stats.Total == 0 {
		t.Error("Total tokens should be > 0 with messages")
	}
	if result.Stats.HumanMessages == 0 {
		t.Error("HumanMessages should be > 0")
	}
	if result.Stats.AssistantMessages == 0 {
		t.Error("AssistantMessages should be > 0")
	}
	if !strings.Contains(result.Output, "| Human messages |") {
		t.Error("Output should contain markdown table rows")
	}
}

// ---------------------------------------------------------------------------
// T240: /copy dispatch test
// ---------------------------------------------------------------------------

func TestCopy_DispatchNoMessages(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/copy") {
		t.Fatal("Should have /copy handler")
	}
	cmd := d.Dispatch("/copy")
	msg := cmd()
	result, ok := msg.(CopyMsg)
	if !ok {
		t.Fatalf("Expected CopyMsg, got %T", msg)
	}
	if result.Error == nil {
		t.Error("Expected error when no messages")
	}
	if !strings.Contains(result.Error.Error(), "no assistant response found") {
		t.Errorf("Unexpected error: %v", result.Error)
	}
}

func TestCopy_WithAssistantMessage(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Hello"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "This is my response"},
		}},
	}
	handler := newCopyHandler(CopyDeps{
		GetMessages: func() []message.Message { return msgs },
	})
	cmd := handler("")
	msg := cmd()
	result := msg.(CopyMsg)
	// In test environment, OSC52 may fail so it falls back to file write,
	// or it may succeed — either way, content should be captured.
	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}
	if result.Content != "This is my response" {
		t.Errorf("Expected content 'This is my response', got %q", result.Content)
	}
	if result.Message != "Copied to clipboard" && !strings.HasPrefix(result.Message, "Written to") {
		t.Errorf("Expected 'Copied to clipboard' or 'Written to...', got %q", result.Message)
	}
}

func TestCopy_NthResponse(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "First response"},
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Follow up"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Second response"},
		}},
	}
	handler := newCopyHandler(CopyDeps{
		GetMessages: func() []message.Message { return msgs },
	})
	// N=2 should get the first (older) assistant response
	cmd := handler("2")
	msg := cmd()
	result := msg.(CopyMsg)
	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}
	if result.Content != "First response" {
		t.Errorf("Expected 'First response', got %q", result.Content)
	}
}

func TestCopy_InvalidArg(t *testing.T) {
	handler := newCopyHandler(CopyDeps{
		GetMessages: func() []message.Message { return nil },
	})
	cmd := handler("abc")
	msg := cmd()
	result := msg.(CopyMsg)
	if result.Error == nil {
		t.Error("Expected error for invalid arg")
	}
}

func TestCopy_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/copy")
	if reg == nil {
		t.Fatal("Should have registration for /copy")
	}
	if reg.ArgumentHint != "[N]" {
		t.Errorf("Unexpected argument hint: %q", reg.ArgumentHint)
	}
}

// ---------------------------------------------------------------------------
// T241: /cost dispatch test
// ---------------------------------------------------------------------------

func TestCost_DispatchReturnsCostMsg(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/cost") {
		t.Fatal("Should have /cost handler")
	}
	cmd := d.Dispatch("/cost")
	msg := cmd()
	result, ok := msg.(CostMsg)
	if !ok {
		t.Fatalf("Expected CostMsg, got %T", msg)
	}
	// Default session: $0.0000, 0 tokens
	if !strings.Contains(result.Message, "Session cost: $0.0000") {
		t.Errorf("Expected 'Session cost: $0.0000' in message, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "Total tokens: 0") {
		t.Errorf("Expected 'Total tokens: 0' in message, got %q", result.Message)
	}
}

func TestCost_WithSessionData(t *testing.T) {
	s := &session.SessionState{
		TotalCostUSD:      0.1234,
		TotalInputTokens:  5000,
		TotalOutputTokens: 3000,
	}
	handler := newCostHandler(CostDeps{
		GetSession: func() *session.SessionState { return s },
	})
	cmd := handler("")
	msg := cmd()
	result := msg.(CostMsg)
	if !strings.Contains(result.Message, "Session cost: $0.1234") {
		t.Errorf("Expected 'Session cost: $0.1234', got %q", result.Message)
	}
	if !strings.Contains(result.Message, "Total tokens: 8000") {
		t.Errorf("Expected 'Total tokens: 8000', got %q", result.Message)
	}
}

// ---------------------------------------------------------------------------
// T242: /desktop dispatch test
// ---------------------------------------------------------------------------

func TestDesktop_DispatchReturnsDesktopMsg(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/desktop") {
		t.Fatal("Should have /desktop handler")
	}
	cmd := d.Dispatch("/desktop")
	msg := cmd()
	result, ok := msg.(DesktopMsg)
	if !ok {
		t.Fatalf("Expected DesktopMsg, got %T", msg)
	}
	// Platform-dependent behavior
	switch runtime.GOOS {
	case "darwin", "windows":
		if result.Error != nil {
			t.Errorf("Unexpected error on %s: %v", runtime.GOOS, result.Error)
		}
		if result.Message != "Opening in Claude Desktop..." {
			t.Errorf("Expected 'Opening in Claude Desktop...', got %q", result.Message)
		}
	default:
		if result.Error == nil {
			t.Error("Expected error on unsupported platform")
		}
		if !strings.Contains(result.Message, "only available on macOS and Windows") {
			t.Errorf("Expected platform message, got %q", result.Message)
		}
	}
}

func TestDesktop_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/desktop")
	if reg == nil {
		t.Fatal("Should have registration for /desktop")
	}
	if reg.Description != "Open in Claude Desktop" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
}

// ---------------------------------------------------------------------------
// T243: /diff tests
// ---------------------------------------------------------------------------

func TestDiff_Registered(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/diff") {
		t.Fatal("Should have /diff handler")
	}
	reg := d.GetRegistration("/diff")
	if reg == nil {
		t.Fatal("Should have registration for /diff")
	}
	if reg.Description != "Show uncommitted changes" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
}

func TestDiff_ReturnsShowDiffMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/diff")
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	msg := cmd()
	result, ok := msg.(ShowDiffMsg)
	if !ok {
		t.Fatalf("Expected ShowDiffMsg, got %T", msg)
	}
	// In a git repo, output should be non-empty (either diff or "No uncommitted changes.")
	if result.Output == "" {
		t.Error("Expected non-empty diff output")
	}
}

// ---------------------------------------------------------------------------
// T244: /doctor tests (verify existing registration)
// ---------------------------------------------------------------------------

func TestDoctor_Registered(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/doctor") {
		t.Fatal("Should have /doctor handler")
	}
	cmd := d.Dispatch("/doctor")
	msg := cmd()
	if _, ok := msg.(ShowDoctorMsg); !ok {
		t.Fatalf("Expected ShowDoctorMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// T245: /effort tests
// ---------------------------------------------------------------------------

func TestEffort_Registered(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/effort") {
		t.Fatal("Should have /effort handler")
	}
	reg := d.GetRegistration("/effort")
	if reg == nil {
		t.Fatal("Should have registration for /effort")
	}
	if reg.ArgumentHint != "[low|medium|high|max|auto]" {
		t.Errorf("Unexpected argument hint: %q", reg.ArgumentHint)
	}
}

func TestEffort_ShowCurrent(t *testing.T) {
	h := newEffortHandler(EffortDeps{
		GetLevel: func() string { return "high" },
		SetLevel: func(level string) error { return nil },
	})
	msg := h("")()
	result, ok := msg.(EffortMsg)
	if !ok {
		t.Fatalf("Expected EffortMsg, got %T", msg)
	}
	if result.Level != "high" {
		t.Errorf("Expected level 'high', got %q", result.Level)
	}
	if !strings.Contains(result.Message, "high") {
		t.Errorf("Message should mention 'high': %q", result.Message)
	}
}

func TestEffort_ShowCurrentAuto(t *testing.T) {
	h := newEffortHandler(EffortDeps{
		GetLevel: func() string { return "auto" },
		SetLevel: func(level string) error { return nil },
	})
	msg := h("")()
	result := msg.(EffortMsg)
	if result.Level != "auto" {
		t.Errorf("Expected level 'auto', got %q", result.Level)
	}
	if result.Message != "Effort level: auto" {
		t.Errorf("Unexpected message: %q", result.Message)
	}
}

func TestEffort_SetLevel(t *testing.T) {
	var saved string
	h := newEffortHandler(EffortDeps{
		GetLevel: func() string { return "auto" },
		SetLevel: func(level string) error { saved = level; return nil },
	})
	msg := h("medium")()
	result := msg.(EffortMsg)
	if result.Level != "medium" {
		t.Errorf("Expected level 'medium', got %q", result.Level)
	}
	if saved != "medium" {
		t.Errorf("Expected SetLevel called with 'medium', got %q", saved)
	}
	if !strings.Contains(result.Message, "Balanced approach") {
		t.Errorf("Message should contain description: %q", result.Message)
	}
}

func TestEffort_InvalidArg(t *testing.T) {
	h := newEffortHandler(EffortDeps{
		GetLevel: func() string { return "auto" },
		SetLevel: func(level string) error { return nil },
	})
	msg := h("banana")()
	result := msg.(EffortMsg)
	if result.Error == nil {
		t.Fatal("Expected error for invalid effort level")
	}
	if !strings.Contains(result.Error.Error(), "Invalid argument: banana") {
		t.Errorf("Unexpected error: %v", result.Error)
	}
}

func TestEffort_Help(t *testing.T) {
	h := newEffortHandler(EffortDeps{
		GetLevel: func() string { return "auto" },
		SetLevel: func(level string) error { return nil },
	})
	for _, arg := range []string{"help", "-h", "--help"} {
		msg := h(arg)()
		result := msg.(EffortMsg)
		if !strings.Contains(result.Message, "Usage: /effort") {
			t.Errorf("help arg %q: expected usage text, got %q", arg, result.Message)
		}
	}
}

func TestEffort_EnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_CODE_EFFORT_LEVEL", "high")
	h := newEffortHandler(EffortDeps{
		GetLevel: func() string { return "auto" },
		SetLevel: func(level string) error { return nil },
	})
	msg := h("medium")()
	result := msg.(EffortMsg)
	if !strings.Contains(result.Message, "CLAUDE_CODE_EFFORT_LEVEL=high overrides") {
		t.Errorf("Expected env override message, got %q", result.Message)
	}
}

func TestEffort_AutoClearsWithEnvWarning(t *testing.T) {
	t.Setenv("CLAUDE_CODE_EFFORT_LEVEL", "max")
	h := newEffortHandler(EffortDeps{
		GetLevel: func() string { return "high" },
		SetLevel: func(level string) error { return nil },
	})
	msg := h("auto")()
	result := msg.(EffortMsg)
	if !strings.Contains(result.Message, "CLAUDE_CODE_EFFORT_LEVEL=max still controls") {
		t.Errorf("Expected env warning message, got %q", result.Message)
	}
}

// ---------------------------------------------------------------------------
// T246: /exit tests
// ---------------------------------------------------------------------------

func TestExit_Registered(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/exit") {
		t.Fatal("Should have /exit handler")
	}
	if !d.HasHandler("/quit") {
		t.Fatal("/quit should be aliased to /exit")
	}
}

func TestExit_ReturnsGoodbyeMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/exit")
	msg := cmd()
	result, ok := msg.(ExitGoodbyeMsg)
	if !ok {
		t.Fatalf("Expected ExitGoodbyeMsg, got %T", msg)
	}
	valid := map[string]bool{
		"Goodbye!":         true,
		"See ya!":          true,
		"Bye!":             true,
		"Catch you later!": true,
	}
	if !valid[result.Message] {
		t.Errorf("Unexpected goodbye message: %q", result.Message)
	}
}

func TestExit_QuitAlias(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/quit")
	msg := cmd()
	if _, ok := msg.(ExitGoodbyeMsg); !ok {
		t.Fatalf("Expected ExitGoodbyeMsg via /quit alias, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// T247: /export tests
// ---------------------------------------------------------------------------

func TestExport_Registered(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/export") {
		t.Fatal("Should have /export handler")
	}
	reg := d.GetRegistration("/export")
	if reg == nil {
		t.Fatal("Should have registration for /export")
	}
	if reg.Description != "Export conversation to file" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
}

func TestExport_NoMessages(t *testing.T) {
	h := newExportHandler(ExportDeps{
		GetMessages: func() []message.Message { return nil },
	})
	msg := h("")()
	result := msg.(ExportMsg)
	if result.Error == nil {
		t.Fatal("Expected error when no messages")
	}
	if !strings.Contains(result.Error.Error(), "no messages to export") {
		t.Errorf("Unexpected error: %v", result.Error)
	}
}

func TestExport_WritesFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentBlock{{Type: message.ContentText, Text: "Hello world"}}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{{Type: message.ContentText, Text: "Hi there"}}},
	}
	h := newExportHandler(ExportDeps{
		GetMessages: func() []message.Message { return msgs },
	})
	msg := h("")()
	result := msg.(ExportMsg)
	if result.Error != nil {
		t.Fatalf("Unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Message, "Conversation exported to:") {
		t.Errorf("Unexpected message: %q", result.Message)
	}
	if !strings.HasSuffix(result.Path, ".txt") {
		t.Errorf("Expected .txt file, got %q", result.Path)
	}
	// Verify file content
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("Failed to read exported file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "User:\nHello world") {
		t.Errorf("Expected user message in export, got %q", content)
	}
	if !strings.Contains(content, "Assistant:\nHi there") {
		t.Errorf("Expected assistant message in export, got %q", content)
	}
}

func TestExport_CustomFilename(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentBlock{{Type: message.ContentText, Text: "test"}}},
	}
	h := newExportHandler(ExportDeps{
		GetMessages: func() []message.Message { return msgs },
	})
	msg := h("my-chat.md")()
	result := msg.(ExportMsg)
	if result.Error != nil {
		t.Fatalf("Unexpected error: %v", result.Error)
	}
	// Should replace .md with .txt
	if !strings.HasSuffix(result.Path, "my-chat.txt") {
		t.Errorf("Expected my-chat.txt, got %q", result.Path)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Hello World!", "hello-world"},
		{"test---file", "test-file"},
		{"-leading-trailing-", "leading-trailing"},
		{"UPPER CASE", "upper-case"},
		{"special@#$chars", "specialchars"},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractFirstPrompt(t *testing.T) {
	short := message.Message{
		Role:    message.RoleUser,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "short"}},
	}
	long := message.Message{
		Role:    message.RoleUser,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: strings.Repeat("a", 100)}},
	}

	got := extractFirstPrompt([]message.Message{short})
	if got != "short" {
		t.Errorf("Expected 'short', got %q", got)
	}

	got = extractFirstPrompt([]message.Message{long})
	if len(got) != 52 { // 49 chars + 3-byte ellipsis
		t.Errorf("Expected 52 bytes (49 chars + ellipsis), got %d: %q", len(got), got)
	}
	if !strings.HasSuffix(got, "\u2026") {
		t.Errorf("Expected ellipsis suffix, got %q", got)
	}
}

func TestIsEffortLevel(t *testing.T) {
	for _, level := range []string{"low", "medium", "high", "max", "auto"} {
		if !isEffortLevel(level) {
			t.Errorf("Expected %q to be a valid effort level", level)
		}
	}
	if isEffortLevel("banana") {
		t.Error("'banana' should not be a valid effort level")
	}
}

// ---------------------------------------------------------------------------
// T248: /extra-usage
// ---------------------------------------------------------------------------

func TestExtraUsage_ClaudeAIUser(t *testing.T) {
	h := newExtraUsageHandler(func() string { return "claude-ai" })
	msg := h("")()
	m, ok := msg.(ExtraUsageMsg)
	if !ok {
		t.Fatalf("expected ExtraUsageMsg, got %T", msg)
	}
	if m.Error != nil {
		t.Fatalf("unexpected error: %v", m.Error)
	}
	if !strings.Contains(m.Message, "https://claude.ai/settings/billing") {
		t.Errorf("expected billing URL, got %q", m.Message)
	}
}

func TestExtraUsage_NonClaudeAIUser(t *testing.T) {
	h := newExtraUsageHandler(func() string { return "console" })
	msg := h("")()
	m := msg.(ExtraUsageMsg)
	if m.Error == nil {
		t.Fatal("expected error for non-claude-ai user")
	}
}

func TestExtraUsage_RegisteredInDispatcher(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/extra-usage") {
		t.Fatal("/extra-usage not registered")
	}
}

// ---------------------------------------------------------------------------
// T249: /fast
// ---------------------------------------------------------------------------

func TestFast_ToggleOn(t *testing.T) {
	var state bool
	h := newFastHandler(FastModeDeps{
		GetEnabled: func() bool { return state },
		SetEnabled: func(b bool) { state = b },
	})
	msg := h("")()
	m := msg.(FastModeMsg)
	if m.Error != nil {
		t.Fatalf("unexpected error: %v", m.Error)
	}
	if !m.Enabled || m.Message != "Fast mode enabled" {
		t.Errorf("expected enabled, got %+v", m)
	}
	if !state {
		t.Error("state should be true")
	}
}

func TestFast_ExplicitOff(t *testing.T) {
	var state bool = true
	h := newFastHandler(FastModeDeps{
		GetEnabled: func() bool { return state },
		SetEnabled: func(b bool) { state = b },
	})
	msg := h("off")()
	m := msg.(FastModeMsg)
	if m.Enabled || m.Message != "Fast mode disabled" {
		t.Errorf("expected disabled, got %+v", m)
	}
}

func TestFast_InvalidArg(t *testing.T) {
	h := newFastHandler(FastModeDeps{
		GetEnabled: func() bool { return false },
		SetEnabled: func(bool) {},
	})
	msg := h("banana")()
	m := msg.(FastModeMsg)
	if m.Error == nil {
		t.Fatal("expected error for invalid arg")
	}
}

func TestFast_RegisteredInDispatcher(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/fast") {
		t.Fatal("/fast not registered")
	}
}

// ---------------------------------------------------------------------------
// T250: /feedback
// ---------------------------------------------------------------------------

func TestFeedback_ShowsURL(t *testing.T) {
	h := newFeedbackHandler()
	msg := h("")()
	m, ok := msg.(FeedbackMsg)
	if !ok {
		t.Fatalf("expected FeedbackMsg, got %T", msg)
	}
	if !strings.Contains(m.Message, "https://github.com/anthropics/claude-code/issues") {
		t.Errorf("expected feedback URL in message, got %q", m.Message)
	}
	if m.URL != feedbackURL {
		t.Errorf("expected URL=%q, got %q", feedbackURL, m.URL)
	}
}

func TestFeedback_DisabledByEnv(t *testing.T) {
	t.Setenv("DISABLE_FEEDBACK_COMMAND", "1")
	d := NewDispatcher()
	cmd := d.Dispatch("/feedback")
	msg := cmd()
	result, ok := msg.(CommandResult)
	if !ok {
		t.Fatalf("expected CommandResult (disabled), got %T", msg)
	}
	if result.Error == nil {
		t.Fatal("expected disabled error")
	}
}

func TestFeedback_RegisteredInDispatcher(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/feedback") {
		t.Fatal("/feedback not registered")
	}
}

// ---------------------------------------------------------------------------
// T251: /files
// ---------------------------------------------------------------------------

func TestFiles_NoFiles(t *testing.T) {
	h := newFilesHandler(FilesDeps{GetFiles: func() []string { return nil }})
	msg := h("")()
	m := msg.(FilesMsg)
	if m.Message != "No files in context" {
		t.Errorf("expected 'No files in context', got %q", m.Message)
	}
}

func TestFiles_WithFiles(t *testing.T) {
	h := newFilesHandler(FilesDeps{GetFiles: func() []string { return []string{"a.go", "b.go"} }})
	msg := h("")()
	m := msg.(FilesMsg)
	if !strings.HasPrefix(m.Message, "Files in context:\n") {
		t.Errorf("expected 'Files in context:' header, got %q", m.Message)
	}
	if !strings.Contains(m.Message, "a.go") || !strings.Contains(m.Message, "b.go") {
		t.Errorf("expected file names, got %q", m.Message)
	}
}

func TestFiles_AntOnlyGate(t *testing.T) {
	t.Setenv("USER_TYPE", "console")
	d := NewDispatcher()
	cmd := d.Dispatch("/files")
	msg := cmd()
	result, ok := msg.(CommandResult)
	if !ok {
		t.Fatalf("expected CommandResult (disabled), got %T", msg)
	}
	if result.Error == nil {
		t.Fatal("expected disabled error for non-ant user")
	}
}

func TestFiles_EnabledForAnt(t *testing.T) {
	t.Setenv("USER_TYPE", "ant")
	d := NewDispatcher()
	reg := d.GetRegistration("/files")
	if reg == nil {
		t.Fatal("/files not registered")
	}
	if reg.IsEnabled != nil && !reg.IsEnabled() {
		t.Fatal("/files should be enabled for ant users")
	}
}

// ---------------------------------------------------------------------------
// T252: /heapdump
// ---------------------------------------------------------------------------

func TestHeapdump_WritesProfile(t *testing.T) {
	// Replace writeHeapProfile with a stub to avoid real profiling
	orig := writeHeapProfile
	defer func() { writeHeapProfile = orig }()
	writeHeapProfile = func(w io.Writer) error {
		_, err := w.Write([]byte("fake-heap"))
		return err
	}

	h := newHeapdumpHandler()
	msg := h("")()
	m, ok := msg.(HeapdumpMsg)
	if !ok {
		t.Fatalf("expected HeapdumpMsg, got %T", msg)
	}
	if m.Error != nil {
		t.Fatalf("unexpected error: %v", m.Error)
	}
	if m.Path == "" {
		t.Fatal("expected non-empty path")
	}
	if !strings.Contains(m.Message, m.Path) {
		t.Errorf("message should contain path, got %q", m.Message)
	}
	// Verify file was written
	data, err := os.ReadFile(m.Path)
	if err != nil {
		t.Fatalf("cannot read heap dump: %v", err)
	}
	if string(data) != "fake-heap" {
		t.Errorf("expected 'fake-heap', got %q", string(data))
	}
	os.Remove(m.Path)
}

func TestHeapdump_Hidden(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/heapdump")
	if reg == nil {
		t.Fatal("/heapdump not registered")
	}
	if !reg.IsHidden {
		t.Error("/heapdump should be hidden")
	}
}

func TestHeapdump_ProfileError(t *testing.T) {
	orig := writeHeapProfile
	defer func() { writeHeapProfile = orig }()
	writeHeapProfile = func(w io.Writer) error {
		return fmt.Errorf("pprof failure")
	}

	h := newHeapdumpHandler()
	msg := h("")()
	m := msg.(HeapdumpMsg)
	if m.Error == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(m.Error.Error(), "pprof failure") {
		t.Errorf("expected pprof failure message, got %q", m.Error.Error())
	}
}

// ---------------------------------------------------------------------------
// T254: /hooks — show/manage hook configuration
// ---------------------------------------------------------------------------

func TestHooksHandler_NoHooks(t *testing.T) {
	h := newHooksHandler(HooksDeps{
		GetHooks:     func() []hooks.IndividualHookConfig { return nil },
		GetToolNames: func() []string { return nil },
	})
	msg := h("")()
	m, ok := msg.(HooksMsg)
	if !ok {
		t.Fatalf("Expected HooksMsg, got %T", msg)
	}
	if !strings.Contains(m.Message, "No hooks configured") {
		t.Errorf("Expected 'No hooks configured' message, got %q", m.Message)
	}
	if !strings.Contains(m.Message, "settings.json") {
		t.Errorf("Expected settings.json hint in message, got %q", m.Message)
	}
}

func TestHooksHandler_WithHooks(t *testing.T) {
	h := newHooksHandler(HooksDeps{
		GetHooks: func() []hooks.IndividualHookConfig {
			return []hooks.IndividualHookConfig{
				{
					Event:  hooks.PreToolUse,
					Config: hooks.HookCommand{Type: hooks.HookCommandTypeBash, Command: "echo pre-tool"},
					Source: hooks.HookSourceUserSettings,
				},
				{
					Event:   hooks.PreToolUse,
					Config:  hooks.HookCommand{Type: hooks.HookCommandTypeBash, Command: "echo bash-only"},
					Matcher: "Bash",
					Source:  hooks.HookSourceProjectSettings,
				},
				{
					Event:  hooks.SessionStart,
					Config: hooks.HookCommand{Type: hooks.HookCommandTypePrompt, Prompt: "Welcome!"},
					Source: hooks.HookSourceLocalSettings,
				},
			}
		},
		GetToolNames: func() []string { return []string{"Bash", "Read", "Write"} },
	})

	msg := h("")()
	m, ok := msg.(HooksMsg)
	if !ok {
		t.Fatalf("Expected HooksMsg, got %T", msg)
	}

	// Should contain section headers
	if !strings.Contains(m.Message, "Configured Hooks") {
		t.Error("Expected 'Configured Hooks' header")
	}
	if !strings.Contains(m.Message, "PreToolUse") {
		t.Error("Expected PreToolUse event section")
	}
	if !strings.Contains(m.Message, "SessionStart") {
		t.Error("Expected SessionStart event section")
	}

	// Should contain hook details
	if !strings.Contains(m.Message, "echo pre-tool") {
		t.Error("Expected 'echo pre-tool' command")
	}
	if !strings.Contains(m.Message, "echo bash-only") {
		t.Error("Expected 'echo bash-only' command")
	}
	if !strings.Contains(m.Message, "Welcome!") {
		t.Error("Expected 'Welcome!' prompt")
	}

	// Should contain source labels
	if !strings.Contains(m.Message, "User") {
		t.Error("Expected User source label")
	}
	if !strings.Contains(m.Message, "Project") {
		t.Error("Expected Project source label")
	}
	if !strings.Contains(m.Message, "Local") {
		t.Error("Expected Local source label")
	}

	// Should contain matcher info
	if !strings.Contains(m.Message, "tool_name=Bash") {
		t.Error("Expected tool_name=Bash matcher")
	}

	// Should contain total count
	if !strings.Contains(m.Message, "Total: 3 hook(s)") {
		t.Errorf("Expected 'Total: 3 hook(s)', got %q", m.Message)
	}
}

func TestHooksHandler_Dispatch(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/hooks")
	if cmd == nil {
		t.Fatal("Expected non-nil command for /hooks")
	}
	msg := cmd()
	if _, ok := msg.(HooksMsg); !ok {
		t.Fatalf("Expected HooksMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// T255: /ide tests
// ---------------------------------------------------------------------------

func TestIDE_Registered(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/ide") {
		t.Fatal("Should have /ide handler")
	}
	reg := d.GetRegistration("/ide")
	if reg == nil {
		t.Fatal("Should have registration for /ide")
	}
	if reg.Description != "Detect installed IDEs and extensions" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
	if reg.Type != CommandTypeLocal {
		t.Errorf("Expected CommandTypeLocal, got %v", reg.Type)
	}
}

func TestIDE_ReturnsIDEMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/ide")
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	msg := cmd()
	result, ok := msg.(IDEMsg)
	if !ok {
		t.Fatalf("Expected IDEMsg, got %T", msg)
	}
	if result.Message == "" {
		t.Error("Expected non-empty message")
	}
	if !strings.Contains(result.Message, "IDE Detection Results") {
		t.Errorf("Expected detection header in message, got %q", result.Message)
	}
}

func TestIDE_WithMockDetector(t *testing.T) {
	orig := ideDetector
	defer func() { ideDetector = orig }()

	ideDetector = func() []IDEInfo {
		return []IDEInfo{
			{Name: "VS Code", Path: "/usr/bin/code", Installed: true, Extension: "installed"},
			{Name: "IntelliJ IDEA", Path: "/opt/idea", Installed: true, Extension: "not-installed"},
		}
	}

	h := newIDEHandler()
	msg := h("")()
	result, ok := msg.(IDEMsg)
	if !ok {
		t.Fatalf("Expected IDEMsg, got %T", msg)
	}
	if len(result.IDEs) != 2 {
		t.Fatalf("Expected 2 IDEs, got %d", len(result.IDEs))
	}
	if !strings.Contains(result.Message, "VS Code") {
		t.Error("Expected VS Code in message")
	}
	if !strings.Contains(result.Message, "Claude extension installed") {
		t.Error("Expected 'Claude extension installed' for VS Code")
	}
	if !strings.Contains(result.Message, "Claude extension not installed") {
		t.Error("Expected 'Claude extension not installed' for IntelliJ")
	}
}

func TestIDE_NoIDEsDetected(t *testing.T) {
	orig := ideDetector
	defer func() { ideDetector = orig }()

	ideDetector = func() []IDEInfo {
		return []IDEInfo{
			{Name: "VS Code", Installed: false, Extension: "unknown"},
		}
	}

	h := newIDEHandler()
	msg := h("")()
	result := msg.(IDEMsg)
	if !strings.Contains(result.Message, "No supported IDEs detected") {
		t.Errorf("Expected 'No supported IDEs detected' in message, got %q", result.Message)
	}
}

func TestIDE_DetectVSCode(t *testing.T) {
	// Just verify detectVSCode returns valid IDEInfo
	info := detectVSCode()
	if info.Name != "VS Code" {
		t.Errorf("Expected name 'VS Code', got %q", info.Name)
	}
	// Extension should be one of the valid values
	validExts := map[string]bool{"installed": true, "not-installed": true, "unknown": true}
	if !validExts[info.Extension] {
		t.Errorf("Unexpected extension status: %q", info.Extension)
	}
}

// ---------------------------------------------------------------------------
// T256: /init tests
// ---------------------------------------------------------------------------

func TestInit_ReturnsPromptMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/init")
	msg := cmd()
	pm, ok := msg.(PromptMsg)
	if !ok {
		t.Fatalf("Expected PromptMsg, got %T", msg)
	}
	if pm.Command != "/init" {
		t.Errorf("Expected command '/init', got %q", pm.Command)
	}
	for _, want := range []string{
		"Please analyze this codebase and create a CLAUDE.md file",
		"Commands that will be commonly used",
		"High-level code architecture and structure",
		"If there's already a CLAUDE.md, suggest improvements",
		"# CLAUDE.md",
	} {
		if !strings.Contains(pm.Text, want) {
			t.Errorf("Prompt should contain %q", want)
		}
	}
}

func TestInit_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/init")
	if reg == nil {
		t.Fatal("Should have registration for /init")
	}
	if reg.Type != CommandTypePrompt {
		t.Errorf("Expected CommandTypePrompt, got %v", reg.Type)
	}
	if reg.Description != "Initialize a new CLAUDE.md file with codebase documentation" {
		t.Errorf("Unexpected description: %q", reg.Description)
	}
}

func TestInit_PromptContainsNoGenericAdvice(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/init")
	msg := cmd()
	pm := msg.(PromptMsg)
	// The prompt itself instructs the model NOT to include generic advice
	if !strings.Contains(pm.Text, "do not repeat yourself") {
		t.Error("Prompt should instruct model not to repeat itself")
	}
	if !strings.Contains(pm.Text, "Don't include generic development practices") {
		t.Error("Prompt should instruct model to skip generic practices")
	}
}

func TestIDE_DetectJetBrains(t *testing.T) {
	// Just verify detectJetBrains returns slice (may be empty on CI)
	results := detectJetBrains()
	for _, info := range results {
		if info.Name == "" {
			t.Error("Expected non-empty IDE name")
		}
		if !info.Installed {
			t.Error("detectJetBrains should only return installed IDEs")
		}
	}
}

// ---------------------------------------------------------------------------
// T257: /init-verifiers tests
// ---------------------------------------------------------------------------

func TestInitVerifiers_ReturnsPromptMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/init-verifiers")
	msg := cmd()
	pm, ok := msg.(PromptMsg)
	if !ok {
		t.Fatalf("Expected PromptMsg, got %T", msg)
	}
	if pm.Command != "/init-verifiers" {
		t.Errorf("Expected command '/init-verifiers', got %q", pm.Command)
	}
	for _, want := range []string{
		"## Goal",
		"verifier skills",
		"## Phase 1: Auto-Detection",
		"## Phase 2: Verification Tool Setup",
		"## Phase 3: Interactive Q&A",
		"## Phase 4: Generate Verifier Skill",
		"## Phase 5: Confirm Creation",
	} {
		if !strings.Contains(pm.Text, want) {
			t.Errorf("Prompt should contain %q", want)
		}
	}
}

func TestInitVerifiers_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/init-verifiers")
	if reg == nil {
		t.Fatal("Should have registration for /init-verifiers")
	}
	if reg.Type != CommandTypePrompt {
		t.Errorf("Expected CommandTypePrompt, got %v", reg.Type)
	}
}

// ---------------------------------------------------------------------------
// T258: /insights tests
// ---------------------------------------------------------------------------

func TestInsights_ReturnsPromptMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/insights")
	msg := cmd()
	pm, ok := msg.(PromptMsg)
	if !ok {
		t.Fatalf("Expected PromptMsg, got %T", msg)
	}
	if pm.Command != "/insights" {
		t.Errorf("Expected command '/insights', got %q", pm.Command)
	}
	if !strings.Contains(pm.Text, "usage insights report") {
		t.Error("Expected prompt text to mention usage insights report")
	}
}

func TestInsights_HasRegistration(t *testing.T) {
	d := NewDispatcher()
	reg := d.GetRegistration("/insights")
	if reg == nil {
		t.Fatal("Expected /insights to be registered")
	}
	if reg.Type != CommandTypePrompt {
		t.Errorf("Expected CommandTypePrompt, got %v", reg.Type)
	}
	if reg.Description == "" {
		t.Error("Expected non-empty description")
	}
}

// T259: /install-github-app
// ---------------------------------------------------------------------------

func TestInstallGitHubApp_Registered(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/install-github-app") {
		t.Fatal("/install-github-app not registered")
	}
	reg := d.GetRegistration("/install-github-app")
	if reg == nil {
		t.Fatal("expected registration")
	}
	if reg.Type != CommandTypeLocal {
		t.Errorf("expected local command, got %s", reg.Type)
	}
	if reg.Source != "builtin" {
		t.Errorf("expected source=builtin, got %q", reg.Source)
	}
}

func TestInstallGitHubApp_ReturnsMessage(t *testing.T) {
	h := newInstallGitHubAppHandler()
	msg := h("")()
	m, ok := msg.(InstallGitHubAppMsg)
	if !ok {
		t.Fatalf("expected InstallGitHubAppMsg, got %T", msg)
	}
	if !strings.Contains(m.Message, "coming soon") {
		t.Errorf("expected 'coming soon' in message, got %q", m.Message)
	}
	if !strings.Contains(m.Message, "https://github.com/apps/claude") {
		t.Errorf("expected GitHub App URL in message, got %q", m.Message)
	}
}

func TestInstallGitHubApp_DispatchReturnsMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/install-github-app")
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	msg := cmd()
	if _, ok := msg.(InstallGitHubAppMsg); !ok {
		t.Fatalf("expected InstallGitHubAppMsg from dispatch, got %T", msg)
	}
}

// T260: /install-slack-app
// ---------------------------------------------------------------------------

func TestInstallSlackApp_ShowsURL(t *testing.T) {
	h := newInstallSlackAppHandler()
	msg := h("")()
	m, ok := msg.(InstallSlackAppMsg)
	if !ok {
		t.Fatalf("expected InstallSlackAppMsg, got %T", msg)
	}
	if !strings.Contains(m.Message, slackAppInstallURL) {
		t.Errorf("expected install URL in message, got %q", m.Message)
	}
	if m.URL != slackAppInstallURL {
		t.Errorf("expected URL=%q, got %q", slackAppInstallURL, m.URL)
	}
}

func TestInstallSlackApp_RegisteredInDispatcher(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/install-slack-app") {
		t.Fatal("/install-slack-app not registered")
	}
}

func TestInstallSlackApp_DispatchReturnsMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/install-slack-app")
	if cmd == nil {
		t.Fatal("expected non-nil cmd from dispatch")
	}
	msg := cmd()
	if _, ok := msg.(InstallSlackAppMsg); !ok {
		t.Fatalf("expected InstallSlackAppMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// T261: /keybindings tests
// ---------------------------------------------------------------------------

func TestKeybindings_ShowsBindings(t *testing.T) {
	h := newKeybindingsHandler()
	msg := h("")()
	m, ok := msg.(KeybindingsMsg)
	if !ok {
		t.Fatalf("expected KeybindingsMsg, got %T", msg)
	}
	if !strings.Contains(m.Message, "Current Keybindings") {
		t.Error("expected header in output")
	}
	if !strings.Contains(m.Message, "[Global]") {
		t.Error("expected [Global] context section")
	}
	if !strings.Contains(m.Message, "[Chat]") {
		t.Error("expected [Chat] context section")
	}
	if !strings.Contains(m.Message, "ctrl+c") {
		t.Error("expected ctrl+c binding in output")
	}
	if !strings.Contains(m.Message, "keybindings.json") {
		t.Error("expected customization hint in output")
	}
}

func TestKeybindings_RegisteredInDispatcher(t *testing.T) {
	d := NewDispatcher()
	if !d.HasHandler("/keybindings") {
		t.Fatal("/keybindings not registered")
	}
}

func TestKeybindings_DispatchReturnsMsg(t *testing.T) {
	d := NewDispatcher()
	cmd := d.Dispatch("/keybindings")
	if cmd == nil {
		t.Fatal("expected non-nil cmd from dispatch")
	}
	msg := cmd()
	if _, ok := msg.(KeybindingsMsg); !ok {
		t.Fatalf("expected KeybindingsMsg, got %T", msg)
	}
}
