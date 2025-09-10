package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"ch.at/providers"
	"ch.at/routing"
)

// LLMResponse contains the response and metadata from an LLM call
type LLMResponse struct {
	Content         string
	InputTokens     int
	OutputTokens    int
	InputHash       string
	OutputHash      string
	Model           string
	FinishReason    string
	ContentFiltered bool
}

// LLMWithRouter calls the language model using the new routing system
// RouterParams contains all parameters for LLM routing
type RouterParams struct {
	MaxTokens        int
	Temperature      float64
	TopP             float64
	Stop             []string
	FrequencyPenalty float64
	PresencePenalty  float64
}

func LLMWithRouter(input interface{}, requestedModel string, params *RouterParams, stream chan<- string) (*LLMResponse, error) {
	return LLMWithRouterConv(input, requestedModel, "", params, stream)
}

// LLMWithRouterConv calls the language model with conversation tracking
func LLMWithRouterConv(input interface{}, requestedModel string, conversationID string, params *RouterParams, stream chan<- string) (*LLMResponse, error) {
	log.Printf("[LLMWithRouter] Starting routing for model: %s, convID: %s", requestedModel, conversationID)
	log.Printf("[AUDIT] LLMWithRouterConv called with model=%s, convID=%s", requestedModel, conversationID)

	// Build unified request
	var messages []providers.Message
	var fullInput string

	switch v := input.(type) {
	case string:
		fullInput = v
		messages = []providers.Message{
			{Role: "user", Content: v},
		}
	case []map[string]string:
		for _, msg := range v {
			messages = append(messages, providers.Message{
				Role:    msg["role"],
				Content: msg["content"],
			})
			fullInput += msg["role"] + ": " + msg["content"] + "\n"
		}
	default:
		return nil, fmt.Errorf("invalid input type")
	}

	// Apply defaults if not provided
	if params == nil {
		params = &RouterParams{}
	}
	if params.MaxTokens <= 0 {
		params.MaxTokens = 500
	}
	if params.Temperature <= 0 {
		params.Temperature = 0.7
	}
	
	log.Printf("[LLMWithRouter] Using params: MaxTokens=%d, Temperature=%f, TopP=%f", 
		params.MaxTokens, params.Temperature, params.TopP)
	
	// Create unified request
	unifiedReq := &providers.UnifiedRequest{
		Model:       requestedModel,
		Messages:    messages,
		Temperature: params.Temperature,
		MaxTokens:   params.MaxTokens,
		TopP:        params.TopP,
		Stop:        params.Stop,
		Stream:      stream != nil,
	}

	// Create request context
	reqCtx := &routing.RequestContext{
		RequestID: fmt.Sprintf("req_%d", time.Now().UnixNano()),
		ModelID:   requestedModel,
	}

	// Get routing decision
	decision, err := modelRouter.RouteRequest(context.Background(), requestedModel, reqCtx)
	if err != nil {
		// NO FALLBACK! FAIL PROPERLY!
		log.Printf("[LLMWithRouter] Routing failed for model %s: %v", requestedModel, err)
		
		// Log the failure to audit
		LogLLMInteraction(
			conversationID,
			requestedModel,
			"FAILED",
			"NONE",
			input,
			"",
			0,
			0,
			err,
		)
		
		// RETURN THE ERROR - DON'T SILENTLY USE WRONG MODEL!
		return nil, fmt.Errorf("model '%s' not found in routing system: %v", requestedModel, err)
	}

	log.Printf("[LLMWithRouter] Selected deployment: %s (provider: %s, model: %s)",
		decision.Primary.ID,
		decision.Primary.Provider,
		decision.Primary.ProviderModelID)

	// Create response object
	response := &LLMResponse{
		Model:       requestedModel,
		InputHash:   generateSignature(fullInput),
		InputTokens: countTokens(fullInput, requestedModel),
	}

	// Beacon LLM request start
	beacon("llm_request_start", map[string]interface{}{
		"model":        requestedModel,
		"deployment":   decision.Primary.ID,
		"provider":     string(decision.Primary.Provider),
		"streaming":    stream != nil,
		"input_hash":   response.InputHash,
		"input_tokens": response.InputTokens,
	})

	// Handle streaming if requested
	if stream != nil {
		defer close(stream)
		err = handleStreamingWithRouter(unifiedReq, decision, stream, response)
	} else {
		// Execute non-streaming request
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		unifiedResp, err := modelRouter.ExecuteRequest(ctx, unifiedReq, decision)
		if err != nil {
			beacon("llm_error", map[string]interface{}{
				"type":       "routing_error",
				"error":      err.Error(),
				"model":      requestedModel,
				"deployment": decision.Primary.ID,
			})
			return nil, err
		}

		// Extract content from response
		log.Printf("[LLMWithRouter] Response has %d choices", len(unifiedResp.Choices))
		if len(unifiedResp.Choices) > 0 {
			response.Content = unifiedResp.Choices[0].Message.Content
			log.Printf("[LLMWithRouter] Extracted content: %q (length: %d)", response.Content, len(response.Content))
			response.OutputHash = generateSignature(response.Content)
			response.FinishReason = unifiedResp.Choices[0].FinishReason
		} else {
			log.Printf("[LLMWithRouter] WARNING: No choices in response!")
		}

		// Use token counts from response if available
		if unifiedResp.Usage.CompletionTokens > 0 {
			response.OutputTokens = unifiedResp.Usage.CompletionTokens
		} else {
			response.OutputTokens = countTokens(response.Content, requestedModel)
		}
	}

	// Beacon LLM request complete
	beacon("llm_request_complete", map[string]interface{}{
		"model":            requestedModel,
		"deployment":       decision.Primary.ID,
		"provider":         string(decision.Primary.Provider),
		"success":          true,
		"streaming":        stream != nil,
		"input_hash":       response.InputHash,
		"output_hash":      response.OutputHash,
		"input_tokens":     response.InputTokens,
		"output_tokens":    response.OutputTokens,
		"total_tokens":     response.InputTokens + response.OutputTokens,
		"finish_reason":    response.FinishReason,
		"content_filtered": response.ContentFiltered,
	})

	// LOG TO AUDIT DATABASE
	LogLLMInteraction(
		conversationID,
		requestedModel,
		decision.Primary.ID,
		string(decision.Primary.Provider),
		input,
		response.Content,
		response.InputTokens,
		response.OutputTokens,
		err,
	)

	return response, err
}

