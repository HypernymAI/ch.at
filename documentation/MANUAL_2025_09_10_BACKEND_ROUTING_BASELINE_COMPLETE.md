# Backend, Routing, and Baseline Fallback - Complete Documentation

## Table of Contents
1. [Overview](#overview)
2. [Router Architecture](#router-architecture)
3. [Provider System](#provider-system)
4. [Baseline Fallback Mechanism](#baseline-fallback-mechanism)
5. [Service Integration](#service-integration)
6. [Model Configuration](#model-configuration)
7. [Health Management](#health-management)
8. [Deployment Strategies](#deployment-strategies)
9. [Error Handling](#error-handling)
10. [Testing and Validation](#testing-and-validation)

## Overview

The backend routing system is the heart of ch.at, providing intelligent model selection, automatic failover, and seamless integration across all protocols. It enables per-transaction routing decisions with baseline fallback capabilities for high availability. The system operates entirely server-side, with no client-side dependencies or tracking.

### Core Components

```
┌──────────────────────────────────────────┐
│          Service Request                   │
│  (HTTP/SSH/DNS/DoNutSentry)              │
└────────────────┬─────────────────────────┘
                 ↓
┌──────────────────────────────────────────┐
│        Service Configuration              │
│         getServiceConfig()                │
└────────────────┬─────────────────────────┘
                 ↓
┌──────────────────────────────────────────┐
│         LLMWithRouter()                   │
│   Per-transaction routing decision        │
└────────────────┬─────────────────────────┘
                 ↓
┌──────────────────────────────────────────┐
│          Router Selection                 │
│   1. Check deployment health              │
│   2. Apply routing strategy               │
│   3. Select deployment                    │
└────────────────┬─────────────────────────┘
                 ↓
┌──────────────────────────────────────────┐
│         Provider Execution                │
│   OneAPI → Baseline Fallback             │
└──────────────────────────────────────────┘
```

## Router Architecture

### Core Router (`llm_router.go`)

The router is the central decision-making engine that handles all LLM requests.

#### Key Functions

```go
// Main routing function - called by all services
func LLMWithRouter(
    messages []map[string]string,
    model string,
    params *RouterParams,
    stream chan<- string,
) (*LLMResponse, error)

// Core routing decision
func (r *Router) Route(
    ctx context.Context,
    model string,
) (*Deployment, error)
```

#### Routing Strategies

1. **Weighted** (Default)
   ```go
   // Distributes load based on deployment weights
   deployment.Weight = 50  // Gets 50% of traffic
   ```

2. **Priority**
   ```go
   // Always selects lowest priority number
   deployment.Priority = 1  // Highest priority
   deployment.Priority = 999  // Baseline fallback
   ```

3. **Round-Robin**
   ```go
   // Cycles through deployments evenly
   nextIndex = (currentIndex + 1) % len(deployments)
   ```

4. **Least-Latency**
   ```go
   // Selects fastest responding deployment
   deployment.AverageLatency = 150ms  // Preferred
   ```

### Per-Transaction Routing

Unlike traditional routing that makes decisions at startup, ch.at evaluates routing for EVERY request:

```go
func LLMWithRouter(...) {
    // Fresh routing decision for each request
    deployment, err := modelRouter.Route(ctx, model)
    if err != nil {
        // No healthy deployments available
        return nil, err
    }
    
    // Execute with selected deployment
    provider := modelRouter.Providers[deployment.Provider]
    return provider.ChatCompletion(ctx, deployment, messages)
}
```

**Benefits:**
- Immediate failover when providers fail
- Load distribution across healthy deployments
- No stale routing decisions
- Dynamic response to health changes

## Provider System

### Provider Interface

All providers implement a common interface:

```go
type Provider interface {
    ChatCompletion(
        ctx context.Context,
        deployment *Deployment,
        messages []Message,
        params *CompletionParams,
    ) (*CompletionResponse, error)
    
    StreamChatCompletion(
        ctx context.Context,
        deployment *Deployment,
        messages []Message,
        params *CompletionParams,
    ) (<-chan StreamChunk, error)
    
    HealthCheck(
        ctx context.Context,
        deployment *Deployment,
    ) error
}
```

### OneAPI Provider (`providers/oneapi.go`)

Primary provider that routes through OneAPI gateway:

```go
type OneAPIProvider struct {
    httpClient *http.Client
    cache      *Cache
}

// Key feature: Appends path to base URL
func (o *OneAPIProvider) ChatCompletion(...) {
    url := deployment.Endpoint.BaseURL + "/v1/chat/completions"
    // Adds channel-specific API key suffix
    apiKey := deployment.Endpoint.Auth.APIKey + "-" + channel
}
```

**Channel Mapping:**
```
Channel 1:  OpenAI Direct
Channel 2:  Anthropic
Channel 3:  Google (Gemini models)  
Channel 4:  Azure (Llama models)
Channel 5:  GCP (llama-8b)
Channel 8:  OpenAI via OneAPI
Channel 10: AWS Bedrock (Claude models, others)
Channel 11: Azure OpenAI (GPT models)
```

### Baseline Provider (`providers/baseline_openai.go`)

Fallback provider for direct API access:

```go
type BaselineOpenAICompatibilityProvider struct {
    httpClient *http.Client
}

// Key difference: Uses URL as-is (no path appending)
func (b *BaselineOpenAICompatibilityProvider) ChatCompletion(...) {
    url := deployment.Endpoint.BaseURL  // Already complete!
    // Uses API key directly without suffix
    apiKey := deployment.Endpoint.Auth.APIKey
}
```

**Configuration:**
```bash
# In .env
BASIC_OPENAI_KEY="sk-..."
BASIC_OPENAI_URL="https://api.openai.com/v1/chat/completions"
BASIC_OPENAI_MODEL="gpt-4.1-nano"
```

## Baseline Fallback Mechanism

### How It Works

The baseline fallback provides resilience when OneAPI is unavailable:

```go
// During initialization
func InitializeModelRouter() {
    // 1. Try to initialize OneAPI deployments
    initializeFullRouter()  
    
    // 2. ALWAYS add baseline fallback
    if basicKey != "" && basicURL != "" {
        addBaselineFallbackDeployment(basicKey, basicURL, basicModel)
    }
}

// Baseline deployment configuration
deployment := &Deployment{
    ID:       "baseline-fallback-gpt-4.1-nano",
    Priority: 999,  // Very low priority (high number)
    Provider: ProviderOpenAI,
    Status: {
        Available: true,
        Healthy:   true,  // Always considered healthy
    },
    Tags: {
        "mode": "baseline",
    },
}
```

### Failover Scenarios

1. **OneAPI Down Completely**
   - All OneAPI deployments marked unhealthy
   - Router selects baseline (priority 999)
   - Requests go directly to OpenAI

2. **Specific Model Unavailable**
   - Only deployments for requested model checked
   - Falls back to baseline if configured for that model
   - Returns error if no baseline exists

3. **Partial OneAPI Failure**
   - Some channels working, others failed
   - Router avoids failed deployments
   - Uses healthy OneAPI deployments first
   - Falls back to baseline as last resort

### Safe Defaults

**GPT-4.1-nano** is chosen as the default baseline because:
- Low cost per token
- Fast response times
- Widely available
- Good for most queries
- OpenAI reliability

## Service Integration

### Service Configuration Pattern

Each service (DNS, SSH, DoNutSentry) uses a consistent configuration pattern:

```go
// In each service handler
config := getServiceConfig("SERVICE_NAME")

messages := []map[string]string{
    {"role": "user", "content": prompt},
}

params := &RouterParams{
    MaxTokens:   config.MaxTokens,
    Temperature: config.Temperature,
}

response, err := LLMWithRouter(messages, config.Model, params, streamChan)
```

### Service-Specific Configurations

#### DNS Service (`dns.go`)
```go
config := getServiceConfig("DNS")
// Defaults: model="llama-8b", max_tokens=200, temperature=0.3
// Low tokens due to DNS packet limits
// Low temperature for factual responses
```

#### SSH Service (`ssh.go`)
```go
config := getServiceConfig("SSH")  
// Defaults: model="llama-8b", max_tokens=1000, temperature=0.7
// Higher tokens for terminal display
// Moderate temperature for conversational responses
```

#### DoNutSentry v1 (`donutsentry.go`)
```go
config := getServiceConfig("DONUTSENTRY")
// Defaults: model="llama-8b", max_tokens=500, temperature=0.7
// Balanced for DNS tunneling overhead
```

#### DoNutSentry v2 (`donutsentry_v2.go`)
```go
config := getServiceConfig("DONUTSENTRY_V2")
// Can be configured separately from v1
// Supports longer responses via paging
// Defaults: max_tokens=2000
```

#### HTTP/API Service (`http.go`)
```go
// Direct model specification from request
model := request.Model  // From API request
if model == "" {
    model = getServiceModel("HTTP")
}
```

### Environment Variable Priority

Services check for configuration in this order:

1. **Service-specific model**
   ```bash
   DNS_LLM_MODEL="gpt-4.1-nano"
   SSH_LLM_MODEL="claude-3.5-haiku"
   ```

2. **Baseline fallback model**
   ```bash
   BASIC_OPENAI_MODEL="gpt-4.1-nano"
   ```

3. **Hardcoded default**
   ```go
   return "llama-8b"  // Final fallback
   ```

## Model Configuration

### Model Registry (`models/registry.go`)

Models are registered with capabilities and metadata:

```go
type Model struct {
    ID           string   // e.g., "llama-8b"
    Name         string   // e.g., "Llama 3 8B"
    Family       string   // e.g., "llama"
    Version      string   // e.g., "3.0"
    Capabilities ModelCapabilities
    Deployments  []string  // List of deployment IDs
    Tags         map[string]string
}

type ModelCapabilities struct {
    MaxTokens        int
    SupportsStreaming bool
    SupportsFunctions bool
    SupportsVision    bool
    InputCost        float64  // $ per 1K tokens
    OutputCost       float64  // $ per 1K tokens
}
```

### Deployment Configuration

Deployments connect models to providers:

```go
type Deployment struct {
    ID              string
    ModelID         string        // References Model.ID
    Provider        ProviderType  // OneAPI, OpenAI, etc.
    ProviderModelID string        // Provider's model name
    Priority        int           // 1-999, lower = higher priority
    Weight          int           // For weighted routing
    Endpoint        EndpointConfig
    Status          DeploymentStatus
    Tags            map[string]string
}

type EndpointConfig struct {
    BaseURL         string
    Timeout         Duration
    MaxRetries      int
    Auth            AuthConfig
    CustomHeaders   map[string]string
}
```

### Configuration Files

#### models.yaml
```yaml
models:
  llama-8b:
    name: "Llama 3 8B"
    family: "llama"
    version: "3.0"
    capabilities:
      max_tokens: 4096
      supports_streaming: true
    deployments:
      - llama-3.1-8b-oneapi-azure
      - llama-3-8b-oneapi-bedrock
```

#### deployments.yaml
```yaml
deployments:
  llama-3.1-8b-oneapi-azure:
    model_id: "llama-8b"
    provider: "oneapi"
    provider_model_id: "Meta-Llama-3.1-8B-Instruct"
    priority: 5
    weight: 50
    endpoint:
      base_url: "${ONE_API_URL:-http://localhost:3000}"
      timeout: 30s
    tags:
      channel: "4"
      region: "us-west"
```

## Health Management

### Health Checker (`routing/health.go`)

Monitors deployment health continuously:

```go
type HealthChecker struct {
    router    *Router
    interval  time.Duration  // 30 seconds default
    timeout   time.Duration  // 5 seconds default
    stopCh    chan struct{}
}

func (hc *HealthChecker) Start() {
    go func() {
        ticker := time.NewTicker(hc.interval)
        for {
            select {
            case <-ticker.C:
                hc.checkAllDeployments()
            case <-hc.stopCh:
                return
            }
        }
    }()
}
```

### Health Check Process

1. **Regular Deployments**
   ```go
   // Sends actual LLM request
   req := &UnifiedRequest{
       Model: deployment.ProviderModelID,
       Messages: []Message{
           {Role: "user", Content: "Hi"},
       },
       MaxTokens: 10,
   }
   err := provider.HealthCheck(ctx, deployment)
   ```

2. **Baseline Deployments**
   ```go
   // Always considered healthy
   if deployment.Tags["mode"] == "baseline" {
       deployment.Status.Healthy = true
       return
   }
   ```

### Health Status Tracking

```go
type DeploymentStatus struct {
    Available           bool
    Healthy            bool
    LastHealthCheck    time.Time
    ConsecutiveFails   int
    AverageLatency     time.Duration
    ErrorRate          float64
}
```

## Deployment Strategies

### Multi-Deployment Configuration

Models can have multiple deployments for redundancy:

```yaml
llama-8b:
  deployments:
    - llama-8b-oneapi-azure      # Priority 5
    - llama-8b-oneapi-bedrock    # Priority 10
    - llama-8b-baseline-openai   # Priority 999
```

### Channel-Based Routing

OneAPI channels provide different model access:

```go
// Automatic channel detection
func getChannelForModel(modelID string) string {
    switch {
    case strings.Contains(modelID, "claude"):
        return "2"  // Anthropic
    case strings.Contains(modelID, "gemini"):
        return "3"  // Google
    case strings.Contains(modelID, "llama"):
        return "4"  // Azure
    case strings.Contains(modelID, "gpt"):
        return "8"  // OpenAI
    default:
        return "10" // Bedrock
    }
}
```

### Regional Deployment

```yaml
tags:
  region: "us-west"
  latency_class: "low"
  cost_tier: "economy"
```

Router can prefer deployments based on:
- Geographic proximity
- Latency requirements
- Cost optimization

## Error Handling

### Error Categories

1. **Routing Errors**
   ```go
   ErrNoHealthyDeployments = errors.New("no healthy deployments available")
   ErrModelNotFound = errors.New("model not found in registry")
   ErrInvalidRequest = errors.New("invalid request parameters")
   ```

2. **Provider Errors**
   ```go
   ErrProviderTimeout = errors.New("provider request timeout")
   ErrProviderRateLimit = errors.New("provider rate limit exceeded")
   ErrProviderUnavailable = errors.New("provider service unavailable")
   ```

3. **Network Errors**
   ```go
   ErrConnectionRefused = errors.New("connection refused")
   ErrDNSResolution = errors.New("DNS resolution failed")
   ErrTLSHandshake = errors.New("TLS handshake failed")
   ```

### Error Recovery

```go
func LLMWithRouter(...) (*LLMResponse, error) {
    // Try primary deployment
    deployment, err := router.Route(ctx, model)
    if err != nil {
        // No deployments available
        return nil, fmt.Errorf("routing failed: %w", err)
    }
    
    // Execute request
    response, err := provider.ChatCompletion(...)
    if err != nil {
        // Mark deployment unhealthy
        deployment.Status.ConsecutiveFails++
        if deployment.Status.ConsecutiveFails >= 3 {
            deployment.Status.Healthy = false
        }
        
        // Try next deployment
        return LLMWithRouter(...)  // Recursive retry
    }
    
    // Success - reset failure count
    deployment.Status.ConsecutiveFails = 0
    return response, nil
}
```

### Circuit Breaker Pattern

```go
type CircuitBreaker struct {
    errorThreshold   float64  // 0.5 = 50% error rate
    successThreshold int      // Successes to close circuit
    timeout          Duration // Time before retry
    state            State    // Open, Closed, HalfOpen
}
```

## Testing and Validation

### Startup Validation

The system validates all service configurations at startup:

```go
func validateServiceConfigurations() error {
    services := []struct {
        name     string
        required bool
    }{
        {"DNS", true},
        {"SSH", true},
        {"DONUTSENTRY", true},
        {"DONUTSENTRY_V2", true},
    }
    
    for _, service := range services {
        model := getServiceModel(service.name)
        
        // Check model exists in registry
        if _, exists := modelRegistry.Get(model); !exists {
            return fmt.Errorf("service %s requires unavailable model %s",
                service.name, model)
        }
        
        // Check healthy deployments exist
        deployments := getHealthyDeployments(model)
        if len(deployments) == 0 {
            return fmt.Errorf("no healthy deployments for %s", model)
        }
    }
    return nil
}
```

### Testing Routing Decisions

```bash
# Test specific model
curl -X POST http://localhost:8080/v1/chat/completions \
  -d '{"model":"llama-8b","messages":[{"role":"user","content":"test"}]}'

# Test baseline fallback (stop OneAPI first)
docker stop oneapi
curl -X POST http://localhost:8080/v1/chat/completions \
  -d '{"model":"gpt-4.1-nano","messages":[{"role":"user","content":"test"}]}'

# Test service routing
dig @localhost -p 8053 test.q.ch.at TXT  # Uses DNS_LLM_MODEL
ssh -p 2222 localhost  # Uses SSH_LLM_MODEL
```

### Health Check Testing

```bash
# Check routing table
curl http://localhost:8080/routing_table

# Monitor health checks
tail -f /tmp/ch.at.log | grep "Health check"

# Force health check failure
iptables -A OUTPUT -p tcp --dport 3000 -j DROP  # Block OneAPI
sleep 35  # Wait for health check
curl http://localhost:8080/routing_table  # See failed deployments
```

### Load Testing

```bash
# Simple load test
for i in {1..100}; do
  curl -X POST http://localhost:8080/v1/chat/completions \
    -d '{"model":"llama-8b","messages":[{"role":"user","content":"hi"}]}' &
done

# Monitor routing distribution
tail -f /tmp/ch.at.log | grep "Selected deployment"
```

## Performance Optimization

### Connection Pooling

```go
httpClient := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
    },
    Timeout: 30 * time.Second,
}
```

### Response Streaming

```go
// Streaming reduces memory usage
func streamResponse(deployment *Deployment, stream chan<- string) {
    reader := bufio.NewReader(resp.Body)
    for {
        line, err := reader.ReadBytes('\n')
        if err != nil {
            break
        }
        
        // Parse SSE chunk
        if bytes.HasPrefix(line, []byte("data: ")) {
            data := line[6:]
            stream <- string(data)
        }
    }
}
```

### Caching

```go
type Cache struct {
    deploymentHealth map[string]HealthStatus
    modelRoutes      map[string]*Deployment
    ttl              time.Duration  // 30 seconds
}
```

## Audit Logging System

### LLM Audit Database (`llm_audit.go`)

When `ENABLE_LLM_AUDIT=true`, the system logs all LLM interactions:

```go
type LLMAudit struct {
    ID             int
    Timestamp      time.Time
    ConversationID string
    Model          string
    Deployment     string
    InputTokens    int
    OutputTokens   int
    InputContent   string  // Full request
    OutputContent  string  // Full response
    Error          string
}
```

### Audit Database Location

```bash
# Default location
~/.ch.at/llm_audit.db

# Query audit logs
sqlite3 ~/.ch.at/llm_audit.db "SELECT * FROM llm_audit ORDER BY timestamp DESC LIMIT 10"

# Check conversation
sqlite3 ~/.ch.at/llm_audit.db "SELECT * FROM llm_audit WHERE conversation_id='UUID'"

# Model usage stats
sqlite3 ~/.ch.at/llm_audit.db "
  SELECT model, 
         COUNT(*) as requests, 
         SUM(input_tokens) as total_input, 
         SUM(output_tokens) as total_output 
  FROM llm_audit 
  GROUP BY model"
```

### Audit Integration Points

```go
// In LLMWithRouter
if auditEnabled {
    audit := &LLMAudit{
        Timestamp:      time.Now(),
        ConversationID: convID,
        Model:          model,
        Deployment:     deployment.ID,
        InputTokens:    countTokens(messages),
        OutputTokens:   countTokens(response),
        InputContent:   formatMessages(messages),
        OutputContent:  response.Content,
    }
    saveAudit(audit)
}
```

## Troubleshooting

### Common Issues

1. **"No healthy deployments available"**
   - Check OneAPI is running: `curl http://localhost:3000/health`
   - Verify API keys in .env
   - Check deployment configuration in yaml files

2. **Baseline not working**
   - Verify BASIC_OPENAI_* variables in .env
   - Check API key validity
   - Test direct API access

3. **Wrong model selected**
   - Check service-specific env vars
   - Verify model exists in models.yaml
   - Look for typos in model names

4. **Slow failover**
   - Health check runs every 30 seconds
   - Reduce interval for faster detection
   - Consider reducing timeout

### Debug Logging

```bash
# Enable all debug logging
DEBUG_MODE=true ROUTER_DEBUG=true ./ch.at

# Check routing decisions
grep "LLMWithRouter" /tmp/ch.at.log

# Monitor health checks  
grep "Health check" /tmp/ch.at.log

# See deployment selection
grep "Selected deployment" /tmp/ch.at.log
```

---

*This backend routing system provides resilient, intelligent model selection with automatic failover, ensuring high availability across all ch.at services.*