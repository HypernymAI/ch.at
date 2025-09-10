package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"ch.at/models"
)

// BaselineOpenAICompatibilityProvider handles direct OpenAI-compatible API calls
// This provider assumes the BaseURL is the COMPLETE endpoint (no path appending)
type BaselineOpenAICompatibilityProvider struct {
	client *http.Client
}

// NewBaselineOpenAICompatibilityProvider creates a new baseline provider
func NewBaselineOpenAICompatibilityProvider() *BaselineOpenAICompatibilityProvider {
	return &BaselineOpenAICompatibilityProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TranslateRequest converts unified request to OpenAI format
func (b *BaselineOpenAICompatibilityProvider) TranslateRequest(ctx context.Context, req *UnifiedRequest, deployment *models.Deployment) (*ProviderRequest, error) {
	// Build OpenAI-compatible request body
	body := map[string]interface{}{
		"model":    deployment.ProviderModelID,
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

	// Build headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Add authentication if configured
	if deployment.Endpoint.Auth.Type == models.AuthAPIKey && deployment.Endpoint.Auth.APIKey != "" {
		headers["Authorization"] = "Bearer " + deployment.Endpoint.Auth.APIKey
	}

	// CRITICAL DIFFERENCE: Use BaseURL AS-IS (it's already the complete endpoint)
	// Don't append /v1/chat/completions like OneAPIProvider does
	return &ProviderRequest{
		URL:     deployment.Endpoint.BaseURL,  // URL is already complete!
		Method:  "POST",
		Headers: headers,
		Body:    body,
		Timeout: deployment.Endpoint.Timeout,
	}, nil
}

// Execute sends the request to the API
func (b *BaselineOpenAICompatibilityProvider) Execute(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error) {
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
	resp, err := b.client.Do(httpReq)
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

// TranslateResponse converts API response to unified format
func (b *BaselineOpenAICompatibilityProvider) TranslateResponse(ctx context.Context, resp *ProviderResponse, deployment *models.Deployment) (*UnifiedResponse, error) {
	// Debug log the raw response
	log.Printf("[BaselineOpenAI] Raw response for %s: %s", deployment.ProviderModelID, string(resp.Body))
	
	// Parse OpenAI-compatible response format
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
	unifiedResp.Metadata["baseline_mode"] = true

	return &unifiedResp, nil
}

// Stream handles streaming responses
func (b *BaselineOpenAICompatibilityProvider) Stream(ctx context.Context, req *ProviderRequest, stream chan<- StreamChunk) error {
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

	resp, err := b.client.Do(httpReq)
	if err != nil {
		stream <- StreamChunk{Error: err}
		return err
	}
	defer resp.Body.Close()

	// Parse SSE stream (Server-Sent Events format)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		
		// SSE lines start with "data: "
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			
			// Check for stream end
			if data == "[DONE]" {
				stream <- StreamChunk{Done: true}
				return nil
			}
			
			// Parse the JSON to extract content
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// Skip malformed JSON
				continue
			}
			
			// Extract the actual content from the chunk
			// Format: {"choices": [{"delta": {"content": "text"}}]}
			if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok && content != "" {
							// Send ONLY the actual text content
							stream <- StreamChunk{
								Data: content,
								Done: false,
							}
						}
					}
				}
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		stream <- StreamChunk{Error: err}
		return err
	}
	
	return nil
}

// ValidateConfig validates deployment configuration
func (b *BaselineOpenAICompatibilityProvider) ValidateConfig(deployment *models.Deployment) error {
	if deployment.Endpoint.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	// For baseline mode, the URL should be complete (include /v1/chat/completions or equivalent)
	if !strings.Contains(deployment.Endpoint.BaseURL, "/") || deployment.Endpoint.BaseURL == "http://" || deployment.Endpoint.BaseURL == "https://" {
		return fmt.Errorf("baseline mode requires complete endpoint URL (e.g., https://api.openai.com/v1/chat/completions)")
	}

	if deployment.ProviderModelID == "" {
		return fmt.Errorf("provider model ID is required")
	}

	// Validate auth if required (but allow empty for local models)
	if deployment.Endpoint.Auth.Type == models.AuthAPIKey && deployment.Endpoint.Auth.APIKey == "" {
		log.Printf("[BaselineOpenAI] Warning: API key is empty (OK for local models)")
	}

	return nil
}

// HealthCheck performs a health check
func (b *BaselineOpenAICompatibilityProvider) HealthCheck(ctx context.Context, deployment *models.Deployment) error {
	// Create a simple completion request
	req := &UnifiedRequest{
		Model: deployment.ProviderModelID,
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		MaxTokens:   10,
		Temperature: 0,
	}

	// Translate to provider request
	providerReq, err := b.TranslateRequest(ctx, req, deployment)
	if err != nil {
		return fmt.Errorf("health check translation failed: %w", err)
	}

	// Execute request with timeout
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := b.Execute(healthCtx, providerReq)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// GetInfo returns provider information
func (b *BaselineOpenAICompatibilityProvider) GetInfo() ProviderInfo {
	return ProviderInfo{
		Name:           "Baseline OpenAI Compatibility",
		Version:        "1.0",
		SupportsStream: true,
		RequiresAuth:   false, // May work without auth for local models
		MaxRequestSize: 4 * 1024 * 1024, // 4MB
		RateLimits: map[string]int{
			"requests_per_minute": 100,    // Conservative default
			"tokens_per_minute":   100000, // Conservative default
		},
	}
}