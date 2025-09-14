package main

import (
	"encoding/json"
	"net/http"
	"time"

	"ch.at/models"
	"ch.at/routing"
)

// Global router instance (will be initialized in main)
var modelRouter *routing.Router
var modelRegistry *models.ModelRegistry
var deploymentRegistry *models.DeploymentRegistry

// ModelResponse for API responses
type ModelResponse struct {
	ID           string                    `json:"id"`
	Object       string                    `json:"object"`
	Name         string                    `json:"name"`
	Family       string                    `json:"family"`
	Capabilities models.ModelCapabilities  `json:"capabilities"`
	Deployments  []string                  `json:"deployments"`
	Created      int64                     `json:"created"`
	OwnedBy      string                    `json:"owned_by"`
}

// DeploymentResponse for API responses
type DeploymentResponse struct {
	ID              string                   `json:"id"`
	ModelID         string                   `json:"model_id"`
	Provider        string                   `json:"provider"`
	ProviderModelID string                   `json:"provider_model_id"`
	Status          models.DeploymentStatus  `json:"status"`
	Metrics         models.DeploymentMetrics `json:"metrics"`
	Tags            map[string]string        `json:"tags"`
}

// handleListModels handles GET /v1/models
func handleListModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all models from registry
	allModels := modelRegistry.List()
	
	// Convert to API response format
	modelResponses := make([]ModelResponse, 0, len(allModels))
	for _, model := range allModels {
		// Determine owned_by based on family
		ownedBy := "organization"
		switch model.Family {
		case "gpt":
			ownedBy = "openai"
		case "claude":
			ownedBy = "anthropic"
		case "llama":
			ownedBy = "meta"
		}

		modelResponses = append(modelResponses, ModelResponse{
			ID:           model.ID,
			Object:       "model",
			Name:         model.Name,
			Family:       model.Family,
			Capabilities: model.Capabilities,
			Deployments:  model.Deployments,
			Created:      model.CreatedAt.Unix(),
			OwnedBy:      ownedBy,
		})
	}

	response := map[string]interface{}{
		"object": "list",
		"data":   modelResponses,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetModel handles GET /v1/models/:model
func handleGetModel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract model ID from path
	modelID := r.URL.Path[len("/v1/models/"):]
	if modelID == "" {
		http.Error(w, "Model ID required", http.StatusBadRequest)
		return
	}

	// Get model from registry
	model, exists := modelRegistry.Get(modelID)
	if !exists {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}

	// Determine owned_by
	ownedBy := "organization"
	switch model.Family {
	case "gpt":
		ownedBy = "openai"
	case "claude":
		ownedBy = "anthropic"
	case "llama":
		ownedBy = "meta"
	}

	response := ModelResponse{
		ID:           model.ID,
		Object:       "model",
		Name:         model.Name,
		Family:       model.Family,
		Capabilities: model.Capabilities,
		Deployments:  model.Deployments,
		Created:      model.CreatedAt.Unix(),
		OwnedBy:      ownedBy,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleListDeployments handles GET /v1/deployments
func handleListDeployments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	modelID := r.URL.Query().Get("model")
	status := r.URL.Query().Get("status")

	// Get deployments
	var deployments []*models.Deployment
	if modelID != "" {
		deployments = deploymentRegistry.GetByModel(modelID)
	} else if status == "healthy" {
		deployments = deploymentRegistry.GetHealthy()
	} else {
		// Get all deployments
		allDeployments := make([]*models.Deployment, 0)
		// Note: Would need to add a List() method to DeploymentRegistry
		deployments = allDeployments
	}

	// Convert to API response format
	deploymentResponses := make([]DeploymentResponse, 0, len(deployments))
	for _, deployment := range deployments {
		deploymentResponses = append(deploymentResponses, DeploymentResponse{
			ID:              deployment.ID,
			ModelID:         deployment.ModelID,
			Provider:        string(deployment.Provider),
			ProviderModelID: deployment.ProviderModelID,
			Status:          deployment.Status,
			Metrics:         deployment.Metrics,
			Tags:            deployment.Tags,
		})
	}

	response := map[string]interface{}{
		"object": "list",
		"data":   deploymentResponses,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetDeployment handles GET /v1/deployments/:deployment
func handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract deployment ID from path
	deploymentID := r.URL.Path[len("/v1/deployments/"):]
	if deploymentID == "" {
		http.Error(w, "Deployment ID required", http.StatusBadRequest)
		return
	}

	// Get deployment from registry
	deployment, exists := deploymentRegistry.Get(deploymentID)
	if !exists {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	response := DeploymentResponse{
		ID:              deployment.ID,
		ModelID:         deployment.ModelID,
		Provider:        string(deployment.Provider),
		ProviderModelID: deployment.ProviderModelID,
		Status:          deployment.Status,
		Metrics:         deployment.Metrics,
		Tags:            deployment.Tags,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealthCheck handles GET /v1/health
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get health status
	healthyDeployments := deploymentRegistry.GetHealthy()
	totalModels := len(modelRegistry.List())

	response := map[string]interface{}{
		"status":              "healthy",
		"timestamp":           time.Now().Unix(),
		"models_available":    totalModels,
		"deployments_healthy": len(healthyDeployments),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}