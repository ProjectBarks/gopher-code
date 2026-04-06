package bundled

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/plugins"
)

// ── T32: initBuiltinPlugins startup scaffold ────────────────────────

func TestInitBuiltinPlugins_DoesNotPanic(t *testing.T) {
	plugins.ClearBuiltinPlugins()
	defer plugins.ClearBuiltinPlugins()

	// Should complete without panic — currently a no-op scaffold.
	InitBuiltinPlugins()

	// Verify no plugins registered (scaffold is empty).
	result := plugins.GetBuiltinPlugins()
	if len(result.Enabled) != 0 || len(result.Disabled) != 0 {
		t.Errorf("expected no plugins from empty scaffold, got enabled=%d disabled=%d",
			len(result.Enabled), len(result.Disabled))
	}
}
