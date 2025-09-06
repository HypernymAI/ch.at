package routing

import (
	"context"
	"log"
	"sync"
	"time"

	"ch.at/models"
)

// HealthChecker monitors deployment health
type HealthChecker struct {
	router        *Router
	interval      time.Duration
	timeout       time.Duration
	
	mu            sync.RWMutex
	running       bool
	stopChan      chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(router *Router, interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		router:   router,
		interval: interval,
		timeout:  timeout,
		stopChan: make(chan struct{}),
	}
}

// Start begins health checking
func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = true
	hc.mu.Unlock()

	go hc.run()
}

// Stop stops health checking
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	if !hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = false
	hc.mu.Unlock()

	close(hc.stopChan)
}

// run is the main health check loop
func (hc *HealthChecker) run() {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	// Initial health check
	hc.checkAll()

	for {
		select {
		case <-ticker.C:
			hc.checkAll()
		case <-hc.stopChan:
			return
		}
	}
}

// checkAll checks all deployments
func (hc *HealthChecker) checkAll() {
	hc.router.mu.RLock()
	deployments := make([]*models.Deployment, 0, len(hc.router.deployments))
	for _, d := range hc.router.deployments {
		deployments = append(deployments, d)
	}
	hc.router.mu.RUnlock()

	var wg sync.WaitGroup
	for _, deployment := range deployments {
		wg.Add(1)
		go func(d *models.Deployment) {
			defer wg.Done()
			hc.checkDeployment(d)
		}(deployment)
	}
	wg.Wait()
}

// checkDeployment checks a single deployment
func (hc *HealthChecker) checkDeployment(deployment *models.Deployment) {
	ctx, cancel := context.WithTimeout(context.Background(), hc.timeout)
	defer cancel()

	// Get provider
	hc.router.mu.RLock()
	provider, exists := hc.router.Providers[deployment.Provider]
	hc.router.mu.RUnlock()

	if !exists {
		hc.updateDeploymentHealth(deployment, false, "provider not found")
		return
	}

	// Perform health check
	start := time.Now()
	err := provider.HealthCheck(ctx, deployment)
	responseTime := time.Since(start)

	if err != nil {
		hc.updateDeploymentHealth(deployment, false, err.Error())
		log.Printf("Health check failed for %s: %v", deployment.ID, err)
	} else {
		hc.updateDeploymentHealth(deployment, true, "")
		hc.updateResponseTime(deployment, responseTime)
		// Log successful health checks for debugging
		if deployment.ModelID == "llama-8b" || deployment.ModelID == "llama-70b" {
			log.Printf("Health check PASSED for %s (model: %s)", deployment.ID, deployment.ModelID)
		}
	}
}

// updateDeploymentHealth updates deployment health status
func (hc *HealthChecker) updateDeploymentHealth(deployment *models.Deployment, healthy bool, errorMsg string) {
	hc.router.mu.Lock()
	defer hc.router.mu.Unlock()

	deployment.Status.LastHealthCheck = time.Now()
	deployment.Status.Healthy = healthy
	deployment.Status.Available = healthy

	if healthy {
		deployment.Status.ConsecutiveFails = 0
		deployment.Status.ErrorMessage = ""
	} else {
		deployment.Status.ConsecutiveFails++
		deployment.Status.ErrorMessage = errorMsg
		
		// Mark as unavailable after too many failures
		if deployment.Status.ConsecutiveFails >= 3 {
			deployment.Status.Available = false
		}
	}
}

// updateResponseTime updates deployment response time
func (hc *HealthChecker) updateResponseTime(deployment *models.Deployment, responseTime time.Duration) {
	hc.router.mu.Lock()
	defer hc.router.mu.Unlock()

	deployment.Status.ResponseTime = responseTime
	
	// Update average latency (simple moving average)
	if deployment.Metrics.AverageLatency == 0 {
		deployment.Metrics.AverageLatency = float64(responseTime.Milliseconds())
	} else {
		deployment.Metrics.AverageLatency = (deployment.Metrics.AverageLatency*0.9 + float64(responseTime.Milliseconds())*0.1)
	}
}