package provider

import "context"

// ModelProvider is the interface every LLM backend must implement.
type ModelProvider interface {
	// Stream sends a request and returns a channel of StreamResults.
	// The channel is closed when the stream ends.
	Stream(ctx context.Context, req ModelRequest) (<-chan StreamResult, error)
	// Name returns a human-readable provider name.
	Name() string
}
