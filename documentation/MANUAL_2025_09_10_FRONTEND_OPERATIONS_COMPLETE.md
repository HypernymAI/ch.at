# Frontend Operations - Complete Documentation

## Table of Contents
1. [Overview](#overview)
2. [Web Interface](#web-interface)
3. [Session Management](#session-management)
4. [API Usage](#api-usage)
5. [Model Selection and Routing](#model-selection-and-routing)
6. [Channel Introspection](#channel-introspection)
7. [Running the Frontend](#running-the-frontend)
8. [Client Integration](#client-integration)
9. [Testing and Debugging](#testing-and-debugging)

## Overview

The ch.at frontend provides multiple interfaces for users to interact with the LLM backend, following a classic server-side rendering philosophy. The system is built without JavaScript dependencies, ensuring maximum compatibility, privacy, and accessibility while eliminating client-side tracking and vulnerabilities.

### Frontend Access Methods

1. **Web Browser** - Pure HTML interface at http://ch.at
2. **Command Line** - curl, wget, httpie
3. **Terminal** - SSH access
4. **DNS Client** - dig, nslookup
5. **API Client** - Any OpenAI-compatible client

### Design Philosophy

- **Classic Server-Side Rendering** - All logic executes on the server, no client-side scripting
- **Zero Client Dependencies** - Works in any browser, including text-based browsers like lynx
- **Privacy by Architecture** - No trackers, analytics, or client-side state management
- **Universal Accessibility** - Functions perfectly with screen readers, keyboard navigation, and assistive technologies
- **Transparency** - Live system status endpoints show exactly what the system is doing

## Web Interface

### HTML Structure (`http.go`)

The web interface is served directly from Go with embedded HTML, following classic web architecture principles:

```go
const htmlHeader = `<!DOCTYPE html>
<html>
<head>
    <title>ch.at</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        /* Pure CSS for styling - no JavaScript */
        body { 
            font-family: system-ui, -apple-system, sans-serif; 
            background: #FFF8F0; 
            color: #2C1F3D;
        }
        .chat { 
            max-width: 700px; 
            margin: 1.25rem auto;
        }
        .q { 
            padding: 1.25rem; 
            background: #E8DCC4; 
            font-style: italic; 
            border-left: 4px solid #6B4C8A; 
        }
        .a { 
            padding: 1.5rem 1.25rem; 
            background: #FFFBF5; 
            border-radius: 8px;
            position: relative;
        }
        .model-badge {
            position: absolute;
            top: 0;
            right: 0;
        }
    </style>
</head>
<body>
    <!-- Pure HTML, server-rendered -->
`
```

### Server-Side Rendering

All functionality is implemented through traditional HTTP form submissions and server responses:

```html
<!-- Query submission via POST -->
<form method="POST" action="/">
    <input type="text" name="q" placeholder="Type your message..." autofocus>
    <input type="submit" value="Send">
    <textarea name="h" style="display:none"><!-- Server-maintained history --></textarea>
    <input type="hidden" name="session" value="sess_123456">
    <input type="hidden" name="seq" value="1">
</form>

<!-- Model selection via radio buttons -->
<input type="radio" name="model" value="llama-8b" checked> Llama 8B
<input type="radio" name="model" value="claude-3.5-haiku"> Claude Haiku
<input type="radio" name="model" value="gpt-4.1-nano"> GPT Nano
```

### Response Streaming

The server uses HTTP chunked transfer encoding for real-time response streaming:

```go
func handleRoot(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Transfer-Encoding", "chunked")
    w.Header().Set("X-Accel-Buffering", "no")
    flusher := w.(http.Flusher)
    
    // Stream the response as it's generated
    ch := make(chan string)
    go LLMWithRouter(messages, model, params, ch)
    
    for chunk := range ch {
        fmt.Fprint(w, chunk)
        flusher.Flush()
    }
}
```

## Session Management

### Server-Side Session Tracking

Sessions are managed server-side to prevent duplicate submissions and maintain conversation context:

```go
var (
    sessionSeqs = make(map[string]int)
    sessionMu   sync.RWMutex
)

// Check for duplicate submission
if sessionID != "" && seqStr != "" {
    sessionMu.RLock()
    lastSeq, exists := sessionSeqs[sessionID]
    sessionMu.RUnlock()
    
    if exists && seq <= lastSeq {
        // This is a duplicate (browser refresh/back button)
        isDuplicate = true
    }
}
```

### Conversation History

History is passed through hidden form fields, maintaining state across requests without cookies:

```html
<textarea name="h" style="display:none">
Q: What is Go?
A: Go is a statically typed, compiled programming language...
§MODEL:llama-8b§

Q: Tell me more
A: Go was designed at Google...
§MODEL:gpt-4.1-nano§
</textarea>
```

### Rate Limiting Protection

Server-side rate limiting prevents abuse and billing disasters:

```go
// Per-IP rate limiting
ipRequestCounts := make(map[string]int)
if requestCount >= 50 { // Max 50 requests per hour per IP
    http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
    return
}
```

## API Usage

### OpenAI-Compatible Endpoint

```bash
# Basic request
curl -X POST http://ch.at/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-8b",
    "messages": [
      {"role": "user", "content": "Hello"}
    ],
    "temperature": 0.7,
    "max_tokens": 150
  }'

# Response format
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "llama-8b",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "Hello! How can I help you today?"
    },
    "finish_reason": "stop"
  }]
}
```

### Streaming API

Server-Sent Events for real-time streaming:

```bash
# Stream response
curl -N -X POST http://ch.at/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"llama-8b","messages":[{"role":"user","content":"count to 10"}],"stream":true}'

# Response format (SSE)
data: {"choices":[{"delta":{"content":"One"}}]}
data: {"choices":[{"delta":{"content":", two"}}]}
data: [DONE]
```

## Model Selection and Routing

### Available Models Display

The interface shows available models organized by provider:

```go
func buildModelTable(selectedModel string) string {
    modelTable := "<table class='model-radio-table'>"
    
    // Group models by provider
    for _, provider := range []string{"meta", "openai", "anthropic", "google"} {
        models := getModelsForProvider(provider)
        modelTable += fmt.Sprintf("<tr><td>%s %s</td><td>", 
            getProviderEmoji(provider), provider)
        
        for _, model := range models {
            checked := ""
            if model == selectedModel {
                checked = "checked"
            }
            modelTable += fmt.Sprintf(
                `<label><input type="radio" name="model" value="%s" %s> %s</label>`,
                model, checked, model)
        }
        modelTable += "</td></tr>"
    }
    return modelTable + "</table>"
}
```

### Model Badges

Each response shows which model was used via server-rendered badges:

```html
<div class="model-badge provider-anthropic">
    <div class="badge-toggle">
        <span class="provider-dot">🟠</span>
        <span class="model-name">claude-3.5-haiku</span>
    </div>
</div>
```

## Channel Introspection

### Routing Table Endpoint

View the complete routing status without any client-side code:

```bash
# HTML view
curl http://localhost:8080/routing_table

# JSON format for tools
curl http://localhost:8080/routing_table -H "Accept: application/json"
```

Response shows:
- Available models and their health status
- Active deployments and channels
- Current routing configuration
- Privacy settings (audit enabled/disabled)

### Health Status

```bash
curl http://localhost:8080/health

{
  "status": "healthy",
  "services": {
    "http": true,
    "https": false,
    "ssh": true,
    "dns": true
  },
  "llm_configured": true,
  "router_active": true,
  "available_models": 18,
  "healthy_deployments": 19
}
```

## Running the Frontend

### Development Mode

```bash
# Build the binary
go build -o ch.at .

# Run with high ports (no sudo needed)
HIGH_PORT_MODE=true ./ch.at

# Access points:
# Web: http://localhost:8080
# API: http://localhost:8080/v1/chat/completions
# SSH: ssh -p 2222 localhost
# DNS: dig @localhost -p 8053 test.q.ch.at TXT
```

### Production Mode

```bash
# With systemd
sudo systemctl start ch.at

# Or direct with logging
sudo ./ch.at 2>&1 | tee -a /var/log/ch.at.log

# With screen session
sudo screen -dmS chat-server bash -c './ch.at 2>&1 | tee -a /var/log/ch.at.log'
```

### Docker Deployment

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o ch.at .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/ch.at /ch.at
COPY --from=builder /app/.env /.env

EXPOSE 80 443 22 53
CMD ["/ch.at"]
```

### Environment Configuration

```bash
# .env file
HIGH_PORT_MODE=false
ENABLE_LLM_AUDIT=false  # Privacy by default
DEBUG_MODE=false

# Model routing
ONE_API_URL=http://localhost:3000
ONE_API_KEY=sk-your-key

# Baseline fallback
BASIC_OPENAI_KEY=sk-your-key
BASIC_OPENAI_URL=https://api.openai.com/v1/chat/completions
BASIC_OPENAI_MODEL=gpt-4.1-nano

# Service-specific models
DNS_LLM_MODEL=llama-8b
SSH_LLM_MODEL=claude-3.5-haiku
```

## Client Integration

### Command Line Clients

#### Simple curl usage:
```bash
# Query with curl
curl "http://ch.at/?q=What+is+rust"

# Path-based query
curl "http://ch.at/what-is-rust"

# API access
curl -X POST http://ch.at/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"llama-8b","messages":[{"role":"user","content":"Hello"}]}'
```

#### Shell script client:
```bash
#!/bin/bash
# ch.at CLI client

CHAT_URL="${CHAT_URL:-http://ch.at}"
MODEL="${MODEL:-llama-8b}"

chat() {
    local message="$1"
    curl -s -X POST "$CHAT_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$MODEL\",
            \"messages\": [{\"role\": \"user\", \"content\": \"$message\"}]
        }" | jq -r '.choices[0].message.content'
}

# Usage
chat "What is Go?"
```

### Python Client

```python
import requests
import json

class ChatClient:
    def __init__(self, base_url="http://ch.at"):
        self.base_url = base_url
        self.session = requests.Session()
    
    def chat(self, message, model="llama-8b"):
        response = self.session.post(
            f"{self.base_url}/v1/chat/completions",
            json={
                "model": model,
                "messages": [{"role": "user", "content": message}]
            }
        )
        return response.json()["choices"][0]["message"]["content"]

# Usage
client = ChatClient()
print(client.chat("What is Python?"))
```

### DoNutSentry Client

For DNS tunneling when queries exceed DNS limits:

```bash
# Direct DNS query (simple)
dig @ch.at "what-is-dns.q.ch.at" TXT

# DoNutSentry v2 for large queries
# Uses session-based protocol with paging
# Domain: *.qp.ch.at
```

## Testing and Debugging

### Functional Testing

```bash
# Test web interface
curl -I http://localhost:8080/
# Should return 200 OK with HTML content-type

# Test form submission
curl -X POST http://localhost:8080/ \
  -d "q=test&h=&model=llama-8b&session=test123&seq=1"

# Test API endpoint
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"llama-8b","messages":[{"role":"user","content":"test"}]}'

# Test model routing
curl http://localhost:8080/routing_table | grep "healthy"
```

### Performance Testing

```bash
# Simple load test
for i in {1..100}; do
  curl -s "http://localhost:8080/?q=test" > /dev/null &
done

# Monitor with logs
tail -f /tmp/ch.at.log | grep "Selected deployment"

# Check rate limiting
for i in {1..60}; do
  curl "http://localhost:8080/?q=test"
  # Should get rate limited after 50 requests
done
```

### Debug Modes

```bash
# Enable debug logging
DEBUG_MODE=true ./ch.at

# Router debugging
ROUTER_DEBUG=true ./ch.at

# Monitor health checks
tail -f /tmp/ch.at.log | grep "Health check"
```

### Session Testing

```bash
# Test duplicate detection
SESSION="sess_test"
SEQ="1"

# First request
curl -X POST http://localhost:8080/ \
  -d "q=test&session=$SESSION&seq=$SEQ"

# Duplicate (should not trigger new LLM call)
curl -X POST http://localhost:8080/ \
  -d "q=test&session=$SESSION&seq=$SEQ"
```

## Accessibility

The server-side architecture ensures perfect accessibility:

### Screen Reader Support

All content is semantic HTML with proper structure:

```html
<div class="chat">
    <div class="q">User question here</div>
    <div class="a">
        Assistant response here
        <div class="model-badge">
            <span class="provider-dot">🔷</span>
            <span class="model-name">llama-8b</span>
        </div>
    </div>
</div>
```

### Keyboard Navigation

Standard HTML form elements work with all keyboard navigation:
- Tab through form fields
- Enter to submit
- No JavaScript traps or custom key handlers

### Text-Based Browsers

Fully functional in lynx, w3m, and other text browsers:

```bash
lynx http://ch.at
w3m http://ch.at
curl http://ch.at
```

---

*The ch.at frontend demonstrates that modern web applications can be powerful, fast, and universally accessible without relying on client-side scripting. This classic approach ensures privacy, security, and compatibility across all devices and browsers.*