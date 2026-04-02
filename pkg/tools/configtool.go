package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ConfigTool reads or modifies configuration settings.
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
func (t *ConfigTool) IsReadOnly() bool    { return true }

func (t *ConfigTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["get", "set", "list"], "description": "Action to perform"},
			"key": {"type": "string", "description": "Configuration key (for get/set)"},
			"value": {"type": "string", "description": "Value to set (for set action)"}
		},
		"required": ["action"],
		"additionalProperties": false
	}`)
}

func (t *ConfigTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Action string `json:"action"`
		Key    string `json:"key"`
		Value  string `json:"value"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}

	switch params.Action {
	case "get":
		if params.Key == "" {
			return ErrorOutput("key is required for get action"), nil
		}
		t.mu.RLock()
		val, ok := t.config[params.Key]
		t.mu.RUnlock()
		if !ok {
			return SuccessOutput(fmt.Sprintf("key %q not set", params.Key)), nil
		}
		return SuccessOutput(fmt.Sprintf("%s = %s", params.Key, val)), nil

	case "set":
		if params.Key == "" {
			return ErrorOutput("key is required for set action"), nil
		}
		t.mu.Lock()
		t.config[params.Key] = params.Value
		t.mu.Unlock()
		return SuccessOutput(fmt.Sprintf("set %s = %s", params.Key, params.Value)), nil

	case "list":
		t.mu.RLock()
		defer t.mu.RUnlock()
		if len(t.config) == 0 {
			return SuccessOutput("No configuration settings"), nil
		}
		keys := make([]string, 0, len(t.config))
		for k := range t.config {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var sb strings.Builder
		for i, k := range keys {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(fmt.Sprintf("%s = %s", k, t.config[k]))
		}
		return SuccessOutput(sb.String()), nil

	default:
		return ErrorOutput(fmt.Sprintf("unknown action %q (must be get, set, or list)", params.Action)), nil
	}
}
