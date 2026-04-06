package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
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

	d.Register("/clear", func(args string) tea.Cmd {
		return func() tea.Msg { return ClearConversationMsg{} }
	})

	d.Register("/help", func(args string) tea.Cmd {
		return func() tea.Msg { return ShowHelpMsg{} }
	})

	d.Register("/quit", func(args string) tea.Cmd {
		return func() tea.Msg { return QuitMsg{} }
	})

	d.Register("/compact", func(args string) tea.Cmd {
		return func() tea.Msg { return CompactMsg{} }
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
}