// handleStreamingWithRouter handles streaming responses through the router
func handleStreamingWithRouter(req *providers.UnifiedRequest, decision *routing.RoutingDecision, stream chan<- string, response *LLMResponse) error {
	// Get provider for the deployment
	provider, exists := modelRouter.Providers[decision.Primary.Provider]
	if !exists {
		return fmt.Errorf("provider not found: %s", decision.Primary.Provider)
	}

	// Translate request
	ctx := context.Background()
	providerReq, err := provider.TranslateRequest(ctx, req, decision.Primary)
	if err != nil {
		return fmt.Errorf("failed to translate request: %w", err)
	}

	// Create stream channel for provider
	providerStream := make(chan providers.StreamChunk)
	
	// Start streaming from provider
	go func() {
		err := provider.Stream(ctx, providerReq, providerStream)
		if err != nil {
			log.Printf("[LLMWithRouter] Stream error: %v", err)
		}
	}()

	// Process stream chunks
	var outputBuilder strings.Builder
	for chunk := range providerStream {
		if chunk.Error != nil {
			return chunk.Error
		}
		
		if chunk.Done {
			break
		}

		// Extract content from chunk (simplified - would need proper parsing)
		stream <- chunk.Data
		outputBuilder.WriteString(chunk.Data)
	}

	// Update response with final content
	response.Content = outputBuilder.String()
	response.OutputHash = generateSignature(response.Content)
	response.OutputTokens = countTokens(response.Content, req.Model)

	return nil
}

// UpdateLLMFunction updates the global LLM function to use routing if available
func UpdateLLMFunction() {
	// Check if router is initialized
	if modelRouter != nil && modelRegistry != nil && deploymentRegistry != nil {
		// We have routing available
		log.Println("[UpdateLLMFunction] Model routing system is available")
		
		// Note: In production, you would replace the LLM function
		// For now, we keep both and let handleChatCompletions decide
	} else {
		log.Println("[UpdateLLMFunction] Model routing system not initialized, using legacy LLM")
	}
}