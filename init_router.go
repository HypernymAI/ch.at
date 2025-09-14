package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"ch.at/config"
	"ch.at/models"
	"ch.at/providers"
	"ch.at/routing"
)

// checkOneAPIAvailable does a quick test to see if OneAPI is reachable
func checkOneAPIAvailable() bool {
	// Quick timeout for health check
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	
	// Try to reach OneAPI health endpoint
	oneAPIURL := os.Getenv("ONE_API_URL")
	if oneAPIURL == "" {
		oneAPIURL = "http://localhost:3000"
	}
	
	resp, err := client.Get(oneAPIURL + "/health")
	if err != nil {
		log.Printf("[checkOneAPIAvailable] OneAPI not reachable at %s: %v", oneAPIURL, err)
		return false
	}
	defer resp.Body.Close()
	
	// Any response means it's up
	log.Printf("[checkOneAPIAvailable] OneAPI is available at %s (status: %d)", oneAPIURL, resp.StatusCode)
	return true
}

// InitializeModelRouter initializes the model routing system
func InitializeModelRouter() error {
	log.Println("[InitializeModelRouter] Starting model router initialization...")

	// Try to initialize full router with OneAPI deployments
	err := initializeFullRouter()
	if err != nil {
		log.Printf("[InitializeModelRouter] Full router initialization failed: %v", err)
		// Continue anyway - we'll add baseline fallback
	} else {
		log.Println("[InitializeModelRouter] Full router initialized with OneAPI deployments")
	}
	
	// ALWAYS add baseline fallback deployment if configured
	// This ensures fallback is available even when OneAPI is working
	basicKey := os.Getenv("BASIC_OPENAI_KEY")
	basicURL := os.Getenv("BASIC_OPENAI_URL")
	basicModel := os.Getenv("BASIC_OPENAI_MODEL")
	
	if basicKey != "" && basicURL != "" && basicModel != "" {
		log.Printf("[InitializeModelRouter] Adding baseline fallback deployment for model: %s", basicModel)
		if err := addBaselineFallbackDeployment(basicKey, basicURL, basicModel); err != nil {
			log.Printf("[InitializeModelRouter] WARNING: Failed to add baseline fallback: %v", err)
		} else {
			log.Println("[InitializeModelRouter] Baseline fallback deployment added successfully")
		}
	} else {
		log.Println("[InitializeModelRouter] No baseline fallback configured (BASIC_OPENAI_* vars not set)")
	}
	
	// If we have no router at all, create a minimal one
	if modelRouter == nil {
		if basicKey != "" && basicURL != "" && basicModel != "" {
			// Create router with just baseline
			log.Println("[InitializeModelRouter] Creating router with baseline fallback only")
			err = initializeBasicFallback(basicKey, basicURL, basicModel)
			if err != nil {
				return fmt.Errorf("failed to create baseline router: %w", err)
			}
		} else {
			return fmt.Errorf("no routing available: neither OneAPI nor baseline fallback configured")
		}
	}
	
	// Validate service configurations
	if err := validateServiceConfigurations(); err != nil {
		return fmt.Errorf("service configuration validation failed: %w", err)
	}
	
	return nil
}

// initializeFullRouter attempts to initialize the full routing system
func initializeFullRouter() error {
	// Determine config directory
	configDir := os.Getenv("LLM_CONFIG_DIR")
	if configDir == "" {
		// Default to ./config directory
		configDir = "./config"
	}

	// Check if config directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		log.Printf("[initializeFullRouter] Config directory not found: %s", configDir)
		return fmt.Errorf("config directory not found")
	}

	// Load configuration
	cfg, err := config.LoadConfig(configDir)
	if err != nil {
		log.Printf("[initializeFullRouter] Failed to load config: %v", err)
		return err
	}

	// Build router and registries
	router, modelReg, deploymentReg, err := config.BuildRouter(cfg)
	if err != nil {
		log.Printf("[initializeFullRouter] Failed to build router: %v", err)
		return err
	}

	// Register providers
	registerProviders(router)

	// Set global instances
	modelRouter = router
	modelRegistry = modelReg
	deploymentRegistry = deploymentReg

	// Start health checker for all deployments
	// This will periodically check OneAPI deployments and mark them unhealthy when down
	healthChecker := routing.NewHealthChecker(router, 30*time.Second, 5*time.Second)
	healthChecker.Start()
	log.Println("[initializeFullRouter] Started health checker for deployments")

	// Log initialization summary
	logInitSummary()

	return nil
}

