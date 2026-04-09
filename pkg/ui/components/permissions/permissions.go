// Package permissions provides permission request UI components.
// Source: components/permissions/ — PermissionDialog, PermissionPrompt,
//         SandboxPermissionRequest, FallbackPermissionRequest, etc.
//
// These render the "Claude wants to X. Allow?" prompts that appear
// when tools need user approval.
package permissions

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// RequestType classifies permission requests for rendering.
type RequestType string

const (
	RequestBash     RequestType = "bash"
	RequestEdit     RequestType = "edit"
	RequestWrite    RequestType = "write"
	RequestSandbox  RequestType = "sandbox"
	RequestWebFetch RequestType = "web_fetch"
	RequestPlanMode RequestType = "plan_mode"
	RequestSkill    RequestType = "skill"
	RequestFallback RequestType = "fallback"
)

// Request describes what the model wants to do and needs permission for.
type Request struct {
	Type        RequestType
	ToolName    string
	Description string // human-readable description of what will happen
	Command     string // for bash: the command to run
	FilePath    string // for edit/write: the file path
	URL         string // for web_fetch: the URL
	IsDangerous bool   // triggers extra warning styling
}

// Decision is the user's response to a permission request.
type Decision string

const (
	DecisionAllow      Decision = "allow"
	DecisionDeny       Decision = "deny"
	DecisionAlwaysAllow Decision = "always_allow"
)

// PermissionDecisionMsg carries the user's permission decision.
type PermissionDecisionMsg struct {
	ToolUseID string
	Decision  Decision
	Request   Request
}

// Model is the permission prompt overlay.
type Model struct {
	request   Request
	toolUseID string
	selected  int // 0=allow, 1=deny, 2=always allow
}

// New creates a permission prompt for the given request.
func New(toolUseID string, req Request) Model {
	return Model{request: req, toolUseID: toolUseID}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.selected > 0 {
				m.selected--
			}
		case tea.KeyDown, 'j':
			if m.selected < 2 {
				m.selected++
			}
		case 'y':
			return m, m.decide(DecisionAllow)
		case 'n':
			return m, m.decide(DecisionDeny)
		case 'a':
			return m, m.decide(DecisionAlwaysAllow)
		case tea.KeyEnter:
			decisions := []Decision{DecisionAllow, DecisionDeny, DecisionAlwaysAllow}
			return m, m.decide(decisions[m.selected])
		case tea.KeyEscape:
			return m, m.decide(DecisionDeny)
		}
	}
	return m, nil
}

func (m Model) decide(d Decision) tea.Cmd {
	return func() tea.Msg {
		return PermissionDecisionMsg{
			ToolUseID: m.toolUseID,
			Decision:  d,
			Request:   m.request,
		}
	}
}

func (m Model) View() string {
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	dangerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	dimStyle := lipgloss.NewStyle().Faint(true)
	titleStyle := lipgloss.NewStyle().Bold(true)

	var sb strings.Builder

	// Title with appropriate severity
	headerStyle := warnStyle
	if m.request.IsDangerous {
		headerStyle = dangerStyle
	}
	sb.WriteString(headerStyle.Render("⚠ Permission Required"))
	sb.WriteString("\n\n")

	// Tool and action description
	sb.WriteString(titleStyle.Render(m.request.ToolName))
	sb.WriteString("\n")
	sb.WriteString(m.request.Description)
	sb.WriteString("\n")

	// Type-specific details
	switch m.request.Type {
	case RequestBash:
		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render("Command: ") + m.request.Command)
	case RequestEdit, RequestWrite:
		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render("File: ") + m.request.FilePath)
	case RequestWebFetch:
		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render("URL: ") + m.request.URL)
	}

	sb.WriteString("\n\n")

	// Options
	options := []struct {
		label string
		key   string
	}{
		{"Allow", "y"},
		{"Deny", "n"},
		{"Always allow for this session", "a"},
	}

	for i, opt := range options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.selected {
			cursor = "> "
			style = selStyle
		}
		sb.WriteString(fmt.Sprintf("%s%s  %s\n",
			cursor,
			style.Render(opt.label),
			dimStyle.Render("("+opt.key+")")))
	}

	return sb.String()
}

