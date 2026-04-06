package provider

import (
	"context"
	"fmt"
)

// QueryOptions holds configuration for a model query dispatched through the
// Claude Code infrastructure. It mirrors the TS Options type from
// services/api/claude.ts:676-707.
type QueryOptions struct {
	Model                string
	QuerySource          QuerySource
	EnablePromptCaching  bool
	FallbackModel        string
	MaxOutputTokens      int      // 0 = use model default
	Temperature          *float64 // nil = API default
	FastMode             bool
	EffortValue          EffortLevel
	// TaskBudget is the API-side task budget (output_config.task_budget).
	// Distinct from the tokenBudget auto-continue feature.
	// Source: claude.ts:703-706
	TaskBudget *TaskBudget
}

// TaskBudget represents the API-side task budget configuration.
// Source: claude.ts:703-706
type TaskBudget struct {
	Total     int // Total task budget
	Remaining int // Remaining budget (decremented across agentic loop)
}

// QueryWithModelRequest holds the parameters for a queryWithModel call.
// Source: claude.ts:3300-3312
type QueryWithModelRequest struct {
	SystemPrompt []string // System prompt blocks (text only)
	UserPrompt   string
	Options      QueryOptions
}

// QueryWithModelResult holds the response from a queryWithModel call.
type QueryWithModelResult struct {
	Response *ModelResponse
}

// QueryWithModel dispatches a non-streaming query through the full Claude Code
// infrastructure including proper authentication, betas, and headers.
// Source: claude.ts:3300-3348
func QueryWithModel(ctx context.Context, provider ModelProvider, req QueryWithModelRequest) (*QueryWithModelResult, error) {
	if req.UserPrompt == "" {
		return nil, fmt.Errorf("queryWithModel: userPrompt is required")
	}

	model := req.Options.Model
	if model == "" {
		return nil, fmt.Errorf("queryWithModel: model is required")
	}

	// Resolve model alias to API model ID
	resolvedModel := resolveModel(model)

	maxTokens := req.Options.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = MaxNonStreamingTokens
	}

	// Build system string from blocks
	system := ""
	if len(req.SystemPrompt) > 0 {
		for i, block := range req.SystemPrompt {
			if i > 0 {
				system += "\n"
			}
			system += block
		}
	}

	// Build the single user message
	messages := []RequestMessage{
		{
			Role: "user",
			Content: []RequestContent{
				{Type: "text", Text: req.UserPrompt},
			},
		},
	}

	modelReq := ModelRequest{
		Model:       resolvedModel,
		System:      system,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Tools:       nil, // queryWithModel does not use tools
		Temperature: req.Options.Temperature,
	}

	// Stream and collect the full response
	ch, err := provider.Stream(ctx, modelReq)
	if err != nil {
		return nil, fmt.Errorf("queryWithModel: stream: %w", err)
	}

	var response *ModelResponse
	for result := range ch {
		if result.Err != nil {
			return nil, fmt.Errorf("queryWithModel: stream event: %w", result.Err)
		}
		if result.Event != nil && result.Event.Type == EventMessageDone {
			response = result.Event.Response
		}
	}

	if response == nil {
		return nil, fmt.Errorf("queryWithModel: no response received")
	}

	return &QueryWithModelResult{Response: response}, nil
}

// MaxNonStreamingTokens is the max output tokens for non-streaming requests.
// Non-streaming requests have a 10min max per the docs.
// Source: claude.ts:3354
const MaxNonStreamingTokens = 64_000

// QueryHaikuRequest holds parameters for a side query using the small/fast model.
// Source: claude.ts:3241-3291
type QueryHaikuRequest struct {
	SystemPrompt []string // System prompt blocks
	UserPrompt   string
	QuerySource  QuerySource
	EnablePromptCaching bool
}

// QueryHaiku dispatches a non-streaming side query using the small fast model
// (Haiku) for auxiliary tasks like title generation, summaries, and memory
// relevance scoring. Thinking is always disabled for side queries.
// Source: claude.ts:3241-3291
func QueryHaiku(ctx context.Context, provider ModelProvider, req QueryHaikuRequest) (*QueryWithModelResult, error) {
	model := GetSmallFastModel()
	return QueryWithModel(ctx, provider, QueryWithModelRequest{
		SystemPrompt: req.SystemPrompt,
		UserPrompt:   req.UserPrompt,
		Options: QueryOptions{
			Model:               model,
			QuerySource:         req.QuerySource,
			EnablePromptCaching: req.EnablePromptCaching,
		},
	})
}
