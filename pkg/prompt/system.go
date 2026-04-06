package prompt

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/projectbarks/gopher-code/pkg/provider"
)

// Source: constants/system.ts

// System-prompt prefix constants — must match TS verbatim because
// splitSysPromptPrefix identifies prefix blocks by content, not position.
const (
	DefaultPrefix               = `You are Claude Code, Anthropic's official CLI for Claude.`
	AgentSDKClaudeCodePresetPrefix = `You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK.`
	AgentSDKPrefix              = `You are a Claude agent, built on Anthropic's Claude Agent SDK.`
)

// CLISyspromptPrefixes is the set of all possible CLI sysprompt prefix values,
// used by splitSysPromptPrefix to identify prefix blocks by content.
var CLISyspromptPrefixes = map[string]struct{}{
	DefaultPrefix:               {},
	AgentSDKClaudeCodePresetPrefix: {},
	AgentSDKPrefix:              {},
}

// PrefixOptions controls the 3-way prefix selector.
type PrefixOptions struct {
	IsNonInteractive     bool
	HasAppendSystemPrompt bool
}

// GetCLISyspromptPrefix returns the appropriate system prompt prefix.
//
// Decision tree:
//   - vertex provider → DefaultPrefix
//   - isNonInteractive + hasAppendSystemPrompt → AgentSDKClaudeCodePresetPrefix
//   - isNonInteractive → AgentSDKPrefix
//   - else → DefaultPrefix
func GetCLISyspromptPrefix(opts *PrefixOptions) string {
	if provider.GetAPIProvider() == provider.ProviderVertex {
		return DefaultPrefix
	}
	if opts != nil && opts.IsNonInteractive {
		if opts.HasAppendSystemPrompt {
			return AgentSDKClaudeCodePresetPrefix
		}
		return AgentSDKPrefix
	}
	return DefaultPrefix
}

// isEnvDefinedFalsy returns true when the env var is set to a falsy value
// (0, false, no, empty string). Returns false if the env var is unset.
func isEnvDefinedFalsy(key string) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "", "0", "false", "no":
		return true
	}
	return false
}

// AttributionConfig holds injectable dependencies for attribution header
// construction, making the function testable without global state for
// the growthbook killswitch and native attestation feature.
type AttributionConfig struct {
	// Version is the build version (MACRO.VERSION equivalent).
	Version string
	// GrowthBookEnabled returns the tengu_attribution_header flag.
	// Defaults to true when nil.
	GrowthBookEnabled func() bool
	// NativeClientAttestation gates the cch=00000 placeholder.
	NativeClientAttestation bool
	// GetWorkload returns the turn-scoped QoS hint. Nil means no workload.
	GetWorkload func() string
}

// GetAttributionHeader builds the x-anthropic-billing-header value.
//
// Disabled when CLAUDE_CODE_ATTRIBUTION_HEADER is set to a falsy value
// or when the GrowthBook tengu_attribution_header killswitch is off.
func GetAttributionHeader(fingerprint string, cfg AttributionConfig) string {
	// Env gate
	if isEnvDefinedFalsy("CLAUDE_CODE_ATTRIBUTION_HEADER") {
		return ""
	}
	// GrowthBook killswitch (default enabled)
	if cfg.GrowthBookEnabled != nil && !cfg.GrowthBookEnabled() {
		return ""
	}

	version := cfg.Version + "." + fingerprint
	entrypoint := os.Getenv("CLAUDE_CODE_ENTRYPOINT")
	if entrypoint == "" {
		entrypoint = "unknown"
	}

	var sb strings.Builder
	sb.WriteString("x-anthropic-billing-header: cc_version=")
	sb.WriteString(version)
	sb.WriteString("; cc_entrypoint=")
	sb.WriteString(entrypoint)
	sb.WriteByte(';')

	if cfg.NativeClientAttestation {
		sb.WriteString(" cch=00000;")
	}

	if cfg.GetWorkload != nil {
		if w := cfg.GetWorkload(); w != "" {
			sb.WriteString(" cc_workload=")
			sb.WriteString(w)
			sb.WriteByte(';')
		}
	}

	return sb.String()
}

