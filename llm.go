package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

var (
	apiKey    string
	apiURL    string
	modelName string
)

func init() {
	if debugMode {
		log.Println("[LLM init] Running LLM init")
	}
	// Force reload from env in case they weren't set yet
	loadLLMConfig()
	if debugMode {
		log.Printf("[LLM init] After loadLLMConfig - apiURL: %s", apiURL)
	}
}

func loadLLMConfig() {
	apiKey = os.Getenv("OPENAI_API_KEY")
	apiURL = os.Getenv("API_URL")
	modelName = os.Getenv("MODEL_NAME")
	
	if debugMode {
		log.Printf("[loadLLMConfig] Loading from env - apiKey exists: %v, apiURL: '%s', modelName: '%s'", len(apiKey) > 0, apiURL, modelName)
		log.Printf("[loadLLMConfig] Variables set - &apiURL: %p, &modelName: %p", &apiURL, &modelName)
	}
}

// LLM calls the language model. If stream is nil, returns complete response via return value.
// If stream is provided, streams response chunks to channel and returns empty string.
// Input can be a string (wrapped as user message) or []map[string]string for full message history.
func LLM(input interface{}, stream chan<- string) (string, error) {

// generateSignature creates a hash signature for content
func generateSignature(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)[:16] // First 16 chars of hash
}

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

// LLM calls the language model. If stream is nil, returns complete response via return value.
// If stream is provided, streams response chunks to channel and returns empty string.
// Input can be a string (wrapped as user message) or []map[string]string for full message history.
func LLM(input interface{}, stream chan<- string) (*LLMResponse, error) {
	if debugMode {
		log.Printf("[LLM] === LLM FUNCTION CALLED ===")
		log.Printf("[LLM] Input type: %T", input)
		log.Printf("[LLM] Initial state - apiURL: '%s', modelName: '%s', apiKey exists: %v", apiURL, modelName, len(apiKey) > 0)
	}
	
	// Ensure config is loaded
	if apiURL == "" {
		if debugMode {
			log.Printf("[LLM] WARNING: apiURL is empty, reloading config...")
		}
		loadLLMConfig()
		if debugMode {
			log.Printf("[LLM] After reload - apiURL: '%s', modelName: '%s'", apiURL, modelName)
		}
	}
	
	// Build messages array
	var messages []map[string]string
	switch v := input.(type) {
	case string:
		messages = []map[string]string{
			{"role": "user", "content": v},
		}
	case []map[string]string:
		messages = v
	default:
		return "", fmt.Errorf("invalid input type")
	}

	// Build request
	requestBody := map[string]interface{}{
		"model":       modelName,
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  500,
	}

	if stream != nil {
		requestBody["stream"] = true
		defer close(stream)
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	// Double-check apiURL is set
	if apiURL == "" {
		if debugMode {
			log.Printf("[LLM] CRITICAL: apiURL is still empty before request!")
			log.Printf("[LLM] Attempting emergency reload...")
		}
		loadLLMConfig()
		if debugMode {
			log.Printf("[LLM] After emergency reload - apiURL: '%s'", apiURL)
		}
		if apiURL == "" {
			log.Printf("[LLM] FATAL: apiURL still empty after reload")
			return nil, fmt.Errorf("API URL not configured")
		}
	}
	
	if debugMode {
		log.Printf("[LLM] Creating HTTP request to: '%s'", apiURL)
		log.Printf("[LLM] Request body size: %d bytes", len(jsonBody))
	}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		if debugMode {
			log.Printf("[LLM] Failed to create request: %v", err)
		}
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Handle streaming response
	if stream != nil {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					return "", nil
				}

				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(data), &chunk); err == nil {
					if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if delta, ok := choice["delta"].(map[string]interface{}); ok {
								if content, ok := delta["content"].(string); ok {
									stream <- content
								}
							}
						}
					}
				}
			}
		}
		return "", scanner.Err()
	}

	// Handle non-streaming response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	if choices, ok := response["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					return content, nil
				}
			}
		}
	}

	return "", fmt.Errorf("unexpected response format")
}
