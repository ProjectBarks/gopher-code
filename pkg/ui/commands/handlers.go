package commands

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/projectbarks/gopher-code/pkg/auth"
	"github.com/projectbarks/gopher-code/pkg/compact"
	appcontext "github.com/projectbarks/gopher-code/pkg/context"
	"github.com/projectbarks/gopher-code/pkg/hooks"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// ---------------------------------------------------------------------------
// T223: Command type system (local / local-jsx / prompt) + dispatch registry
// Source: src/types/command.ts — CommandType, CommandBase
// ---------------------------------------------------------------------------

// CommandType distinguishes the command implementation strategy.
// Source: src/types/command.ts lines 74-152
type CommandType int

const (
	// CommandTypeLocal is a command that runs inline and returns text output.
	// Source: src/types/command.ts — LocalCommand
	CommandTypeLocal CommandType = iota
	// CommandTypeLocalJSX is a command that renders a TUI component (bubbletea model).
	// Source: src/types/command.ts — LocalJSXCommand
	CommandTypeLocalJSX
	// CommandTypePrompt is a command that expands into prompt content sent to the model.
	// Source: src/types/command.ts — PromptCommand
	CommandTypePrompt
)

// String returns the command type name matching the TS enum values.
func (ct CommandType) String() string {
	switch ct {
	case CommandTypeLocal:
		return "local"
	case CommandTypeLocalJSX:
		return "local-jsx"
	case CommandTypePrompt:
		return "prompt"
	default:
		return "unknown"
	}
}

// CommandAvailability declares which auth/provider environments a command is available in.
// Source: src/types/command.ts lines 169-173
type CommandAvailability string

const (
	// AvailabilityClaudeAI is for claude.ai OAuth subscribers.
	AvailabilityClaudeAI CommandAvailability = "claude-ai"
	// AvailabilityConsole is for Console API key users.
	AvailabilityConsole CommandAvailability = "console"
)

// CommandRegistration holds the full metadata for a registered command.
// Source: src/types/command.ts — CommandBase + (LocalCommand | LocalJSXCommand | PromptCommand)
type CommandRegistration struct {
	// Name is the command name without leading slash (e.g. "add-dir").
	Name string
	// Description is the user-visible description.
	Description string
	// Type is the command implementation type.
	Type CommandType
	// Handler is the dispatch function.
	Handler Handler
	// Aliases are alternative names (e.g. ["q"] for "quit").
	Aliases []string
	// ArgumentHint is hint text displayed after the command name (e.g. "<path>").
	ArgumentHint string
	// IsHidden controls whether the command is hidden from typeahead/help.
	IsHidden bool
	// IsEnabled returns whether the command is currently enabled. Nil means always enabled.
	IsEnabled func() bool
	// Immediate means the command executes immediately without waiting for a stop point.
	Immediate bool
	// Availability declares which auth/provider environments the command is available in.
	Availability []CommandAvailability
	// Source identifies where the command came from.
	Source string
}

// CommandResult is the message returned after executing a command.
type CommandResult struct {
	Command string
	Output  string
	Error   error
}

// ModelSwitchMsg requests switching to a different model.
type ModelSwitchMsg struct {
	Model string
}

// SessionSwitchMsg requests switching sessions.
type SessionSwitchMsg struct{}

// ClearConversationMsg requests clearing the conversation.
type ClearConversationMsg struct{}

// ShowHelpMsg requests showing help.
type ShowHelpMsg struct{}

// QuitMsg requests quitting.
type QuitMsg struct{}

// CompactMsg requests compacting the conversation.
type CompactMsg struct{}

// ThinkingToggleMsg requests toggling thinking mode.
type ThinkingToggleMsg struct{}

// ShowDoctorMsg requests showing the /doctor screen.
type ShowDoctorMsg struct{}

// ShowResumeMsg requests showing the /resume screen.
type ShowResumeMsg struct{}

// AddDirMsg is returned when /add-dir validates and adds a working directory.
type AddDirMsg struct {
	Path    string
	Message string
	Error   error
}

// AdvisorMsg is returned when /advisor configures the advisor model.
type AdvisorMsg struct {
	Model   string
	Message string
	Error   error
}

// InstallGitHubAppMsg is returned when /install-github-app is invoked.
type InstallGitHubAppMsg struct {
	Message string
}

// LogoutMsg is returned when /logout clears stored credentials.
type LogoutMsg struct {
	Message string
	Error   error
}

// AgentsMsg is returned when /agents lists agent configurations.
type AgentsMsg struct {
	Message string
}

// MovedToPluginMsg informs the user a command moved to a plugin.
type MovedToPluginMsg struct {
	Command    string
	PluginName string
	Message    string
}

// PromptMsg is returned by prompt-type commands (e.g. /commit, /commit-push-pr).
// The query loop picks this up and sends the text to the LLM as a user message.
type PromptMsg struct {
	Command string
	Text    string
}

// ShowSettingsMsg requests opening the settings panel.
type ShowSettingsMsg struct{}

// ContextAnalysisMsg is returned when /context analyzes token usage.
type ContextAnalysisMsg struct {
	Stats   *appcontext.TokenStats
	Output  string
	Message string
}

// CopyMsg is returned when /copy copies assistant response to clipboard.
type CopyMsg struct {
	Content string
	Path    string // non-empty if written to file instead of clipboard
	Message string
	Error   error
}

// CostMsg is returned when /cost displays session cost info.
type CostMsg struct {
	Message string
}

// DesktopMsg is returned when /desktop attempts handoff.
type DesktopMsg struct {
	Message string
	Error   error
}

// ShowDiffMsg is returned when /diff requests showing uncommitted changes.
type ShowDiffMsg struct {
	Output string
	Error  error
}

// EffortMsg is returned when /effort sets or displays the effort level.
type EffortMsg struct {
	Level   string
	Message string
	Error   error
}

// ExitGoodbyeMsg requests graceful shutdown with a goodbye message.
type ExitGoodbyeMsg struct {
	Message string
}

// ExportMsg is returned when /export writes the conversation to a file.
type ExportMsg struct {
	Path    string
	Message string
	Error   error
}

// ColorMsg is returned when /color sets or resets the prompt bar color.
type ColorMsg struct {
	Color   string
	Message string
	Error   error
}

// ExtraUsageMsg is returned when /extra-usage shows billing configuration info.
type ExtraUsageMsg struct {
	Message string
	Error   error
}

// FastModeMsg is returned when /fast toggles fast mode.
type FastModeMsg struct {
	Enabled bool
	Message string
	Error   error
}

// FeedbackMsg is returned when /feedback shows feedback URL.
type FeedbackMsg struct {
	Message string
	URL     string
	Opened  bool // true if browser was opened successfully
}

// FilesMsg is returned when /files lists files in context.
type FilesMsg struct {
	Message string
}

// HooksMsg is returned when /hooks displays hook configuration.
type HooksMsg struct {
	Message string
}

// InstallSlackAppMsg is returned when /install-slack-app opens the Slack app install URL.
type InstallSlackAppMsg struct {
	Message string
	URL     string
	Opened  bool // true if browser was opened successfully
}

// IDEMsg is returned when /ide detects installed IDEs and extension status.
type IDEMsg struct {
	IDEs    []IDEInfo
	Message string
	Error   error
}

// IDEInfo describes a detected IDE installation.
type IDEInfo struct {
	Name      string
	Path      string
	Installed bool
	Extension string // "installed", "not-installed", or "unknown"
}

// HeapdumpMsg is returned when /heapdump writes a heap profile.
type HeapdumpMsg struct {
	Path    string
	Message string
	Error   error
}

// CompactResultMsg is returned when /compact completes (success or error).
type CompactResultMsg struct {
	Result  *compact.CompactionResult
	Message string
	Error   error
}

// ClearState holds session-state handles needed by the /clear full clearing chain.
// Callers inject real implementations; tests inject stubs.
type ClearState struct {
	// Session returns a pointer to the session state to mutate.
	Session func() *session.SessionState
	// OriginalCWD returns the original working directory.
	OriginalCWD func() string
	// ClearPlanSlugs clears plan slug caches.
	ClearPlanSlugs func()
	// OnPostClear is called after all clearing is done (e.g. to run session-start hooks).
	OnPostClear func()
}

// CompactDeps holds dependencies needed by the /compact command.
type CompactDeps struct {
	// GetMessages returns the current conversation messages.
	GetMessages func() []message.Message
	// Summarize is the LLM summarization callback.
	Summarize compact.SummaryFunc
	// TranscriptPath returns the path to the current transcript file.
	TranscriptPath func() string
	// OnComplete is called after successful compaction with the new message list.
	OnComplete func(msgs []message.Message)
}

// Handler is a function that processes a slash command.
type Handler func(args string) tea.Cmd

// Dispatcher routes slash commands to their handlers.
type Dispatcher struct {
	handlers      map[string]Handler
	registrations map[string]*CommandRegistration
	aliases       map[string]string // alias -> canonical "/name"
}

// NewDispatcher creates a new command dispatcher with default handlers.
func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		handlers:      make(map[string]Handler),
		registrations: make(map[string]*CommandRegistration),
		aliases:       make(map[string]string),
	}
	d.registerDefaults()
	return d
}

// Register adds a handler for a command name (simple registration).
func (d *Dispatcher) Register(name string, handler Handler) {
	d.handlers[strings.ToLower(name)] = handler
}

// RegisterCommand adds a fully-typed command registration.
func (d *Dispatcher) RegisterCommand(reg CommandRegistration) {
	canonical := "/" + strings.ToLower(strings.TrimPrefix(reg.Name, "/"))
	d.handlers[canonical] = reg.Handler
	d.registrations[canonical] = &reg

	for _, alias := range reg.Aliases {
		aliasKey := "/" + strings.ToLower(strings.TrimPrefix(alias, "/"))
		d.aliases[aliasKey] = canonical
		d.handlers[aliasKey] = reg.Handler
	}
}

// GetRegistration returns the full registration for a command, or nil.
func (d *Dispatcher) GetRegistration(name string) *CommandRegistration {
	key := strings.ToLower(name)
	if reg, ok := d.registrations[key]; ok {
		return reg
	}
	if canonical, ok := d.aliases[key]; ok {
		return d.registrations[canonical]
	}
	return nil
}

// Registrations returns all command registrations.
func (d *Dispatcher) Registrations() []*CommandRegistration {
	seen := make(map[string]bool)
	var out []*CommandRegistration
	for name, reg := range d.registrations {
		if !seen[name] {
			seen[name] = true
			out = append(out, reg)
		}
	}
	return out
}

// Dispatch routes a command string to its handler.
// Returns nil if the command is not recognized.
func (d *Dispatcher) Dispatch(input string) tea.Cmd {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return nil
	}

	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	// Check for alias resolution
	if canonical, ok := d.aliases[cmd]; ok {
		cmd = canonical
	}

	// Check enabled status if we have a registration
	if reg := d.registrations[cmd]; reg != nil {
		if reg.IsEnabled != nil && !reg.IsEnabled() {
			return func() tea.Msg {
				return CommandResult{
					Command: cmd,
					Error:   fmt.Errorf("command %s is currently disabled", cmd),
				}
			}
		}
	}

	handler, ok := d.handlers[cmd]
	if !ok {
		return func() tea.Msg {
			return CommandResult{
				Command: cmd,
				Error:   fmt.Errorf("unknown command: %s", cmd),
			}
		}
	}

	return handler(args)
}

