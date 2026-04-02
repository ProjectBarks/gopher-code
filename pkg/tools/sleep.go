package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// SleepTool pauses execution for a specified duration.
type SleepTool struct{}

func (t *SleepTool) Name() string        { return "Sleep" }
func (t *SleepTool) Description() string { return "Wait for a specified duration" }
func (t *SleepTool) IsReadOnly() bool    { return true }

func (t *SleepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"duration_ms": {"type": "integer", "description": "Duration to sleep in milliseconds"}
		},
		"required": ["duration_ms"],
		"additionalProperties": false
	}`)
}

func (t *SleepTool) Execute(ctx context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		DurationMS int `json:"duration_ms"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.DurationMS <= 0 {
		params.DurationMS = 1000
	}
	if params.DurationMS > 300000 {
		params.DurationMS = 300000 // max 5 minutes
	}

	select {
	case <-time.After(time.Duration(params.DurationMS) * time.Millisecond):
		return SuccessOutput(fmt.Sprintf("Slept for %dms", params.DurationMS)), nil
	case <-ctx.Done():
		return ErrorOutput("sleep interrupted"), nil
	}
}
