# Frontend Operations - Complete Documentation

## Table of Contents
1. [Overview](#overview)
2. [Web Interface](#web-interface)
3. [Session Management](#session-management)
4. [API Usage](#api-usage)
5. [Model Selection and Routing](#model-selection-and-routing)
6. [Channel Introspection](#channel-introspection)
7. [Banner System](#banner-system)
8. [Running the Frontend](#running-the-frontend)
9. [Client Integration](#client-integration)
10. [Testing and Debugging](#testing-and-debugging)

## Overview

The ch.at frontend provides multiple interfaces for users to interact with the LLM backend, all without requiring JavaScript (though enhanced features are available with JS enabled). The system emphasizes privacy, transparency, and accessibility.

### Frontend Access Methods

1. **Web Browser** - HTML interface at http://ch.at
2. **Command Line** - curl, wget, httpie
3. **Terminal** - SSH access
4. **DNS Client** - dig, nslookup
5. **API Client** - Any OpenAI-compatible client
6. **WebSocket** - Real-time streaming

### Design Principles

- **No JavaScript Required** - Core functionality works without JS
- **Progressive Enhancement** - JS adds features but isn't required
- **Privacy First** - No tracking, client-side storage only
- **Accessibility** - Works with screen readers, keyboard navigation
- **Transparency** - Live system status visible to users

## Web Interface

### HTML Structure (`http.go`)

The web interface is served directly from Go with embedded HTML:

```go
const htmlContent = `<!DOCTYPE html>
<html>
<head>
    <title>ch.at - Universal Basic Intelligence</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        /* Minimal CSS for readability */
        body { 
            max-width: 800px; 
            margin: 0 auto; 
            padding: 20px;
            font-family: -apple-system, system-ui, sans-serif;
        }
        .chat-container { 
            border: 1px solid #ccc; 
            border-radius: 8px;
            padding: 20px;
        }
        .response { 
            white-space: pre-wrap;
            background: #f5f5f5;
            padding: 10px;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <h1>ch.at</h1>
    <div class="chat-container">
        <form method="GET" action="/">
            <textarea name="q" placeholder="Ask anything..."></textarea>
            <button type="submit">Send</button>
        </form>
        <div id="response"></div>
    </div>
</body>
</html>`
```

### No-JavaScript Mode

Core functionality without JavaScript:

```html
<!-- Query via GET parameter -->
<form method="GET" action="/">
    <input name="q" value="What is Go?">
    <button>Ask</button>
</form>

<!-- Server returns complete page with response -->
<div class="response">
    Go is a statically typed, compiled programming language...
</div>
```

### Progressive Enhancement

With JavaScript enabled, adds:

```javascript
// Model selection dropdown
const modelSelect = document.createElement('select');
modelSelect.id = 'model-select';
modelSelect.innerHTML = `
    <option value="">Auto-select</option>
    <option value="llama-8b">Llama 8B (Fast)</option>
    <option value="claude-3.5-haiku">Claude Haiku</option>
    <option value="gpt-4.1-nano">GPT-4.1 Nano</option>
`;

// Real-time streaming
const eventSource = new EventSource('/stream?q=' + query);
eventSource.onmessage = (event) => {
    responseDiv.innerHTML += event.data;
};

// Conversation history
const history = JSON.parse(localStorage.getItem('chat_history') || '[]');
history.push({
    query: query,
    response: response,
    timestamp: Date.now()
});
localStorage.setItem('chat_history', JSON.stringify(history));
```

## Session Management

### Client-Side Sessions

Sessions are managed entirely client-side for privacy:

```javascript
// Session data structure
const session = {
    id: generateUUID(),
    started: Date.now(),
    messages: [],
    model: 'auto',
    settings: {
        temperature: 0.7,
        maxTokens: 1000,
        streamingEnabled: true
    }
};

// Store in localStorage
localStorage.setItem('current_session', JSON.stringify(session));

// No server-side session storage
// Each request is stateless
```

### Conversation Context

For multi-turn conversations:

```javascript
// Build context from history
function buildContext() {
    const session = JSON.parse(localStorage.getItem('current_session'));
    const context = session.messages.slice(-10); // Last 10 messages
    
    return {
        messages: context.map(msg => ({
            role: msg.role,
            content: msg.content
        })),
        model: session.model
    };
}

// Send with context
fetch('/v1/chat/completions', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
        ...buildContext(),
        messages: [...context.messages, {role: 'user', content: newQuery}]
    })
});
```

### Session Persistence

```javascript
// Export session
function exportSession() {
    const session = localStorage.getItem('current_session');
    const blob = new Blob([session], {type: 'application/json'});
    const url = URL.createObjectURL(blob);
    
    const a = document.createElement('a');
    a.href = url;
    a.download = `chat_session_${Date.now()}.json`;
    a.click();
}

// Import session
function importSession(file) {
    const reader = new FileReader();
    reader.onload = (e) => {
        localStorage.setItem('current_session', e.target.result);
        location.reload();
    };
    reader.readAsText(file);
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
  }],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 12,
    "total_tokens": 22
  }
}
```

### Streaming API

```javascript
// Server-Sent Events streaming
const response = await fetch('/v1/chat/completions', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
        model: 'llama-8b',
        messages: [{role: 'user', content: 'Tell me a story'}],
        stream: true
    })
});

const reader = response.body.getReader();
const decoder = new TextDecoder();

while (true) {
    const {done, value} = await reader.read();
    if (done) break;
    
    const chunk = decoder.decode(value);
    const lines = chunk.split('\n');
    
    for (const line of lines) {
        if (line.startsWith('data: ')) {
            const data = JSON.parse(line.slice(6));
            if (data.choices[0].delta.content) {
                console.log(data.choices[0].delta.content);
            }
        }
    }
}
```

### WebSocket Interface

```javascript
// WebSocket for bidirectional streaming
const ws = new WebSocket('ws://ch.at/ws');

ws.onopen = () => {
    ws.send(JSON.stringify({
        type: 'chat',
        model: 'llama-8b',
        message: 'Hello'
    }));
};

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    if (data.type === 'chunk') {
        document.getElementById('response').innerHTML += data.content;
    } else if (data.type === 'complete') {
        console.log('Response complete:', data.usage);
    }
};

ws.onerror = (error) => {
    console.error('WebSocket error:', error);
};
```

## Model Selection and Routing

### Model Discovery

```javascript
// Get available models
async function getAvailableModels() {
    const response = await fetch('/routing_table', {
        headers: {'Accept': 'application/json'}
    });
    const data = await response.json();
    
    return data.models.filter(m => m.healthy).map(m => ({
        id: m.id,
        name: m.name,
        family: m.family,
        tier: getTier(m.id)
    }));
}

function getTier(modelId) {
    const tiers = {
        fast: ['llama-8b', 'gpt-nano', 'claude-haiku'],
        balanced: ['llama-70b', 'gpt-mini', 'claude-sonnet'],
        frontier: ['llama-405b', 'gpt-5', 'claude-opus']
    };
    
    for (const [tier, models] of Object.entries(tiers)) {
        if (models.includes(modelId)) return tier;
    }
    return 'standard';
}
```

### Dynamic Model Selection UI

```javascript
// Populate model dropdown
async function populateModelSelect() {
    const models = await getAvailableModels();
    const select = document.getElementById('model-select');
    
    // Clear existing options
    select.innerHTML = '<option value="">Auto-select</option>';
    
    // Group by tier
    const tiers = {fast: [], balanced: [], frontier: []};
    models.forEach(m => {
        if (tiers[m.tier]) tiers[m.tier].push(m);
    });
    
    // Add grouped options
    for (const [tier, tierModels] of Object.entries(tiers)) {
        if (tierModels.length === 0) continue;
        
        const optgroup = document.createElement('optgroup');
        optgroup.label = tier.charAt(0).toUpperCase() + tier.slice(1);
        
        tierModels.forEach(model => {
            const option = document.createElement('option');
            option.value = model.id;
            option.textContent = model.name;
            optgroup.appendChild(option);
        });
        
        select.appendChild(optgroup);
    }
}
```

### Tier-Based Selection

```javascript
// Request by tier instead of specific model
async function chatWithTier(tier, message) {
    return fetch('/v1/chat/completions', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            model: `tier:${tier}`,  // e.g., "tier:fast"
            messages: [{role: 'user', content: message}]
        })
    });
}

// Usage
chatWithTier('fast', 'Quick question about JavaScript')
    .then(response => response.json())
    .then(data => console.log(data.choices[0].message.content));
```

## Channel Introspection

### Viewing Active Channels

```javascript
// Get channel information
async function getChannelInfo() {
    const response = await fetch('/routing_table', {
        headers: {'Accept': 'application/json'}
    });
    const data = await response.json();
    
    // Extract channel mappings
    const channels = {};
    data.models.forEach(model => {
        model.deployments.forEach(deployment => {
            const channel = deployment.channel;
            if (!channels[channel]) {
                channels[channel] = {
                    id: channel,
                    provider: getProviderName(channel),
                    models: []
                };
            }
            channels[channel].models.push(model.id);
        });
    });
    
    return channels;
}

function getProviderName(channel) {
    const mapping = {
        '2': 'Anthropic',
        '3': 'Google',
        '4': 'Azure',
        '8': 'OpenAI',
        '10': 'AWS Bedrock',
        '11': 'Azure OpenAI'
    };
    return mapping[channel] || 'Unknown';
}
```

### Channel Status Display

```html
<!-- Channel status widget -->
<div id="channel-status">
    <h3>Provider Status</h3>
    <div class="channels">
        <div class="channel online">
            <span class="indicator">●</span>
            Anthropic (Channel 2): 3 models
        </div>
        <div class="channel offline">
            <span class="indicator">●</span>
            OpenAI (Channel 8): 0 models
        </div>
    </div>
</div>

<style>
.channel.online .indicator { color: green; }
.channel.offline .indicator { color: red; }
</style>
```

## Banner System

### System Messages

The frontend can display important system messages:

```javascript
// Check for system banners
async function checkSystemBanners() {
    const response = await fetch('/system/banners');
    const banners = await response.json();
    
    banners.forEach(banner => {
        displayBanner(banner);
    });
}

function displayBanner(banner) {
    const div = document.createElement('div');
    div.className = `banner banner-${banner.type}`;
    div.innerHTML = `
        <strong>${banner.title}</strong>
        <p>${banner.message}</p>
        ${banner.dismissible ? '<button onclick="this.parentElement.remove()">×</button>' : ''}
    `;
    
    document.body.insertBefore(div, document.body.firstChild);
}

// Banner types
const bannerTypes = {
    info: {bg: '#e3f2fd', color: '#1976d2'},
    warning: {bg: '#fff3e0', color: '#f57c00'},
    error: {bg: '#ffebee', color: '#c62828'},
    success: {bg: '#e8f5e9', color: '#388e3c'}
};
```

### Terms of Service Banner

```javascript
// Check ToS acceptance
function checkToSAcceptance() {
    const acceptedVersion = localStorage.getItem('tos_accepted_version');
    
    fetch('/terms_of_service', {
        headers: {'Accept': 'application/json'}
    })
    .then(response => response.json())
    .then(tos => {
        if (tos.version !== acceptedVersion) {
            showToSBanner(tos);
        }
    });
}

function showToSBanner(tos) {
    const banner = document.createElement('div');
    banner.className = 'tos-banner';
    banner.innerHTML = `
        <h2>Terms of Service Updated</h2>
        <p>Version ${tos.version} - Effective ${tos.effective_date}</p>
        <p>${tos.agreement}</p>
        <div class="actions">
            <a href="/terms_of_service" target="_blank">Read Full Terms</a>
            <button onclick="acceptToS('${tos.version}')">Accept</button>
        </div>
    `;
    
    document.body.insertBefore(banner, document.body.firstChild);
}

function acceptToS(version) {
    localStorage.setItem('tos_accepted_version', version);
    document.querySelector('.tos-banner').remove();
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

```bash
# Build and run
docker build -t ch.at .
docker run -p 80:80 -p 443:443 -p 22:22 -p 53:53/udp ch.at
```

### Environment Configuration

```bash
# .env file for frontend settings
HIGH_PORT_MODE=false
ENABLE_LLM_AUDIT=false
DEBUG_MODE=false

# Model defaults for UI
DEFAULT_MODEL="llama-8b"
DEFAULT_TEMPERATURE="0.7"
DEFAULT_MAX_TOKENS="1000"

# UI feature flags
ENABLE_MODEL_SELECTION=true
ENABLE_CONVERSATION_HISTORY=true
ENABLE_STREAMING=true
ENABLE_WEBSOCKET=true
```

## Client Integration

### DoNutSentry Client

For DNS tunneling queries that exceed DNS limits:

```javascript
// DoNutSentry v2 Client
const DoNutSentryClient = require('./donutsentry-client');

const client = new DoNutSentryClient({
    domain: 'qp.ch.at',
    dnsServers: ['127.0.0.1:8053']
});

// Send large query via DNS
const response = await client.query(veryLongQuery);
console.log(response.response);

// DNS-HTTP Bridge for browsers
const bridge = new DNSHTTPBridge({
    dnsServer: '127.0.0.1:8053',
    httpPort: 8081
});

// Browser fetch via bridge
async function queryViaDNS(query) {
    const response = await fetch(`http://localhost:8081/dns/query`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({query, type: 'TXT'})
    });
    return response.json();
}
```

**Running DoNutSentry Bridge:**
```bash
# Start DNS-HTTP bridge
node dns-http-bridge.js

# Test with curl
curl -X POST http://localhost:8081/dns/query \
  -d '{"domain":"test.qp.ch.at","type":"TXT"}'
```

### Python Client

```python
import requests
import json

class ChatClient:
    def __init__(self, base_url="http://ch.at"):
        self.base_url = base_url
        self.session = requests.Session()
    
    def chat(self, message, model="llama-8b", stream=False):
        response = self.session.post(
            f"{self.base_url}/v1/chat/completions",
            json={
                "model": model,
                "messages": [{"role": "user", "content": message}],
                "stream": stream
            },
            stream=stream
        )
        
        if stream:
            return self._handle_stream(response)
        else:
            return response.json()["choices"][0]["message"]["content"]
    
    def _handle_stream(self, response):
        for line in response.iter_lines():
            if line.startswith(b'data: '):
                data = json.loads(line[6:])
                if data.get("choices", [{}])[0].get("delta", {}).get("content"):
                    yield data["choices"][0]["delta"]["content"]

# Usage
client = ChatClient()
response = client.chat("What is Python?")
print(response)
```

### JavaScript/Node.js Client

```javascript
class ChatClient {
    constructor(baseUrl = 'http://ch.at') {
        this.baseUrl = baseUrl;
    }
    
    async chat(message, options = {}) {
        const response = await fetch(`${this.baseUrl}/v1/chat/completions`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                model: options.model || 'llama-8b',
                messages: [{role: 'user', content: message}],
                temperature: options.temperature || 0.7,
                max_tokens: options.maxTokens || 1000,
                stream: options.stream || false
            })
        });
        
        if (options.stream) {
            return this.handleStream(response);
        } else {
            const data = await response.json();
            return data.choices[0].message.content;
        }
    }
    
    async *handleStream(response) {
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        
        while (true) {
            const {done, value} = await reader.read();
            if (done) break;
            
            const chunk = decoder.decode(value);
            const lines = chunk.split('\n');
            
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    try {
                        const data = JSON.parse(line.slice(6));
                        if (data.choices?.[0]?.delta?.content) {
                            yield data.choices[0].delta.content;
                        }
                    } catch (e) {
                        // Ignore parsing errors
                    }
                }
            }
        }
    }
}

// Usage
const client = new ChatClient();
const response = await client.chat('What is JavaScript?');
console.log(response);
```

### Command Line Client

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

# Interactive mode
if [ $# -eq 0 ]; then
    echo "ch.at CLI - Type 'exit' to quit"
    while true; do
        read -p "> " input
        [ "$input" = "exit" ] && break
        chat "$input"
    done
else
    # Single query mode
    chat "$*"
fi
```

## Testing and Debugging

### Frontend Testing

```javascript
// Test suite for frontend
describe('Chat Frontend', () => {
    test('should load without JavaScript', async () => {
        const response = await fetch('http://localhost:8080');
        const html = await response.text();
        expect(html).toContain('<form');
        expect(html).toContain('name="q"');
    });
    
    test('should handle form submission', async () => {
        const response = await fetch('http://localhost:8080?q=test');
        const html = await response.text();
        expect(response.status).toBe(200);
        expect(html).toContain('response');
    });
    
    test('should return valid API response', async () => {
        const response = await fetch('http://localhost:8080/v1/chat/completions', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                model: 'llama-8b',
                messages: [{role: 'user', content: 'test'}]
            })
        });
        
        const data = await response.json();
        expect(data).toHaveProperty('choices');
        expect(data.choices[0]).toHaveProperty('message');
    });
});
```

### Browser Console Debugging

```javascript
// Debug helpers for browser console
window.chatDebug = {
    // Check routing table
    async checkRouting() {
        const response = await fetch('/routing_table', {
            headers: {'Accept': 'application/json'}
        });
        const data = await response.json();
        console.table(data.models);
        return data;
    },
    
    // Test model availability
    async testModel(modelId) {
        const response = await fetch('/v1/chat/completions', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                model: modelId,
                messages: [{role: 'user', content: 'test'}],
                max_tokens: 10
            })
        });
        
        if (response.ok) {
            const data = await response.json();
            console.log(`✅ ${modelId} working:`, data.choices[0].message.content);
        } else {
            console.error(`❌ ${modelId} failed:`, await response.text());
        }
    },
    
    // Clear all local storage
    clearAll() {
        localStorage.clear();
        sessionStorage.clear();
        console.log('All storage cleared');
    },
    
    // Export conversation
    exportChat() {
        const history = localStorage.getItem('chat_history');
        const blob = new Blob([history], {type: 'application/json'});
        const url = URL.createObjectURL(blob);
        window.open(url);
    }
};

// Usage in console:
// chatDebug.checkRouting()
// chatDebug.testModel('llama-8b')
```

### Network Debugging

```bash
# Monitor WebSocket connections
wscat -c ws://localhost:8080/ws

# Test streaming endpoint
curl -N -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"llama-8b","messages":[{"role":"user","content":"count to 10"}],"stream":true}'

# Check response headers
curl -I http://localhost:8080/

# Test CORS headers (if enabled)
curl -H "Origin: http://example.com" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type" \
  -X OPTIONS \
  http://localhost:8080/v1/chat/completions -v
```

### Performance Monitoring

```javascript
// Frontend performance metrics
class PerformanceMonitor {
    constructor() {
        this.metrics = [];
    }
    
    async measureRequest(fn, label) {
        const start = performance.now();
        try {
            const result = await fn();
            const duration = performance.now() - start;
            
            this.metrics.push({
                label,
                duration,
                timestamp: Date.now(),
                success: true
            });
            
            console.log(`${label}: ${duration.toFixed(2)}ms`);
            return result;
        } catch (error) {
            const duration = performance.now() - start;
            
            this.metrics.push({
                label,
                duration,
                timestamp: Date.now(),
                success: false,
                error: error.message
            });
            
            console.error(`${label} failed after ${duration.toFixed(2)}ms:`, error);
            throw error;
        }
    }
    
    getStats() {
        const successful = this.metrics.filter(m => m.success);
        const failed = this.metrics.filter(m => !m.success);
        
        return {
            total: this.metrics.length,
            successful: successful.length,
            failed: failed.length,
            avgDuration: successful.reduce((a, b) => a + b.duration, 0) / successful.length,
            p95: this.percentile(successful.map(m => m.duration), 0.95),
            p99: this.percentile(successful.map(m => m.duration), 0.99)
        };
    }
    
    percentile(arr, p) {
        const sorted = arr.sort((a, b) => a - b);
        const index = Math.ceil(sorted.length * p) - 1;
        return sorted[index];
    }
}

// Usage
const monitor = new PerformanceMonitor();

await monitor.measureRequest(
    () => fetch('/v1/chat/completions', {method: 'POST', ...}),
    'API Request'
);

console.table(monitor.getStats());
```

## Accessibility

### Keyboard Navigation

```javascript
// Enhanced keyboard support
document.addEventListener('keydown', (e) => {
    // Ctrl+Enter to submit
    if (e.ctrlKey && e.key === 'Enter') {
        const textarea = document.querySelector('textarea[name="q"]');
        if (textarea && textarea.value) {
            document.querySelector('form').submit();
        }
    }
    
    // Escape to clear input
    if (e.key === 'Escape') {
        const textarea = document.querySelector('textarea[name="q"]');
        if (textarea) {
            textarea.value = '';
            textarea.focus();
        }
    }
    
    // Ctrl+L to clear conversation
    if (e.ctrlKey && e.key === 'l') {
        e.preventDefault();
        document.getElementById('response').innerHTML = '';
    }
});
```

### Screen Reader Support

```html
<!-- ARIA labels for accessibility -->
<div role="main" aria-label="Chat Interface">
    <form role="form" aria-label="Message Input">
        <label for="message-input" class="sr-only">Enter your message</label>
        <textarea 
            id="message-input"
            name="q" 
            aria-label="Message input"
            aria-required="true"
            placeholder="Ask anything...">
        </textarea>
        <button type="submit" aria-label="Send message">Send</button>
    </form>
    
    <div id="response" 
         role="region" 
         aria-label="Chat response"
         aria-live="polite"
         aria-atomic="false">
        <!-- Response content -->
    </div>
</div>
```

---

*The ch.at frontend provides multiple access methods while maintaining privacy, accessibility, and transparency. Whether accessed through a browser, terminal, or API, users get consistent, reliable access to LLM capabilities.*