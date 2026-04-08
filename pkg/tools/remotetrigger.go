package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/projectbarks/gopher-code/pkg/auth"
)

// Source: tools/RemoteTriggerTool/RemoteTriggerTool.ts + prompt.ts
//
// Manages scheduled remote Claude Code agents (triggers) via the claude.ai API.
// OAuth token is added automatically — never exposed to the shell.

// RemoteTriggerTool manages scheduled remote agent triggers via the CCR API.
type RemoteTriggerTool struct{}

func (t *RemoteTriggerTool) Name() string { return "RemoteTrigger" }
func (t *RemoteTriggerTool) Description() string {
	return "Manage scheduled remote Claude Code agents (triggers) via the claude.ai CCR API"
}
func (t *RemoteTriggerTool) IsReadOnly() bool   { return false }
func (t *RemoteTriggerTool) SearchHint() string  { return "manage scheduled remote agent triggers" }
func (t *RemoteTriggerTool) MaxResultSizeChars() int { return 100_000 }

// Prompt returns the tool's system prompt.
// Source: tools/RemoteTriggerTool/prompt.ts
func (t *RemoteTriggerTool) Prompt() string {
	return `Call the claude.ai remote-trigger API. Use this instead of curl — the OAuth token is added automatically in-process and never exposed.

Actions:
- list: GET /v1/code/triggers
- get: GET /v1/code/triggers/{trigger_id}
- create: POST /v1/code/triggers (requires body)
- update: POST /v1/code/triggers/{trigger_id} (requires body, partial update)
- run: POST /v1/code/triggers/{trigger_id}/run

The response is the raw JSON from the API.`
}

func (t *RemoteTriggerTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["list", "get", "create", "update", "run"]},
			"trigger_id": {"type": "string", "description": "Required for get, update, and run"},
			"body": {"type": "object", "description": "JSON body for create and update"}
		},
		"required": ["action"],
		"additionalProperties": false
	}`)
}

type remoteTriggerInput struct {
	Action    string         `json:"action"`
	TriggerID string         `json:"trigger_id"`
	Body      map[string]any `json:"body"`
}

const triggersBeta = "ccr-triggers-2026-01-30"

// Execute dispatches the trigger API request.
// Source: RemoteTriggerTool.ts:78-151
func (t *RemoteTriggerTool) Execute(ctx context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params remoteTriggerInput
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}

	cfg, _ := auth.GetOAuthConfig()
	token := auth.GetOAuthToken()
	if token == "" {
		return ErrorOutput("Not authenticated with a claude.ai account. Run /login and try again."), nil
	}

	base := cfg.BaseAPIURL + "/v1/code/triggers"

	var method, url string
	var body []byte

	switch params.Action {
	case "list":
		method, url = "GET", base
	case "get":
		if params.TriggerID == "" {
			return ErrorOutput("get requires trigger_id"), nil
		}
		method, url = "GET", base+"/"+params.TriggerID
	case "create":
		if params.Body == nil {
			return ErrorOutput("create requires body"), nil
		}
		method, url = "POST", base
		body, _ = json.Marshal(params.Body)
	case "update":
		if params.TriggerID == "" {
			return ErrorOutput("update requires trigger_id"), nil
		}
		if params.Body == nil {
			return ErrorOutput("update requires body"), nil
		}
		method, url = "POST", base+"/"+params.TriggerID
		body, _ = json.Marshal(params.Body)
	case "run":
		if params.TriggerID == "" {
			return ErrorOutput("run requires trigger_id"), nil
		}
		method, url = "POST", base+"/"+params.TriggerID+"/run"
		body = []byte("{}")
	default:
		return ErrorOutput(fmt.Sprintf("unknown action: %s", params.Action)), nil
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to create request: %s", err)), nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", triggersBeta)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("API request failed: %s", err)), nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return SuccessOutput(fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(respBody))), nil
}
