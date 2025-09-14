# DoNutSentry - DNS-based LLM Query Protocol

DoNutSentry extends ch.at to work through any DNS resolver, enabling LLM queries from restricted networks, embedded devices, and firewalled environments.

## Overview

DoNutSentry solves a critical limitation: ch.at currently requires direct DNS queries to its IP address, which fails behind corporate firewalls, captive portals, and restricted networks. By implementing recursive DNS resolver support with wildcard subdomains, DoNutSentry makes ch.at universally accessible.

## Features

### 1. Universal DNS Access
- Works through any DNS resolver (corporate, public, local)
- Bypasses firewall restrictions that block direct DNS
- Compatible with captive portals and restricted networks

### 2. Smart Query Encoding
- **Simple mode**: Clean URLs for basic queries (`what-is-rust.q.ch.at`)
- **Base32 mode**: Handles special characters and Unicode
- **Session mode**: Automatic chunking for queries of any size

### 3. Anonymous Sessions
- RSA key exchange for secure session establishment
- No user tracking or authentication required
- Ephemeral sessions with automatic cleanup

## How It Works

### Simple Queries (< 50 chars, alphanumeric)
```
User: "what is dns"
DNS:  what-is-dns.q.ch.at
Response: DNS TXT record with answer
```

### Complex Queries (special characters, Unicode)
```
User: "What is AI? 你好"
DNS:  j5gxc4tfoqwwe33pnm.q.ch.at (base32 encoded)
Response: DNS TXT record with answer
```

### Large Queries (session mode)
```
1. Init:    <pubkey_hash>.init.q.ch.at → encrypted session_id
2. Chunk 1: <session_id>.0.<data>.q.ch.at → ACK
3. Chunk 2: <session_id>.1.<data>.q.ch.at → ACK
4. Execute: <session_id>.2.exec.q.ch.at → Full response
```

## Client Usage

### JavaScript/TypeScript

```javascript
import { DoNutSentryClient } from '@ch.at/donutsentry-client';

const client = new DoNutSentryClient();

// Simple query
const result = await client.query('what is dns');
console.log(result.response);

// Complex query - automatically handled
const result2 = await client.query('Explain quantum computing in detail...');
console.log(result2.response);
```

### Command Line

```bash
# Simple query
dig what-is-rust.q.ch.at TXT

# Base32 encoded query
echo -n "What is AI?" | base32 | tr -d '=' | tr '[:upper:]' '[:lower:]'
dig <encoded>.q.ch.at TXT
```

## Technical Specifications

### DNS Protocol
- Wildcard domain: `*.q.ch.at`
- Label limit: 63 characters per label
- Total domain limit: 255 characters
- Response format: DNS TXT records (up to 4KB)

### Encoding Methods
1. **Simple**: Replace spaces with hyphens, lowercase, alphanumeric only
2. **Base32**: RFC 4648 standard, DNS-safe alphabet, no padding

### Session Protocol
- Session ID: 32 bytes (cryptographically random)
- Key exchange: RSA-2048 with OAEP padding
- Chunk format: `<session_id>.<chunk_num>.<data>.q.ch.at`
- Max chunks: Limited by server memory/timeout

## Server Implementation Guide

### 1. DNS Configuration
```
; Delegate wildcard to ch.at nameserver
*.q.ch.at.    IN    NS    ns1.ch.at.
```

### 2. Handler Implementation (Go)
```go
func handleDoNutSentryQuery(w dns.ResponseWriter, r *dns.Msg, q dns.Question) {
    fullName := strings.ToLower(q.Name)
    
    // Strip .q.ch.at suffix
    subdomain := strings.TrimSuffix(fullName, ".q.ch.at.")
    
    if strings.HasSuffix(subdomain, ".init") {
        handleSessionInit(w, r, subdomain)
    } else if strings.HasSuffix(subdomain, ".exec") {
        handleSessionExec(w, r, subdomain)
    } else if isSessionChunk(subdomain) {
        handleSessionChunk(w, r, subdomain)
    } else {
        // Simple query
        handleSimpleQuery(w, r, subdomain)
    }
}
```

### 3. Session Storage
```go
type Session struct {
    ID        string
    PublicKey *rsa.PublicKey
    Chunks    map[int]string
    CreatedAt time.Time
}

var sessions = &sync.Map{} // Thread-safe session storage
```

## Security Considerations

1. **Query Privacy**: Queries visible to DNS resolvers in path
2. **Session Security**: RSA encryption for session ID exchange
3. **Rate Limiting**: Per-IP limits to prevent abuse
4. **Input Validation**: Strict base32 validation, size limits

## Limitations

- DNS queries are not encrypted (except session ID)
- 4KB maximum response size (DNS TXT limit)
- Queries visible to network administrators
- Subject to DNS caching behavior

## Performance

- Simple query: ~50-200ms (standard DNS resolution)
- Session init: +~100ms (RSA operations)
- Chunk transmission: ~50ms per chunk
- Total for large query: 500ms-2s depending on size

## Directory Structure

```
DoNutSentry/
├── README.md                           # This file
├── DoNutSentry_v1_workflow.md         # Implementation specification
├── client-js/                         # JavaScript/TypeScript client
│   ├── src/                          # Source code
│   ├── package.json                  # NPM package
│   └── README.md                     # Client documentation
├── __production_steps/               # Implementation history
└── roadmap_sweep_processor/          # Future features (v2+)
```

## Future Enhancements (v2+)

- Encrypted queries using sweep space protocol
- Multi-resolution semantic embeddings
- Agent-to-agent communication
- Distributed session state

## Contributing

1. Server implementation in `dns.go`
2. Client libraries for other languages
3. Performance optimizations
4. Security auditing

## License

MIT - Same as ch.at