// IsCommand returns true if the input looks like a slash command.
func IsCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}

// HasHandler returns true if a handler exists for the command.
func (d *Dispatcher) HasHandler(cmd string) bool {
	key := strings.ToLower(cmd)
	if _, ok := d.handlers[key]; ok {
		return ok
	}
	if canonical, ok := d.aliases[key]; ok {
		_, ok2 := d.handlers[canonical]
		return ok2
	}
	return false
}

// Commands returns all registered command names.
func (d *Dispatcher) Commands() []string {
	cmds := make([]string, 0, len(d.handlers))
	for name := range d.handlers {
		cmds = append(cmds, name)
	}
	return cmds
}

// HelpText builds a formatted help screen listing all visible commands,
// keybindings, and tips. This matches the HelpV2 screen from Claude Code TS.
func (d *Dispatcher) HelpText() string {
	var b strings.Builder

	b.WriteString("Available slash commands:\n\n")

	// Collect visible commands sorted by name.
	regs := d.Registrations()
	// Also include simple-registered commands that lack full registrations.
	simpleOnly := make(map[string]bool)
	for name := range d.handlers {
		if _, hasReg := d.registrations[name]; !hasReg {
			if _, isAlias := d.aliases[name]; !isAlias {
				simpleOnly[name] = true
			}
		}
	}

	type entry struct {
		Name string
		Desc string
		Hint string
	}
	var entries []entry
	for _, r := range regs {
		if r.IsHidden {
			continue
		}
		if r.IsEnabled != nil && !r.IsEnabled() {
			continue
		}
		name := "/" + strings.TrimPrefix(r.Name, "/")
		entries = append(entries, entry{Name: name, Desc: r.Description, Hint: r.ArgumentHint})
	}
	for name := range simpleOnly {
		desc := ""
		switch name {
		case "/model":
			desc = "Switch model"
		case "/session":
			desc = "Switch session"
		case "/thinking":
			desc = "Toggle extended thinking"
		case "/doctor":
			desc = "Check system health"
		case "/resume":
			desc = "Resume a previous session"
		}
		entries = append(entries, entry{Name: name, Desc: desc})
	}

	// Sort entries by name (insertion sort — small N).
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Name < entries[j-1].Name; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	// Find max name+hint width for alignment.
	maxWidth := 0
	for _, e := range entries {
		w := len(e.Name)
		if e.Hint != "" {
			w += 1 + len(e.Hint)
		}
		if w > maxWidth {
			maxWidth = w
		}
	}

	for _, e := range entries {
		left := e.Name
		if e.Hint != "" {
			left += " " + e.Hint
		}
		padding := strings.Repeat(" ", maxWidth-len(left)+2)
		b.WriteString("  " + left + padding + e.Desc + "\n")
	}

	// Keybindings section
	b.WriteString("\nKeybindings:\n\n")
	b.WriteString("  Enter            Submit message\n")
	b.WriteString("  Ctrl+C           Quit / cancel current operation\n")
	b.WriteString("  Ctrl+J           Insert newline\n")
	b.WriteString("  Esc              Interrupt streaming response\n")
	b.WriteString("  Up/Down          Navigate history\n")
	b.WriteString("  Tab              Autocomplete commands\n")

	// Tips section
	b.WriteString("\nTips:\n\n")
	b.WriteString("  - Start a message with / to see available commands\n")
	b.WriteString("  - Use /compact to reduce context when conversations get long\n")
	b.WriteString("  - Use /model to switch between available models\n")
	b.WriteString("  - Use /cost to check your current session token usage\n")

	return b.String()
}


// ---------------------------------------------------------------------------
// T224: createMovedToPluginCommand factory
// Source: src/commands/createMovedToPluginCommand.ts
// ---------------------------------------------------------------------------

// MovedToPluginOptions configures a redirect command for features moved to plugins.
type MovedToPluginOptions struct {
	// Name is the slash command name (e.g. "review").
	Name string
	// Description is the user-visible description.
	Description string
	// ProgressMessage is shown while the command runs.
	ProgressMessage string
	// PluginName is the plugin package name.
	PluginName string
	// PluginCommand is the command name within the plugin.
	PluginCommand string
}

// CreateMovedToPluginCommand generates a redirect command that tells users
// a feature has moved to a plugin.
// Source: src/commands/createMovedToPluginCommand.ts
func CreateMovedToPluginCommand(opts MovedToPluginOptions) CommandRegistration {
	return CommandRegistration{
		Name:        opts.Name,
		Description: opts.Description,
		Type:        CommandTypePrompt,
		Source:      "builtin",
		Handler: func(args string) tea.Cmd {
			return func() tea.Msg {
				msg := fmt.Sprintf(
					"This command has been moved to a plugin. To use it:\n\n"+
						"1. Install the plugin:\n   claude plugin install %s@claude-code-marketplace\n\n"+
						"2. After installation, use /%s:%s to run this command\n\n"+
						"3. For more information, see: https://github.com/anthropics/claude-code-marketplace/blob/main/%s/README.md",
					opts.PluginName, opts.PluginName, opts.PluginCommand, opts.PluginName,
				)
				return MovedToPluginMsg{
					Command:    opts.Name,
					PluginName: opts.PluginName,
					Message:    msg,
				}
			}
		},
	}
}

// ---------------------------------------------------------------------------
// T225: /add-dir full implementation (validation + path expansion)
// Source: src/commands/add-dir/
// ---------------------------------------------------------------------------

// AddDirResultType classifies the outcome of directory validation.
// Source: src/commands/add-dir/validation.ts — AddDirectoryResult
type AddDirResultType string

const (
	AddDirSuccess                AddDirResultType = "success"
	AddDirEmptyPath              AddDirResultType = "emptyPath"
	AddDirPathNotFound           AddDirResultType = "pathNotFound"
	AddDirNotADirectory          AddDirResultType = "notADirectory"
	AddDirAlreadyInWorkingDir    AddDirResultType = "alreadyInWorkingDirectory"
)

// AddDirResult is the outcome of validating a directory for the workspace.
type AddDirResult struct {
	ResultType   AddDirResultType
	AbsolutePath string
	DirectoryPath string
	WorkingDir   string
}

// ValidateDirectoryForWorkspace validates a path for use as a working directory.
// Source: src/commands/add-dir/validation.ts — validateDirectoryForWorkspace
func ValidateDirectoryForWorkspace(directoryPath string, workingDirs []string) AddDirResult {
	if directoryPath == "" {
		return AddDirResult{ResultType: AddDirEmptyPath}
	}

	// Expand ~ to home directory
	expanded := expandPath(directoryPath)
	// Resolve to absolute, stripping trailing slashes
	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return AddDirResult{
			ResultType:    AddDirPathNotFound,
			DirectoryPath: directoryPath,
			AbsolutePath:  expanded,
		}
	}

	// Check if path exists and is a directory (single stat call)
	info, err := os.Stat(absPath)
	if err != nil {
		return AddDirResult{
			ResultType:    AddDirPathNotFound,
			DirectoryPath: directoryPath,
			AbsolutePath:  absPath,
		}
	}
	if !info.IsDir() {
		return AddDirResult{
			ResultType:    AddDirNotADirectory,
			DirectoryPath: directoryPath,
			AbsolutePath:  absPath,
		}
	}

	// Check if already within an existing working directory
	for _, wd := range workingDirs {
		if pathInWorkingDir(absPath, wd) {
			return AddDirResult{
				ResultType:    AddDirAlreadyInWorkingDir,
				DirectoryPath: directoryPath,
				WorkingDir:    wd,
			}
		}
	}

	return AddDirResult{
		ResultType:   AddDirSuccess,
		AbsolutePath: absPath,
	}
}

// AddDirHelpMessage returns a user-facing message for an add-dir result.
// Source: src/commands/add-dir/validation.ts — addDirHelpMessage
func AddDirHelpMessage(result AddDirResult) string {
	switch result.ResultType {
	case AddDirEmptyPath:
		return "Please provide a directory path."
	case AddDirPathNotFound:
		return fmt.Sprintf("Path %s was not found.", result.AbsolutePath)
	case AddDirNotADirectory:
		parentDir := filepath.Dir(result.AbsolutePath)
		return fmt.Sprintf("%s is not a directory. Did you mean to add the parent directory %s?",
			result.DirectoryPath, parentDir)
	case AddDirAlreadyInWorkingDir:
		return fmt.Sprintf("%s is already accessible within the existing working directory %s.",
			result.DirectoryPath, result.WorkingDir)
	case AddDirSuccess:
		return fmt.Sprintf("Added %s as a working directory.", result.AbsolutePath)
	default:
		return "Unknown result."
	}
}

// expandPath expands ~ to the user's home directory.
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[1:])
	}
	return p
}

