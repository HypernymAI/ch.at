package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ch.at/models"
)

// OneAPIProvider handles OpenAI-compatible gateway (one-api)
type OneAPIProvider struct {
	client *http.Client
}

// NewOneAPIProvider creates a new OneAPI provider
func NewOneAPIProvider() *OneAPIProvider {
	return &OneAPIProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TranslateRequest converts unified request to OneAPI format
func (o *OneAPIProvider) TranslateRequest(ctx context.Context, req *UnifiedRequest, deployment *models.Deployment) (*ProviderRequest, error) {
	// For one-api, use the provider:model format from ProviderModelID
	modelName := deployment.ProviderModelID
	
	// Strip provider prefix if present (e.g., "openai:gpt-3.5-turbo" -> "gpt-3.5-turbo")
	// The provider info is already encoded in the API key suffix
	if colonIdx := strings.Index(modelName, ":"); colonIdx != -1 {
		modelName = modelName[colonIdx+1:]
	}

	// Build OpenAI-compatible request body
	body := map[string]interface{}{
		"model":    modelName, // Now just the model name without provider prefix
		"messages": req.Messages,
		"stream":   req.Stream,
	}

	// Add optional parameters
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.TopP > 0 {
		body["top_p"] = req.TopP
	}
	if len(req.Stop) > 0 {
		body["stop"] = req.Stop
	}
	if len(req.Functions) > 0 {
		body["functions"] = req.Functions
	}
	if req.ResponseFormat != nil {
		body["response_format"] = req.ResponseFormat
	}
	if req.User != "" {
		body["user"] = req.User
	}

	// Build headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Add authentication if configured
	if deployment.Endpoint.Auth.Type == models.AuthAPIKey && deployment.Endpoint.Auth.APIKey != "" {
		headers["Authorization"] = "Bearer " + deployment.Endpoint.Auth.APIKey
	}

	// Add custom headers
	for k, v := range deployment.Endpoint.CustomHeaders {
		headers[k] = v
	}

	return &ProviderRequest{
		URL:     deployment.Endpoint.BaseURL + "/v1/chat/completions",
		Method:  "POST",
		Headers: headers,
		Body:    body,
		Timeout: deployment.Endpoint.Timeout,
	}, nil
}

// Execute sends the request to OneAPI
func (o *OneAPIProvider) Execute(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error) {
	// Marshal body to JSON
	jsonBody, err := json.Marshal(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Execute request
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	var body json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract response headers
	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	return &ProviderResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}

// TranslateResponse converts OneAPI response to unified format
func (o *OneAPIProvider) TranslateResponse(ctx context.Context, resp *ProviderResponse, deployment *models.Deployment) (*UnifiedResponse, error) {
	// OneAPI already returns OpenAI-compatible format
	var unifiedResp UnifiedResponse
	if err := json.Unmarshal(resp.Body, &unifiedResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Add metadata
	if unifiedResp.Metadata == nil {
		unifiedResp.Metadata = make(map[string]interface{})
	}
	unifiedResp.Metadata["deployment_id"] = deployment.ID
	unifiedResp.Metadata["provider"] = string(deployment.Provider)
	unifiedResp.Metadata["provider_model"] = deployment.ProviderModelID

	return &unifiedResp, nil
}

// Stream handles streaming responses from OneAPI
func (o *OneAPIProvider) Stream(ctx context.Context, req *ProviderRequest, stream chan<- StreamChunk) error {
	defer close(stream)

	// Ensure streaming is enabled in request
	if body, ok := req.Body.(map[string]interface{}); ok {
		body["stream"] = true
	}

	// Execute streaming request
	jsonBody, err := json.Marshal(req.Body)
	if err != nil {
		stream <- StreamChunk{Error: err}
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewBuffer(jsonBody))
	if err != nil {
		stream <- StreamChunk{Error: err}
		return err
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		stream <- StreamChunk{Error: err}
		return err
	}
	defer resp.Body.Close()

	// Parse SSE stream
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk json.RawMessage
		if err := decoder.Decode(&chunk); err != nil {
			if err.Error() == "EOF" {
				stream <- StreamChunk{Done: true}
				return nil
			}
			stream <- StreamChunk{Error: err}
			return err
		}

		stream <- StreamChunk{
			Data: string(chunk),
			Done: false,
		}
	}
}

// ValidateConfig validates OneAPI deployment configuration
func (o *OneAPIProvider) ValidateConfig(deployment *models.Deployment) error {
	if deployment.Endpoint.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	if deployment.ProviderModelID == "" {
		return fmt.Errorf("provider model ID is required")
	}

	// Validate auth if required
	if deployment.Endpoint.Auth.Type == models.AuthAPIKey && deployment.Endpoint.Auth.APIKey == "" {
		return fmt.Errorf("API key is required but not provided")
	}

	return nil
}

// HealthCheck performs a health check on OneAPI endpoint
func (o *OneAPIProvider) HealthCheck(ctx context.Context, deployment *models.Deployment) error {
	// Create a simple completion request
	req := &UnifiedRequest{
		Model: deployment.ProviderModelID,
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		MaxTokens:   1,
		Temperature: 0,
	}

	// Translate to provider request
	providerReq, err := o.TranslateRequest(ctx, req, deployment)
	if err != nil {
		return fmt.Errorf("health check translation failed: %w", err)
	}

	// Execute request with timeout
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := o.Execute(healthCtx, providerReq)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// GetInfo returns provider information
func (o *OneAPIProvider) GetInfo() ProviderInfo {
	return ProviderInfo{
		Name:           "OneAPI Gateway",
		Version:        "1.0",
		SupportsStream: true,
		RequiresAuth:   true,
		MaxRequestSize: 4 * 1024 * 1024, // 4MB
		RateLimits: map[string]int{
			"requests_per_minute": 1000,
			"tokens_per_minute":   1000000,
		},
	}
}