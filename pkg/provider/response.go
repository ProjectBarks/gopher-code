package provider

import "encoding/json"

type StopReason string

const (
	StopReasonEndTurn      StopReason = "end_turn"
	StopReasonToolUse      StopReason = "tool_use"
	StopReasonMaxTokens    StopReason = "max_tokens"
	StopReasonStopSequence StopReason = "stop_sequence"
)

type Usage struct {
	InputTokens              int  `json:"input_tokens"`
	OutputTokens             int  `json:"output_tokens"`
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     *int `json:"cache_read_input_tokens,omitempty"`
}

type ResponseContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type ModelResponse struct {
	ID         string            `json:"id"`
	Content    []ResponseContent `json:"content"`
	StopReason *StopReason       `json:"stop_reason,omitempty"`
	Usage      Usage             `json:"usage"`
}

type StreamEventType string

const (
	EventTextDelta         StreamEventType = "text_delta"
	EventContentBlockStart StreamEventType = "content_block_start"
	EventInputJsonDelta    StreamEventType = "input_json_delta"
	EventContentBlockStop  StreamEventType = "content_block_stop"
	EventMessageDone       StreamEventType = "message_done"
	EventUsageDelta        StreamEventType = "usage_delta"
)

type StreamEvent struct {
	Type        StreamEventType
	Index       int
	Text        string           // for TextDelta
	Content     *ResponseContent // for ContentBlockStart
	PartialJSON string           // for InputJsonDelta
	Response    *ModelResponse   // for MessageDone
}

type StreamResult struct {
	Event *StreamEvent
	Err   error
}
