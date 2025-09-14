package providers_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	
	"ch.at/models"
	"ch.at/providers"
	"github.com/joho/godotenv"
)

func init() {
	// Load .env file from parent directory (where ch.at is)
	envPath := filepath.Join("..", ".env")
	if err := godotenv.Load(envPath); err != nil {
		// Try current directory as fallback
		godotenv.Load(".env")
	}
}

func TestOneAPIStreamingSSEFormat(t *testing.T) {
	// Create OneAPI provider
	provider := providers.NewOneAPIProvider()
	
	// Get API key from environment
	apiKey := os.Getenv("ONE_API_KEY")
	if apiKey == "" {
		// Fallback to the base OneAPI key
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		t.Skip("No API key found in environment. Set ONE_API_KEY or OPENAI_API_KEY in .env file")
	}
	
	// Get base URL from environment or use default
	baseURL := os.Getenv("ONE_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}
	
	// Create a test deployment
	deployment := &models.Deployment{
		ID:              "test-llama",
		ModelID:         "llama-8b",
		Provider:        models.ProviderOneAPI,
		ProviderModelID: "llama-3-8b",
		Endpoint: models.Endpoint{
			BaseURL: baseURL,
			Auth: models.Auth{
				Type:   models.AuthAPIKey,
				APIKey: apiKey,
			},
		},
		Status: models.DeploymentStatus{
			Available: true,
		},
	}
	
	// Create test request
	req := &providers.UnifiedRequest{
		Model: "llama-3-8b",
		Messages: []providers.Message{
			{Role: "user", Content: "Say hello in 3 words"},
		},
		Stream:      true,
		MaxTokens:   10,
		Temperature: 0,
	}
	
	// Translate to provider request
	providerReq, err := provider.TranslateRequest(context.Background(), req, deployment)
	if err != nil {
		t.Fatalf("Failed to translate request: %v", err)
	}
	
	// Create stream channel
	stream := make(chan providers.StreamChunk)
	
	// Start streaming
	go func() {
		err := provider.Stream(context.Background(), providerReq, stream)
		if err != nil {
			t.Errorf("Stream error: %v", err)
		}
	}()
	
	// Collect chunks
	var content string
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("Chunk error: %v", chunk.Error)
		}
		
		if chunk.Done {
			break
		}
		
		// The chunk.Data should be raw JSON from SSE
		fmt.Printf("Chunk received: %s\n", chunk.Data)
		content += chunk.Data
	}
	
	if content == "" {
		t.Fatal("No content received from stream")
	}
	
	fmt.Printf("Total content collected: %d bytes\n", len(content))
	t.Log("Test passed - SSE streaming works")
}