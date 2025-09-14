# ch.at v2.1 - Universal Basic Intelligence

A lightweight language model chat service accessible through HTTP, SSH, DNS, and API. One binary, no JavaScript, no tracking.

## 🔒 Privacy & Telemetry - Clear Disclosure

**YOUR PRIVACY IS PROTECTED BY DEFAULT:**
- **Audit logging is DISABLED by default** - no conversation storage unless you explicitly enable it
- **The production ch.at service NEVER logs user queries** - we only track server load/balancing metrics
- **Zero-knowledge telemetry** - we use SHA256 hashes (one-way, irreversible) for rate limiting and service health
- **You control your data** - all history is client-side, no accounts, no tracking

**Why Some Telemetry Exists:**
Every production service needs basic metrics for:
- Rate limiting (preventing abuse)
- Load balancing (distributing traffic)
- Service health (knowing when things break)
- Token counting (understanding usage patterns)

**For Developers:**
- Full debug auditing is available but **must be explicitly enabled** via `ENABLE_LLM_AUDIT=true`
- UDP telemetry beacons provide optimal breakpoints for debugging and orchestration
- These beacons are gratis parts of Hypernym's extended tech for maximally agentic orchestration
- See [System Architecture Manual](documentation/MANUAL_2025_09_10_SYSTEM_ARCHITECTURE_COMPLETE.md) for complete telemetry details

## 📚 Developer Documentation

For deep technical understanding, refer to our comprehensive manuals:

- **[System Architecture Manual](documentation/MANUAL_2025_09_10_SYSTEM_ARCHITECTURE_COMPLETE.md)** - Complete system design, telemetry, deployment strategies, and monitoring
- **[Backend & Routing Manual](documentation/MANUAL_2025_09_10_BACKEND_ROUTING_BASELINE_COMPLETE.md)** - Router architecture, provider system, baseline fallback, and service integration  
- **[Frontend Operations Manual](documentation/MANUAL_2025_09_10_FRONTEND_OPERATIONS_COMPLETE.md)** - Web interface, API usage, client integration, and testing

## Usage

```bash
# Web (no JavaScript)
open https://ch.at

# Terminal
curl ch.at/?q=hello             # Streams response with curl's default buffering
curl -N ch.at/?q=hello          # Streams response without buffering (smoother)
curl ch.at/what-is-rust         # Path-based (cleaner URLs, hyphens become spaces)
ssh ch.at

# DNS tunneling
dig @ch.at "what-is-2+2" TXT

# API (OpenAI-compatible, see https://platform.openai.com/docs/api-reference/chat/create)
curl ch.at/v1/chat/completions --data '{"messages": [{"role": "user", "content": "What is curl? Be brief."}]}'
```

## Design

- ~1,300 lines of Go, three direct dependencies
- Single static binary
- No accounts, no logs, no tracking
- Configuration through source code (edit and recompile)
- Modern router architecture with per-transaction routing
- Automatic failover to baseline providers

## Privacy

Privacy by design:

- No authentication or user tracking
- No server-side conversation storage
- No logs whatsoever (unless explicitly enabled)
- Web history stored client-side only

**Privacy Standard: Zero-Knowledge Telemetry**

The ch.at service uses privacy-preserving telemetry:
- Only SHA256 hashes of content (irreversible)
- Token counts for usage metrics
- No actual queries or responses stored
- No IP addresses or user identification
- GDPR compliant - no personal data processed

**One-liner:** "We count tokens and hash content - your actual queries are never stored."

**⚠️ PRIVACY WARNING**: Your queries are sent to LLM providers (OpenAI, Anthropic, etc.) who may log and store them according to their policies. While ch.at doesn't log anything, the upstream providers might. Never send passwords, API keys, or sensitive information.

**Current Production Model**: Configurable - supports OpenAI, Anthropic, OneAPI gateway, and more.

## Installation

### Quick Start (Baseline - Just OpenAI)

The simplest deployment - just add your OpenAI key:

```bash
# Clone the repository
git clone https://github.com/hypernym-ai/ch.at
cd ch.at

# Copy the example config (already has OpenAI endpoint!)
cp .env.example .env

# Edit .env and add your OpenAI API key:
BASIC_OPENAI_KEY="sk-your-openai-api-key-here"
# BASIC_OPENAI_URL and BASIC_OPENAI_MODEL are already set in .env.example

# Build and run
go build -o ch.at .
./ch.at

# That's it! Visit http://localhost:8080
```

### Advanced Setup (Multi-Model with OneAPI)