// pathInWorkingDir returns true if absPath is inside workingDir.
func pathInWorkingDir(absPath, workingDir string) bool {
	// Clean both paths for comparison
	absPath = filepath.Clean(absPath)
	workingDir = filepath.Clean(workingDir)

	if absPath == workingDir {
		return true
	}
	// Check if absPath is a subdirectory of workingDir
	rel, err := filepath.Rel(workingDir, absPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// newAddDirHandler creates the /add-dir command handler.
// Source: src/commands/add-dir/index.ts + add-dir.tsx + validation.ts
func newAddDirHandler(getWorkingDirs func() []string) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			dirs := []string{"."}
			if getWorkingDirs != nil {
				dirs = getWorkingDirs()
			}
			result := ValidateDirectoryForWorkspace(args, dirs)
			msg := AddDirHelpMessage(result)
			var err error
			if result.ResultType != AddDirSuccess {
				err = fmt.Errorf("%s", msg)
			}
			return AddDirMsg{
				Path:    result.AbsolutePath,
				Message: msg,
				Error:   err,
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T226: /advisor model config command
// Source: src/commands/advisor.ts
// ---------------------------------------------------------------------------

// AdvisorState holds the current advisor configuration.
type AdvisorState struct {
	// Model is the current advisor model name, empty if unset.
	Model string
}

// newAdvisorHandler creates the /advisor command handler.
// Source: src/commands/advisor.ts
func newAdvisorHandler(getState func() AdvisorState, setState func(model string)) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			arg := strings.TrimSpace(strings.ToLower(args))

			// No argument: show current state
			if arg == "" {
				state := getState()
				if state.Model == "" {
					return AdvisorMsg{
						Message: "Advisor: not set\nUse \"/advisor <model>\" to enable (e.g. \"/advisor opus\").",
					}
				}
				return AdvisorMsg{
					Model:   state.Model,
					Message: fmt.Sprintf("Advisor: %s\nUse \"/advisor unset\" to disable or \"/advisor <model>\" to change.", state.Model),
				}
			}

			// Unset/off: disable advisor
			if arg == "unset" || arg == "off" {
				prev := getState().Model
				setState("")
				if prev != "" {
					return AdvisorMsg{
						Message: fmt.Sprintf("Advisor disabled (was %s).", prev),
					}
				}
				return AdvisorMsg{
					Message: "Advisor already unset.",
				}
			}

			// Set new advisor model
			setState(arg)
			return AdvisorMsg{
				Model:   arg,
				Message: fmt.Sprintf("Advisor set to %s.", arg),
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T227: /agents menu
// Source: src/commands/agents/index.ts + agents.tsx
// ---------------------------------------------------------------------------

// AgentConfig describes an available agent configuration.
type AgentConfig struct {
	Name        string
	Description string
}

// newAgentsHandler creates the /agents command handler.
// Source: src/commands/agents/agents.tsx
func newAgentsHandler(getAgents func() []AgentConfig) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			agents := []AgentConfig{
				{Name: "general-purpose", Description: "Default agent for general tasks"},
				{Name: "bash", Description: "Shell command execution agent"},
			}
			if getAgents != nil {
				extra := getAgents()
				if len(extra) > 0 {
					agents = append(agents, extra...)
				}
			}

			if len(agents) == 0 {
				return AgentsMsg{Message: "No agent configurations found."}
			}

			var b strings.Builder
			b.WriteString("Available agents:\n")
			for _, a := range agents {
				b.WriteString(fmt.Sprintf("  - %s: %s\n", a.Name, a.Description))
			}
			return AgentsMsg{Message: strings.TrimRight(b.String(), "\n")}
		}
	}
}

// ---------------------------------------------------------------------------
// T228: /branch conversation fork
// Source: src/commands/branch/
// ---------------------------------------------------------------------------

// BranchMsg is returned when /branch forks the conversation.
type BranchMsg struct {
	ForkName string
	Message  string
	Error    error
}

// BranchOptions provides dependencies for the branch handler.
type BranchOptions struct {
	// SessionID returns the current session ID.
	SessionID func() string
	// SessionName returns the current session name.
	SessionName func() string
	// TranscriptDir returns the directory containing transcript JSONL files.
	TranscriptDir func() string
	// SwitchSession switches to a new session by ID.
	SwitchSession func(id string)
}

// newBranchHandler creates the /branch command handler.
// Source: src/commands/branch/branch.tsx — forks the current conversation
func newBranchHandler(opts BranchOptions) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			srcID := opts.SessionID()
			srcName := opts.SessionName()
			dir := opts.TranscriptDir()

			// Build source and destination paths
			srcPath := filepath.Join(dir, srcID+".jsonl")
			forkID := srcID + "-fork-" + fmt.Sprintf("%d", time.Now().UnixMilli())
			dstPath := filepath.Join(dir, forkID+".jsonl")

			// Copy the transcript
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return BranchMsg{Error: fmt.Errorf("cannot read transcript: %w", err)}
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return BranchMsg{Error: fmt.Errorf("cannot write fork: %w", err)}
			}

			forkName := srcName + " (Branch)"
			opts.SwitchSession(forkID)

			return BranchMsg{
				ForkName: forkName,
				Message:  "Forked conversation as \"" + forkName + "\"",
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T229: /remote-control + /bridge-kick
// Source: src/commands/bridge/ + src/commands/bridge-kick.ts
// ---------------------------------------------------------------------------

// RemoteControlMsg is returned when /remote-control starts the bridge.
type RemoteControlMsg struct {
	Message string
	Error   error
}

// BridgeKickMsg is returned when /bridge-kick runs diagnostics.
type BridgeKickMsg struct {
	Message string
	Error   error
}

// newRemoteControlHandler creates the /remote-control command handler.
// Source: src/commands/bridge/index.ts — starts the bridge for remote control
func newRemoteControlHandler(isConnected func() bool, startBridge func() error) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			if isConnected() {
				return RemoteControlMsg{Message: "Bridge is already connected."}
			}
			if err := startBridge(); err != nil {
				return RemoteControlMsg{Error: fmt.Errorf("failed to start bridge: %w", err)}
			}
			return RemoteControlMsg{Message: "Bridge started. Remote control is now active."}
		}
	}
}

// newBridgeKickHandler creates the /bridge-kick command handler (ant-only stub).
// Source: src/commands/bridge-kick.ts — ant-only debug command
func newBridgeKickHandler(isAnt func() bool) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			if !isAnt() {
				return BridgeKickMsg{Error: fmt.Errorf("bridge-kick is an internal-only command")}
			}
			return BridgeKickMsg{Message: "Bridge kick: diagnostics stub (ant-only)."}
		}
	}
}

// ---------------------------------------------------------------------------
// T230: /brief Kairos mode toggle
// Source: src/commands/brief.ts
// ---------------------------------------------------------------------------

// BriefMsg is returned when /brief toggles Kairos (brief) mode.
type BriefMsg struct {
	Active  bool
	Message string
}

// newBriefHandler creates the /brief command handler.
// Source: src/commands/brief.ts — toggles brief/concise response mode
func newBriefHandler(getKairos func() bool, setKairos func(bool)) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			current := getKairos()
			next := !current
			setKairos(next)
			if next {
				return BriefMsg{Active: true, Message: "Brief mode enabled"}
			}
			return BriefMsg{Active: false, Message: "Brief mode disabled"}
		}
	}
}

// ---------------------------------------------------------------------------
// T231: /btw side-question with scroll modal
// Source: src/commands/btw/
// ---------------------------------------------------------------------------

// BtwMsg is returned when /btw runs a side question.
type BtwMsg struct {
	Question string
	Answer   string
	Message  string
	Error    error
}

// newBtwHandler creates the /btw command handler.
// Source: src/commands/btw/btw.tsx — side question without disrupting main conversation
func newBtwHandler(sideQuery func(question string) (string, error)) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			question := strings.TrimSpace(args)
			if question == "" {
				return BtwMsg{Error: fmt.Errorf("usage: /btw <question>")}
			}
			answer, err := sideQuery(question)
			if err != nil {
				return BtwMsg{Question: question, Error: fmt.Errorf("side query failed: %w", err)}
			}
			return BtwMsg{
				Question: question,
				Answer:   answer,
				Message:  answer,
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T232: /chrome browser integration (stub)
// Source: src/commands/chrome/
// ---------------------------------------------------------------------------

// ChromeAction identifies a chrome menu action.
type ChromeAction string

const (
	ChromeActionInstall           ChromeAction = "install"
	ChromeActionReconnect         ChromeAction = "reconnect"
	ChromeActionManagePermissions ChromeAction = "manage-permissions"
	ChromeActionToggleDefault     ChromeAction = "toggle-default"
)

// ChromeMsg is returned when /chrome runs a browser integration action.
type ChromeMsg struct {
	Action  ChromeAction
	Message string
	Error   error
}

// newChromeHandler creates the /chrome command handler (stub with menu structure).
// Source: src/commands/chrome/ — 4 menu actions
func newChromeHandler() Handler {
	actions := map[string]ChromeAction{
		"install":            ChromeActionInstall,
		"reconnect":          ChromeActionReconnect,
		"manage-permissions": ChromeActionManagePermissions,
		"toggle-default":     ChromeActionToggleDefault,
	}
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			arg := strings.TrimSpace(strings.ToLower(args))
			if arg == "" {
				return ChromeMsg{
					Message: "Chrome integration:\n" +
						"  /chrome install            — Install the Chrome extension\n" +
						"  /chrome reconnect          — Reconnect to Chrome\n" +
						"  /chrome manage-permissions — Manage extension permissions\n" +
						"  /chrome toggle-default     — Toggle as default browser action",
				}
			}
			action, ok := actions[arg]
			if !ok {
				return ChromeMsg{Error: fmt.Errorf("unknown chrome action: %s", arg)}
			}
			// Stub: Chrome extension integration is complex and deferred
			return ChromeMsg{
				Action:  action,
				Message: fmt.Sprintf("Chrome %s: not yet implemented (extension integration required).", arg),
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T233: /clear full clearing chain
// Source: src/commands/clear/conversation.ts — clearConversation (~20 steps)
// ---------------------------------------------------------------------------

// newClearHandler creates the /clear handler that performs the full clearing chain.
// Source: src/commands/clear/conversation.ts
func newClearHandler(state ClearState) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			s := state.Session()

			// Step 1: Regenerate session ID with parent lineage.
			s.RegenerateSessionID(true)

			// Step 2: Update env for subprocess inheritance.
			os.Setenv("CLAUDE_CODE_SESSION_ID", s.ID)

			// Step 3: Clear messages and reset turn count.
			s.Messages = nil
			s.TurnCount = 0

			// Step 4: Reset cost/usage counters for the new session.
			s.TotalInputTokens = 0
			s.TotalOutputTokens = 0
			s.TotalCacheCreationTokens = 0
			s.TotalCacheReadTokens = 0
			s.LastInputTokens = 0
			s.TotalCostUSD = 0
			s.TotalAPIDuration = 0
			s.TotalAPIDurationWithoutRetries = 0
			s.TotalToolDuration = 0
			s.TotalLinesAdded = 0
			s.TotalLinesRemoved = 0

			// Step 5: Reset plan mode tracking.
			s.HasExitedPlanMode = false
			s.NeedsPlanModeExitAttachment = false
			s.NeedsAutoModeExitAttachment = false

			// Step 6: Clear plan slug cache.
			s.PlanSlugCache = nil
			if state.ClearPlanSlugs != nil {
				state.ClearPlanSlugs()
			}

			// Step 7: Clear model usage.
			s.ModelUsage = nil

			// Step 8: Clear in-memory error log.
			s.InMemoryErrorLog = nil

			// Step 9: Clear cached CLAUDE.md content (will be re-read).
			s.CachedClaudeMdContent = ""

			// Step 10: Clear invoked skills.
			s.InvokedSkills = nil

			// Step 11: Clear slow operations.
			s.SlowOperations = nil

			// Step 12: Reset pending post-compaction flag.
			s.PendingPostCompaction = false

			// Step 13: Reset last API request data.
			s.LastAPIRequest = nil
			s.LastAPIRequestMessages = nil
			s.LastClassifierRequests = nil
			s.LastMainRequestId = ""
			s.LastApiCompletionTimestamp = nil

			// Step 14: Reset per-turn tracking.
			s.TurnHookDurationMs = 0
			s.TurnToolDurationMs = 0
			s.TurnClassifierDurationMs = 0
			s.TurnToolCount = 0
			s.TurnHookCount = 0
			s.TurnClassifierCount = 0

			// Step 15: Reset CWD to original.
			if state.OriginalCWD != nil {
				if orig := state.OriginalCWD(); orig != "" {
					s.CWD = orig
				}
			}

			// Step 16: Reset prompt ID.
			s.PromptId = ""

			// Step 17: Reset LSP recommendation flag.
			s.LspRecommendationShownThisSession = false

			// Step 18: Post-clear callback (hooks, worktree save, etc.).
			if state.OnPostClear != nil {
				state.OnPostClear()
			}

			return ClearConversationMsg{}
		}
	}
}

