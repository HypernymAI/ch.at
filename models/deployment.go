package models

import (
	"time"
)

// Deployment represents a specific model instance on a provider
type Deployment struct {
	// Identification
	ID      string `json:"id" yaml:"id"`
	ModelID string `json:"model_id" yaml:"model_id"`

	// Provider configuration
	Provider        ProviderType `json:"provider" yaml:"provider"`
	ProviderModelID string       `json:"provider_model_id" yaml:"provider_model_id"` // For one-api: "provider:model"

	// Endpoint configuration
	Endpoint EndpointConfig `json:"endpoint" yaml:"endpoint"`

	// Routing configuration
	Priority int `json:"priority" yaml:"priority"` // Lower is higher priority
	Weight   int `json:"weight" yaml:"weight"`     // For weighted routing

	// Runtime state
	Status  DeploymentStatus  `json:"status"`
	Metrics DeploymentMetrics `json:"metrics"`

	// Metadata
	Tags      map[string]string `json:"tags" yaml:"tags"`
	CreatedAt time.Time         `json:"created_at" yaml:"created_at"`
}

// ProviderType represents supported cloud providers
type ProviderType string

const (
	ProviderOneAPI    ProviderType = "oneapi"
	ProviderOpenAI    ProviderType = "openai"
	ProviderAzure     ProviderType = "azure"
	ProviderBedrock   ProviderType = "bedrock"
	ProviderVertex    ProviderType = "vertex"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderLocal     ProviderType = "local"
)

// EndpointConfig contains provider-specific endpoint configuration
type EndpointConfig struct {
	// Connection
	BaseURL    string        `json:"base_url" yaml:"base_url"`
	Timeout    time.Duration `json:"timeout" yaml:"timeout"`
	MaxRetries int           `json:"max_retries" yaml:"max_retries"`

	// Provider-specific
	APIVersion     string `json:"api_version,omitempty" yaml:"api_version,omitempty"`
	Region         string `json:"region,omitempty" yaml:"region,omitempty"`
	ProjectID      string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	DeploymentName string `json:"deployment_name,omitempty" yaml:"deployment_name,omitempty"`

	// Format
	UseOpenAIFormat bool   `json:"use_openai_format" yaml:"use_openai_format"`
	ModelPrefix     string `json:"model_prefix,omitempty" yaml:"model_prefix,omitempty"`

	// Authentication
	Auth AuthConfig `json:"auth" yaml:"auth"`

	// Headers
	CustomHeaders map[string]string `json:"custom_headers,omitempty" yaml:"custom_headers,omitempty"`
}

// AuthType defines authentication methods
type AuthType string

const (
	AuthAPIKey  AuthType = "api_key"
	AuthAWS     AuthType = "aws_iam"
	AuthGCP     AuthType = "gcp_oauth"
	AuthAzureAD AuthType = "azure_ad"
	AuthNone    AuthType = "none"
)

// AuthConfig for various authentication methods
type AuthConfig struct {
	Type             AuthType    `json:"type" yaml:"type"`
	APIKey           string      `json:"-"` // Never serialize
	BearerToken      string      `json:"-"`
	AWSCredentials   *AWSAuth    `json:"-"`
	GCPCredentials   *GCPAuth    `json:"-"`
	AzureCredentials *AzureAuth  `json:"-"`
}

// AWSAuth for Bedrock
type AWSAuth struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"-"`
	SessionToken    string `json:"-"`
	Region          string `json:"region"`
}

// GCPAuth for Vertex AI
type GCPAuth struct {
	ServiceAccountJSON string      `json:"-"`
	TokenSource        interface{} `json:"-"`
}

// AzureAuth for Azure OpenAI
type AzureAuth struct {
	TenantID     string `json:"tenant_id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"-"`
}

// DeploymentStatus tracks deployment health
type DeploymentStatus struct {
	Available        bool          `json:"available"`
	Healthy          bool          `json:"healthy"`
	LastHealthCheck  time.Time     `json:"last_health_check"`
	LastSuccessful   time.Time     `json:"last_successful"`
	ConsecutiveFails int           `json:"consecutive_fails"`
	ErrorMessage     string        `json:"error_message,omitempty"`
	ResponseTime     time.Duration `json:"response_time"`
}

// DeploymentMetrics tracks performance and cost
type DeploymentMetrics struct {
	// Request metrics
	TotalRequests   int64 `json:"total_requests"`
	SuccessRequests int64 `json:"success_requests"`
	FailedRequests  int64 `json:"failed_requests"`

	// Latency metrics (milliseconds)
	AverageLatency float64 `json:"average_latency"`
	P50Latency     float64 `json:"p50_latency"`
	P95Latency     float64 `json:"p95_latency"`
	P99Latency     float64 `json:"p99_latency"`

	// Token metrics
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`

	// Cost metrics
	TotalCost float64 `json:"total_cost"`

	// Time window
	WindowStart time.Time `json:"window_start"`
	LastUpdated time.Time `json:"last_updated"`
}

// DeploymentRegistry manages all deployments
type DeploymentRegistry struct {
	deployments map[string]*Deployment
}

// NewDeploymentRegistry creates a new deployment registry
func NewDeploymentRegistry() *DeploymentRegistry {
	return &DeploymentRegistry{
		deployments: make(map[string]*Deployment),
	}
}

// Register adds a deployment to the registry
func (r *DeploymentRegistry) Register(deployment *Deployment) {
	r.deployments[deployment.ID] = deployment
}

// Get retrieves a deployment by ID
func (r *DeploymentRegistry) Get(id string) (*Deployment, bool) {
	deployment, exists := r.deployments[id]
	return deployment, exists
}

// GetByModel returns all deployments for a model
func (r *DeploymentRegistry) GetByModel(modelID string) []*Deployment {
	var deployments []*Deployment
	for _, deployment := range r.deployments {
		if deployment.ModelID == modelID {
			deployments = append(deployments, deployment)
		}
	}
	return deployments
}

// GetHealthy returns all healthy deployments
func (r *DeploymentRegistry) GetHealthy() []*Deployment {
	var deployments []*Deployment
	for _, deployment := range r.deployments {
		if deployment.Status.Healthy && deployment.Status.Available {
			deployments = append(deployments, deployment)
		}
	}
	return deployments
}