// Package doctor aggregates all diagnostic data for the /doctor health-check screen.
// Source: Doctor.tsx — getDoctorDiagnostic aggregator
package doctor

import (
	"os"
	"runtime"

	uidoctor "github.com/projectbarks/gopher-code/pkg/ui/doctor"
)

// Version is injected at build time or set by caller.
var Version = "0.2.0"

// DiagnosticData holds all collected diagnostic information.
// Source: Doctor.tsx — getDoctorDiagnostic return type
type DiagnosticData struct {
	// Core diagnostic info (T61)
	Version            string
	InstallationType   string
	InstallationPath   string
	InvokedBinary      string
	ConfigInstallMethod string
	PackageManager     string

	// Dist tags (T62)
	DistTags    *uidoctor.DistTags
	DistTagsErr error

	// Context warnings (T63)
	ContextWarnings *uidoctor.ContextWarnings

	// PID lock info (T64)
	VersionLocks *uidoctor.VersionLockInfo

	// Agent info (T65)
	AgentInfo *uidoctor.AgentInfo

	// Env-var validation (T66)
	EnvValidation []uidoctor.EnvValidationResult

	// Settings/keybinding/MCP warnings (T67)
	SettingsErrors      []uidoctor.SettingsError
	KeybindingWarnings  []uidoctor.KeybindingWarning
	MCPWarnings         []uidoctor.MCPParsingWarning

	// Sandbox status (T68)
	Sandbox uidoctor.SandboxStatus

	// Update settings
	AutoUpdates   string
	UpdateChannel string
}

// CollectOptions configures which diagnostic sections to collect.
type CollectOptions struct {
	// FetchDistTags is an optional callback for fetching dist tags.
	// If nil, dist tags are skipped.
	FetchDistTags uidoctor.FetchDistTagsFunc

	// EnvBounds is the list of env-var bounds to validate.
	// If nil, DefaultEnvBounds() is used.
	EnvBounds []uidoctor.EnvBound

	// SettingsErrors are pre-collected settings validation errors.
	SettingsErrors []uidoctor.SettingsError

	// KeybindingWarnings are pre-collected keybinding parse warnings.
	KeybindingWarnings []uidoctor.KeybindingWarning

	// MCPWarnings are pre-collected MCP config parse warnings.
	MCPWarnings []uidoctor.MCPParsingWarning

	// ContextWarnings are pre-collected context warnings.
	ContextWarnings *uidoctor.ContextWarnings

	// VersionLocks are pre-collected PID lock info.
	VersionLocks *uidoctor.VersionLockInfo

	// AgentInfo is pre-collected agent directory scan results.
	AgentInfo *uidoctor.AgentInfo
}

// Collect gathers all diagnostic data.
// Source: Doctor.tsx — getDoctorDiagnostic function
func Collect(opts CollectOptions) *DiagnosticData {
	data := &DiagnosticData{
		Version:            Version,
		InstallationType:   "go-binary",
		InstallationPath:   executablePath(),
		InvokedBinary:      os.Args[0],
		ConfigInstallMethod: "direct",
		AutoUpdates:        "enabled",
		UpdateChannel:      "latest",
	}

	// T62: Dist tags
	if opts.FetchDistTags != nil {
		data.DistTags, data.DistTagsErr = opts.FetchDistTags()
	}

	// T63: Context warnings
	data.ContextWarnings = opts.ContextWarnings

	// T64: PID locks
	data.VersionLocks = opts.VersionLocks

	// T65: Agent info
	data.AgentInfo = opts.AgentInfo

	// T66: Env-var validation
	bounds := opts.EnvBounds
	if bounds == nil {
		bounds = uidoctor.DefaultEnvBounds()
	}
	data.EnvValidation = uidoctor.ValidateEnvBounds(bounds)

	// T67: Settings/keybinding/MCP warnings
	data.SettingsErrors = opts.SettingsErrors
	data.KeybindingWarnings = opts.KeybindingWarnings
	data.MCPWarnings = opts.MCPWarnings

	// T68: Sandbox
	data.Sandbox = uidoctor.DetectSandboxStatus()

	return data
}

func executablePath() string {
	path, err := os.Executable()
	if err != nil {
		return runtime.GOARCH + "-unknown"
	}
	return path
}
