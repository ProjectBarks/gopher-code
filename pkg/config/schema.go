package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Source: utils/settings/schemaOutput.ts, utils/settings/types.ts

// SettingsJSONSchema is the JSON Schema for Claude Code settings files.
// Derived from the TS SettingsSchema Zod type.
// Source: utils/settings/types.ts
const SettingsJSONSchema = `{
  "type": "object",
  "properties": {
    "model": {"type": "string"},
    "max_turns": {"type": "integer", "minimum": 1},
    "permission_mode": {"type": "string", "enum": ["default", "acceptEdits", "bypassPermissions", "dontAsk", "plan", "auto"]},
    "allowed_tools": {"type": "array", "items": {"type": "string"}},
    "disallowed_tools": {"type": "array", "items": {"type": "string"}},
    "hooks": {"type": "array", "items": {
      "type": "object",
      "properties": {
        "type": {"type": "string"},
        "matcher": {"type": "string"},
        "command": {"type": "string"},
        "timeout": {"type": "integer"}
      }
    }},
    "system_prompt": {"type": "string"},
    "append_system_prompt": {"type": "string"},
    "theme": {"type": "string"},
    "verbose": {"type": "boolean"},
    "api_url": {"type": "string"},
    "api_version": {"type": "string"},
    "permissions": {"type": "object", "properties": {
      "allow": {"type": "array", "items": {"type": "string"}},
      "deny": {"type": "array", "items": {"type": "string"}},
      "ask": {"type": "array", "items": {"type": "string"}}
    }},
    "sandbox": {"type": "object", "properties": {
      "excludedCommands": {"type": "array", "items": {"type": "string"}}
    }},
    "autoDreamEnabled": {"type": "boolean"}
  },
  "additionalProperties": true
}`

// SchemaValidationError describes a schema validation error.
// Source: utils/settings/validation.ts — ValidationError type
type SchemaValidationError struct {
	File         string `json:"file,omitempty"`         // relative file path
	Path         string `json:"path"`                   // field path in dot notation
	Message      string `json:"message"`                // human-readable error
	Expected     string `json:"expected,omitempty"`      // expected value or type
	InvalidValue any    `json:"invalidValue,omitempty"` // the actual invalid value
	Suggestion   string `json:"suggestion,omitempty"`    // fix suggestion
	DocLink      string `json:"docLink,omitempty"`       // documentation URL
}

func (e *SchemaValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return e.Message
}

// ValidateSettingsJSON validates a settings JSON blob against the schema.
// Returns a list of validation errors (empty = valid).
// Source: utils/settings/schemaOutput.ts:5-8
func ValidateSettingsJSON(data []byte) []SchemaValidationError {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return []SchemaValidationError{{Message: "invalid JSON: " + err.Error()}}
	}
	return validateSettingsMap(raw, "")
}

// validateSettingsMap performs basic type checking against the schema.
// This is a lightweight validator — not a full JSON Schema implementation,
// but catches the most common errors (wrong types, unknown permission modes).
func validateSettingsMap(data map[string]interface{}, prefix string) []SchemaValidationError {
	var errs []SchemaValidationError

	for key, val := range data {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		switch key {
		case "model", "system_prompt", "append_system_prompt", "theme", "api_url", "api_version":
			if _, ok := val.(string); !ok && val != nil {
				errs = append(errs, SchemaValidationError{Path: path, Message: "must be a string"})
			}
		case "max_turns":
			if num, ok := val.(float64); ok {
				if num < 1 {
					errs = append(errs, SchemaValidationError{Path: path, Message: "must be >= 1"})
				}
			} else if val != nil {
				errs = append(errs, SchemaValidationError{Path: path, Message: "must be an integer"})
			}
		case "permission_mode":
			if s, ok := val.(string); ok {
				validModes := map[string]bool{
					"default": true, "acceptEdits": true, "bypassPermissions": true,
					"dontAsk": true, "plan": true, "auto": true,
				}
				if !validModes[s] {
					errs = append(errs, SchemaValidationError{
						Path:    path,
						Message: fmt.Sprintf("invalid mode %q, must be one of: default, acceptEdits, bypassPermissions, dontAsk, plan, auto", s),
					})
				}
			} else if val != nil {
				errs = append(errs, SchemaValidationError{Path: path, Message: "must be a string"})
			}
		case "verbose", "autoDreamEnabled":
			if _, ok := val.(bool); !ok && val != nil {
				errs = append(errs, SchemaValidationError{Path: path, Message: "must be a boolean"})
			}
		case "allowed_tools", "disallowed_tools":
			if arr, ok := val.([]interface{}); ok {
				for i, item := range arr {
					if _, ok := item.(string); !ok {
						errs = append(errs, SchemaValidationError{
							Path:    fmt.Sprintf("%s[%d]", path, i),
							Message: "must be a string",
						})
					}
				}
			} else if val != nil {
				errs = append(errs, SchemaValidationError{Path: path, Message: "must be an array"})
			}
		case "permissions":
			if obj, ok := val.(map[string]interface{}); ok {
				for k, v := range obj {
					if k != "allow" && k != "deny" && k != "ask" {
						continue
					}
					if arr, ok := v.([]interface{}); ok {
						for i, item := range arr {
							if _, ok := item.(string); !ok {
								errs = append(errs, SchemaValidationError{
									Path:    fmt.Sprintf("%s.%s[%d]", path, k, i),
									Message: "must be a string",
								})
							}
						}
					}
				}
			}
		}
	}

	return errs
}

// IsValidPermissionMode checks if a string is a valid permission mode.
func IsValidPermissionMode(mode string) bool {
	switch mode {
	case "default", "acceptEdits", "bypassPermissions", "dontAsk", "plan", "auto":
		return true
	}
	return false
}

// FormatValidationErrors formats errors for display.
func FormatValidationErrors(errs []SchemaValidationError) string {
	if len(errs) == 0 {
		return ""
	}
	var parts []string
	for _, e := range errs {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, "\n")
}