// ---------------------------------------------------------------------------
// Permission request routing — Source: components/permissions/PermissionRequest.tsx
// ---------------------------------------------------------------------------

// RequestTypeForTool returns the request type for a given tool name.
// Source: PermissionRequest.tsx:permissionComponentForTool
func RequestTypeForTool(toolName string) RequestType {
	switch toolName {
	case "Bash":
		return RequestBash
	case "Edit", "FileEdit":
		return RequestEdit
	case "Write", "FileWrite":
		return RequestWrite
	case "PowerShell":
		return RequestBash // same UI as Bash
	case "WebFetch":
		return RequestWebFetch
	case "NotebookEdit":
		return RequestEdit
	case "ExitPlanMode":
		return RequestPlanMode
	case "EnterPlanMode":
		return RequestPlanMode
	case "Skill":
		return RequestSkill
	case "AskUserQuestion":
		return RequestFallback // handled by separate ask_question component
	case "Glob", "Grep", "Read", "FileRead":
		return RequestFallback // read-only, usually auto-allowed
	default:
		return RequestFallback
	}
}

// BuildRequest creates a Request from tool invocation details.
func BuildRequest(toolName string, description string, params map[string]string) Request {
	reqType := RequestTypeForTool(toolName)
	req := Request{
		Type:        reqType,
		ToolName:    toolName,
		Description: description,
	}

	// Populate type-specific fields from params
	if cmd, ok := params["command"]; ok {
		req.Command = cmd
	}
	if fp, ok := params["file_path"]; ok {
		req.FilePath = fp
	}
	if fp, ok := params["filePath"]; ok && req.FilePath == "" {
		req.FilePath = fp
	}
	if url, ok := params["url"]; ok {
		req.URL = url
	}

	// Mark dangerous commands
	if reqType == RequestBash && req.Command != "" {
		req.IsDangerous = isDangerousCommand(req.Command)
	}

	return req
}

// isDangerousCommand checks if a bash command is potentially dangerous.
func isDangerousCommand(cmd string) bool {
	dangerous := []string{
		"rm -rf", "rm -r /", "dd if=",
		"mkfs.", "> /dev/",
		"chmod 777", "chmod -R 777",
		":(){ :|:& };:",
		"curl | sh", "wget | sh",
		"curl | bash", "wget | bash",
	}
	lower := strings.ToLower(cmd)
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return true
		}
	}
	return false
}

// PermissionQueue manages pending permission requests.
type PermissionQueue struct {
	pending []PendingRequest
}

// PendingRequest is a queued permission request awaiting user decision.
type PendingRequest struct {
	ToolUseID string
	Request   Request
}

// NewPermissionQueue creates an empty queue.
func NewPermissionQueue() *PermissionQueue {
	return &PermissionQueue{}
}

// Enqueue adds a permission request to the queue.
func (q *PermissionQueue) Enqueue(toolUseID string, req Request) {
	q.pending = append(q.pending, PendingRequest{ToolUseID: toolUseID, Request: req})
}

// Dequeue removes and returns the next pending request, or nil.
func (q *PermissionQueue) Dequeue() *PendingRequest {
	if len(q.pending) == 0 {
		return nil
	}
	r := q.pending[0]
	q.pending = q.pending[1:]
	return &r
}

// Peek returns the next pending request without removing it.
func (q *PermissionQueue) Peek() *PendingRequest {
	if len(q.pending) == 0 {
		return nil
	}
	return &q.pending[0]
}

// Len returns the number of pending requests.
func (q *PermissionQueue) Len() int { return len(q.pending) }

// IsEmpty returns true if there are no pending requests.
func (q *PermissionQueue) IsEmpty() bool { return len(q.pending) == 0 }

// Clear removes all pending requests.
func (q *PermissionQueue) Clear() { q.pending = nil }

// RenderCompactPermission returns a one-line permission summary.
func RenderCompactPermission(req Request) string {
	switch req.Type {
	case RequestBash:
		return fmt.Sprintf("Run: %s", req.Command)
	case RequestEdit:
		return fmt.Sprintf("Edit: %s", req.FilePath)
	case RequestWrite:
		return fmt.Sprintf("Write: %s", req.FilePath)
	case RequestWebFetch:
		return fmt.Sprintf("Fetch: %s", req.URL)
	default:
		return req.Description
	}
}
