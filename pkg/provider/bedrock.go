package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// BedrockProvider implements the ModelProvider interface for AWS Bedrock.
// This is a stub that documents the interface — full AWS Sigv4 signing
// requires the AWS SDK.
type BedrockProvider struct {
	region  string
	modelID string
	client  *http.Client
}

// NewBedrockProvider creates a new BedrockProvider for the given AWS region
// and model ID.
func NewBedrockProvider(region, modelID string) *BedrockProvider {
	return &BedrockProvider{
		region:  region,
		modelID: resolveModel(modelID),
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

// Name returns the provider name.
func (p *BedrockProvider) Name() string { return "bedrock" }

// Stream sends a streaming request to the Bedrock invoke-with-response-stream
// endpoint. Currently returns an error because full Sigv4 signing requires
// the AWS SDK.
func (p *BedrockProvider) Stream(ctx context.Context, req ModelRequest) (<-chan StreamResult, error) {
	// Bedrock endpoint: https://bedrock-runtime.{region}.amazonaws.com/model/{modelID}/invoke-with-response-stream
	// Uses AWS Sigv4 signing
	return nil, fmt.Errorf("bedrock provider requires AWS credentials (set AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION)")
}
