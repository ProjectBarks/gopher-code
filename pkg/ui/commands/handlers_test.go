package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
	for _, name := range []string{"add-dir", "advisor", "agents"} {
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