// addBaselineFallbackDeployment adds a baseline fallback deployment to existing router
func addBaselineFallbackDeployment(apiKey, apiURL, modelID string) error {
	// Ensure router exists
	if modelRouter == nil || modelRegistry == nil || deploymentRegistry == nil {
		return fmt.Errorf("router not initialized")
	}
	
	// Check if model already exists
	if _, exists := modelRegistry.Get(modelID); !exists {
		// Create and register model
		model := &models.Model{
			ID:   modelID,
			Name: modelID + " (Baseline Fallback)",
			Family: detectModelFamily(modelID),
			Deployments: []string{"baseline-fallback-" + modelID},
		}
		modelRegistry.Register(model)
		modelRouter.RegisterModel(model)
	}
	
	// Create baseline deployment with LOW priority (high number)
	// This ensures it's only used when OneAPI deployments fail
	deployment := &models.Deployment{
		ID:       "baseline-fallback-" + modelID,
		ModelID:  modelID,
		Provider: models.ProviderOpenAI,
		ProviderModelID: modelID,
		Priority: 999,  // Very low priority - use only as last resort
		Weight:   10,   // Low weight in weighted routing
		Endpoint: models.EndpointConfig{
			BaseURL: apiURL,
			Timeout: 30 * time.Second,
			MaxRetries: 3,
			Auth: models.AuthConfig{
				Type: models.AuthAPIKey,
				APIKey: apiKey,
			},
		},
		Status: models.DeploymentStatus{
			Available: true,
			Healthy:   true,
		},
		Tags: map[string]string{
			"mode": "baseline",
			"source": "fallback",
			"tier": "fallback",
		},
	}
	
	deploymentRegistry.Register(deployment)
	modelRouter.RegisterDeployment(deployment)
	
	// Register baseline provider if not already registered
	if modelRouter.Providers[models.ProviderOpenAI] == nil {
		baselineProvider := providers.NewBaselineOpenAICompatibilityProvider()
		modelRouter.RegisterProvider(models.ProviderOpenAI, baselineProvider)
	}
	
	log.Printf("[addBaselineFallbackDeployment] Added baseline deployment %s with priority %d", deployment.ID, deployment.Priority)
	
	return nil
}

// initializeBasicFallback creates a minimal router with a single deployment using baseline provider
func initializeBasicFallback(apiKey, apiURL, modelID string) error {
	// Create registries
	modelReg := models.NewModelRegistry()
	deploymentReg := models.NewDeploymentRegistry()
	
	// Create single model
	model := &models.Model{
		ID:   modelID,
		Name: modelID,
		Family: detectModelFamily(modelID),
		Deployments: []string{"basic-fallback"},
	}
	modelReg.Register(model)
	
	// Create single deployment
	deployment := &models.Deployment{
		ID:       "basic-fallback",
		ModelID:  modelID,
		Provider: models.ProviderOpenAI,
		ProviderModelID: modelID,
		Priority: 1,
		Weight:   100,
		Endpoint: models.EndpointConfig{
			BaseURL: apiURL,
			Timeout: 30 * time.Second,
			MaxRetries: 3,
			Auth: models.AuthConfig{
				Type: models.AuthAPIKey,
				APIKey: apiKey,
			},
		},
		Status: models.DeploymentStatus{
			Available: true,
			Healthy:   true,
		},
		Tags: map[string]string{
			"mode": "baseline",
			"source": "fallback",
		},
	}
	
	deploymentReg.Register(deployment)
	
	// Create router with weighted strategy (for single deployment it doesn't matter)
	router := routing.NewRouter(routing.StrategyWeighted)
	
	// Register baseline provider for direct OpenAI-compatible API calls
	baselineProvider := providers.NewBaselineOpenAICompatibilityProvider()
	router.RegisterProvider(models.ProviderOpenAI, baselineProvider)
	
	// Register model with router (CRITICAL - router has its own model map!)
	router.RegisterModel(model)
	
	// Register deployment with router
	router.RegisterDeployment(deployment)
	
	// Set globals
	modelRouter = router
	modelRegistry = modelReg
	deploymentRegistry = deploymentReg
	
	// Start health checker (even for baseline-only mode)
	healthChecker := routing.NewHealthChecker(router, 30*time.Second, 5*time.Second)
	healthChecker.Start()
	
	log.Printf("[initializeBasicFallback] Basic fallback active - Model: %s, URL: %s", modelID, apiURL)
	beacon("router_fallback_activated", map[string]interface{}{
		"model": modelID,
		"url":   apiURL,
		"mode":  "basic",
	})
	
	return nil
}

