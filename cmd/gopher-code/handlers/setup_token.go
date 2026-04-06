// Source: src/cli/handlers/util.tsx — setupTokenHandler
package handlers

import (
	"fmt"
	"io"
	"os"
)

// Verbatim user-visible strings from TS source.
// Source: src/cli/handlers/util.tsx lines 31–42
const (
	SetupTokenAuthWarning1 = "Warning: You already have authentication configured via environment variable or API key helper."
	SetupTokenAuthWarning2 = "The setup-token command will create a new OAuth token which you can use instead."
	SetupTokenStarting     = "This will guide you through long-lived (1-year) auth token setup for your Claude account. Claude subscription required."
)

// SetupTokenOpts configures the setup-token handler.
type SetupTokenOpts struct {
	Output io.Writer // defaults to os.Stdout

	// AuthChecker reports whether Anthropic OAuth auth is already enabled.
	// When it returns false the handler prints an auth-conflict warning.
	// Nil means use the default check (env-var / keyring / helper detection).
	AuthChecker func() bool
}

// isAnthropicAuthEnabled returns true when no external auth source is
// configured (env var, API key helper, 3P provider). This is a simplified
// port of isAnthropicAuthEnabled() in src/utils/auth.ts; the full version
// will be wired up as more auth infrastructure lands.
func isAnthropicAuthEnabled() bool {
	// 3P providers disable Anthropic auth
	for _, env := range []string{
		"CLAUDE_CODE_USE_BEDROCK",
		"CLAUDE_CODE_USE_VERTEX",
		"CLAUDE_CODE_USE_FOUNDRY",
	} {
		if v := os.Getenv(env); v == "1" || v == "true" {
			return false
		}
	}

	// External API key disables Anthropic auth
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return false
	}

	// External auth token disables Anthropic auth
	if os.Getenv("ANTHROPIC_AUTH_TOKEN") != "" {
		return false
	}

	return true
}

// SetupToken handles `claude setup-token`.
// It prints auth-conflict warnings when an external auth source is detected,
// emits the starting message, and launches the ConsoleOAuthFlow.
// Source: src/cli/handlers/util.tsx — setupTokenHandler
func SetupToken(opts SetupTokenOpts) int {
	w := output(opts.Output)

	// Analytics: tengu_setup_token_command (stub — T-analytics)

	checker := opts.AuthChecker
	if checker == nil {
		checker = isAnthropicAuthEnabled
	}

	// Show auth-conflict warning when external auth is already configured.
	// TS: const showAuthWarning = !isAnthropicAuthEnabled()
	if !checker() {
		fmt.Fprintln(w, SetupTokenAuthWarning1)
		fmt.Fprintln(w, SetupTokenAuthWarning2)
		fmt.Fprintln(w)
	}

	// Starting message (verbatim from TS source)
	fmt.Fprintln(w, SetupTokenStarting)

	// TODO(T244+): launch ConsoleOAuthFlow TUI component here.
	// For now the handler prints the messages and returns success.
	return 0
}
