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

// LLMWithRouter calls the language model using the new routing system
func LLMWithRouter(input interface{}, requestedModel string, stream chan<- string) (*LLMResponse, error) {
	log.Printf("[LLMWithRouter] Starting routing for model: %s", requestedModel)

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

	// Create unified request
	unifiedReq := &providers.UnifiedRequest{
		Model:       requestedModel,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   500,
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
		// Fallback to old LLM function if routing fails
		log.Printf("[LLMWithRouter] Routing failed, falling back to legacy: %v", err)
		return LLM(input, stream)
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
		if len(unifiedResp.Choices) > 0 {
			response.Content = unifiedResp.Choices[0].Message.Content
			response.OutputHash = generateSignature(response.Content)
			response.FinishReason = unifiedResp.Choices[0].FinishReason
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