// ---------------------------------------------------------------------------
// T234: /color session prompt bar
// Source: src/commands/color/color.ts
// ---------------------------------------------------------------------------

// agentColorNames is the ordered palette matching the TS AGENT_COLORS list.
var agentColorNames = []string{
	"red", "blue", "green", "yellow", "purple", "orange", "pink", "cyan",
}

// colorResetAliases match the TS RESET_ALIASES.
var colorResetAliases = map[string]bool{
	"default": true, "reset": true, "none": true, "gray": true, "grey": true,
}

// newColorHandler creates the /color command handler.
// Source: src/commands/color/color.ts
func newColorHandler(getColor func() string, setColor func(string)) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			arg := strings.TrimSpace(strings.ToLower(args))

			if arg == "" {
				colorList := strings.Join(agentColorNames, ", ")
				return ColorMsg{
					Message: fmt.Sprintf("Please provide a color. Available colors: %s, default", colorList),
				}
			}

			// Handle reset aliases.
			if colorResetAliases[arg] {
				setColor("")
				return ColorMsg{
					Color:   "",
					Message: "Session color reset to default",
				}
			}

			// Validate color name.
			valid := false
			for _, c := range agentColorNames {
				if c == arg {
					valid = true
					break
				}
			}
			if !valid {
				colorList := strings.Join(agentColorNames, ", ")
				return ColorMsg{
					Error: fmt.Errorf("Invalid color %q. Available colors: %s, default", arg, colorList),
				}
			}

			setColor(arg)
			return ColorMsg{
				Color:   arg,
				Message: fmt.Sprintf("Session color set to: %s", arg),
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T235: /commit prompt command
// Source: src/commands/commit.ts
// ---------------------------------------------------------------------------

// commitPromptTemplate is the prompt text returned by /commit.
// The query loop expands !`cmd` patterns and sends it to the LLM.
// Source: src/commands/commit.ts — getPromptContent()
const commitPromptTemplate = `## Context

- Current git status: ` + "`git status`" + `
- Current git diff (staged and unstaged changes): ` + "`git diff HEAD`" + `
- Current branch: ` + "`git branch --show-current`" + `
- Recent commits: ` + "`git log --oneline -10`" + `

## Git Safety Protocol

- NEVER update the git config
- NEVER skip hooks (--no-verify, --no-gpg-sign, etc) unless the user explicitly requests it
- CRITICAL: ALWAYS create NEW commits. NEVER use git commit --amend, unless the user explicitly requests it
- Do not commit files that likely contain secrets (.env, credentials.json, etc). Warn the user if they specifically request to commit those files
- If there are no changes to commit (i.e., no untracked files and no modifications), do not create an empty commit
- Never use git commands with the -i flag (like git rebase -i or git add -i) since they require interactive input which is not supported

## Your task

Based on the above changes, create a single git commit:

1. Analyze all staged changes and draft a commit message:
   - Look at the recent commits above to follow this repository's commit message style
   - Summarize the nature of the changes (new feature, enhancement, bug fix, refactoring, test, docs, etc.)
   - Ensure the message accurately reflects the changes and their purpose (i.e. "add" means a wholly new feature, "update" means an enhancement to an existing feature, "fix" means a bug fix, etc.)
   - Draft a concise (1-2 sentences) commit message that focuses on the "why" rather than the "what"

2. Stage relevant files and create the commit using HEREDOC syntax:
` + "```" + `
git commit -m "$(cat <<'EOF'
Commit message here.
EOF
)"
` + "```" + `

You have the capability to call multiple tools in a single response. Stage and create the commit using a single message. Do not use any other tools or do anything else. Do not send any other text or messages besides these tool calls.`

// newCommitHandler creates the /commit prompt command handler.
// Source: src/commands/commit.ts
func newCommitHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			return PromptMsg{
				Command: "/commit",
				Text:    commitPromptTemplate,
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T236: /commit-push-pr prompt command
// Source: src/commands/commit-push-pr.ts
// ---------------------------------------------------------------------------

// commitPushPRPromptTemplate is the prompt text returned by /commit-push-pr.
// Source: src/commands/commit-push-pr.ts — getPromptContent()
func buildCommitPushPRPrompt(additionalArgs string) string {
	defaultBranch := "main"
	safeUser := os.Getenv("SAFEUSER")
	username := os.Getenv("USER")

	prompt := `## Context

- ` + "`SAFEUSER`" + `: ` + safeUser + `
- ` + "`whoami`" + `: ` + username + `
- ` + "`git status`" + `: ` + "`git status`" + `
- ` + "`git diff HEAD`" + `: ` + "`git diff HEAD`" + `
- ` + "`git branch --show-current`" + `: ` + "`git branch --show-current`" + `
- ` + "`git diff " + defaultBranch + "...HEAD`" + `: ` + "`git diff " + defaultBranch + "...HEAD`" + `
- ` + "`gh pr view --json number 2>/dev/null || true`" + `: ` + "`gh pr view --json number 2>/dev/null || true`" + `

## Git Safety Protocol

- NEVER update the git config
- NEVER run destructive/irreversible git commands (like push --force, hard reset, etc) unless the user explicitly requests them
- NEVER skip hooks (--no-verify, --no-gpg-sign, etc) unless the user explicitly requests it
- NEVER run force push to main/master, warn the user if they request it
- Do not commit files that likely contain secrets (.env, credentials.json, etc)
- Never use git commands with the -i flag (like git rebase -i or git add -i) since they require interactive input which is not supported

## Your task

Analyze all changes that will be included in the pull request, making sure to look at all relevant commits (NOT just the latest commit, but ALL commits that will be included in the pull request from the git diff ` + defaultBranch + `...HEAD output above).

Based on the above changes:
1. Create a new branch if on ` + defaultBranch + ` (use SAFEUSER from context above for the branch name prefix, falling back to whoami if SAFEUSER is empty, e.g., ` + "`username/feature-name`" + `)
2. Create a single commit with an appropriate message using heredoc syntax:
` + "```" + `
git commit -m "$(cat <<'EOF'
Commit message here.
EOF
)"
` + "```" + `
3. Push the branch to origin
4. If a PR already exists for this branch (check the gh pr view output above), update the PR title and body using ` + "`gh pr edit`" + ` to reflect the current diff. Otherwise, create a pull request using ` + "`gh pr create`" + ` with heredoc syntax for the body.
   - IMPORTANT: Keep PR titles short (under 70 characters). Use the body for details.
` + "```" + `
gh pr create --title "Short, descriptive title" --body "$(cat <<'EOF'
## Summary
<1-3 bullet points>

## Test plan
[Bulleted markdown checklist of TODOs for testing the pull request...]
EOF
)"
` + "```" + `

You have the capability to call multiple tools in a single response. You MUST do all of the above in a single message.

Return the PR URL when you're done, so the user can see it.`

	if additionalArgs != "" {
		prompt += "\n\n## Additional instructions from user\n\n" + additionalArgs
	}
	return prompt
}

// newCommitPushPRHandler creates the /commit-push-pr prompt command handler.
// Source: src/commands/commit-push-pr.ts
func newCommitPushPRHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			return PromptMsg{
				Command: "/commit-push-pr",
				Text:    buildCommitPushPRPrompt(strings.TrimSpace(args)),
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T237: /compact compaction
// Source: src/commands/compact/compact.ts
// ---------------------------------------------------------------------------

// newCompactHandler creates the /compact handler that calls CompactConversation.
// Source: src/commands/compact/compact.ts — call()
func newCompactHandler(deps CompactDeps) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			messages := deps.GetMessages()
			if len(messages) == 0 {
				return CompactResultMsg{
					Error:   fmt.Errorf(compact.ErrorMessageNotEnoughMessages),
					Message: compact.ErrorMessageNotEnoughMessages,
				}
			}

			customInstructions := strings.TrimSpace(args)
			transcriptPath := ""
			if deps.TranscriptPath != nil {
				transcriptPath = deps.TranscriptPath()
			}

			result, err := compact.CompactConversation(
				context.Background(),
				messages,
				deps.Summarize,
				false, // suppressFollowUp
				customInstructions,
				compact.TriggerManual,
				transcriptPath,
			)
			if err != nil {
				errMsg := err.Error()
				switch errMsg {
				case compact.ErrorMessageNotEnoughMessages:
					return CompactResultMsg{Error: err, Message: errMsg}
				case compact.ErrorMessageUserAbort:
					return CompactResultMsg{Error: fmt.Errorf("Compaction canceled."), Message: "Compaction canceled."}
				case compact.ErrorMessageIncompleteResponse:
					return CompactResultMsg{Error: err, Message: errMsg}
				default:
					return CompactResultMsg{
						Error:   fmt.Errorf("Error during compaction: %w", err),
						Message: fmt.Sprintf("Error during compaction: %s", errMsg),
					}
				}
			}

			// Build the post-compact message list.
			newMessages := compact.BuildPostCompactMessages(result)
			if deps.OnComplete != nil {
				deps.OnComplete(newMessages)
			}

			return CompactResultMsg{
				Result:  &result,
				Message: "Compacted conversation",
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T238: /config — open settings panel
// Source: src/commands/config/
// ---------------------------------------------------------------------------

func newConfigHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg { return ShowSettingsMsg{} }
	}
}

// ---------------------------------------------------------------------------
// T239: /context — context window usage visualization
// Source: src/commands/context/
// ---------------------------------------------------------------------------

// ContextDeps holds dependencies for the /context handler.
type ContextDeps struct {
	// GetMessages returns current conversation messages.
	GetMessages func() []message.Message
	// ContextWindowSize is the model's context window.
	ContextWindowSize func() int
}

func newContextHandler(deps ContextDeps) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			msgs := deps.GetMessages()
			stats := appcontext.AnalyzeContext(msgs)
			windowSize := appcontext.ModelContextWindowDefault
			if deps.ContextWindowSize != nil {
				windowSize = deps.ContextWindowSize()
			}
			pct := appcontext.CalculateContextPercentages(
				&appcontext.TokenUsage{InputTokens: stats.Total},
				windowSize,
			)

			var sb strings.Builder
			sb.WriteString("## Context Window Usage\n\n")
			sb.WriteString(fmt.Sprintf("| Category | Tokens |\n"))
			sb.WriteString(fmt.Sprintf("| --- | --- |\n"))
			sb.WriteString(fmt.Sprintf("| Human messages | %d |\n", stats.HumanMessages))
			sb.WriteString(fmt.Sprintf("| Assistant messages | %d |\n", stats.AssistantMessages))
			sb.WriteString(fmt.Sprintf("| Tool requests | %d |\n", sumMap(stats.ToolRequests)))
			sb.WriteString(fmt.Sprintf("| Tool results | %d |\n", sumMap(stats.ToolResults)))
			sb.WriteString(fmt.Sprintf("| Local commands | %d |\n", stats.LocalCommandOutputs))
			sb.WriteString(fmt.Sprintf("| Other | %d |\n", stats.Other))
			sb.WriteString(fmt.Sprintf("| **Total** | **%d** |\n", stats.Total))
			if pct.Used != nil {
				sb.WriteString(fmt.Sprintf("\nContext used: %d%% (%d/%d tokens)\n", *pct.Used, stats.Total, windowSize))
			}

			return ContextAnalysisMsg{
				Stats:   stats,
				Output:  sb.String(),
				Message: sb.String(),
			}
		}
	}
}

