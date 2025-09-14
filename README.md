# ch.at - Universal Basic Intelligence

A lightweight language model chat service accessible through HTTP, SSH, DNS, and API. One binary, no JavaScript, no tracking.

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


## Privacy

Privacy by design:

- No authentication or user tracking
- No server-side conversation storage
- No logs whatsoever
- Web history stored client-side only

**⚠️ PRIVACY WARNING**: Your queries are sent to LLM providers (OpenAI, Anthropic, etc.) who may log and store them according to their policies. While ch.at doesn't log anything, the upstream providers might. Never send passwords, API keys, or sensitive information.

**Current Production Model**: OpenAI's GPT-4o. We plan to expand model access in the future.

## Installation

### Quick Start

```bash
# Configure your API key
# Supports OpenAI, Anthropic Claude, or local models (Ollama)
# Option 1: OpenAI API
export API_KEY="YOUR_API_KEY_HERE"
export API_URL="https://api.openai.com/v1/chat/completions"
export MODEL_NAME="gpt-3.5-turbo" # or gpt-4, etc.
# Option 2: Anthropic Claude API 
export API_KEY="YOUR_API_KEY_HERE"
export API_URL="https://api.anthropic.com/v1/messages"
export MODEL_NAME="claude-3-haiku" # or claude-3-opus, etc.
# Option 3: Local LLM 
export API_KEY="" # No API key needed for local models
export API_URL="http://localhost:11434/api/chat" # Ollama example
export MODEL_NAME="llama2" # or mixtral, phi, etc.

# For HTTPS, you'll need cert.pem and key.pem files:
# Option 1: Use Let's Encrypt (recommended for production)
# Option 2: Use your existing certificates
# Option 3: Self-signed for testing:
#   openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

# Build and run
go build -o chat .
sudo ./chat  # Needs root for ports 80/443/53/22
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
HIGH_PORT_MODE=true ./chat

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

### Deployment

#### Nanos Unikernel (Recommended)

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

#### Traditional Deployment

```bash
# Systemd service
sudo cp chat /usr/local/bin/
sudo systemctl enable chat.service
```

#### Docker

```sh
cp .env.example .env # fill your api key
docker compose up -d
```

## Configuration

Edit constants in source files:
- Ports: `chat.go` (set to 0 to disable)
- Rate limits: `util.go`
- Remove service: Delete its .go file

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

