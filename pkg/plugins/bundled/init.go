// Package bundled initializes built-in plugins that ship with the CLI.
//
// Not all bundled features should be built-in plugins — use this for
// features that users should be able to explicitly enable/disable via the
// /plugin UI. For features with complex setup or automatic-enabling logic,
// use pkg/skills/ bundled skills instead.
//
// To add a new built-in plugin:
//  1. Import plugins from the parent package
//  2. Call plugins.RegisterBuiltinPlugin() with the plugin definition
//
// Source: src/plugins/bundled/index.ts
package bundled

import (
	"github.com/projectbarks/gopher-code/pkg/plugins"
)

// InitBuiltinPlugins registers all built-in plugins. Called during CLI startup.
// Source: src/plugins/bundled/index.ts — initBuiltinPlugins
func InitBuiltinPlugins() {
	// No built-in plugins registered yet — this is the scaffolding for
	// migrating bundled skills that should be user-toggleable.
	//
	// Example (when adding a plugin):
	//   plugins.RegisterBuiltinPlugin(plugins.BuiltinPluginDefinition{
	//       Name:        "example",
	//       Description: "Example built-in plugin",
	//   })

	// Ensure the plugins package is linked into the binary.
	_ = plugins.BUILTIN_MARKETPLACE_NAME
}