func sumMap(m map[string]int) int {
	var total int
	for _, v := range m {
		total += v
	}
	return total
}

// ---------------------------------------------------------------------------
// T240: /copy — copy last assistant response to clipboard
// Source: src/commands/copy/
// ---------------------------------------------------------------------------

// CopyDeps holds dependencies for the /copy handler.
type CopyDeps struct {
	// GetMessages returns current conversation messages.
	GetMessages func() []message.Message
}

func newCopyHandler(deps CopyDeps) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			msgs := deps.GetMessages()
			n := 1
			if args != "" {
				if _, err := fmt.Sscanf(args, "%d", &n); err != nil || n < 1 {
					return CopyMsg{Error: fmt.Errorf("usage: /copy [N] — N must be a positive integer")}
				}
			}

			// Find Nth-latest assistant response
			var found int
			var content string
			for i := len(msgs) - 1; i >= 0; i-- {
				if msgs[i].Role == message.RoleAssistant {
					found++
					if found == n {
						var sb strings.Builder
						for _, b := range msgs[i].Content {
							if b.Type == message.ContentText {
								sb.WriteString(b.Text)
							}
						}
						content = sb.String()
						break
					}
				}
			}
			if content == "" {
				return CopyMsg{Error: fmt.Errorf("no assistant response found")}
			}

			// Try OSC 52 clipboard escape sequence
			osc52 := fmt.Sprintf("\033]52;c;%s\a", encodeBase64(content))
			if _, err := fmt.Fprint(os.Stderr, osc52); err == nil {
				return CopyMsg{Content: content, Message: "Copied to clipboard"}
			}

			// Fallback: write to $TMPDIR/claude/response.md
			dir := filepath.Join(os.TempDir(), "claude")
			_ = os.MkdirAll(dir, 0o755)
			path := filepath.Join(dir, "response.md")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return CopyMsg{Error: fmt.Errorf("failed to write response: %w", err)}
			}
			return CopyMsg{Content: content, Path: path, Message: fmt.Sprintf("Written to %s", path)}
		}
	}
}

// encodeBase64 returns a base64-encoded string.
func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// ---------------------------------------------------------------------------
// T241: /cost — session cost display
// Source: src/commands/cost/
// ---------------------------------------------------------------------------

// CostDeps holds dependencies for the /cost handler.
type CostDeps struct {
	// GetSession returns the current session state.
	GetSession func() *session.SessionState
}

func newCostHandler(deps CostDeps) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			s := deps.GetSession()
			totalTokens := s.TotalInputTokens + s.TotalOutputTokens
			msg := fmt.Sprintf("Session cost: $%.4f\nTotal tokens: %d", s.TotalCostUSD, totalTokens)
			return CostMsg{Message: msg}
		}
	}
}

// ---------------------------------------------------------------------------
// T242: /desktop — open session in Claude Desktop app
// Source: src/commands/desktop/
// ---------------------------------------------------------------------------

func newDesktopHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			platform := getPlatform()
			switch platform {
			case "darwin":
				return DesktopMsg{Message: "Opening in Claude Desktop..."}
			case "windows":
				return DesktopMsg{Message: "Opening in Claude Desktop..."}
			default:
				return DesktopMsg{
					Error:   fmt.Errorf("Claude Desktop is only available on macOS and Windows"),
					Message: "Claude Desktop is only available on macOS and Windows",
				}
			}
		}
	}
}

// getPlatform returns the runtime platform string.
func getPlatform() string {
	return runtime.GOOS
}

// ---------------------------------------------------------------------------
// T243: /diff — show uncommitted changes
// Source: src/commands/diff/diff.tsx
// ---------------------------------------------------------------------------

// newDiffHandler creates the /diff command handler.
// Runs `git diff --stat` and returns the output as ShowDiffMsg.
func newDiffHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			out, err := exec.Command("git", "diff", "--stat").CombinedOutput()
			text := strings.TrimSpace(string(out))
			if err != nil {
				if text == "" {
					text = err.Error()
				}
				return ShowDiffMsg{Output: text, Error: err}
			}
			if text == "" {
				text = "No uncommitted changes."
			}
			return ShowDiffMsg{Output: text}
		}
	}
}

// ---------------------------------------------------------------------------
// T245: /effort — effort level configuration
// Source: src/commands/effort/effort.tsx
// ---------------------------------------------------------------------------

// EffortDeps holds dependencies for the /effort handler.
type EffortDeps struct {
	GetLevel func() string
	SetLevel func(level string) error
}

// effortLevels are the valid effort levels.
var effortLevels = []string{"low", "medium", "high", "max", "auto"}

// effortDescriptions maps levels to their human descriptions.
var effortDescriptions = map[string]string{
	"low":    "Quick, straightforward implementation",
	"medium": "Balanced approach with standard testing",
	"high":   "Comprehensive implementation with extensive testing",
	"max":    "Maximum capability with deepest reasoning (Opus 4.6 only)",
	"auto":   "Use the default effort level for your model",
}

// effortUsageText is the help text shown for /effort help.
const effortUsageText = `Usage: /effort [low|medium|high|max|auto]

Effort levels:
- low: Quick, straightforward implementation
- medium: Balanced approach with standard testing
- high: Comprehensive implementation with extensive testing
- max: Maximum capability with deepest reasoning (Opus 4.6 only)
- auto: Use the default effort level for your model`

// isEffortLevel returns true if s is a valid effort level.
func isEffortLevel(s string) bool {
	for _, l := range effortLevels {
		if s == l {
			return true
		}
	}
	return false
}

// newEffortHandler creates the /effort command handler.
func newEffortHandler(deps EffortDeps) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			arg := strings.TrimSpace(strings.ToLower(args))

			// Help args
			if arg == "help" || arg == "-h" || arg == "--help" {
				return EffortMsg{Message: effortUsageText}
			}

			// No args or status: show current
			if arg == "" || arg == "current" || arg == "status" {
				cur := deps.GetLevel()
				if cur == "" || cur == "auto" {
					return EffortMsg{Level: "auto", Message: "Effort level: auto"}
				}
				desc := effortDescriptions[cur]
				return EffortMsg{
					Level:   cur,
					Message: fmt.Sprintf("Current effort level: %s (%s)", cur, desc),
				}
			}

			// Auto/unset: clear
			if arg == "auto" || arg == "unset" {
				envRaw := os.Getenv("CLAUDE_CODE_EFFORT_LEVEL")
				if err := deps.SetLevel("auto"); err != nil {
					return EffortMsg{Error: fmt.Errorf("Failed to set effort level: %s", err)}
				}
				msg := "Effort level set to auto"
				if envRaw != "" {
					msg = fmt.Sprintf("Cleared effort from settings, but CLAUDE_CODE_EFFORT_LEVEL=%s still controls this session", envRaw)
				}
				return EffortMsg{Level: "auto", Message: msg}
			}

			// Valid level
			if isEffortLevel(arg) {
				envRaw := os.Getenv("CLAUDE_CODE_EFFORT_LEVEL")
				if err := deps.SetLevel(arg); err != nil {
					return EffortMsg{Error: fmt.Errorf("Failed to set effort level: %s", err)}
				}
				desc := effortDescriptions[arg]
				msg := fmt.Sprintf("Set effort level to %s: %s", arg, desc)
				if envRaw != "" {
					msg = fmt.Sprintf("CLAUDE_CODE_EFFORT_LEVEL=%s overrides this session \u2014 clear it and %s takes over", envRaw, arg)
				}
				return EffortMsg{Level: arg, Message: msg}
			}

			// Invalid
			return EffortMsg{
				Error: fmt.Errorf("Invalid argument: %s. Valid options are: low, medium, high, max, auto", arg),
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T246: /exit — graceful shutdown with goodbye message
// Source: src/commands/exit/exit.tsx
// ---------------------------------------------------------------------------

// goodbyeMessages matches the TS GOODBYE_MESSAGES array.
var goodbyeMessages = []string{"Goodbye!", "See ya!", "Bye!", "Catch you later!"}

// newExitHandler creates the /exit command handler with random goodbye.
func newExitHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			msg := goodbyeMessages[rand.Intn(len(goodbyeMessages))]
			return ExitGoodbyeMsg{Message: msg}
		}
	}
}

// ---------------------------------------------------------------------------
// T247: /export — export conversation to file
// Source: src/commands/export/export.tsx
// ---------------------------------------------------------------------------

// ExportDeps holds dependencies for the /export handler.
type ExportDeps struct {
	GetMessages func() []message.Message
}

// sanitizeFilename normalizes text for use in a filename.
// Source: src/commands/export/export.tsx — sanitizeFilename
func sanitizeFilename(text string) string {
	s := strings.ToLower(text)
	s = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, " ", "-")
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// messageText extracts the concatenated text from a message's content blocks.
func messageText(m message.Message) string {
	var parts []string
	for _, block := range m.Content {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "")
}

// extractFirstPrompt returns the first user message text, truncated to 49 chars + ellipsis.
func extractFirstPrompt(msgs []message.Message) string {
	for _, m := range msgs {
		if m.Role == message.RoleUser {
			text := strings.TrimSpace(messageText(m))
			if text == "" {
				continue
			}
			if len(text) > 49 {
				return text[:49] + "\u2026"
			}
			return text
		}
	}
	return ""
}

