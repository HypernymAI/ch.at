package routing

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"ch.at/models"
	"ch.at/providers"
)

// Router manages model deployments and routing decisions
type Router struct {
	// Configuration
	models      map[string]*models.Model
	deployments map[string]*models.Deployment
	strategy    RoutingStrategy

	// Providers (exported for health checker)
	Providers map[models.ProviderType]providers.Provider

	// Runtime state
	mu              sync.RWMutex
	roundRobinIndex map[string]int
	healthChecker   *HealthChecker

	// Circuit breakers
	circuitBreakers map[string]*CircuitBreaker
}

// RoutingStrategy defines how to select deployments
type RoutingStrategy string

const (
	StrategyRoundRobin   RoutingStrategy = "round_robin"
	StrategyWeighted     RoutingStrategy = "weighted"
	StrategyLeastLatency RoutingStrategy = "least_latency"
	StrategyLeastCost    RoutingStrategy = "least_cost"
	StrategyPriority     RoutingStrategy = "priority"
)

// NewRouter creates a new router
func NewRouter(strategy RoutingStrategy) *Router {
	return &Router{
		models:          make(map[string]*models.Model),
		deployments:     make(map[string]*models.Deployment),
		Providers:       make(map[models.ProviderType]providers.Provider),
		strategy:        strategy,
		roundRobinIndex: make(map[string]int),
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// RegisterModel registers a model
func (r *Router) RegisterModel(model *models.Model) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[model.ID] = model
}

// RegisterDeployment registers a deployment
func (r *Router) RegisterDeployment(deployment *models.Deployment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deployments[deployment.ID] = deployment
	
	// Add deployment to model's deployment list
	if model, exists := r.models[deployment.ModelID]; exists {
		model.Deployments = append(model.Deployments, deployment.ID)
	}
	
	// Initialize circuit breaker for this deployment
	r.circuitBreakers[deployment.ID] = NewCircuitBreaker(deployment.ID, 5, 60*time.Second)
}

// RegisterProvider registers a provider
func (r *Router) RegisterProvider(providerType models.ProviderType, provider providers.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Providers[providerType] = provider
}

// RouteRequest makes a routing decision for a model request
func (r *Router) RouteRequest(ctx context.Context, modelID string, reqCtx *RequestContext) (*RoutingDecision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get model
	model, exists := r.models[modelID]
	if !exists {
		// Try to find a deployment with this ID as provider model
		for _, deployment := range r.deployments {
			if deployment.ProviderModelID == modelID {
				model = r.models[deployment.ModelID]
				if model != nil {
					break
				}
			}
		}
		if model == nil {
			return nil, fmt.Errorf("model not found: %s", modelID)
		}
	}

	// Get available deployments
	availableDeployments := r.getAvailableDeployments(model.Deployments)
	if len(availableDeployments) == 0 {
		return nil, fmt.Errorf("no available deployments for model %s", modelID)
	}

	// Apply routing strategy
	primary := r.selectDeployment(availableDeployments, reqCtx)
	if primary == nil {
		return nil, fmt.Errorf("failed to select primary deployment")
	}

	// Select fallbacks
	fallbacks := r.selectFallbacks(availableDeployments, primary, reqCtx)

	return &RoutingDecision{
		RequestID: reqCtx.RequestID,
		ModelID:   model.ID,
		Primary:   primary,
		Fallbacks: fallbacks,
		Strategy:  r.strategy,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"total_deployments":     len(model.Deployments),
			"available_deployments": len(availableDeployments),
		},
	}, nil
}

// getAvailableDeployments returns healthy deployments
func (r *Router) getAvailableDeployments(deploymentIDs []string) []*models.Deployment {
	var available []*models.Deployment
	
	for _, id := range deploymentIDs {
		deployment, exists := r.deployments[id]
		if !exists {
			continue
		}

		// Check circuit breaker
		if cb, exists := r.circuitBreakers[id]; exists && !cb.Allow() {
			continue
		}

		// Check deployment health
		if deployment.Status.Available && deployment.Status.ConsecutiveFails < 3 {
			available = append(available, deployment)
		}
	}

	return available
}

// selectDeployment selects a deployment based on routing strategy
func (r *Router) selectDeployment(deployments []*models.Deployment, reqCtx *RequestContext) *models.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	switch r.strategy {
	case StrategyRoundRobin:
		return r.selectRoundRobin(deployments, reqCtx)
	case StrategyWeighted:
		return r.selectWeighted(deployments, reqCtx)
	case StrategyPriority:
		return r.selectPriority(deployments, reqCtx)
	case StrategyLeastLatency:
		return r.selectLeastLatency(deployments, reqCtx)
	case StrategyLeastCost:
		return r.selectLeastCost(deployments, reqCtx)
	default:
		return deployments[0]
	}
}

// selectRoundRobin selects using round-robin
func (r *Router) selectRoundRobin(deployments []*models.Deployment, reqCtx *RequestContext) *models.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	key := reqCtx.ModelID
	index := r.roundRobinIndex[key] % len(deployments)
	r.roundRobinIndex[key] = index + 1
	
	return deployments[index]
}

