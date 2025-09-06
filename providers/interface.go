package providers

import (
	"context"
	"encoding/json"
	"time"

	"ch.at/models"
)

// Provider interface for all LLM providers
type Provider interface {
	// Translate request to provider format
	TranslateRequest(ctx context.Context, req *UnifiedRequest, deployment *models.Deployment) (*ProviderRequest, error)

	// Execute request
	Execute(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)

	// Translate response to unified format
	TranslateResponse(ctx context.Context, resp *ProviderResponse, deployment *models.Deployment) (*UnifiedResponse, error)

	// Stream response
	Stream(ctx context.Context, req *ProviderRequest, stream chan<- StreamChunk) error

	// Validate deployment configuration
	ValidateConfig(deployment *models.Deployment) error

	// Health check
	HealthCheck(ctx context.Context, deployment *models.Deployment) error

	// Get provider info
	GetInfo() ProviderInfo
}

// UnifiedRequest is the standard request format
type UnifiedRequest struct {
	Model          string                 `json:"model"`
	Messages       []Message              `json:"messages"`
	Temperature    float64                `json:"temperature,omitempty"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	TopP           float64                `json:"top_p,omitempty"`
	Stream         bool                   `json:"stream,omitempty"`
	Stop           []string               `json:"stop,omitempty"`
	Functions      []Function             `json:"functions,omitempty"`
	ResponseFormat *ResponseFormat        `json:"response_format,omitempty"`
	User           string                 `json:"user,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// Function represents a function that can be called
type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ResponseFormat specifies the format of the response
type ResponseFormat struct {
	Type string `json:"type"` // "text" or "json_object"
}

// UnifiedResponse is the standard response format
type UnifiedResponse struct {
	ID       string                 `json:"id"`
	Object   string                 `json:"object"`
	Created  int64                  `json:"created"`
	Model    string                 `json:"model"`
	Choices  []Choice               `json:"choices"`
	Usage    Usage                  `json:"usage"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Choice represents a response choice
type Choice struct {
	Index        int      `json:"index"`
	Message      Message  `json:"message"`
	FinishReason string   `json:"finish_reason"`
	Delta        *Message `json:"delta,omitempty"` // For streaming
}

// Usage tracks token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ProviderRequest is the request to send to the provider
type ProviderRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    interface{}
	Timeout time.Duration
}

// ProviderResponse is the response from the provider
type ProviderResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       json.RawMessage
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	Data  string
	Error error
	Done  bool
}

// ProviderInfo contains provider metadata
type ProviderInfo struct {
	Name            string
	Version         string
	SupportsStream  bool
	RequiresAuth    bool
	MaxRequestSize  int
	RateLimits      map[string]int
}