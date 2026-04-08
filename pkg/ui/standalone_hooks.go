// T415: Standalone hooks integration — wires the remaining top-level hooks from
// pkg/ui/hooks into the AppModel so they are reachable from main().
//
// Hooks wired here:
//   - ApiKeyVerification  (hooks/useApiKeyVerification.ts)
//   - GlobalKeybindings   (hooks/useGlobalKeybindings.tsx)
//   - MergedTools          (hooks/useMergedTools.ts)
//   - InteractionTracker + NotifyAfterTimeout (hooks/useNotifyAfterTimeout.ts)
//   - TerminalSizeTracker  (hooks/useTerminalSize.ts)
//   - UpdateNotification   (hooks/useUpdateNotification.ts)
package ui

import (
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks"
)

// initStandaloneHooks wires the remaining standalone hook structs into the
// AppModel. Called from NewAppModel after the session is set.
func (a *AppModel) initStandaloneHooks() {
	// Terminal size tracker — starts with current dimensions.
	a.terminalSize = hooks.NewTerminalSizeTracker(a.width, a.height)

	// Global keybindings state machine.
	a.globalKeys = hooks.NewGlobalKeybindings()

	// Merged tools pool — starts in the session's permission mode.
	mode := permissions.ModeDefault
	if a.session != nil && a.session.Config.PermissionMode != "" {
		mode = a.session.Config.PermissionMode
	}
	a.mergedTools = hooks.NewMergedTools(mode)

	// API key verification — wired with no-op stubs; the real providers are
	// injected by the startup sequence once auth config is resolved.
	a.apiKeyVerification = hooks.NewApiKeyVerification(hooks.ApiKeyVerificationConfig{
		AuthEnabled:    func() bool { return false },
		IsSubscriber:   func() bool { return false },
		KeyProvider:    func(skipHelper bool) hooks.KeyResult { return hooks.KeyResult{} },
		HelperWarmer:   nil,
		Verifier:       func(apiKey string, silent bool) (bool, error) { return true, nil },
		NonInteractive: func() bool { return false },
	})

	// Interaction tracker for idle timeout notifications.
	a.interactionTracker = hooks.NewInteractionTracker()

	// Update notification deduplication — seeded with "0.0.0" (real version
	// injected later by the startup sequence).
	a.updateNotification = hooks.NewUpdateNotification("0.0.0")
}

// TerminalSizeTracker returns the terminal size tracker for external access.
func (a *AppModel) TerminalSizeTracker() *hooks.TerminalSizeTracker {
	return a.terminalSize
}

// GlobalKeybindings returns the global keybinding state.
func (a *AppModel) GlobalKeybindings() *hooks.GlobalKeybindings {
	return a.globalKeys
}

// MergedTools returns the merged tool pool.
func (a *AppModel) MergedTools() *hooks.MergedTools {
	return a.mergedTools
}

// ApiKeyVerification returns the API key verification state machine.
func (a *AppModel) ApiKeyVerification() *hooks.ApiKeyVerification {
	return a.apiKeyVerification
}

// InteractionTracker returns the user interaction tracker.
func (a *AppModel) InteractionTracker() *hooks.InteractionTracker {
	return a.interactionTracker
}

// UpdateNotificationTracker returns the update notification deduplicator.
func (a *AppModel) UpdateNotificationTracker() *hooks.UpdateNotification {
	return a.updateNotification
}
