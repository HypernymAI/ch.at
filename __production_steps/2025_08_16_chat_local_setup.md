# Running ch.at Locally - Setup Guide

## Issues Encountered

1. **DNS Port Binding**: Port 53 requires root/sudo
2. **Missing API Credentials**: Needs valid LLM API key
3. **Process Management**: Background processes hanging
4. **DNS Testing**: Local DNS queries timing out

## Proper Setup Steps

### 1. Configure Environment Variables

```bash
# For OpenAI
export API_KEY="your-actual-openai-key"
export API_URL="https://api.openai.com/v1/chat/completions"
export MODEL_NAME="gpt-3.5-turbo"

# For Local Ollama
export API_KEY=""  # Empty for local
export API_URL="http://localhost:11434/api/chat"
export MODEL_NAME="llama2"  # or whatever model you have

# For Anthropic Claude
export API_KEY="your-anthropic-key"
export API_URL="https://api.anthropic.com/v1/messages"
export MODEL_NAME="claude-3-haiku"
```

### 2. Modify Ports for Non-Root Testing

Edit `chat.go`:
```go
const (
    HTTP_PORT  = 8080  // Instead of 80
    HTTPS_PORT = 0     // Disabled
    SSH_PORT   = 2222  // Instead of 22
    DNS_PORT   = 5353  // Instead of 53
)
```

### 3. Build and Run

```bash
# Build
go build -o chat .

# Run without sudo (high ports)
./chat

# Or run with sudo (standard ports)
sudo ./chat
```

### 4. Test Each Service

```bash
# Test HTTP
curl http://localhost:8080/?q=hello

# Test SSH
ssh -p 2222 localhost

# Test DNS (standard port)
dig @localhost "test" TXT

# Test DNS (high port)
dig @localhost -p 5353 "test" TXT
```

## Common Problems and Solutions

### Problem: DNS queries timing out
**Solution**: Make sure you're querying with `.ch.at` suffix:
```bash
# Wrong
dig @localhost -p 5353 "test" TXT

# Right (current implementation)
dig @localhost -p 5353 "test.ch.at" TXT
```

### Problem: "API key required" errors
**Solution**: Set proper environment variables before running

### Problem: Process hangs when running
**Solution**: Run in foreground first to see errors:
```bash
./chat  # See errors directly
```

### Problem: "Address already in use"
**Solution**: Kill existing processes:
```bash
ps aux | grep chat | grep -v grep
kill <PID>
```

### Problem: DNS not responding on macOS
**Solution**: Check if mDNSResponder is using port 5353:
```bash
sudo lsof -i :5353
# If mDNSResponder is using it, choose different port like 5354
```

## Recommended Development Setup

1. **Use high ports** (no sudo required)
2. **Run in foreground** during development
3. **Use local LLM** (Ollama) to avoid API costs
4. **Enable only needed services** (set others to 0)

Example minimal config for DNS testing:
```go
const (
    HTTP_PORT  = 0     // Disabled
    HTTPS_PORT = 0     // Disabled
    SSH_PORT   = 0     // Disabled
    DNS_PORT   = 5354  // Custom DNS port
)
```

## Testing DoNutSentry Locally

Once ch.at is running with DNS enabled:

```javascript
// Test with local server
const client = new DoNutSentryClient({
    domain: 'q.localhost',  // Would need server support
    dnsServers: ['127.0.0.1:5354']
});
```

Note: Current ch.at implementation requires `.ch.at` suffix, so local testing needs server-side modifications to support custom domains or wildcard handling.

## Next Steps for Local Testing

1. Modify `dns.go` to support configurable domain suffix
2. Add wildcard subdomain handling
3. Implement DoNutSentry query detection
4. Test with local client

The client is ready - the server needs updates to support the new protocol.