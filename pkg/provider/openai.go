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
	"time"
)

// OpenAICompatProvider implements the ModelProvider interface for any
// OpenAI-compatible chat completions API (OpenAI, Ollama, vLLM, LM Studio, etc.).
type OpenAICompatProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAICompatProvider creates a new OpenAI-compatible provider.
func NewOpenAICompatProvider(baseURL, apiKey, model string) *OpenAICompatProvider {
	return &OpenAICompatProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   resolveModel(model),
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

// Name returns the provider name.
func (p *OpenAICompatProvider) Name() string { return "openai-compat" }

// openAIChatRequest is the JSON body sent to /v1/chat/completions.
type openAIChatRequest struct {
	Model       string             `json:"model"`
	Messages    []openAIMessage    `json:"messages"`
	Stream      bool               `json:"stream"`
	Tools       []openAITool       `json:"tools"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
}

type openAIMessage struct {
	Role       string              `json:"role"`
	Content    string              `json:"content,omitempty"`
	ToolCalls  []openAIToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string           `json:"type"`
	Function openAIFunction   `json:"function"`
}

type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openAIToolCall struct {
	Index    *int                `json:"index,omitempty"`
	ID       string              `json:"id,omitempty"`
	Type     string              `json:"type,omitempty"`
	Function openAIToolCallFunc  `json:"function"`
}

type openAIToolCallFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// openAIChunk is a single chunk from the streaming response.
type openAIChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Choices []openAIChoice    `json:"choices"`
	Usage   *openAIUsage      `json:"usage,omitempty"`
}

type openAIChoice struct {
	Index        int               `json:"index"`
	Delta        openAIDelta       `json:"delta"`
	FinishReason *string           `json:"finish_reason"`
}

type openAIDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   *string          `json:"content,omitempty"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Stream sends a streaming request to an OpenAI-compatible chat completions
// endpoint and returns a channel that yields StreamResult values.
func (p *OpenAICompatProvider) Stream(ctx context.Context, req ModelRequest) (<-chan StreamResult, error) {
	msgs := convertToOpenAIMessages(req)

	body := openAIChatRequest{
		Model:       req.Model,
		Messages:    msgs,
		Stream:      true,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	// Convert tool definitions.
	for _, t := range req.Tools {
		body.Tools = append(body.Tools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, classifyHTTPError(resp.StatusCode, bodyBytes)
	}

	ch := make(chan StreamResult, 16)
	go p.readOpenAIStream(ctx, resp, ch)
	return ch, nil
}

// convertToOpenAIMessages translates Anthropic-style messages to OpenAI format.
func convertToOpenAIMessages(req ModelRequest) []openAIMessage {
	var msgs []openAIMessage

	// Add system message if present.
	if req.System != "" {
		msgs = append(msgs, openAIMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			// Check if this contains tool results.
			var toolResults []RequestContent
			var textParts []string
			for _, c := range m.Content {
				if c.Type == "tool_result" {
					toolResults = append(toolResults, c)
				} else if c.Type == "text" {
					textParts = append(textParts, c.Text)
				}
			}

			if len(toolResults) > 0 {
				// Emit tool result messages.
				for _, tr := range toolResults {
					msgs = append(msgs, openAIMessage{
						Role:       "tool",
						Content:    tr.Content,
						ToolCallID: tr.ToolUseID,
					})
				}
			}
			if len(textParts) > 0 {
				msgs = append(msgs, openAIMessage{
					Role:    "user",
					Content: strings.Join(textParts, "\n"),
				})
			}
			// If it was purely tool_result blocks with no text, we already
			// emitted them above.
			if len(toolResults) == 0 && len(textParts) == 0 {
				// Fallback: concatenate all content text fields
				var all []string
				for _, c := range m.Content {
					if c.Text != "" {
						all = append(all, c.Text)
					}
				}
				msgs = append(msgs, openAIMessage{
					Role:    "user",
					Content: strings.Join(all, "\n"),
				})
			}

		case "assistant":
			var text []string
			var toolCalls []openAIToolCall
			idx := 0
			for _, c := range m.Content {
				switch c.Type {
				case "text":
					text = append(text, c.Text)
				case "tool_use":
					tc := openAIToolCall{
						Index: &idx,
						ID:    c.ID,
						Type:  "function",
						Function: openAIToolCallFunc{
							Name:      c.Name,
							Arguments: string(c.Input),
						},
					}
					toolCalls = append(toolCalls, tc)
					idx++
				}
			}
			msg := openAIMessage{
				Role:    "assistant",
				Content: strings.Join(text, "\n"),
			}
			if len(toolCalls) > 0 {
				msg.ToolCalls = toolCalls
			}
			msgs = append(msgs, msg)
		}
	}

	return msgs
}

// readOpenAIStream reads SSE events from the OpenAI-compatible streaming
// response and translates them into our StreamResult format.
func (p *OpenAICompatProvider) readOpenAIStream(ctx context.Context, resp *http.Response, ch chan<- StreamResult) {
	defer close(ch)
	defer resp.Body.Close()

	var (
		messageID    string
		inputTokens  int
		outputTokens int
		stopReason   *StopReason
		// Content blocks accumulated.
		contentBlocks []ResponseContent
		// Track text assembled across deltas.
		textAccum string
		// Track tool calls being assembled.
		toolCalls    = make(map[int]*openAIToolCall)
		textBlockIdx = 0
		sentTextStart bool
	)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- StreamResult{Err: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openAIChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			sendErr(ctx, ch, fmt.Errorf("parse openai chunk: %w", err))
			continue
		}

		if messageID == "" && chunk.ID != "" {
			messageID = chunk.ID
		}

		if chunk.Usage != nil {
			inputTokens = chunk.Usage.PromptTokens
			outputTokens = chunk.Usage.CompletionTokens
		}

		for _, choice := range chunk.Choices {
			// Handle text content delta.
			if choice.Delta.Content != nil && *choice.Delta.Content != "" {
				if !sentTextStart {
					sendEvent(ctx, ch, StreamEvent{
						Type:    EventContentBlockStart,
						Index:   textBlockIdx,
						Content: &ResponseContent{Type: "text"},
					})
					sentTextStart = true
				}
				textAccum += *choice.Delta.Content
				sendEvent(ctx, ch, StreamEvent{
					Type:  EventTextDelta,
					Index: textBlockIdx,
					Text:  *choice.Delta.Content,
				})
			}

			// Handle tool call deltas.
			for _, tc := range choice.Delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}

				existing, ok := toolCalls[idx]
				if !ok {
					// New tool call — finish the text block first if needed.
					if sentTextStart {
						sendEvent(ctx, ch, StreamEvent{
							Type:  EventContentBlockStop,
							Index: textBlockIdx,
						})
						contentBlocks = append(contentBlocks, ResponseContent{
							Type: "text",
							Text: textAccum,
						})
						textBlockIdx++
						sentTextStart = false
					}

					newTC := &openAIToolCall{
						ID:   tc.ID,
						Type: tc.Type,
						Function: openAIToolCallFunc{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
					toolCalls[idx] = newTC

					blockIdx := textBlockIdx + idx
					sendEvent(ctx, ch, StreamEvent{
						Type:  EventContentBlockStart,
						Index: blockIdx,
						Content: &ResponseContent{
							Type: "tool_use",
							ID:   tc.ID,
							Name: tc.Function.Name,
						},
					})
					if tc.Function.Arguments != "" {
						sendEvent(ctx, ch, StreamEvent{
							Type:        EventInputJsonDelta,
							Index:       blockIdx,
							PartialJSON: tc.Function.Arguments,
						})
					}
				} else {
					// Append to existing tool call.
					existing.Function.Arguments += tc.Function.Arguments
					blockIdx := textBlockIdx + idx
					if tc.Function.Arguments != "" {
						sendEvent(ctx, ch, StreamEvent{
							Type:        EventInputJsonDelta,
							Index:       blockIdx,
							PartialJSON: tc.Function.Arguments,
						})
					}
				}
			}

			// Handle finish reason.
			if choice.FinishReason != nil {
				sr := mapOpenAIFinishReason(*choice.FinishReason)
				stopReason = &sr
			}
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-ctx.Done():
		default:
			ch <- StreamResult{Err: fmt.Errorf("reading openai stream: %w", err)}
		}
	}

	// Close the text block if still open.
	if sentTextStart {
		sendEvent(ctx, ch, StreamEvent{
			Type:  EventContentBlockStop,
			Index: textBlockIdx,
		})
		contentBlocks = append(contentBlocks, ResponseContent{
			Type: "text",
			Text: textAccum,
		})
	}

	// Close tool call blocks and add them to content blocks.
	for idx, tc := range toolCalls {
		blockIdx := textBlockIdx + idx
		if sentTextStart {
			blockIdx++
		}
		sendEvent(ctx, ch, StreamEvent{
			Type:  EventContentBlockStop,
			Index: blockIdx,
		})
		contentBlocks = append(contentBlocks, ResponseContent{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	// Send usage delta.
	sendEvent(ctx, ch, StreamEvent{
		Type: EventUsageDelta,
	})

	// Emit the final MessageDone event.
	finalResp := &ModelResponse{
		ID:         messageID,
		Content:    contentBlocks,
		StopReason: stopReason,
		Usage: Usage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
	}
	sendEvent(ctx, ch, StreamEvent{
		Type:     EventMessageDone,
		Response: finalResp,
	})
}

// mapOpenAIFinishReason converts an OpenAI finish_reason to our StopReason.
func mapOpenAIFinishReason(reason string) StopReason {
	switch reason {
	case "stop":
		return StopReasonEndTurn
	case "tool_calls":
		return StopReasonToolUse
	case "length":
		return StopReasonMaxTokens
	default:
		return StopReason(reason)
	}
}
