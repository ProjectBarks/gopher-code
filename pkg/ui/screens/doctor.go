// Package screens provides full-screen Bubbletea models for modal UI flows
// like /doctor, /resume, etc.
package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	pkgdoctor "github.com/projectbarks/gopher-code/pkg/doctor"
	"github.com/projectbarks/gopher-code/pkg/ui/doctor"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// DoctorDoneMsg is sent when the user dismisses the doctor screen.
type DoctorDoneMsg struct{}

// DoctorDiagnostic holds the core installation diagnostic info.
// Source: Doctor.tsx — DiagnosticInfo fields
type DoctorDiagnostic struct {
	Version            string
	InstallationType   string
	InstallationPath   string
	InvokedBinary      string
	ConfigInstallMethod string
	PackageManager     string
	AutoUpdates        string
	HasUpdatePerms     *bool    // nil = unknown
	Warnings           []DoctorWarning
	MultipleInstalls   []InstallEntry
	Recommendation     string
}

// DoctorWarning is a diagnostic warning with a fix suggestion.
type DoctorWarning struct {
	Issue string
	Fix   string
}

// InstallEntry describes a found installation.
type InstallEntry struct {
	Type string
	Path string
}

// DoctorConfig holds all data needed to render the doctor screen.
// Callers populate this before creating the model.
type DoctorConfig struct {
	Diagnostic      *DoctorDiagnostic
	DistTags        *doctor.DistTags
	DistTagsErr     error
	AutoUpdates     string
	UpdateChannel   string
	ContextWarnings *doctor.ContextWarnings
	VersionLocks    *doctor.VersionLockInfo
	AgentInfo       *doctor.AgentInfo

	// T66: Env-var validation results
	EnvValidation []doctor.EnvValidationResult

	// T67: Settings/keybinding/MCP warnings
	SettingsErrors     []doctor.SettingsError
	KeybindingWarnings []doctor.KeybindingWarning
	MCPWarnings        []doctor.MCPParsingWarning

	// T68: Sandbox status
	Sandbox *doctor.SandboxStatus
}

// NewDoctorConfigFromDiagnostic builds a DoctorConfig from an aggregated DiagnosticData.
// Source: Doctor.tsx — wiring getDoctorDiagnostic into the UI
func NewDoctorConfigFromDiagnostic(d *pkgdoctor.DiagnosticData) DoctorConfig {
	cfg := DoctorConfig{
		Diagnostic: &DoctorDiagnostic{
			Version:            d.Version,
			InstallationType:   d.InstallationType,
			InstallationPath:   d.InstallationPath,
			InvokedBinary:      d.InvokedBinary,
			ConfigInstallMethod: d.ConfigInstallMethod,
			PackageManager:     d.PackageManager,
			AutoUpdates:        d.AutoUpdates,
		},
		DistTags:           d.DistTags,
		DistTagsErr:        d.DistTagsErr,
		AutoUpdates:        d.AutoUpdates,
		UpdateChannel:      d.UpdateChannel,
		ContextWarnings:    d.ContextWarnings,
		VersionLocks:       d.VersionLocks,
		AgentInfo:          d.AgentInfo,
		EnvValidation:      d.EnvValidation,
		SettingsErrors:     d.SettingsErrors,
		KeybindingWarnings: d.KeybindingWarnings,
		MCPWarnings:        d.MCPWarnings,
		Sandbox:            &d.Sandbox,
	}
	return cfg
}

// DoctorModel is the Bubbletea model for the /doctor health-check screen.
// It renders a scrollable view of diagnostic sections.
// Source: Doctor.tsx — aggregates diagnostics into Pane with PressEnterToContinue
type DoctorModel struct {
	config   DoctorConfig
	width    int
	height   int
	scroll   int // scroll offset (lines)
	rendered string
}

// NewDoctorModel creates a new doctor screen model with the given config.
func NewDoctorModel(cfg DoctorConfig) *DoctorModel {
	return &DoctorModel{config: cfg}
}