// BuildSystemPrompt constructs the full system prompt with environment context.
func BuildSystemPrompt(base string, cwd string, model string) string {
	var sb strings.Builder

	if base != "" {
		sb.WriteString(base)
	} else {
		sb.WriteString(DefaultSystemPrompt())
	}

	// Add environment section
	sb.WriteString("\n\n# Environment\n")
	sb.WriteString(fmt.Sprintf("- Platform: %s\n", runtime.GOOS))
	sb.WriteString(fmt.Sprintf("- Architecture: %s\n", runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("- Current date: %s\n", time.Now().Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("- Working directory: %s\n", cwd))
	sb.WriteString(fmt.Sprintf("- Model: %s\n", model))

	// Git info
	if gitInfo := getGitInfo(cwd); gitInfo != "" {
		sb.WriteString(gitInfo)
	}

	return sb.String()
}

// CyberRiskInstruction is the security boundary instruction from the Safeguards team.
// Source: constants/cyberRiskInstruction.ts
// DO NOT MODIFY WITHOUT SAFEGUARDS TEAM REVIEW
const CyberRiskInstruction = `IMPORTANT: Assist with authorized security testing, defensive security, CTF challenges, and educational contexts. Refuse requests for destructive techniques, DoS attacks, mass targeting, supply chain compromise, or detection evasion for malicious purposes. Dual-use security tools (C2 frameworks, credential testing, exploit development) require clear authorization context: pentesting engagements, CTF competitions, security research, or defensive use cases.`

// DefaultSystemPrompt returns the default system prompt for gopher-code.
// Source: constants/prompts.ts — getSystemPrompt()
func DefaultSystemPrompt() string {
	return `You are an interactive agent that helps users with software engineering tasks. Use the tools available to you to assist the user.

` + CyberRiskInstruction + `
IMPORTANT: You must NEVER generate or guess URLs for the user unless you are confident that the URLs are for helping the user with programming. You may use URLs provided by the user in their messages or local files.

# System
- All text you output outside of tool use is displayed to the user.
- You can use Github-flavored markdown for formatting.
- Tools are executed based on the user's permission settings.

# Doing tasks
- The user will primarily request you to perform software engineering tasks.
- In general, do not propose changes to code you haven't read. If a user asks about or wants you to modify a file, read it first.
- Do not create files unless they're absolutely necessary.
- Avoid giving time estimates or predictions.

# Executing actions with care
- Carefully consider the reversibility and blast radius of actions.
- For actions that are hard to reverse or affect shared systems, check with the user before proceeding.

# Using your tools
- To read files use the Read tool instead of cat, head, tail, or sed
- To edit files use the Edit tool instead of sed or awk
- To create files use the Write tool instead of echo redirection
- To search for files use the Glob tool instead of find or ls
- To search content use the Grep tool instead of grep or rg

# Tone and style
- Your responses should be short and concise.
- Go straight to the point. Try the simplest approach first.
- Keep your text output brief and direct.`
}

func getGitInfo(cwd string) string {
	// Get branch
	branch, err := runCmd(cwd, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("- Git branch: %s\n", strings.TrimSpace(branch)))

	// Get short status (first 5 lines)
	status, err := runCmd(cwd, "git", "status", "--short")
	if err == nil && strings.TrimSpace(status) != "" {
		lines := strings.Split(strings.TrimSpace(status), "\n")
		if len(lines) > 5 {
			lines = lines[:5]
			lines = append(lines, fmt.Sprintf("... and %d more", len(strings.Split(status, "\n"))-5))
		}
		sb.WriteString("- Git status:\n")
		for _, l := range lines {
			sb.WriteString(fmt.Sprintf("  %s\n", l))
		}
	}

	return sb.String()
}

func runCmd(cwd string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	return string(out), err
}