// detectModelFamily attempts to detect the model family from the model ID
func detectModelFamily(modelID string) string {
	lower := strings.ToLower(modelID)
	switch {
	case strings.Contains(lower, "gpt"):
		return "gpt"
	case strings.Contains(lower, "claude"):
		return "claude"
	case strings.Contains(lower, "gemini"):
		return "gemini"
	case strings.Contains(lower, "llama"):
		return "llama"
	case strings.Contains(lower, "mistral"):
		return "mistral"
	default:
		return "unknown"
	}
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

// validateServiceConfigurations ensures all services have valid models configured
func validateServiceConfigurations() error {
	// Critical services that must have valid models
	services := []struct {
		name        string
		envVar      string
		description string
		required    bool
	}{
		{"DNS", "DNS_LLM_MODEL", "DNS query responses", true},
		{"SSH", "SSH_LLM_MODEL", "SSH interactive sessions", true},
		{"DONUTSENTRY", "DONUTSENTRY_LLM_MODEL", "DonutSentry v1 DNS tunneling", true},
		{"DONUTSENTRY_V2", "DONUTSENTRY_V2_LLM_MODEL", "DonutSentry v2 encrypted DNS", true},
	}
	
	log.Println("[ValidateServices] Validating service model configurations...")
	
	for _, service := range services {
		// Get the model this service will use
		log.Printf("[DEBUG] About to call getServiceModel for %s", service.name)
		model := getServiceModel(service.name)
		log.Printf("[DEBUG] getServiceModel(%s) returned: %s", service.name, model)
		
		// Check if explicitly configured
		explicit := os.Getenv(service.envVar)
		if explicit != "" {
			log.Printf("[ValidateServices] %s: Explicitly configured to use '%s'", service.name, explicit)
		} else {
			log.Printf("[ValidateServices] %s: Using fallback model '%s'", service.name, model)
		}
		
		// Validate the model exists in the router
		if modelRouter == nil {
			if service.required {
				return fmt.Errorf("service %s requires model '%s' but router not initialized", service.name, model)
			}
			log.Printf("[ValidateServices] WARNING: %s model '%s' cannot be validated (router not initialized)", service.name, model)
			continue
		}
		
		// Check if it's a tier request
		if strings.HasPrefix(model, "tier:") {
			// For now, accept tier specifications - they'll be resolved at runtime
			log.Printf("[ValidateServices] %s: Tier-based selection '%s' will be resolved at runtime", service.name, model)
			continue
		}
		
		// Check if the model exists in the registry
		if modelRegistry != nil {
			modelObj, exists := modelRegistry.Get(model)
			if !exists || modelObj == nil {
				if service.required {
					return fmt.Errorf("service %s requires model '%s' which is not available in router", service.name, model)
				}
				log.Printf("[ValidateServices] WARNING: %s model '%s' not found in registry", service.name, model)
			} else {
				// Check if model has any healthy deployments
				healthyCount := 0
				for _, deploymentID := range modelObj.Deployments {
					if deployment, exists := deploymentRegistry.Get(deploymentID); exists && deployment != nil && deployment.Status.Healthy {
						healthyCount++
					}
				}
				if healthyCount == 0 {
					log.Printf("[ValidateServices] WARNING: %s model '%s' has no healthy deployments", service.name, model)
				} else {
					log.Printf("[ValidateServices] ✓ %s: Model '%s' validated (%d healthy deployments)", service.name, model, healthyCount)
				}
			}
		}
	}
	
	// Log service configuration summary
	log.Println("[ValidateServices] Service configuration summary:")
	log.Printf("[ValidateServices]   DNS: %s (temp=%.1f, max_tokens=%d)", 
		getServiceModel("DNS"), 
		getServiceTemperature("DNS"), 
		getServiceMaxTokens("DNS"))
	log.Printf("[ValidateServices]   SSH: %s (temp=%.1f, max_tokens=%d)", 
		getServiceModel("SSH"),
		getServiceTemperature("SSH"),
		getServiceMaxTokens("SSH"))
	log.Printf("[ValidateServices]   DonutSentry v1: %s (temp=%.1f, max_tokens=%d)", 
		getServiceModel("DONUTSENTRY"),
		getServiceTemperature("DONUTSENTRY"),
		getServiceMaxTokens("DONUTSENTRY"))
	log.Printf("[ValidateServices]   DonutSentry v2: %s (temp=%.1f, max_tokens=%d)", 
		getServiceModel("DONUTSENTRY_V2"),
		getServiceTemperature("DONUTSENTRY_V2"),
		getServiceMaxTokens("DONUTSENTRY_V2"))
	
	return nil
}