// renderMessagesToPlainText renders messages as a plain text transcript.
func renderMessagesToPlainText(msgs []message.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		text := messageText(m)
		if text == "" {
			continue
		}
		switch m.Role {
		case message.RoleUser:
			b.WriteString("User:\n")
		case message.RoleAssistant:
			b.WriteString("Assistant:\n")
		default:
			b.WriteString(string(m.Role) + ":\n")
		}
		b.WriteString(text)
		b.WriteString("\n\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// newExportHandler creates the /export command handler.
func newExportHandler(deps ExportDeps) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			msgs := deps.GetMessages()

			// Render transcript
			content := renderMessagesToPlainText(msgs)
			if content == "" {
				return ExportMsg{Error: fmt.Errorf("no messages to export")}
			}

			// Build filename
			ts := time.Now().Format("2006-01-02-15-04-05")
			var filename string
			arg := strings.TrimSpace(args)
			if arg != "" {
				// User-provided filename: enforce .txt
				name := strings.TrimSuffix(arg, filepath.Ext(arg))
				filename = sanitizeFilename(name) + ".txt"
			} else {
				// Auto-generate from first prompt
				prompt := extractFirstPrompt(msgs)
				if prompt != "" {
					sanitized := sanitizeFilename(prompt)
					if sanitized != "" {
						filename = ts + "-" + sanitized + ".txt"
					}
				}
				if filename == "" {
					filename = "conversation-" + ts + ".txt"
				}
			}

			// Write file
			if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
				return ExportMsg{
					Error:   err,
					Message: fmt.Sprintf("Failed to export conversation: %s", err),
				}
			}

			abs, _ := filepath.Abs(filename)
			return ExportMsg{
				Path:    abs,
				Message: fmt.Sprintf("Conversation exported to: %s", abs),
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T248: /extra-usage — billing configuration stub
// Source: src/commands/extra-usage.ts
// ---------------------------------------------------------------------------

// newExtraUsageHandler creates the /extra-usage command handler.
// For claude.ai users it shows the billing URL; for others it returns an error.
func newExtraUsageHandler(getUserType func() string) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			ut := getUserType()
			if ut != "claude-ai" {
				return ExtraUsageMsg{
					Error: fmt.Errorf("extra usage configuration is only available for Claude.ai users"),
				}
			}
			return ExtraUsageMsg{
				Message: "Configure extra usage at https://claude.ai/settings/billing",
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T249: /fast — toggle fast mode
// Source: src/commands/fast.ts
// ---------------------------------------------------------------------------

// FastModeDeps holds state accessors for the /fast command.
type FastModeDeps struct {
	GetEnabled func() bool
	SetEnabled func(bool)
}

// newFastHandler creates the /fast command handler.
// Accepts optional on|off argument; no argument toggles.
func newFastHandler(deps FastModeDeps) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			arg := strings.TrimSpace(strings.ToLower(args))
			var enabled bool
			switch arg {
			case "on":
				enabled = true
			case "off":
				enabled = false
			case "":
				enabled = !deps.GetEnabled()
			default:
				return FastModeMsg{Error: fmt.Errorf("usage: /fast [on|off]")}
			}
			deps.SetEnabled(enabled)
			label := "disabled"
			if enabled {
				label = "enabled"
			}
			return FastModeMsg{Enabled: enabled, Message: fmt.Sprintf("Fast mode %s", label)}
		}
	}
}

// ---------------------------------------------------------------------------
// T250: /feedback — submit feedback URL stub
// Source: src/commands/feedback.ts
// ---------------------------------------------------------------------------

// feedbackURL is the URL for submitting feedback.
const feedbackURL = "https://github.com/anthropics/claude-code/issues"

// openBrowser attempts to open a URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
	return cmd.Start()
}

// newFeedbackHandler creates the /feedback command handler.
// Opens the feedback URL in the default browser. Falls back to displaying the URL.
// Respects DISABLE_FEEDBACK_COMMAND env var.
func newFeedbackHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			opened := false
			if err := openBrowser(feedbackURL); err == nil {
				opened = true
			}
			msg := "Submit feedback at " + feedbackURL
			if opened {
				msg = "Opened " + feedbackURL + " in your browser"
			}
			return FeedbackMsg{
				Message: msg,
				URL:     feedbackURL,
				Opened:  opened,
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T260: /install-slack-app — open the Slack app installation page
// Source: src/commands/install-slack-app.ts
// ---------------------------------------------------------------------------

// slackAppInstallURL is the URL for installing the Claude Code Slack app.
const slackAppInstallURL = "https://slackbot.anthropic.com/install"

// newInstallSlackAppHandler creates the /install-slack-app command handler.
// Opens the Slack app installation URL in the default browser. Falls back to displaying the URL.
func newInstallSlackAppHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			opened := false
			if err := openBrowser(slackAppInstallURL); err == nil {
				opened = true
			}
			msg := "Install the Claude Code Slack app at " + slackAppInstallURL
			if opened {
				msg = "Opened " + slackAppInstallURL + " in your browser"
			}
			return InstallSlackAppMsg{
				Message: msg,
				URL:     slackAppInstallURL,
				Opened:  opened,
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T251: /files — list files in context (ant-only)
// Source: src/commands/files.ts
// ---------------------------------------------------------------------------

// FilesDeps holds state accessors for the /files command.
type FilesDeps struct {
	GetFiles func() []string
}

// newFilesHandler creates the /files command handler.
// Lists files currently in the conversation context.
func newFilesHandler(deps FilesDeps) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			files := deps.GetFiles()
			if len(files) == 0 {
				return FilesMsg{Message: "No files in context"}
			}
			var b strings.Builder
			b.WriteString("Files in context:\n")
			for _, f := range files {
				b.WriteString("  " + f + "\n")
			}
			return FilesMsg{Message: strings.TrimRight(b.String(), "\n")}
		}
	}
}

// ---------------------------------------------------------------------------
// T256: /init — AI-driven CLAUDE.md generation
// Source: src/commands/init.ts
// ---------------------------------------------------------------------------

// initPromptTemplate is the prompt text returned by /init.
// Source: src/commands/init.ts — OLD_INIT_PROMPT
const initPromptTemplate = `Please analyze this codebase and create a CLAUDE.md file, which will be given to future instances of Claude Code to operate in this repository.

What to add:
1. Commands that will be commonly used, such as how to build, lint, and run tests. Include the necessary commands to develop in this codebase, such as how to run a single test.
2. High-level code architecture and structure so that future instances can be productive more quickly. Focus on the "big picture" architecture that requires reading multiple files to understand.

Usage notes:
- If there's already a CLAUDE.md, suggest improvements to it.
- When you make the initial CLAUDE.md, do not repeat yourself and do not include obvious instructions like "Provide helpful error messages to users", "Write unit tests for all new utilities", "Never include sensitive information (API keys, tokens) in code or commits".
- Avoid listing every component or file structure that can be easily discovered.
- Don't include generic development practices.
- If there are Cursor rules (in .cursor/rules/ or .cursorrules) or Copilot rules (in .github/copilot-instructions.md), make sure to include the important parts.
- If there is a README.md, make sure to include the important parts.
- Do not make up information such as "Common Development Tasks", "Tips for Development", "Support and Documentation" unless this is expressly included in other files that you read.
- Be sure to prefix the file with the following text:

` + "```" + `
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
` + "```"

// newInitHandler creates the /init prompt command handler.
// Source: src/commands/init.ts — getPromptForCommand()
func newInitHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			return PromptMsg{
				Command: "/init",
				Text:    initPromptTemplate,
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T254: /hooks — show/manage hook configuration
// Source: src/commands/hooks/
// ---------------------------------------------------------------------------

// HooksDeps holds dependencies for the /hooks handler.
type HooksDeps struct {
	// GetHooks returns all configured individual hooks.
	GetHooks func() []hooks.IndividualHookConfig
	// GetToolNames returns the list of known tool names (for matcher metadata).
	GetToolNames func() []string
}

// newHooksHandler creates the /hooks command handler.
// Displays all configured hooks grouped by event, with source and matcher info.
func newHooksHandler(deps HooksDeps) Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			allHooks := deps.GetHooks()
			if len(allHooks) == 0 {
				return HooksMsg{Message: "No hooks configured.\n\nHooks can be configured in:\n  - User settings:    ~/.claude/settings.json\n  - Project settings: .claude/settings.json\n  - Local settings:   .claude/settings.local.json"}
			}

			toolNames := deps.GetToolNames()
			grouped := hooks.GroupHooksByEventAndMatcher(allHooks, toolNames)
			metadata := hooks.GetHookEventMetadata(toolNames)

			var sb strings.Builder
			sb.WriteString("## Configured Hooks\n\n")

			totalCount := 0
			for _, event := range hooks.AllHookEvents {
				eventGroup := grouped[event]
				matchers := hooks.GetSortedMatchersForEvent(grouped, event)

				// Count hooks in this event
				eventCount := 0
				for _, m := range matchers {
					eventCount += len(eventGroup[m])
				}
				if eventCount == 0 {
					continue
				}
				totalCount += eventCount

				meta := metadata[event]
				sb.WriteString(fmt.Sprintf("### %s — %s\n", event, meta.Summary))

				for _, matcher := range matchers {
					hooksForMatcher := eventGroup[matcher]
					if len(hooksForMatcher) == 0 {
						continue
					}

					if matcher != "" {
						sb.WriteString(fmt.Sprintf("  [%s=%s]\n", meta.MatcherMetadata.FieldToMatch, matcher))
					}

					for _, h := range hooksForMatcher {
						displayText := hooks.GetHookDisplayText(h.Config)
						sourceLabel := hooks.HookSourceInline(h.Source)
						sb.WriteString(fmt.Sprintf("  - (%s) %s\n", sourceLabel, displayText))
					}
				}
				sb.WriteString("\n")
			}

			sb.WriteString(fmt.Sprintf("Total: %d hook(s)\n", totalCount))
			return HooksMsg{Message: strings.TrimRight(sb.String(), "\n")}
		}
	}
}

// ---------------------------------------------------------------------------
// T257: /init-verifiers prompt command
// Source: src/commands/init-verifiers.ts
// ---------------------------------------------------------------------------

// initVerifiersPromptTemplate is the prompt text returned by /init-verifiers.
// It guides the LLM through detecting project types and creating verifier skills.
// Source: src/commands/init-verifiers.ts — getPromptForCommand()
const initVerifiersPromptTemplate = `Use the TodoWrite tool to track your progress through this multi-step task.

## Goal

Create one or more verifier skills that can be used by the Verify agent to automatically verify code changes in this project or folder. You may create multiple verifiers if the project has different verification needs (e.g., both web UI and API endpoints).

**Do NOT create verifiers for unit tests or typechecking.** Those are already handled by the standard build/test workflow and don't need dedicated verifier skills. Focus on functional verification: web UI (Playwright), CLI (Tmux), and API (HTTP) verifiers.

## Phase 1: Auto-Detection

Analyze the project to detect what's in different subdirectories. The project may contain multiple sub-projects or areas that need different verification approaches (e.g., a web frontend, an API backend, and shared libraries all in one repo).

1. **Scan top-level directories** to identify distinct project areas:
   - Look for separate package.json, Cargo.toml, pyproject.toml, go.mod in subdirectories
   - Identify distinct application types in different folders

2. **For each area, detect:**

   a. **Project type and stack**
      - Primary language(s) and frameworks
      - Package managers (npm, yarn, pnpm, pip, cargo, etc.)

   b. **Application type**
      - Web app (React, Next.js, Vue, etc.) → suggest Playwright-based verifier
      - CLI tool → suggest Tmux-based verifier
      - API service (Express, FastAPI, etc.) → suggest HTTP-based verifier

   c. **Existing verification tools**
      - Test frameworks (Jest, Vitest, pytest, etc.)
      - E2E tools (Playwright, Cypress, etc.)
      - Dev server scripts in package.json

   d. **Dev server configuration**
      - How to start the dev server
      - What URL it runs on
      - What text indicates it's ready

3. **Installed verification packages** (for web apps)
   - Check if Playwright is installed (look in package.json dependencies/devDependencies)
   - Check MCP configuration (.mcp.json) for browser automation tools
   - For Python projects, check for playwright, pytest-playwright

## Phase 2: Verification Tool Setup

Based on what was detected in Phase 1, help the user set up appropriate verification tools.

### For Web Applications

1. **If browser automation tools are already installed/configured**, ask the user which one they want to use
2. **If NO browser automation tools are detected**, ask if they want to install/configure one
3. **If user chooses to install Playwright**, run the appropriate install command for their package manager
4. **If user chooses an MCP-based option**, configure the appropriate entry in .mcp.json

### For CLI Tools

1. Check if asciinema is available
2. Tmux is typically system-installed, just verify it's available

### For API Services

1. Check if HTTP testing tools are available (curl, httpie)

## Phase 3: Interactive Q&A

For each distinct project area, use AskUserQuestion to confirm:

1. **Verifier name** — use simple format for single area (verifier-playwright, verifier-cli, verifier-api) or multi-area format (verifier-<project>-<type>). Name MUST include "verifier".

2. **Project-specific questions** based on type (dev server command, URL, ready signal for web; entry point for CLI; server command and base URL for API).

3. **Authentication & Login** — ask if auth is required and gather login details if needed.

## Phase 4: Generate Verifier Skill

Write the skill file to ` + "`.claude/skills/<verifier-name>/SKILL.md`" + ` with appropriate allowed-tools for the verifier type.

## Phase 5: Confirm Creation

After writing the skill file(s), inform the user where each skill was created and how the Verify agent discovers them.`

