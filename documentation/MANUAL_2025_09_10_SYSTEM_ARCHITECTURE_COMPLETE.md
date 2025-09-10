# ch.at System Architecture - Complete Technical Documentation

## Table of Contents
1. [System Overview](#system-overview)
2. [Core Architecture](#core-architecture)
3. [Multi-Protocol Support](#multi-protocol-support)
4. [Privacy-Preserving Telemetry](#privacy-preserving-telemetry)
5. [Token and Rate Management](#token-and-rate-management)
6. [Deployment Architecture](#deployment-architecture)
7. [Security Architecture](#security-architecture)
8. [Monitoring and Observability](#monitoring-and-observability)

## System Overview

ch.at is a universal language model gateway that provides access to LLM capabilities through multiple protocols (HTTP, SSH, DNS, API) while maintaining strict privacy standards and operational transparency.

### Design Principles
- **Zero-Knowledge by Default**: No user tracking, no persistent storage of conversations
- **Protocol Diversity**: Access through HTTP, SSH, DNS, and OpenAI-compatible API
- **Single Binary**: ~1,300 lines of Go, minimal dependencies
- **Transparency First**: Live endpoints showing system state and data handling
- **Router-Based Architecture**: Dynamic model routing with fallback capabilities

### System Components

```
┌─────────────────────────────────────────────────────────┐
│                    User Access Layer                      │
├───────────┬───────────┬───────────┬────────────┬────────┤
│   HTTP    │    SSH    │    DNS    │  API       │  WS    │
│  Port 80  │  Port 22  │  Port 53  │  /v1/chat  │  /ws   │
├───────────┴───────────┴───────────┴────────────┴────────┤
│                  Protocol Handlers                        │
│  http.go    ssh.go    dns.go    donutsentry.go          │
├──────────────────────────────────────────────────────────┤
│                    Router Layer                           │
│         llm_router.go + routing/router.go                │
├──────────────────────────────────────────────────────────┤
│                  Provider Layer                           │
│   OneAPI Provider │ Baseline Provider │ Direct APIs      │
├──────────────────────────────────────────────────────────┤
│                 Model Deployments                         │
│  Claude │ GPT │ Llama │ Gemini │ Local Models           │
└──────────────────────────────────────────────────────────┘
```

## Core Architecture

### Main Components

#### 1. Protocol Handlers
Each protocol has a dedicated handler that transforms protocol-specific requests into unified LLM calls:

- **HTTP Handler** (`http.go`)
  - Serves web interface (no JavaScript)
  - Handles API endpoints
  - Manages WebSocket connections
  - Provides transparency endpoints

- **SSH Handler** (`ssh.go`)
  - Interactive terminal sessions
  - Streaming responses
  - Command history support

- **DNS Handler** (`dns.go`)
  - TXT record queries
  - 500-byte response limit
  - 4-second timeout handling
  - Automatic response compression

- **DoNutSentry** (`donutsentry.go`, `donutsentry_v2.go`)
  - DNS tunneling for large queries
  - Session-based communication
  - End-to-end encryption
  - Async processing support

#### 2. Router System (`llm_router.go`)
Central routing engine that:
- Selects appropriate model deployment
- Handles failover scenarios
- Manages per-transaction routing
- Tracks deployment health

#### 3. Provider Abstraction
- **OneAPI Provider**: Gateway to multiple model providers
- **Baseline Provider**: Direct fallback to OpenAI-compatible APIs
- **Provider Interface**: Unified API for all providers

### Request Flow

```
1. User Request → Protocol Handler
2. Protocol Handler → Service Configuration
3. Service Config → LLMWithRouter()
4. Router → Select Deployment (per-transaction)
5. Provider → Execute Request
6. Response → Stream/Buffer
7. Protocol Handler → User Response
```

## Multi-Protocol Support

### HTTP/HTTPS (Ports 80/443 or 8080/8443)

**Features:**
- No JavaScript required
- Server-side rendering
- Path-based queries (`/what-is-rust`)
- Query parameter support (`/?q=hello`)
- Session support (client-side storage)
- Model selection UI
- Conversation history (LocalStorage)

**Endpoints:**
- `/` - Main interface
- `/v1/chat/completions` - OpenAI-compatible API
- `/health` - System health check
- `/routing_table` - Live routing status
- `/terms_of_service` - Current ToS
- `/ws` - WebSocket streaming

### SSH (Port 22 or 2222)

**Features:**
- Interactive terminal chat
- Real-time streaming
- Multi-line input support
- Clear command (`/clear`)
- Exit commands (`/exit`, `/quit`)
- No authentication required

**Usage:**
```bash
ssh ch.at
# or for high-port mode
ssh -p 2222 localhost
```

### DNS (Port 53 or 8053)

**Features:**
- TXT record responses
- Automatic query optimization
- 500-character limit
- Sub-4-second responses
- Base32 encoding for binary data

**Query Format:**
```bash
dig @ch.at "what-is-dns" TXT
# Response limited to DNS constraints
```

### DoNutSentry v1 & v2 (DNS Tunneling)

**v1 Features:**
- Simple query/response
- Basic encryption
- Limited to DNS packet sizes

**v2 Features:**
- Session-based communication
- Query/response paging
- XOR encryption (zero overhead)
- Async processing
- Arbitrary query sizes
- Perfect forward secrecy

**Domain Structure:**
```
v1: <query>.q.ch.at
v2: <data>.<operation>.qp.ch.at
```

## Privacy-Preserving Telemetry

### Zero-Knowledge Telemetry Design

The system implements privacy-preserving telemetry that provides operational insights without compromising user privacy.

#### What is Collected (When Enabled)

**Hashed Content Only:**
```go
type TelemetryBeacon struct {
    Timestamp       int64
    ContentHash     string  // SHA256 hash - one-way, irreversible
    InputTokens     int     // Count only
    OutputTokens    int     // Count only
    Model          string   // e.g., "llama-8b"
    Deployment     string   // e.g., "llama-8b-oneapi-azure"
    ResponseTimeMs  int64
    Success        bool
    ErrorType      string   // Generic error category
}
```

**Never Collected:**
- Actual queries or responses
- IP addresses
- User identifiers
- Session information
- Browser fingerprints
- Location data

#### Audit Logging (Optional)

Separate from telemetry, audit logging can be enabled for debugging:

```bash
# In .env
ENABLE_LLM_AUDIT=true  # Default: false
```

When enabled, stores in SQLite:
- Conversation IDs (UUIDs)
- Full request/response pairs
- Timestamps
- Token counts
- Model routing decisions

**Database Schema:**
```sql
CREATE TABLE llm_audit (
    id INTEGER PRIMARY KEY,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    conversation_id TEXT,
    model TEXT,
    deployment TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    input_content TEXT,    -- Only when audit enabled
    output_content TEXT,   -- Only when audit enabled
    error TEXT
);
```

### Privacy Controls

Users can verify privacy status in real-time:

```bash
# Check current status
curl http://localhost:8080/routing_table | grep "Privacy Status"

# JSON verification
curl -s http://localhost:8080/routing_table -H "Accept: application/json" | jq .audit_enabled
```

## Token and Rate Management

### Token Limits by Service

Services have different token constraints based on protocol limitations:

```go
defaults := map[string]int{
    "DNS":           200,   // DNS packet size constraints
    "SSH":          1000,   // Terminal display limits
    "DONUTSENTRY":   500,   // DNS tunneling overhead
    "DONUTSENTRY_V2": 2000, // Paged responses allow more
    "HTTP":         4000,   // Browser/network limits
    "API":          8000,   // Full API access
}
```

### Rate Limiting

**Per-IP Rate Limiting:**
```go
type RateLimiter struct {
    RequestsPerMinute  int     // Default: 60
    BurstSize         int     // Default: 10
    CleanupInterval   Duration // Default: 1 minute
}
```

**Per-Model Rate Limits (via Router):**
```yaml
rate_limiting:
  enabled: true
  default_rps: 100
  burst: 200
  per_model_limits:
    gpt-4: 50
    gpt-4-turbo: 75
    claude-3-opus: 30
    claude-3-sonnet: 60
    claude-3-haiku: 150
    llama-8b: 200
    llama-70b: 50
```

### Token Counting

The system tracks token usage for:
1. **Cost Management**: Understanding provider costs
2. **Rate Limiting**: Preventing abuse
3. **Performance**: Optimizing response times
4. **Analytics**: Usage patterns (hashed only)

## Deployment Architecture

### Deployment Configurations

#### Development Mode
```bash
# High ports, no sudo required
HIGH_PORT_MODE=true ./ch.at
# HTTP: 8080, HTTPS: 8443, SSH: 2222, DNS: 8053
```

#### Production Mode
```bash
# Standard ports, requires root
sudo ./ch.at
# HTTP: 80, HTTPS: 443, SSH: 22, DNS: 53
```

### Model Deployment Strategy

```yaml
Deployment Priority:
1. Priority 1-10: Primary OneAPI deployments
2. Priority 11-50: Secondary/backup deployments  
3. Priority 999: Baseline fallback (last resort)

Selection Strategy:
- Weighted: Balance load across deployments
- Priority: Always use lowest priority number
- Round-Robin: Cycle through deployments
- Least-Latency: Choose fastest responding
```

### High Availability Configuration

```
┌─────────────────────┐
│   Load Balancer     │
├──────────┬──────────┤
│ Region A │ Region B │
├──────────┼──────────┤
│  ch.at   │  ch.at   │
│  Node 1  │  Node 2  │
├──────────┴──────────┤
│   Shared OneAPI     │
│   Gateway Cluster   │
├─────────────────────┤
│  Model Providers    │
│ Claude│GPT│Llama    │
└─────────────────────┘
```

## Security Architecture

### Security Layers

1. **Transport Security**
   - TLS 1.3 for HTTPS
   - SSH encryption for terminal access
   - DNS-over-HTTPS support planned

2. **Authentication & Authorization**
   - Currently: No authentication (privacy by design)
   - Future: Optional JWT-based auth
   - ToS agreement enforcement planned

3. **Input Validation**
   - Request size limits
   - Character encoding validation
   - SQL injection prevention
   - Command injection protection

4. **Rate Limiting**
   - Per-IP limits
   - Per-model limits
   - Burst protection
   - DDoS mitigation

### Security Boundaries

```
External → [TLS] → Router → [API Keys] → Providers
                ↓
          [Audit Log]
                ↓
          Local SQLite (Optional)
```

## Monitoring and Observability

### Health Monitoring

**Health Check Endpoint:**
```json
GET /health
{
  "status": "healthy",
  "router_initialized": true,
  "models_available": 18,
  "healthy_deployments": 19,
  "audit_enabled": false,
  "uptime_seconds": 3600
}
```

### Deployment Health Tracking

The system performs health checks every 30 seconds:

```go
type HealthChecker struct {
    Interval   Duration  // 30 seconds
    Timeout    Duration  // 5 seconds
    MaxFails   int       // 3 consecutive fails
}
```

**Health Check Process:**
1. Send test query to deployment
2. Verify response format
3. Check response time
4. Update deployment status
5. Trigger failover if needed

### Telemetry Beacons

When telemetry is enabled, the system emits beacons for:
- Request start/completion
- Routing decisions
- Provider selection
- Error conditions
- Performance metrics

**Beacon Example:**
```json
{
  "event": "llm_request_complete",
  "timestamp": 1234567890,
  "content_hash": "a7b9c2d4...",  // SHA256 hash only
  "tokens": {
    "input": 150,
    "output": 200
  },
  "performance": {
    "routing_ms": 5,
    "provider_ms": 1200,
    "total_ms": 1205
  },
  "deployment": "llama-8b-oneapi-azure"
}
```

### Operational Metrics

**Key Metrics Tracked:**
- Requests per second by protocol
- Token usage by model
- Error rates by deployment
- Response time percentiles (p50, p95, p99)
- Failover events
- Cache hit rates

### Debug Modes

```bash
# Enable debug logging
DEBUG_MODE=true ./ch.at

# Enable SQL query logging
SQL_DEBUG=true ./ch.at

# Enable router decision logging
ROUTER_DEBUG=true ./ch.at
```

## System Requirements

### Minimum Requirements
- CPU: 2 cores
- RAM: 2GB
- Disk: 100MB (plus audit logs if enabled)
- Network: 100Mbps
- OS: Linux/macOS/Windows

### Recommended Production
- CPU: 4+ cores
- RAM: 8GB
- Disk: 10GB SSD
- Network: 1Gbps
- OS: Linux (Ubuntu 22.04 LTS)

### Scaling Considerations

**Vertical Scaling:**
- More CPU cores improve concurrent request handling
- More RAM allows larger response caching
- SSD improves audit log performance

**Horizontal Scaling:**
- Stateless design allows multiple nodes
- Load balancer for distribution
- Shared OneAPI gateway recommended
- Session affinity not required

## Disaster Recovery

### Backup Strategy

**What to Backup:**
1. Configuration files (`.env`, `config/*.yaml`)
2. TLS certificates
3. Audit logs (if enabled)

**Backup Schedule:**
- Configuration: On change
- Certificates: Before expiration
- Audit logs: Daily (if enabled)

### Recovery Procedures

**Service Failure:**
```bash
# Quick restart
systemctl restart ch.at

# Full recovery
1. Stop service
2. Restore configuration
3. Verify OneAPI connectivity
4. Start service
5. Verify health endpoint
```

**Provider Failure:**
- Automatic failover to baseline
- Manual provider switching via config
- Health checks detect recovery

## Performance Optimization

### Caching Strategy
- DNS responses cached for 60 seconds
- Model selection cached per request
- Health status cached for 30 seconds

### Response Streaming
- HTTP: Server-Sent Events
- SSH: Direct stream
- DNS: Buffered (protocol limitation)
- WebSocket: Real-time chunks

### Connection Pooling
```go
HTTPClient: &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

## Additional Modules

### Discriminator Module
The system includes an optional discriminator module that can route queries to specialized handlers:

- **Chaos Rectification**: Handles queries containing "magic" keyword
- **Code Module**: Programming-specific queries
- **Research Module**: Academic/research queries
- **Creative Module**: Creative writing tasks

Configuration via environment variables:
```bash
ENABLE_MODULE_CHAOS=true
ENABLE_MODULE_CODE=false
ENABLE_MODULE_RESEARCH=false
ENABLE_MODULE_CREATIVE=false
```

## Configuration Reference

### Environment Variables
```bash
# Core Configuration
HIGH_PORT_MODE=true/false      # Use high ports (8080, 8443, 2222, 8053)
ENABLE_LLM_AUDIT=true/false    # Enable/disable audit logging
DEBUG_MODE=true/false           # Debug logging

# OneAPI Configuration
ONE_API_URL=http://localhost:3000
ONE_API_KEY=sk-...

# Baseline Fallback
BASIC_OPENAI_KEY=sk-...
BASIC_OPENAI_URL=https://api.openai.com/v1/chat/completions
BASIC_OPENAI_MODEL=gpt-4.1-nano

# Service-Specific Models
DNS_LLM_MODEL=llama-8b
SSH_LLM_MODEL=claude-3.5-haiku
DONUTSENTRY_LLM_MODEL=llama-8b
DONUTSENTRY_V2_LLM_MODEL=claude-3.5-haiku
```

### Configuration Files
```
config/
├── models.yaml        # Model definitions
├── deployments.yaml   # Deployment configurations
└── routing.yaml       # Routing strategies
```

## Error Codes

### HTTP Status Codes
- 200: Success
- 400: Bad Request (invalid parameters)
- 404: Model not found
- 429: Rate limit exceeded
- 500: Internal server error
- 503: Service unavailable (no healthy deployments)

### Router Error Messages
- `"no healthy deployments available"`: All deployments for model are down
- `"model not found in routing system"`: Model not registered
- `"rate limit exceeded"`: Too many requests
- `"context too long"`: Exceeds token limit
- `"provider timeout"`: Upstream provider didn't respond

## Future Architecture Plans

### Planned Enhancements

1. **Authentication System**
   - JWT-based auth
   - API key management
   - Usage quotas

2. **Enhanced Monitoring**
   - Prometheus metrics
   - Grafana dashboards
   - Alert manager integration

3. **Advanced Routing**
   - ML-based model selection
   - Cost optimization routing
   - Predictive failover

4. **Protocol Extensions**
   - GraphQL endpoint
   - gRPC support
   - MQTT for IoT

---

*This architecture enables ch.at to provide universal LLM access while maintaining privacy, transparency, and operational excellence.*