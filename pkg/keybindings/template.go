package keybindings

import (
	"strings"
)

// Source: keybindings/template.ts

// GenerateTemplate produces a keybindings.json template with all default bindings.
// Users can copy this as a starting point for customization.
func GenerateTemplate() string {
	var sb strings.Builder
	sb.WriteString("// Keybindings configuration\n")
	sb.WriteString("// Customize keyboard shortcuts by editing this file.\n")
	sb.WriteString("// See /keybindings for current bindings.\n")
	sb.WriteString("{\n")

	bindings := DefaultBindingMap()
	first := true
	for ctx, ctxBindings := range bindings {
		if !first {
			sb.WriteString(",\n")
		}
		first = false
		sb.WriteString("  \"" + string(ctx) + "\": {\n")
		bFirst := true
		for key, action := range ctxBindings {
			if !bFirst {
				sb.WriteString(",\n")
			}
			bFirst = false
			sb.WriteString("    \"" + string(key) + "\": \"" + string(action) + "\"")
		}
		sb.WriteString("\n  }")
	}

	sb.WriteString("\n}\n")
	return sb.String()
}