// Init implements tea.Model.
func (m *DoctorModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *DoctorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rendered = "" // invalidate cache
		return m, nil

	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEnter:
			return m, func() tea.Msg { return DoctorDoneMsg{} }
		case tea.KeyUp, 'k':
			if m.scroll > 0 {
				m.scroll--
			}
			return m, nil
		case tea.KeyDown, 'j':
			m.scroll++
			return m, nil
		case tea.KeyEscape:
			return m, func() tea.Msg { return DoctorDoneMsg{} }
		case 'c':
			if msg.Mod == tea.ModCtrl {
				return m, func() tea.Msg { return DoctorDoneMsg{} }
			}
		case 'd':
			if msg.Mod == tea.ModCtrl {
				return m, func() tea.Msg { return DoctorDoneMsg{} }
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m *DoctorModel) View() tea.View {
	if m.width == 0 {
		return tea.NewView("Checking installation status...")
	}

	content := m.renderContent()
	lines := strings.Split(content, "\n")

	// Clamp scroll
	viewHeight := m.height - 2 // leave room for footer
	if viewHeight < 1 {
		viewHeight = 1
	}
	maxScroll := len(lines) - viewHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}

	// Slice visible lines
	end := m.scroll + viewHeight
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[m.scroll:end]

	// Footer
	t := theme.Current()
	dim := lipgloss.NewStyle().Faint(true)
	footer := dim.Render("Press Enter to continue")
	if len(lines) > viewHeight {
		pct := 100
		if maxScroll > 0 {
			pct = m.scroll * 100 / maxScroll
		}
		footer = dim.Render(t.TextSecondary().Render("  [scroll: j/k]")) + "  " + footer
		_ = pct
	}

	result := strings.Join(visible, "\n") + "\n" + footer
	return tea.NewView(result)
}

// renderContent builds the full diagnostic text from all sections.
func (m *DoctorModel) renderContent() string {
	if m.rendered != "" {
		return m.rendered
	}

	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	warn := t.TextWarning()
	dim := lipgloss.NewStyle().Faint(true)

	var sections []string

	// === Diagnostics section ===
	if diag := m.config.Diagnostic; diag != nil {
		var lines []string
		lines = append(lines, bold.Render("Diagnostics"))
		lines = append(lines, "└ Currently running: "+diag.InstallationType+" ("+diag.Version+")")
		if diag.PackageManager != "" {
			lines = append(lines, "└ Package manager: "+diag.PackageManager)
		}
		lines = append(lines, "└ Path: "+diag.InstallationPath)
		lines = append(lines, "└ Invoked: "+diag.InvokedBinary)
		lines = append(lines, "└ Config install method: "+diag.ConfigInstallMethod)

		if diag.Recommendation != "" {
			parts := strings.SplitN(diag.Recommendation, "\n", 2)
			lines = append(lines, "")
			lines = append(lines, warn.Render("Recommendation: "+parts[0]))
			if len(parts) > 1 {
				lines = append(lines, dim.Render(parts[1]))
			}
		}

		if len(diag.MultipleInstalls) > 1 {
			lines = append(lines, "")
			lines = append(lines, warn.Render("Warning: Multiple installations found"))
			for _, inst := range diag.MultipleInstalls {
				lines = append(lines, "└ "+inst.Type+" at "+inst.Path)
			}
		}

		for _, w := range diag.Warnings {
			lines = append(lines, "")
			lines = append(lines, warn.Render("Warning: "+w.Issue))
			lines = append(lines, "Fix: "+w.Fix)
		}

		sections = append(sections, strings.Join(lines, "\n"))
	}

	// === Updates / dist tags section ===
	distSection := doctor.RenderDistTags(
		m.config.DistTags,
		m.config.DistTagsErr,
		m.config.AutoUpdates,
		m.config.UpdateChannel,
	)
	if distSection != "" {
		sections = append(sections, distSection)
	}

	// === Version Locks section ===
	lockSection := doctor.RenderPIDLocks(m.config.VersionLocks)
	if lockSection != "" {
		sections = append(sections, lockSection)
	}

	// === Agents section ===
	agentSection := doctor.RenderAgents(m.config.AgentInfo)
	if agentSection != "" {
		sections = append(sections, agentSection)
	}

	// === Context Warnings section ===
	ctxSection := doctor.RenderContextWarnings(m.config.ContextWarnings)
	if ctxSection != "" {
		sections = append(sections, ctxSection)
	}

	// === T66: Env-var validation section ===
	envSection := doctor.RenderEnvValidation(m.config.EnvValidation)
	if envSection != "" {
		sections = append(sections, envSection)
	}

	// === T67: Settings errors section ===
	settingsSection := doctor.RenderSettingsErrors(m.config.SettingsErrors)
	if settingsSection != "" {
		sections = append(sections, settingsSection)
	}

	// === T67: Keybinding warnings section ===
	kbSection := doctor.RenderKeybindingWarnings(m.config.KeybindingWarnings)
	if kbSection != "" {
		sections = append(sections, kbSection)
	}

	// === T67: MCP warnings section ===
	mcpSection := doctor.RenderMCPWarnings(m.config.MCPWarnings)
	if mcpSection != "" {
		sections = append(sections, mcpSection)
	}

	// === T68: Sandbox section ===
	if m.config.Sandbox != nil {
		sandboxSection := doctor.RenderSandbox(*m.config.Sandbox)
		if sandboxSection != "" {
			sections = append(sections, sandboxSection)
		}
	}

	m.rendered = strings.Join(sections, "\n\n")
	return m.rendered
}