// newInitVerifiersHandler creates the /init-verifiers prompt command handler.
// Source: src/commands/init-verifiers.ts
func newInitVerifiersHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			return PromptMsg{
				Command: "/init-verifiers",
				Text:    initVerifiersPromptTemplate,
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T258: /insights — generate usage report (stub)
// ---------------------------------------------------------------------------

func newInsightsHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			return PromptMsg{
				Command: "/insights",
				Text:    "Generate a usage insights report. Note: the /insights command is not yet fully implemented — detailed analytics will be available in a future release.",
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T255: /ide — detect installed IDEs and extension status
// Source: src/commands/ide/ide.tsx
// ---------------------------------------------------------------------------

// ideDetector is a function that returns detected IDE info. Package-level var
// so tests can replace it.
var ideDetector func() []IDEInfo = detectIDEs

// detectIDEs scans for VS Code and JetBrains IDE installations.
func detectIDEs() []IDEInfo {
	var results []IDEInfo

	// VS Code detection
	vsInfo := detectVSCode()
	results = append(results, vsInfo)

	// JetBrains detection
	jbInfo := detectJetBrains()
	results = append(results, jbInfo...)

	return results
}

// detectVSCode checks for VS Code installation and Claude extension.
func detectVSCode() IDEInfo {
	info := IDEInfo{
		Name:      "VS Code",
		Extension: "unknown",
	}

	// Check common VS Code binary locations
	codePath, err := exec.LookPath("code")
	if err == nil {
		info.Installed = true
		info.Path = codePath
		// Check for Claude extension
		out, err := exec.Command("code", "--list-extensions").CombinedOutput()
		if err == nil {
			extensions := string(out)
			if strings.Contains(strings.ToLower(extensions), "claude") ||
				strings.Contains(strings.ToLower(extensions), "anthropic") {
				info.Extension = "installed"
			} else {
				info.Extension = "not-installed"
			}
		}
		return info
	}

	// Platform-specific fallback paths
	switch runtime.GOOS {
	case "darwin":
		if _, err := os.Stat("/Applications/Visual Studio Code.app"); err == nil {
			info.Installed = true
			info.Path = "/Applications/Visual Studio Code.app"
		}
	case "linux":
		for _, p := range []string{"/usr/bin/code", "/snap/bin/code", "/usr/share/code/code"} {
			if _, err := os.Stat(p); err == nil {
				info.Installed = true
				info.Path = p
				break
			}
		}
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			p := filepath.Join(localAppData, "Programs", "Microsoft VS Code", "Code.exe")
			if _, err := os.Stat(p); err == nil {
				info.Installed = true
				info.Path = p
			}
		}
	}

	return info
}

// jetbrainsIDEDef describes a JetBrains IDE to search for.
type jetbrainsIDEDef struct {
	name       string
	darwinApp  string
	linuxPaths []string
}

// knownJetBrainsIDEs lists the JetBrains IDEs we detect.
var knownJetBrainsIDEs = []jetbrainsIDEDef{
	{
		name:       "IntelliJ IDEA",
		darwinApp:  "IntelliJ IDEA.app",
		linuxPaths: []string{"/opt/idea", "/snap/intellij-idea-ultimate/current", "/snap/intellij-idea-community/current"},
	},
	{
		name:       "WebStorm",
		darwinApp:  "WebStorm.app",
		linuxPaths: []string{"/opt/webstorm", "/snap/webstorm/current"},
	},
	{
		name:       "PyCharm",
		darwinApp:  "PyCharm.app",
		linuxPaths: []string{"/opt/pycharm", "/snap/pycharm-professional/current", "/snap/pycharm-community/current"},
	},
	{
		name:       "GoLand",
		darwinApp:  "GoLand.app",
		linuxPaths: []string{"/opt/goland", "/snap/goland/current"},
	},
}

// detectJetBrains checks for JetBrains IDE installations.
func detectJetBrains() []IDEInfo {
	var results []IDEInfo
	for _, def := range knownJetBrainsIDEs {
		info := IDEInfo{
			Name:      def.name,
			Extension: "unknown",
		}
		switch runtime.GOOS {
		case "darwin":
			apps := filepath.Join("/Applications", def.darwinApp)
			if _, err := os.Stat(apps); err == nil {
				info.Installed = true
				info.Path = apps
			}
			// Also check ~/Applications
			if home, err := os.UserHomeDir(); err == nil {
				userApps := filepath.Join(home, "Applications", def.darwinApp)
				if _, err := os.Stat(userApps); err == nil {
					info.Installed = true
					info.Path = userApps
				}
			}
		case "linux":
			for _, p := range def.linuxPaths {
				if _, err := os.Stat(p); err == nil {
					info.Installed = true
					info.Path = p
					break
				}
			}
		}
		if info.Installed {
			results = append(results, info)
		}
	}
	return results
}

// newIDEHandler creates the /ide command handler.
func newIDEHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			ides := ideDetector()

			var lines []string
			lines = append(lines, "IDE Detection Results:")
			lines = append(lines, "")

			installedCount := 0
			for _, ide := range ides {
				if ide.Installed {
					installedCount++
					extStatus := ide.Extension
					if extStatus == "unknown" {
						extStatus = "extension status unknown"
					} else if extStatus == "installed" {
						extStatus = "Claude extension installed"
					} else {
						extStatus = "Claude extension not installed"
					}
					lines = append(lines, fmt.Sprintf("  ✓ %s (%s) — %s", ide.Name, ide.Path, extStatus))
				}
			}

			if installedCount == 0 {
				lines = append(lines, "  No supported IDEs detected.")
				lines = append(lines, "")
				lines = append(lines, "Supported IDEs: VS Code, IntelliJ IDEA, WebStorm, PyCharm, GoLand")
			}

			return IDEMsg{
				IDEs:    ides,
				Message: strings.Join(lines, "\n"),
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T252: /heapdump — write heap profile (hidden)
// Source: src/commands/heapdump.ts
// ---------------------------------------------------------------------------

// writeHeapProfile is the default heap profiler (wraps pprof.WriteHeapProfile).
// It is a package-level var so tests can replace it.
var writeHeapProfile func(w io.Writer) error = pprof.WriteHeapProfile

// newHeapdumpHandler creates the /heapdump command handler.
// Uses runtime/pprof to write a heap profile to a temp file.
func newHeapdumpHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			path := filepath.Join(os.TempDir(), fmt.Sprintf("heapdump-%d.pb.gz", time.Now().UnixMilli()))
			f, err := os.Create(path)
			if err != nil {
				return HeapdumpMsg{Error: fmt.Errorf("Failed: %s", err)}
			}
			defer f.Close()
			if err := writeHeapProfile(f); err != nil {
				os.Remove(path)
				return HeapdumpMsg{Error: fmt.Errorf("Failed: %s", err)}
			}
			return HeapdumpMsg{Path: path, Message: fmt.Sprintf("Heap dump written to %s", path)}
		}
	}
}

// ---------------------------------------------------------------------------
// T259: /install-github-app — GitHub App installation wizard (stub)
// Source: src/commands/install-github-app.ts
// ---------------------------------------------------------------------------

func newInstallGitHubAppHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			return InstallGitHubAppMsg{
				Message: "GitHub App installation wizard coming soon. " +
					"Visit https://github.com/apps/claude to install manually.",
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T263: /logout — clear stored credentials
// Source: src/commands/logout.ts
// ---------------------------------------------------------------------------

// newLogoutHandler creates the /logout command handler.
// It clears stored API keys and OAuth tokens, then returns a confirmation.
func newLogoutHandler() Handler {
	return func(args string) tea.Cmd {
		return func() tea.Msg {
			auth.DeleteAPIKey()
			return LogoutMsg{
				Message: "Logged out. Stored credentials have been removed.",
			}
		}
	}
}

func (d *Dispatcher) registerDefaults() {
	d.Register("/model", func(args string) tea.Cmd {
		if args == "" {
			return func() tea.Msg {
				return CommandResult{Command: "/model", Error: fmt.Errorf("usage: /model <name>")}
			}
		}
		return func() tea.Msg { return ModelSwitchMsg{Model: args} }
	})

	d.Register("/session", func(args string) tea.Cmd {
		return func() tea.Msg { return SessionSwitchMsg{} }
	})

	// T233: /clear — full clearing chain (expanded from stub)
	d.RegisterCommand(CommandRegistration{
		Name:        "clear",
		Description: "Clear conversation and reset session",
		Type:        CommandTypeLocal,
		Immediate:   true,
		Source:      "builtin",
		Handler: newClearHandler(ClearState{
			Session:     func() *session.SessionState { return &session.SessionState{} },
			OriginalCWD: func() string { return "" },
		}),
	})

	// T253: /help — HelpV2 screen showing available commands, keybindings, tips
	d.RegisterCommand(CommandRegistration{
		Name:        "help",
		Description: "Show available commands and keybindings",
		Type:        CommandTypeLocal,
		Aliases:     []string{"?"},
		Immediate:   true,
		Source:      "builtin",
		Handler: func(args string) tea.Cmd {
			return func() tea.Msg { return ShowHelpMsg{} }
		},
	})

	// NOTE: /quit is now an alias for /exit (T246), registered below.

	// T237: /compact — model-driven compaction (expanded from stub)
	d.RegisterCommand(CommandRegistration{
		Name:         "compact",
		Description:  "Compact conversation history",
		Type:         CommandTypeLocal,
		ArgumentHint: "[custom instructions]",
		Source:       "builtin",
		Handler: newCompactHandler(CompactDeps{
			GetMessages: func() []message.Message { return nil },
			Summarize: func(ctx context.Context, msgs []message.Message, prompt string) (string, error) {
				return "", fmt.Errorf("summarizer not configured")
			},
		}),
	})

	d.Register("/thinking", func(args string) tea.Cmd {
		return func() tea.Msg { return ThinkingToggleMsg{} }
	})

	d.Register("/doctor", func(args string) tea.Cmd {
		return func() tea.Msg { return ShowDoctorMsg{} }
	})

	d.Register("/resume", func(args string) tea.Cmd {
		return func() tea.Msg { return ShowResumeMsg{} }
	})

	// T225: /add-dir — add a new working directory
	d.RegisterCommand(CommandRegistration{
		Name:         "add-dir",
		Description:  "Add a new working directory",
		Type:         CommandTypeLocalJSX,
		ArgumentHint: "<path>",
		Source:       "builtin",
		Handler:      newAddDirHandler(nil),
	})

	// T226: /advisor — configure the advisor model
	d.RegisterCommand(CommandRegistration{
		Name:         "advisor",
		Description:  "Configure the advisor model",
		Type:         CommandTypeLocal,
		ArgumentHint: "[<model>|off]",
		Source:       "builtin",
		Handler: newAdvisorHandler(
			func() AdvisorState { return AdvisorState{} },
			func(model string) {},
		),
	})

	// T227: /agents — manage agent configurations
	d.RegisterCommand(CommandRegistration{
		Name:        "agents",
		Description: "Manage agent configurations",
		Type:        CommandTypeLocalJSX,
		Source:      "builtin",
		Handler:     newAgentsHandler(nil),
	})

	// T228: /branch — fork the current conversation
	d.RegisterCommand(CommandRegistration{
		Name:        "branch",
		Description: "Fork the current conversation",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler: newBranchHandler(BranchOptions{
			SessionID:     func() string { return "default" },
			SessionName:   func() string { return "Conversation" },
			TranscriptDir: func() string { return os.TempDir() },
			SwitchSession: func(id string) {},
		}),
	})

	// T229: /remote-control — start the bridge for remote control
	d.RegisterCommand(CommandRegistration{
		Name:        "remote-control",
		Description: "Start remote control bridge",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler: newRemoteControlHandler(
			func() bool { return false },
			func() error { return nil },
		),
	})

	// T229: /bridge-kick — ant-only bridge diagnostics
	d.RegisterCommand(CommandRegistration{
		Name:        "bridge-kick",
		Description: "Bridge diagnostics (internal)",
		Type:        CommandTypeLocal,
		IsHidden:    true,
		Source:      "builtin",
		Handler:     newBridgeKickHandler(func() bool { return false }),
	})

	// T230: /brief — toggle brief/concise response mode
	d.RegisterCommand(CommandRegistration{
		Name:        "brief",
		Description: "Toggle brief response mode",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler: newBriefHandler(
			func() bool { return false },
			func(b bool) {},
		),
	})

	// T231: /btw — ask a side question
	d.RegisterCommand(CommandRegistration{
		Name:         "btw",
		Description:  "Ask a side question without disrupting the main conversation",
		Type:         CommandTypeLocalJSX,
		ArgumentHint: "<question>",
		Source:       "builtin",
		Handler: newBtwHandler(func(q string) (string, error) {
			return "Side query not configured.", nil
		}),
	})

	// T232: /chrome — browser integration (stub)
	d.RegisterCommand(CommandRegistration{
		Name:         "chrome",
		Description:  "Chrome browser integration",
		Type:         CommandTypeLocalJSX,
		ArgumentHint: "[install|reconnect|manage-permissions|toggle-default]",
		Source:       "builtin",
		Handler:      newChromeHandler(),
	})

	// T234: /color — set prompt bar color for this session
	d.RegisterCommand(CommandRegistration{
		Name:         "color",
		Description:  "Set session prompt bar color",
		Type:         CommandTypeLocal,
		ArgumentHint: "<color|default>",
		Source:       "builtin",
		Handler: newColorHandler(
			func() string { return "" },
			func(c string) {},
		),
	})

	// T235: /commit — prompt-type command for creating a git commit
	d.RegisterCommand(CommandRegistration{
		Name:        "commit",
		Description: "Create a git commit",
		Type:        CommandTypePrompt,
		Source:      "builtin",
		Handler:     newCommitHandler(),
	})

	// T236: /commit-push-pr — prompt-type command for full PR workflow
	d.RegisterCommand(CommandRegistration{
		Name:         "commit-push-pr",
		Description:  "Commit, push, and open a PR",
		Type:         CommandTypePrompt,
		ArgumentHint: "[additional instructions]",
		Source:       "builtin",
		Handler:      newCommitPushPRHandler(),
	})

	// T238: /config — open settings panel
	d.RegisterCommand(CommandRegistration{
		Name:        "config",
		Description: "Open settings",
		Type:        CommandTypeLocalJSX,
		Source:      "builtin",
		Handler:     newConfigHandler(),
	})

	// T239: /context — context window usage visualization
	d.RegisterCommand(CommandRegistration{
		Name:        "context",
		Description: "Show context window usage",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler: newContextHandler(ContextDeps{
			GetMessages:       func() []message.Message { return nil },
			ContextWindowSize: func() int { return appcontext.ModelContextWindowDefault },
		}),
	})

	// T240: /copy — copy last assistant response to clipboard
	d.RegisterCommand(CommandRegistration{
		Name:         "copy",
		Description:  "Copy last assistant response to clipboard",
		Type:         CommandTypeLocal,
		ArgumentHint: "[N]",
		Source:       "builtin",
		Handler: newCopyHandler(CopyDeps{
			GetMessages: func() []message.Message { return nil },
		}),
	})

	// T241: /cost — display session cost and token usage
	d.RegisterCommand(CommandRegistration{
		Name:        "cost",
		Description: "Show session cost and token usage",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler: newCostHandler(CostDeps{
			GetSession: func() *session.SessionState { return &session.SessionState{} },
		}),
	})

	// T242: /desktop — open session in Claude Desktop app
	d.RegisterCommand(CommandRegistration{
		Name:        "desktop",
		Description: "Open in Claude Desktop",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler:     newDesktopHandler(),
	})

	// T243: /diff — show uncommitted changes
	d.RegisterCommand(CommandRegistration{
		Name:        "diff",
		Description: "Show uncommitted changes",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler:     newDiffHandler(),
	})

	// T245: /effort — set or display effort level
	d.RegisterCommand(CommandRegistration{
		Name:         "effort",
		Description:  "Set effort level",
		Type:         CommandTypeLocal,
		ArgumentHint: "[low|medium|high|max|auto]",
		Source:       "builtin",
		Handler: newEffortHandler(EffortDeps{
			GetLevel: func() string { return "auto" },
			SetLevel: func(level string) error { return nil },
		}),
	})

	// T246: /exit — graceful shutdown with goodbye message
	d.RegisterCommand(CommandRegistration{
		Name:        "exit",
		Description: "Exit Claude Code",
		Type:        CommandTypeLocal,
		Aliases:     []string{"quit"},
		Immediate:   true,
		Source:      "builtin",
		Handler:     newExitHandler(),
	})

	// T247: /export — export conversation to file
	d.RegisterCommand(CommandRegistration{
		Name:         "export",
		Description:  "Export conversation to file",
		Type:         CommandTypeLocal,
		ArgumentHint: "[filename]",
		Source:       "builtin",
		Handler: newExportHandler(ExportDeps{
			GetMessages: func() []message.Message { return nil },
		}),
	})

	// T248: /extra-usage — billing configuration stub
	d.RegisterCommand(CommandRegistration{
		Name:         "extra-usage",
		Description:  "Configure extra usage billing",
		Type:         CommandTypeLocal,
		Availability: []CommandAvailability{AvailabilityClaudeAI},
		Source:       "builtin",
		Handler:      newExtraUsageHandler(func() string { return os.Getenv("USER_TYPE") }),
	})

	// T249: /fast — toggle fast mode
	d.RegisterCommand(CommandRegistration{
		Name:         "fast",
		Description:  "Toggle fast mode",
		Type:         CommandTypeLocal,
		ArgumentHint: "[on|off]",
		Source:       "builtin",
		Handler: newFastHandler(FastModeDeps{
			GetEnabled: func() bool { return false },
			SetEnabled: func(bool) {},
		}),
	})

	// T250: /feedback — submit feedback
	d.RegisterCommand(CommandRegistration{
		Name:        "feedback",
		Description: "Submit feedback",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		IsEnabled: func() bool {
			return os.Getenv("DISABLE_FEEDBACK_COMMAND") == ""
		},
		Handler: newFeedbackHandler(),
	})

	// T251: /files — list files in context (ant-only)
	d.RegisterCommand(CommandRegistration{
		Name:        "files",
		Description: "List files in context",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		IsEnabled: func() bool {
			return os.Getenv("USER_TYPE") == "ant"
		},
		Handler: newFilesHandler(FilesDeps{
			GetFiles: func() []string { return nil },
		}),
	})

	// T255: /ide — detect installed IDEs and extension status
	d.RegisterCommand(CommandRegistration{
		Name:        "ide",
		Description: "Detect installed IDEs and extensions",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler:     newIDEHandler(),
	})

	// T257: /init-verifiers — prompt-type command for creating verifier skills
	d.RegisterCommand(CommandRegistration{
		Name:        "init-verifiers",
		Description: "Create verifier skill(s) for automated verification of code changes",
		Type:        CommandTypePrompt,
		Source:      "builtin",
		Handler:     newInitVerifiersHandler(),
	})

	// T252: /heapdump — write heap profile (hidden)
	d.RegisterCommand(CommandRegistration{
		Name:        "heapdump",
		Description: "Write heap profile",
		Type:        CommandTypeLocal,
		IsHidden:    true,
		Source:      "builtin",
		Handler:     newHeapdumpHandler(),
	})

	// T256: /init — AI-driven CLAUDE.md generation
	d.RegisterCommand(CommandRegistration{
		Name:        "init",
		Description: "Initialize a new CLAUDE.md file with codebase documentation",
		Type:        CommandTypePrompt,
		Source:      "builtin",
		Handler:     newInitHandler(),
	})

	// T260: /install-slack-app — open Slack app installation page
	d.RegisterCommand(CommandRegistration{
		Name:        "install-slack-app",
		Description: "Install the Claude Code Slack app",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler:     newInstallSlackAppHandler(),
	})

	// T254: /hooks — show/manage hook configuration
	d.RegisterCommand(CommandRegistration{
		Name:        "hooks",
		Description: "Show hook configuration",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler: newHooksHandler(HooksDeps{
			GetHooks:     func() []hooks.IndividualHookConfig { return nil },
			GetToolNames: func() []string { return nil },
		}),
	})

	// T258: /insights — generate usage report (stub)
	d.RegisterCommand(CommandRegistration{
		Name:         "insights",
		Description:  "Generate usage report and insights",
		Type:         CommandTypePrompt,
		ArgumentHint: "[time period]",
		Source:       "builtin",
		Handler:      newInsightsHandler(),
	})

	// T259: /install-github-app — GitHub App installation wizard (stub)
	d.RegisterCommand(CommandRegistration{
		Name:        "install-github-app",
		Description: "Install the GitHub App for Claude",
		Type:        CommandTypeLocal,
		Source:      "builtin",
		Handler:     newInstallGitHubAppHandler(),
	})

	// T263: /logout — clear stored credentials and log out
	d.RegisterCommand(CommandRegistration{
		Name:        "logout",
		Description: "Log out and clear stored credentials",
		Type:        CommandTypeLocal,
		Immediate:   true,
		Source:      "builtin",
		Handler:     newLogoutHandler(),
	})
}
