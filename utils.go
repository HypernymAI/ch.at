package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"strconv"
)

// generateSignature creates a hash signature for content
// Used for deduplication and tracking in telemetry
func generateSignature(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)[:16] // First 16 chars of hash
}

// ServiceConfig holds configuration for a service's LLM usage
type ServiceConfig struct {
	Model       string
	MaxTokens   int
	Temperature float64
}

// getServiceConfig returns the complete LLM configuration for a service
func getServiceConfig(serviceName string) ServiceConfig {
	return ServiceConfig{
		Model:       getServiceModel(serviceName),
		MaxTokens:   getServiceMaxTokens(serviceName),
		Temperature: getServiceTemperature(serviceName),
	}
}

// getServiceModel returns the model to use for a service, with fallback logic
func getServiceModel(serviceName string) string {
	// First try service-specific model (e.g., DNS_LLM_MODEL)
	serviceModel := os.Getenv(serviceName + "_LLM_MODEL")
	log.Printf("[getServiceModel] %s_LLM_MODEL = '%s'", serviceName, serviceModel)
	if serviceModel != "" {
		return serviceModel
	}
	
	// Fall back to BASIC_OPENAI_MODEL (baseline fallback)
	basicModel := os.Getenv("BASIC_OPENAI_MODEL")
	log.Printf("[getServiceModel] BASIC_OPENAI_MODEL = '%s'", basicModel)
	if basicModel != "" {
		return basicModel
	}
	
	// Final fallback - will use default model from router
	log.Printf("[getServiceModel] Using final fallback: llama-8b")
	return "llama-8b"
}

// getServiceMaxTokens returns max tokens for a service with defaults
func getServiceMaxTokens(serviceName string) int {
	// Service-specific defaults
	defaults := map[string]int{
		"DNS":         200,  // DNS responses must be short
		"SSH":         1000, // SSH can have longer responses
		"DONUTSENTRY": 500,  // DonutSentry moderate length
	}
	
	// Try service-specific env var
	envVar := os.Getenv(serviceName + "_LLM_MAX_TOKENS")
	if envVar != "" {
		if val, err := strconv.Atoi(envVar); err == nil {
			return val
		}
	}
	
	// Return service default or generic default
	if defaultVal, ok := defaults[serviceName]; ok {
		return defaultVal
	}
	return 500 // Generic default
}

// getServiceTemperature returns temperature for a service with defaults
func getServiceTemperature(serviceName string) float64 {
	// Service-specific defaults
	defaults := map[string]float64{
		"DNS":         0.3, // Lower temperature for factual DNS responses
		"SSH":         0.7, // Moderate creativity for SSH
		"DONUTSENTRY": 0.7, // Moderate creativity for DonutSentry
	}
	
	// Try service-specific env var
	envVar := os.Getenv(serviceName + "_LLM_TEMPERATURE")
	if envVar != "" {
		if val, err := strconv.ParseFloat(envVar, 64); err == nil {
			return val
		}
	}
	
	// Return service default or generic default
	if defaultVal, ok := defaults[serviceName]; ok {
		return defaultVal
	}
	return 0.7 // Generic default
}