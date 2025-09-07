package main

import (
	"fmt"
	"io/ioutil"
	"log"
)

func processChaosRectification(input string) string {
	// TEMPORARY: Use smaller test doc to prove it works
	chaosDoc, docErr := ioutil.ReadFile("documentation/CHAOS_TEST.md")
	if docErr != nil {
		log.Printf("Error reading chaos doc: %v", docErr)
		return fmt.Sprintf("Error: Could not load chaos rectification document: %v", docErr)
	}
	
	// Build the full prompt with the chaos rectification document
	prompt := fmt.Sprintf(`You are a chaos rectifier. Using the complete Chaos Rectification Architecture document below as your processing logic, analyze the agent input and extract:

1. Primary actions (what emerges from resonance points where oscillations stabilize)
2. Targets (what the oscillations orbit around) 
3. Followup actions and their targets (unresolved oscillations that need attention later)

COMPLETE CHAOS RECTIFICATION ARCHITECTURE:
%s

AGENT INPUT TO PROCESS:
%s

Apply the chaos rectification principles from the document above to extract actions, targets, and followups from the agent chaos:`, string(chaosDoc), input)
	
	// Use messages format that matches what works in normal requests
	messages := []map[string]string{
		{"role": "user", "content": prompt},
	}
	
	// Call LLMWithRouter EXACTLY like handleChatCompletions does - with nil channel for non-streaming
	response, err := LLMWithRouter(messages, "llama-8b", nil)
	
	if err != nil {
		log.Printf("Chaos rectification error: %v", err)
		// The "invalid character 'd'" error means the API is returning an error message
		// that starts with 'd' instead of JSON. This is likely "deployment not found" 
		// or similar. Let's use a model that's actually working.
		
		// Try with llama-70b which seems to be working
		response, err = LLMWithRouter(messages, "llama-70b", nil)
		if err != nil {
			// If that fails too, show the actual error
			return fmt.Sprintf("Error processing chaos with llama-70b: %v", err)
		}
	}
	
	if response == nil {
		return "Error: No response from LLM"
	}
	
	return response.Content
}