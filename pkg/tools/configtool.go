package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ConfigTool reads or modifies configuration settings.
// Source: tools/ConfigTool/ConfigTool.ts
type ConfigTool struct {
	mu     sync.RWMutex
	config map[string]string
}

// NewConfigTool creates a ConfigTool with an empty in-memory config.
func NewConfigTool() *ConfigTool {
	return &ConfigTool{config: make(map[string]string)}
}

func (t *ConfigTool) Name() string        { return "Config" }
func (t *ConfigTool) Description() string { return "Read or modify configuration settings" }

// IsReadOnly depends on whether value is provided.
// Source: ConfigTool.ts:90-92
func (t *ConfigTool) IsReadOnly() bool { return true }

// Source: ConfigTool.ts:36-48
func (t *ConfigTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"setting": {
				"type": "string",
				"description": "The setting key (e.g., \"theme\", \"model\", \"verbose\")"
			},
			"value": {
				"description": "The new value. Omit to get current value."
			}
		},
		"required": ["setting"],
		"additionalProperties": false
	}`)
}

func (t *ConfigTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Setting string      `json:"setting"`
		Value   interface{} `json:"value,omitempty"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Setting == "" {
		return ErrorOutput("setting is required"), nil
	}

	// Get mode: value not provided
	// Source: ConfigTool.ts:111
	if params.Value == nil {
		t.mu.RLock()
		val, ok := t.config[params.Setting]
		t.mu.RUnlock()
		if !ok {
			return SuccessOutput(fmt.Sprintf("%s is not set", params.Setting)), nil
		}
		return SuccessOutput(fmt.Sprintf("%s = %s", params.Setting, val)), nil
	}

	// Set mode: value provided
	valStr := fmt.Sprintf("%v", params.Value)
	t.mu.Lock()
	t.config[params.Setting] = valStr
	t.mu.Unlock()
	return SuccessOutput(fmt.Sprintf("Set %s = %s", params.Setting, valStr)), nil
}
