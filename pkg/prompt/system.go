package prompt

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

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

// DefaultSystemPrompt returns the default system prompt for gopher-code.
func DefaultSystemPrompt() string {
	return `You are an interactive agent that helps users with software engineering tasks. Use the tools available to you to assist the user.

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