For access to multiple models (Claude, Llama, Gemini), use [OneAPI](https://github.com/songquanpeng/one-api):

```bash
# Configure OneAPI settings in .env
ONE_API_URL=http://localhost:3000
ONE_API_KEY=sk-your-one-api-key

# OneAPI provides channels for different providers
# Channel 2: Anthropic (Claude)
# Channel 3: Google (Gemini)  
# Channel 4: Azure (Llama)
# Channel 8: OpenAI (GPT)
# Channel 10: AWS Bedrock
# Channel 11: Azure OpenAI

# Build and run
go build -o ch.at .
sudo ./ch.at  # Needs root for ports 80/443/53/22
```

### Running with Screen Session

For development or long-running sessions, use screen with proper environment variables:

```bash
# Development mode with high ports (no sudo needed)
screen -dmS chat-server bash -c 'HIGH_PORT_MODE=true ./ch.at 2>&1 | tee -a /tmp/ch.at.log'

# Production mode (requires sudo)
sudo screen -dmS chat-server bash -c './ch.at 2>&1 | tee -a /tmp/ch.at.log'

# View the running session
screen -r chat-server

# Detach from session: Ctrl-A then D
# Stop the server
screen -S chat-server -X quit
```

### Testing

```bash
# Build the self-test tool
go build -o selftest ./cmd/selftest

# Run all protocol tests
./selftest http://localhost

# Test specific queries
curl localhost/what-is-go
curl localhost/?q=hello
```

### High Port Configuration

For development or non-privileged operation, use the HIGH_PORT_MODE environment variable:

```bash
# Run on high ports (no sudo required)
HIGH_PORT_MODE=true ./ch.at

# This sets:
# HTTP:  8080 (instead of 80)
# HTTPS: 8443 (instead of 443)
# SSH:   2222 (instead of 22)
# DNS:   8053 (instead of 53)
```

Test the service:
```bash
# Check health status
curl http://localhost:8080/health

# Test chat functionality
./selftest http://localhost:8080
```

## Deployment

### Nanos Unikernel (Recommended)

Deploy as a minimal VM with just your app:

```bash
# Install OPS
curl https://ops.city/get.sh -sSfL | sh

# Create ops.json with your ports
echo '{"RunConfig":{"Ports":["80","443","22","53"]}}' > ops.json

# Test locally
CGO_ENABLED=0 GOOS=linux go build
ops run chat -c ops.json

# Deploy to AWS
ops image create chat -c ops.json -t aws
ops instance create chat -t aws

# Deploy to Google Cloud
ops image create chat -c ops.json -t gcp
ops instance create chat -t gcp
```

### Traditional Deployment

```bash
# Systemd service
sudo cp ch.at /usr/local/bin/
sudo systemctl enable ch.at.service
```

### Docker

```sh
cp .env.example .env # fill your api key
docker compose up -d
```

## Available Models

### Currently Working Models via OneAPI

**Claude Models (Channel 10 - AWS Bedrock):**
- `claude-3.5-haiku` - Fast tier (200K context, 8K output)
- `claude-3.5-sonnet` - Balanced tier (200K context, 8K output)
- `claude-3.7-sonnet` - Latest balanced tier (200K context, 131K output)
- `claude-4-sonnet` - Frontier tier (200K context, 65K output)
- `claude-4-opus` - Frontier tier (200K context, 65K output)
- `claude-4.1-opus` - Highest frontier tier (200K context, 65K output)

**Gemini Models (Channel 3 - Google):**
- `gemini-1.5-flash` - Fast tier (1M input, 8K output)
- `gemini-2.5-flash` - Balanced tier (1M input, 65K output)
- `gemini-2.5-pro` - Frontier tier (1M input, 65K output - set max_tokens ≥1000 for proper responses)

**GPT Models (Various Channels):**
- `gpt-4.1-nano` - Economy tier (Channel 11 - Azure)
- `gpt-mini` (gpt-4.1-mini) - Balanced tier
- `gpt-41` (gpt-4.1) - Advanced tier
- `gpt-5-nano` - Fast GPT-5 tier
- `gpt-5-mini` - Balanced GPT-5 tier
- `gpt-5` - Full GPT-5

**Llama Models (Channels 4/10 - Azure/Bedrock):**
- `llama-8b` - Small, fast (default model)
- `llama-70b` - Large, capable
- `llama-405b` - Largest Llama model
- `llama-scout` - Llama 4 Scout variant
- `llama-maverick` - Llama 4 Maverick variant

## Configuration

### Environment Variables (.env)

```bash
# BASIC SETUP (Baseline fallback - always works)
BASIC_OPENAI_KEY=sk-your-key-here
BASIC_OPENAI_URL=https://api.openai.com/v1/chat/completions
BASIC_OPENAI_MODEL=gpt-4.1-nano

# ADVANCED: OneAPI for multi-model support (optional)
ONE_API_URL=http://localhost:3000
ONE_API_KEY=sk-your-one-api-key

# Port configuration
HIGH_PORT_MODE=true  # Use 8080/8443/2222/8053 instead of 80/443/22/53

# Privacy settings
ENABLE_LLM_AUDIT=false  # Keep false for privacy (default)

# Service-specific models (optional)
DNS_LLM_MODEL=gpt-4.1-nano      # Short responses for DNS
SSH_LLM_MODEL=claude-3.5-haiku  # Terminal sessions
DONUTSENTRY_LLM_MODEL=llama-8b
DONUTSENTRY_V2_LLM_MODEL=claude-3.5-haiku
```

Edit constants in source files:
- Ports: `chat.go` (set to 0 to disable)
- Rate limits: `util.go`
- Remove service: Delete its .go file

## Transparency Endpoints

Live system status and configuration:

### `/routing_table` - Model Routing and Privacy Status
```bash
# HTML view
curl http://localhost:8080/routing_table

# JSON format
curl http://localhost:8080/routing_table -H "Accept: application/json"

# Check privacy status
curl -s http://localhost:8080/routing_table -H "Accept: application/json" | jq .audit_enabled
# Should return: false (unless you explicitly enabled it)
```

### `/terms_of_service` - Terms and Privacy Policy
```bash
# View current terms
curl http://localhost:8080/terms_of_service

# Check version
curl -s http://localhost:8080/terms_of_service -H "Accept: application/json" | jq .version
```

### `/health` - System Health
```bash
curl http://localhost:8080/health
```

## DoNutSentry - DNS Tunneling for Large Queries

DoNutSentry enables sending queries of any size through DNS by implementing paging, sessions, and encryption.

### DoNutSentry v1 - Simple DNS Tunneling

Basic DNS tunneling for queries that fit in standard DNS packets:

```bash
# Query format: <query>.q.ch.at
dig @ch.at "what-is-dns.q.ch.at" TXT

# Features:
# - Simple query/response
# - Basic XOR encryption
# - Limited to DNS packet size (~255 chars)
```

### DoNutSentry v2 - Advanced DNS Tunneling

Session-based protocol for unlimited query sizes:

```bash
# Domain: *.qp.ch.at
# Features:
# - Session management with key exchange
# - Query/response paging (unlimited size)
# - XOR encryption with perfect forward secrecy
# - Async processing (handles DNS timeouts)
# - Zero-overhead encryption

# Protocol Flow:
# 1. Initialize session: <client_keys>.init.qp.ch.at
# 2. Send query pages: <session>.<page>.<data>.qp.ch.at
# 3. Execute query: <session>.<total_pages>.exec.qp.ch.at
# 4. Poll status: <session>.status.qp.ch.at
# 5. Retrieve pages: <session>.page.<N>.qp.ch.at
```

**Architecture Diagram:**

![DoNutSentry v2 Architecture](documentation/donutsentry_v2_architecture.png)
*[Static image placeholder - add your diagram here]*

**Video Demo:**

[![DoNutSentry v2 Demo](https://img.youtube.com/vi/YOUR_VIDEO_ID/0.jpg)](https://www.youtube.com/watch?v=YOUR_VIDEO_ID)
*[YouTube video link placeholder - add your video ID]*

**Client Libraries:**

```javascript
// Node.js client
const DoNutSentryClient = require('@ch.at/donutsentry-client');
const client = new DoNutSentryClient({
    domain: 'qp.ch.at',
    dnsServers: ['127.0.0.1:8053']
});

const response = await client.query('Very long query that exceeds DNS limits...');
console.log(response);
```

**DNS-HTTP Bridge (for browsers):**

```bash
# Start bridge
node dns-http-bridge.js

# Browser can now query via HTTP
fetch('http://localhost:8081/dns/query', {
    method: 'POST',
    body: JSON.stringify({query: 'test', type: 'TXT'})
})
```

For complete DoNutSentry documentation, see [DoNutSentry v2 Manual](documentation/DoNutSentry_v2_Complete_Manual.md).

## Limitations

- **DNS**: Responses limited to ~500 bytes. Complex queries may time out after 4s. DNS queries automatically request concise, plain-text responses
- **History**: Limited to 64KB to ensure compatibility across systems
- **Rate limiting**: Basic IP-based limiting to prevent abuse
- **No encryption**: SSH is encrypted, but HTTP/DNS are not

## License

MIT License - see LICENSE file

## Contributing

Before adding features:
- Does it increase accessibility?
- Is it under 50 lines?
- Is it necessary?

---

*ch.at v2.1 - Universal access to language models with privacy, transparency, and resilience.*

*Built by [Hypernym](https://hypernym.ai) - Maximally agentic orchestration through privacy-preserving telemetry.*