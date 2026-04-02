package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	defaultAnthropicBaseURL = "https://api.anthropic.com"
	anthropicVersion        = "2023-06-01"
)

// resolveModel converts model aliases to full model IDs using the full model system.
// Source: model/model.ts:445-506
func resolveModel(model string) string {
	return NormalizeModelStringForAPI(ParseUserSpecifiedModel(model))
}

// AnthropicProvider implements the ModelProvider interface for the Anthropic
// Messages API with SSE streaming support.
type AnthropicProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	model      string
}

// NewAnthropicProvider creates a new AnthropicProvider with the given API key
// and model name.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:     apiKey,
		baseURL:    defaultAnthropicBaseURL,
		httpClient: &http.Client{},
		model:      resolveModel(model),
	}
}

// SetBaseURL overrides the default Anthropic API base URL.
func (p *AnthropicProvider) SetBaseURL(url string) {
	p.baseURL = url
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// apiRequest is the JSON body sent to the Anthropic Messages API.
type apiRequest struct {
	Model       string           `json:"model"`
	MaxTokens   int              `json:"max_tokens"`
	System      string           `json:"system,omitempty"`
	Messages    []RequestMessage `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Stream      bool             `json:"stream"`
	Temperature *float64         `json:"temperature,omitempty"`
}

// SSE event JSON structures for unmarshaling.

type sseMessageStart struct {
	Type    string `json:"type"`
	Message struct {
		ID         string  `json:"id"`
		Role       string  `json:"role"`
		Model      string  `json:"model"`
		Usage      Usage   `json:"usage"`
		StopReason *string `json:"stop_reason"`
	} `json:"message"`
}

type sseContentBlockStart struct {
	Type         string          `json:"type"`
	Index        int             `json:"index"`
	ContentBlock json.RawMessage `json:"content_block"`
}

type sseContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta"`
}

type sseContentBlockStop struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

type sseMessageDelta struct {
	Type  string `json:"type"`
	Delta struct {
		StopReason   *string `json:"stop_reason"`
		StopSequence *string `json:"stop_sequence"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type sseError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// contentBlockJSON is used to parse the content_block field in
// content_block_start events.
type contentBlockJSON struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// Stream sends a streaming request to the Anthropic Messages API and returns
// a channel that yields StreamResult values as they arrive via SSE.
func (p *AnthropicProvider) Stream(ctx context.Context, req ModelRequest) (<-chan StreamResult, error) {
	body := apiRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		System:      req.System,
		Messages:    req.Messages,
		Tools:       req.Tools,
		Stream:      true,
		Temperature: req.Temperature,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("content-type", "application/json")

	// Add beta headers — Source: utils/betas.ts:397-428
	if betas := GetMergedBetas(req.Model, true); len(betas) > 0 {
		httpReq.Header.Set("anthropic-beta", strings.Join(betas, ","))
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		retryAfter := resp.Header.Get("Retry-After")
		return nil, ClassifyHTTPError(resp.StatusCode, bodyBytes, retryAfter)
	}

	ch := make(chan StreamResult, 16)

	go p.readSSEStream(ctx, resp, ch)

	return ch, nil
}

// classifyHTTPError is the legacy error classifier, now delegating to ClassifyHTTPError.
func classifyHTTPError(statusCode int, body []byte) error {
	return ClassifyHTTPError(statusCode, body, "")
}

// readSSEStream reads SSE events from the HTTP response body and sends
// StreamResults on the channel. It closes the channel and the response body
// when done.
func (p *AnthropicProvider) readSSEStream(ctx context.Context, resp *http.Response, ch chan<- StreamResult) {
	defer close(ch)
	defer resp.Body.Close()

	// State accumulated across events for building the final ModelResponse.
	var (
		messageID   string
		model       string
		inputTokens int
		stopReason  *StopReason
		outputTokens int
		contentBlocks []ResponseContent
		// Track content blocks being built from deltas.
		blockTexts     = make(map[int]string)
		blockToolJSON  = make(map[int]string)
		blockMeta      = make(map[int]contentBlockJSON) // type, id, name
	)

	scanner := bufio.NewScanner(resp.Body)
	var currentEventType string

	for scanner.Scan() {
		// Check for context cancellation.
		select {
		case <-ctx.Done():
			ch <- StreamResult{Err: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "event:") {
			currentEventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			p.handleSSEData(ctx, currentEventType, data, ch,
				&messageID, &model, &inputTokens, &stopReason, &outputTokens,
				&contentBlocks, blockTexts, blockToolJSON, blockMeta)
			continue
		}

		// Empty line = event boundary; reset event type.
		if line == "" {
			currentEventType = ""
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-ctx.Done():
		default:
			ch <- StreamResult{Err: fmt.Errorf("reading SSE stream: %w", err)}
		}
	}
}

// handleSSEData processes a single SSE data payload.
func (p *AnthropicProvider) handleSSEData(
	ctx context.Context,
	eventType string,
	data string,
	ch chan<- StreamResult,
	messageID *string,
	model *string,
	inputTokens *int,
	stopReason **StopReason,
	outputTokens *int,
	contentBlocks *[]ResponseContent,
	blockTexts map[int]string,
	blockToolJSON map[int]string,
	blockMeta map[int]contentBlockJSON,
) {
	// Determine the event type from either the SSE event: line or the data
	// JSON "type" field. When no event: line was set, parse the type from JSON.
	resolvedType := eventType
	if resolvedType == "" {
		var probe struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &probe); err == nil {
			resolvedType = probe.Type
		}
	}

	switch resolvedType {
	case "message_start":
		var evt sseMessageStart
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			sendErr(ctx, ch, fmt.Errorf("parse message_start: %w", err))
			return
		}
		*messageID = evt.Message.ID
		*model = evt.Message.Model
		*inputTokens = evt.Message.Usage.InputTokens

	case "content_block_start":
		var evt sseContentBlockStart
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			sendErr(ctx, ch, fmt.Errorf("parse content_block_start: %w", err))
			return
		}
		var cb contentBlockJSON
		if err := json.Unmarshal(evt.ContentBlock, &cb); err != nil {
			sendErr(ctx, ch, fmt.Errorf("parse content_block: %w", err))
			return
		}
		blockMeta[evt.Index] = cb

		rc := &ResponseContent{Type: cb.Type}
		if cb.Type == "text" {
			rc.Text = cb.Text
		} else if cb.Type == "tool_use" {
			rc.ID = cb.ID
			rc.Name = cb.Name
		}

		sendEvent(ctx, ch, StreamEvent{
			Type:    EventContentBlockStart,
			Index:   evt.Index,
			Content: rc,
		})

	case "content_block_delta":
		var evt sseContentBlockDelta
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			sendErr(ctx, ch, fmt.Errorf("parse content_block_delta: %w", err))
			return
		}

		switch evt.Delta.Type {
		case "text_delta":
			blockTexts[evt.Index] += evt.Delta.Text
			sendEvent(ctx, ch, StreamEvent{
				Type:  EventTextDelta,
				Index: evt.Index,
				Text:  evt.Delta.Text,
			})
		case "input_json_delta":
			blockToolJSON[evt.Index] += evt.Delta.PartialJSON
			sendEvent(ctx, ch, StreamEvent{
				Type:        EventInputJsonDelta,
				Index:       evt.Index,
				PartialJSON: evt.Delta.PartialJSON,
			})
		}

	case "content_block_stop":
		var evt sseContentBlockStop
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			sendErr(ctx, ch, fmt.Errorf("parse content_block_stop: %w", err))
			return
		}

		// Finalize the content block.
		meta := blockMeta[evt.Index]
		rc := ResponseContent{Type: meta.Type}
		switch meta.Type {
		case "text":
			rc.Text = blockTexts[evt.Index]
		case "tool_use":
			rc.ID = meta.ID
			rc.Name = meta.Name
			rc.Input = json.RawMessage(blockToolJSON[evt.Index])
		}
		*contentBlocks = append(*contentBlocks, rc)

		sendEvent(ctx, ch, StreamEvent{
			Type:  EventContentBlockStop,
			Index: evt.Index,
		})

	case "message_delta":
		var evt sseMessageDelta
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			sendErr(ctx, ch, fmt.Errorf("parse message_delta: %w", err))
			return
		}
		if evt.Delta.StopReason != nil {
			sr := StopReason(*evt.Delta.StopReason)
			*stopReason = &sr
		}
		*outputTokens = evt.Usage.OutputTokens

		sendEvent(ctx, ch, StreamEvent{
			Type: EventUsageDelta,
		})

	case "message_stop":
		resp := &ModelResponse{
			ID:         *messageID,
			Content:    *contentBlocks,
			StopReason: *stopReason,
			Usage: Usage{
				InputTokens:  *inputTokens,
				OutputTokens: *outputTokens,
			},
		}
		sendEvent(ctx, ch, StreamEvent{
			Type:     EventMessageDone,
			Response: resp,
		})

	case "ping":
		// Skip ping events.

	case "error":
		var evt sseError
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			sendErr(ctx, ch, fmt.Errorf("parse error event: %w", err))
			return
		}
		sendErr(ctx, ch, fmt.Errorf("stream error (%s): %s", evt.Error.Type, evt.Error.Message))

	default:
		// Unknown event type; skip.
	}
}

// sendEvent sends a StreamEvent on the channel, respecting context cancellation.
func sendEvent(ctx context.Context, ch chan<- StreamResult, evt StreamEvent) {
	select {
	case <-ctx.Done():
	case ch <- StreamResult{Event: &evt}:
	}
}

// sendErr sends an error on the channel, respecting context cancellation.
func sendErr(ctx context.Context, ch chan<- StreamResult, err error) {
	select {
	case <-ctx.Done():
	case ch <- StreamResult{Err: err}:
	}
}
