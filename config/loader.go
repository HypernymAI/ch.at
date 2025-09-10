package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"ch.at/models"
	"ch.at/routing"
)

// Config represents the complete configuration
type Config struct {
	Models      map[string]ModelConfig      `yaml:"models"`
	Deployments map[string]DeploymentConfig `yaml:"deployments"`
	Routing     RoutingConfig               `yaml:"routing"`
}

// ModelConfig from YAML
type ModelConfig struct {
	Name         string                   `yaml:"name"`
	Family       string                   `yaml:"family"`
	Version      string                   `yaml:"version"`
	Capabilities models.ModelCapabilities `yaml:"capabilities"`
	Deployments  []string                 `yaml:"deployments"`
	Tags         map[string]string        `yaml:"tags"`
}

// DeploymentConfig from YAML
type DeploymentConfig struct {
	ModelID         string                 `yaml:"model_id"`
	Provider        string                 `yaml:"provider"`
	ProviderModelID string                 `yaml:"provider_model_id"`
	Priority        int                    `yaml:"priority"`
	Weight          int                    `yaml:"weight"`
	Endpoint        EndpointConfig         `yaml:"endpoint"`
	Tags            map[string]string      `yaml:"tags"`
}

// EndpointConfig from YAML
type EndpointConfig struct {
	BaseURL         string            `yaml:"base_url"`
	Timeout         string            `yaml:"timeout"`
	MaxRetries      int               `yaml:"max_retries"`
	APIVersion      string            `yaml:"api_version,omitempty"`
	Region          string            `yaml:"region,omitempty"`
	ProjectID       string            `yaml:"project_id,omitempty"`
	DeploymentName  string            `yaml:"deployment_name,omitempty"`
	UseOpenAIFormat bool              `yaml:"use_openai_format"`
	ModelPrefix     string            `yaml:"model_prefix,omitempty"`
	Auth            AuthConfig        `yaml:"auth"`
	CustomHeaders   map[string]string `yaml:"custom_headers,omitempty"`
}

// AuthConfig from YAML
type AuthConfig struct {
	Type string `yaml:"type"`
}

// RoutingConfig from YAML
type RoutingConfig struct {
	Strategy     string                 `yaml:"strategy"`
	HealthCheck  HealthCheckConfig      `yaml:"health_check"`
	Fallback     FallbackConfig         `yaml:"fallback"`
	Metrics      MetricsConfig          `yaml:"metrics"`
}

// HealthCheckConfig from YAML
type HealthCheckConfig struct {
	Enabled             bool   `yaml:"enabled"`
	Interval            string `yaml:"interval"`
	Timeout             string `yaml:"timeout"`
	MaxConsecutiveFails int    `yaml:"max_consecutive_fails"`
	CheckOnStartup      bool   `yaml:"check_on_startup"`
}

// FallbackConfig from YAML
type FallbackConfig struct {
	Enabled          bool `yaml:"enabled"`
	MaxFallbacks     int  `yaml:"max_fallbacks"`
	PreferSameRegion bool `yaml:"prefer_same_region"`
	PreferGateway    bool `yaml:"prefer_gateway"`
}

// MetricsConfig from YAML
type MetricsConfig struct {
	Enabled        bool     `yaml:"enabled"`
	WindowSize     string   `yaml:"window_size"`
	Percentiles    []int    `yaml:"percentiles"`
	ExportInterval string   `yaml:"export_interval"`
}

// LoadConfig loads configuration from YAML files
func LoadConfig(configDir string) (*Config, error) {
	config := &Config{
		Models:      make(map[string]ModelConfig),
		Deployments: make(map[string]DeploymentConfig),
	}

	// Load models.yaml
	modelsPath := filepath.Join(configDir, "models.yaml")
	if err := loadYAMLFile(modelsPath, &struct {
		Models map[string]ModelConfig `yaml:"models"`
	}{Models: config.Models}); err != nil {
		return nil, fmt.Errorf("failed to load models.yaml: %w", err)
	}

	// Load deployments.yaml
	deploymentsPath := filepath.Join(configDir, "deployments.yaml")
	if err := loadYAMLFile(deploymentsPath, &struct {
		Deployments map[string]DeploymentConfig `yaml:"deployments"`
	}{Deployments: config.Deployments}); err != nil {
		return nil, fmt.Errorf("failed to load deployments.yaml: %w", err)
	}

	// Load routing.yaml
	routingPath := filepath.Join(configDir, "routing.yaml")
	var routingWrapper struct {
		Routing RoutingConfig `yaml:"routing"`
	}
	if err := loadYAMLFile(routingPath, &routingWrapper); err != nil {
		return nil, fmt.Errorf("failed to load routing.yaml: %w", err)
	}
	config.Routing = routingWrapper.Routing

	// Expand environment variables
	expandEnvVars(config)

	return config, nil
}

