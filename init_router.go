package main

import (
	"log"
	"os"

	"ch.at/config"
	"ch.at/models"
	"ch.at/providers"
	"ch.at/routing"
)

// InitializeModelRouter initializes the model routing system
func InitializeModelRouter() error {
	log.Println("[InitializeModelRouter] Starting model router initialization...")

	// Determine config directory
	configDir := os.Getenv("LLM_CONFIG_DIR")
	if configDir == "" {
		// Default to ./config directory
		configDir = "./config"
	}

	// Check if config directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		log.Printf("[InitializeModelRouter] Config directory not found: %s", configDir)
		log.Println("[InitializeModelRouter] Model routing will not be available")
		return nil // Don't fail, just disable routing
	}

	// Load configuration
	cfg, err := config.LoadConfig(configDir)
	if err != nil {
		log.Printf("[InitializeModelRouter] Failed to load config: %v", err)
		log.Println("[InitializeModelRouter] Falling back to legacy LLM mode")
		return nil // Don't fail, just use legacy mode
	}

	// Build router and registries
	router, modelReg, deploymentReg, err := config.BuildRouter(cfg)
	if err != nil {
		log.Printf("[InitializeModelRouter] Failed to build router: %v", err)
		return err
	}

	// Register providers
	registerProviders(router)

	// Set global instances
	modelRouter = router
	modelRegistry = modelReg
	deploymentRegistry = deploymentReg

	// Log initialization summary
	logInitSummary()

	log.Println("[InitializeModelRouter] Model router initialized successfully")
	return nil
}

// registerProviders registers all provider implementations
func registerProviders(router *routing.Router) {
	// Register OneAPI provider (primary)
	oneAPIProvider := providers.NewOneAPIProvider()
	router.RegisterProvider(models.ProviderOneAPI, oneAPIProvider)

	// Register other providers as needed
	// router.RegisterProvider(models.ProviderOpenAI, providers.NewOpenAIProvider())
	// router.RegisterProvider(models.ProviderAzure, providers.NewAzureProvider())
	// router.RegisterProvider(models.ProviderBedrock, providers.NewBedrockProvider())
	// router.RegisterProvider(models.ProviderVertex, providers.NewVertexProvider())
	// router.RegisterProvider(models.ProviderAnthropic, providers.NewAnthropicProvider())

	log.Println("[registerProviders] Registered OneAPI provider")
}

// logInitSummary logs initialization summary
func logInitSummary() {
	if modelRegistry == nil || deploymentRegistry == nil {
		return
	}

	models := modelRegistry.List()
	healthyDeployments := deploymentRegistry.GetHealthy()

	log.Printf("[InitSummary] Loaded %d models", len(models))
	log.Printf("[InitSummary] Registered %d healthy deployments", len(healthyDeployments))

	// Log available models
	for _, model := range models {
		log.Printf("[InitSummary] Model: %s (%s) - %d deployments",
			model.ID, model.Name, len(model.Deployments))
	}
}

// CheckRouterHealth checks if the router is healthy
func CheckRouterHealth() bool {
	if modelRouter == nil || modelRegistry == nil || deploymentRegistry == nil {
		return false
	}

	// Check if we have at least one healthy deployment
	healthyDeployments := deploymentRegistry.GetHealthy()
	return len(healthyDeployments) > 0
}

// GetRouterStatus returns router status information
func GetRouterStatus() map[string]interface{} {
	status := map[string]interface{}{
		"initialized": modelRouter != nil,
		"healthy":     false,
		"models":      0,
		"deployments": 0,
	}

	if modelRouter == nil {
		return status
	}

	models := modelRegistry.List()
	healthyDeployments := deploymentRegistry.GetHealthy()

	status["healthy"] = len(healthyDeployments) > 0
	status["models"] = len(models)
	status["deployments"] = len(healthyDeployments)

	return status
}