// selectWeighted selects using weighted random
func (r *Router) selectWeighted(deployments []*models.Deployment, reqCtx *RequestContext) *models.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0
	for _, d := range deployments {
		totalWeight += d.Weight
	}

	if totalWeight == 0 {
		return deployments[0]
	}

	// Random selection based on weight
	random := rand.Intn(totalWeight)
	cumulative := 0
	
	for _, d := range deployments {
		cumulative += d.Weight
		if random < cumulative {
			return d
		}
	}

	return deployments[len(deployments)-1]
}

// selectPriority selects based on priority
func (r *Router) selectPriority(deployments []*models.Deployment, reqCtx *RequestContext) *models.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	// Sort by priority (lower is better)
	sorted := make([]*models.Deployment, len(deployments))
	copy(sorted, deployments)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	return sorted[0]
}

// selectLeastLatency selects deployment with lowest latency
func (r *Router) selectLeastLatency(deployments []*models.Deployment, reqCtx *RequestContext) *models.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	var best *models.Deployment
	var bestLatency float64 = 999999

	for _, d := range deployments {
		if d.Metrics.AverageLatency < bestLatency {
			best = d
			bestLatency = d.Metrics.AverageLatency
		}
	}

	if best == nil {
		return deployments[0]
	}
	return best
}

// selectLeastCost selects deployment with lowest cost
func (r *Router) selectLeastCost(deployments []*models.Deployment, reqCtx *RequestContext) *models.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	// Get model to check costs
	model := r.models[reqCtx.ModelID]
	if model == nil {
		return deployments[0]
	}

	// For now, return first deployment
	// In production, would calculate actual costs
	return deployments[0]
}

// selectFallbacks selects fallback deployments
func (r *Router) selectFallbacks(deployments []*models.Deployment, primary *models.Deployment, reqCtx *RequestContext) []*models.Deployment {
	var fallbacks []*models.Deployment
	maxFallbacks := 3

	for _, d := range deployments {
		if d.ID == primary.ID {
			continue
		}
		fallbacks = append(fallbacks, d)
		if len(fallbacks) >= maxFallbacks {
			break
		}
	}

	return fallbacks
}

// ExecuteRequest executes a request with routing and fallback
func (r *Router) ExecuteRequest(ctx context.Context, req *providers.UnifiedRequest, decision *RoutingDecision) (*providers.UnifiedResponse, error) {
	// Try primary deployment
	resp, err := r.tryDeployment(ctx, req, decision.Primary)
	if err == nil {
		return resp, nil
	}

	// Record failure
	r.recordFailure(decision.Primary.ID)

	// Try fallbacks
	for _, fallback := range decision.Fallbacks {
		resp, err = r.tryDeployment(ctx, req, fallback)
		if err == nil {
			return resp, nil
		}
		r.recordFailure(fallback.ID)
	}

	return nil, fmt.Errorf("all deployments failed")
}

// tryDeployment attempts to execute request on a deployment
func (r *Router) tryDeployment(ctx context.Context, req *providers.UnifiedRequest, deployment *models.Deployment) (*providers.UnifiedResponse, error) {
	// Get provider
	provider, exists := r.Providers[deployment.Provider]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", deployment.Provider)
	}

	// Translate request
	providerReq, err := provider.TranslateRequest(ctx, req, deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to translate request: %w", err)
	}

	// Execute request
	providerResp, err := provider.Execute(ctx, providerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	// Translate response
	unifiedResp, err := provider.TranslateResponse(ctx, providerResp, deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to translate response: %w", err)
	}

	// Record success
	r.recordSuccess(deployment.ID)

	return unifiedResp, nil
}

// recordSuccess records successful request
func (r *Router) recordSuccess(deploymentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if deployment, exists := r.deployments[deploymentID]; exists {
		deployment.Status.ConsecutiveFails = 0
		deployment.Status.LastSuccessful = time.Now()
		deployment.Metrics.SuccessRequests++
		deployment.Metrics.TotalRequests++
	}

	if cb, exists := r.circuitBreakers[deploymentID]; exists {
		cb.RecordSuccess()
	}
}

// recordFailure records failed request
func (r *Router) recordFailure(deploymentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if deployment, exists := r.deployments[deploymentID]; exists {
		deployment.Status.ConsecutiveFails++
		deployment.Metrics.FailedRequests++
		deployment.Metrics.TotalRequests++
	}

	if cb, exists := r.circuitBreakers[deploymentID]; exists {
		cb.RecordFailure()
	}
}

// RoutingDecision represents a routing choice with fallbacks
type RoutingDecision struct {
	RequestID string                 `json:"request_id"`
	ModelID   string                 `json:"model_id"`
	Primary   *models.Deployment     `json:"primary"`
	Fallbacks []*models.Deployment   `json:"fallbacks"`
	Strategy  RoutingStrategy        `json:"strategy"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// RequestContext provides context for routing decisions
type RequestContext struct {
	RequestID      string
	ModelID        string
	UserID         string
	SessionID      string
	Priority       int
	MaxLatency     time.Duration
	MaxCost        float64
	Region         string
	UserPreference map[string]interface{}
}