// loadYAMLFile loads a YAML file into a structure
func loadYAMLFile(path string, v interface{}) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}

// expandEnvVars expands environment variables in configuration
func expandEnvVars(config *Config) {
	for id, deployment := range config.Deployments {
		deployment.Endpoint.BaseURL = expandEnv(deployment.Endpoint.BaseURL)
		deployment.Endpoint.Region = expandEnv(deployment.Endpoint.Region)
		deployment.Endpoint.ProjectID = expandEnv(deployment.Endpoint.ProjectID)
		config.Deployments[id] = deployment
	}
}

// expandEnv expands environment variables in a string
func expandEnv(s string) string {
	if strings.Contains(s, "${") {
		return os.Expand(s, func(key string) string {
			// Handle default values like ${VAR:-default}
			parts := strings.SplitN(key, ":-", 2)
			value := os.Getenv(parts[0])
			if value == "" && len(parts) > 1 {
				return parts[1]
			}
			return value
		})
	}
	return s
}

// BuildRouter creates a router from configuration
func BuildRouter(config *Config) (*routing.Router, *models.ModelRegistry, *models.DeploymentRegistry, error) {
	// Convert strategy string to RoutingStrategy
	var strategy routing.RoutingStrategy
	switch config.Routing.Strategy {
	case "round_robin":
		strategy = routing.StrategyRoundRobin
	case "weighted":
		strategy = routing.StrategyWeighted
	case "least_latency":
		strategy = routing.StrategyLeastLatency
	case "least_cost":
		strategy = routing.StrategyLeastCost
	case "priority":
		strategy = routing.StrategyPriority
	default:
		strategy = routing.StrategyWeighted
	}

	// Create router
	router := routing.NewRouter(strategy)

	// Create registries
	modelRegistry := models.NewModelRegistry()
	deploymentRegistry := models.NewDeploymentRegistry()

	// Register models
	for id, modelConfig := range config.Models {
		model := &models.Model{
			ID:           id,
			Name:         modelConfig.Name,
			Family:       modelConfig.Family,
			Version:      modelConfig.Version,
			Capabilities: modelConfig.Capabilities,
			Deployments:  modelConfig.Deployments,
			Tags:         modelConfig.Tags,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		modelRegistry.Register(model)
		router.RegisterModel(model)
	}

	// Register deployments
	for id, deploymentConfig := range config.Deployments {
		// Parse timeout
		timeout, _ := time.ParseDuration(deploymentConfig.Endpoint.Timeout)
		if timeout == 0 {
			timeout = 30 * time.Second
		}

		// Get auth type
		authType := models.AuthAPIKey
		switch deploymentConfig.Endpoint.Auth.Type {
		case "none":
			authType = models.AuthNone
		case "aws_iam":
			authType = models.AuthAWS
		case "gcp_oauth":
			authType = models.AuthGCP
		case "azure_ad":
			authType = models.AuthAzureAD
		}

		// Get API key from environment based on channel or model name
		apiKey := ""
		if authType == models.AuthAPIKey {
			// First check if there's a channel tag
			channel := deploymentConfig.Tags["channel"]
			modelName := deploymentConfig.ProviderModelID
			
			// Map channel to API key suffix
			switch channel {
			case "1":
				apiKey = os.Getenv("ONE_API_KEY_OPENAI")
			case "2":
				apiKey = os.Getenv("ONE_API_KEY_CLAUDE")
			case "3":
				apiKey = os.Getenv("ONE_API_KEY_GEMINI")
			case "4":
				apiKey = os.Getenv("ONE_API_KEY_AZURE")
			case "6":
				apiKey = os.Getenv("ONE_API_KEY_VERTEX_US_CENTRAL1")
			case "7":
				apiKey = os.Getenv("ONE_API_KEY_VERTEX_US_EAST5")
			case "8":
				apiKey = os.Getenv("ONE_API_KEY_AZURE_GPT5")
			case "9":
				apiKey = os.Getenv("ONE_API_KEY_COHERE")
			case "10":
				apiKey = os.Getenv("ONE_API_KEY")
			case "11":
				apiKey = os.Getenv("ONE_API_KEY_AZURE_GPT41_NANO")
			default:
				// Fall back to model name detection
				if strings.HasPrefix(modelName, "gpt-3") || strings.HasPrefix(modelName, "gpt-4") && !strings.HasPrefix(modelName, "gpt-4.1") && !strings.HasPrefix(modelName, "gpt-5") {
					apiKey = os.Getenv("ONE_API_KEY_OPENAI")
				} else if strings.HasPrefix(modelName, "claude-") {
					apiKey = os.Getenv("ONE_API_KEY_CLAUDE")
				} else if strings.HasPrefix(modelName, "gemini-") {
					apiKey = os.Getenv("ONE_API_KEY_GEMINI")
				} else if strings.HasPrefix(modelName, "gpt-4.1") || strings.HasPrefix(modelName, "gpt-5") {
					apiKey = os.Getenv("ONE_API_KEY_AZURE_GPT5")
				} else if strings.HasPrefix(modelName, "Meta-Llama") || strings.HasPrefix(modelName, "Llama-") {
					apiKey = os.Getenv("ONE_API_KEY_AZURE")
				} else if strings.HasPrefix(modelName, "llama-") {
					apiKey = os.Getenv("ONE_API_KEY")
				} else {
					apiKey = os.Getenv("ONE_API_KEY")
				}
			}
			
			// If still no key, try base ONE_API_KEY
			if apiKey == "" {
				apiKey = os.Getenv("ONE_API_KEY")
			}
			
			if apiKey != "" {
				fmt.Printf("[DEBUG] Found API key for deployment %s (channel: %s, model: %s): suffix=%s\n", 
					id, channel, modelName, apiKey[len(apiKey)-2:])
			} else {
				fmt.Printf("[DEBUG] No API key found for deployment %s (channel: %s, model: %s)\n", 
					id, channel, modelName)
			}
		}

		deployment := &models.Deployment{
			ID:              id,
			ModelID:         deploymentConfig.ModelID,
			Provider:        models.ProviderType(deploymentConfig.Provider),
			ProviderModelID: deploymentConfig.ProviderModelID,
			Priority:        deploymentConfig.Priority,
			Weight:          deploymentConfig.Weight,
			Endpoint: models.EndpointConfig{
				BaseURL:         deploymentConfig.Endpoint.BaseURL,
				Timeout:         timeout,
				MaxRetries:      deploymentConfig.Endpoint.MaxRetries,
				APIVersion:      deploymentConfig.Endpoint.APIVersion,
				Region:          deploymentConfig.Endpoint.Region,
				ProjectID:       deploymentConfig.Endpoint.ProjectID,
				DeploymentName:  deploymentConfig.Endpoint.DeploymentName,
				UseOpenAIFormat: deploymentConfig.Endpoint.UseOpenAIFormat,
				ModelPrefix:     deploymentConfig.Endpoint.ModelPrefix,
				Auth: models.AuthConfig{
					Type:   authType,
					APIKey: apiKey,
				},
				CustomHeaders: deploymentConfig.Endpoint.CustomHeaders,
			},
			Status: models.DeploymentStatus{
				Available: true,
				Healthy:   true,
			},
			Tags:      deploymentConfig.Tags,
			CreatedAt: time.Now(),
		}
		
		deploymentRegistry.Register(deployment)
		router.RegisterDeployment(deployment)
	}

	// Set up health checker if enabled
	if config.Routing.HealthCheck.Enabled {
		interval, _ := time.ParseDuration(config.Routing.HealthCheck.Interval)
		timeout, _ := time.ParseDuration(config.Routing.HealthCheck.Timeout)
		
		if interval == 0 {
			interval = 30 * time.Second
		}
		if timeout == 0 {
			timeout = 5 * time.Second
		}

		healthChecker := routing.NewHealthChecker(router, interval, timeout)
		if config.Routing.HealthCheck.CheckOnStartup {
			healthChecker.Start()
		}
	}

	return router, modelRegistry, deploymentRegistry, nil
}