package models

import (
	"time"
)

// Model represents an abstract LLM model that can have multiple deployments
type Model struct {
	// Identification
	ID      string `json:"id" yaml:"id"`           // e.g., "llama-8b", "gpt-4"
	Name    string `json:"name" yaml:"name"`       // Display name
	Family  string `json:"family" yaml:"family"`   // Model family: llama, gpt, claude
	Version string `json:"version" yaml:"version"` // Model version

	// Capabilities
	Capabilities ModelCapabilities `json:"capabilities" yaml:"capabilities"`

	// Deployments
	Deployments []string `json:"deployments" yaml:"deployments"` // List of deployment IDs

	// Metadata
	CreatedAt time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time         `json:"updated_at" yaml:"updated_at"`
	Tags      map[string]string `json:"tags" yaml:"tags"`
}

// ModelCapabilities defines what a model can do
type ModelCapabilities struct {
	// Token limits
	MaxTokens     int `json:"max_tokens" yaml:"max_tokens"`
	ContextWindow int `json:"context_window" yaml:"context_window"`

	// Features
	SupportsVision    bool `json:"supports_vision" yaml:"supports_vision"`
	SupportsFunctions bool `json:"supports_functions" yaml:"supports_functions"`
	SupportsStreaming bool `json:"supports_streaming" yaml:"supports_streaming"`
	SupportsJSON      bool `json:"supports_json" yaml:"supports_json"`

	// Performance
	TokensPerSecond float64 `json:"tokens_per_second" yaml:"tokens_per_second"`

	// Cost (per 1k tokens)
	InputCost  float64 `json:"input_cost" yaml:"input_cost"`
	OutputCost float64 `json:"output_cost" yaml:"output_cost"`

	// Technical
	TokenizerType string   `json:"tokenizer_type" yaml:"tokenizer_type"`
	Languages     []string `json:"languages" yaml:"languages"`
}

// ModelRegistry manages all available models
type ModelRegistry struct {
	models map[string]*Model
}

// NewModelRegistry creates a new model registry
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models: make(map[string]*Model),
	}
}

// Register adds a model to the registry
func (r *ModelRegistry) Register(model *Model) {
	r.models[model.ID] = model
}

// Get retrieves a model by ID
func (r *ModelRegistry) Get(id string) (*Model, bool) {
	model, exists := r.models[id]
	return model, exists
}

// List returns all registered models
func (r *ModelRegistry) List() []*Model {
	models := make([]*Model, 0, len(r.models))
	for _, model := range r.models {
		models = append(models, model)
	}
	return models
}

// GetByFamily returns all models in a family
func (r *ModelRegistry) GetByFamily(family string) []*Model {
	var models []*Model
	for _, model := range r.models {
		if model.Family == family {
			models = append(models, model)
		}
	}
	return models
}