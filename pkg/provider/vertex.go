package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// VertexProvider implements the ModelProvider interface for Google Vertex AI.
// This is a stub that documents the interface — full OAuth2/ADC authentication
// requires the Google Cloud SDK.
type VertexProvider struct {
	projectID string
	region    string
	modelID   string
	client    *http.Client
}

// NewVertexProvider creates a new VertexProvider for the given Google Cloud
// project, region, and model ID.
func NewVertexProvider(projectID, region, modelID string) *VertexProvider {
	return &VertexProvider{
		projectID: projectID,
		region:    region,
		modelID:   modelID,
		client:    &http.Client{Timeout: 5 * time.Minute},
	}
}

// Name returns the provider name.
func (p *VertexProvider) Name() string { return "vertex" }

// Stream sends a streaming request to the Vertex AI endpoint. Currently
// returns an error because full authentication requires the Google Cloud SDK.
func (p *VertexProvider) Stream(ctx context.Context, req ModelRequest) (<-chan StreamResult, error) {
	// Vertex endpoint: https://{region}-aiplatform.googleapis.com/v1/projects/{project}/locations/{region}/publishers/anthropic/models/{model}:streamRawPredict
	// Uses Google Cloud OAuth2 / Application Default Credentials
	return nil, fmt.Errorf("vertex provider requires Google Cloud credentials (set GOOGLE_APPLICATION_CREDENTIALS)")
}
