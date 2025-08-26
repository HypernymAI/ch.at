# DoNutSentry v1 - Recursive DNS Resolver Support

## Problem Statement

Currently, ch.at requires direct DNS queries to its IP address:
```bash
dig @ch.at "what is rust" TXT
```

This fails in many environments:
- Corporate networks that force all DNS through internal resolvers
- Firewalls that block external DNS servers
- Networks that only allow DNS to specific resolvers (8.8.8.8, 1.1.1.1)
- Captive portals that intercept all DNS

Users behind these restrictions cannot access ch.at via DNS, limiting the protocol's universality.

## Solution: Recursive DNS Support

Enable queries through ANY DNS resolver by implementing wildcard subdomain handling. Instead of querying ch.at directly, users query subdomains that get routed through standard DNS infrastructure.

## Technical Implementation

### 1. DNS Configuration

Configure authoritative DNS for ch.at:
```
; Delegate all *.q.ch.at to our nameserver
*.q.ch.at.    IN    NS    ns1.ch.at.
*.q.ch.at.    IN    NS    ns2.ch.at.

; Nameservers
ns1.ch.at.    IN    A     34.28.5.90
ns2.ch.at.    IN    A     34.28.5.91  ; redundancy
```

This makes ch.at's DNS server authoritative for all subdomains under q.ch.at.

### 2. Query Encoding Formats

#### Simple ASCII Queries
For basic queries without special characters:
```bash
# Old way (direct)
dig @ch.at "what is rust" TXT

# v1 way (recursive)
dig what-is-rust.q.ch.at TXT
```

Encoding rules:
- Replace spaces with hyphens
- Remove punctuation
- Lowercase everything

#### Base32 Encoded Queries
For queries with special characters, spaces, or non-ASCII:
```bash
# Query: "What is machine learning?"
# Step 1: Convert to bytes
echo -n "What is machine learning?" | xxd -p
# 57686174206973206d616368696e65206c6561726e696e673f

# Step 2: Base32 encode (RFC 4648, no padding)
echo -n "What is machine learning?" | base32 | tr -d '='
# J5GXC4TFOQWWC3TTMUWXIZLTOQWWC3TTORUW4ZDFNZSA

# Step 3: DNS query (lowercase for DNS compatibility)
dig j5gxc4tfoqwwc3ttmuwxizltoqwwc3ttoruw4zdfnzsa.q.ch.at TXT
```

#### Compressed Queries (LZSS)
For queries exceeding DNS label limits (63 characters):

```python
# Compression example
import lzss

query = "Explain how large language models work, including transformer architecture, attention mechanisms, and training process"
compressed = lzss.compress(query.encode())
# Result: ~30 bytes (from 115 bytes)

# Base32 encode
import base64
encoded = base64.b32encode(compressed).decode().rstrip('=').lower()
# Result: "nfxgc4tf..." (fits in single DNS label)

# DNS query
dig {encoded}.q.ch.at TXT
```

### 3. Server-Side Implementation

#### DNS Handler Modifications
```go
// dns.go modifications
func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
    for _, q := range r.Question {
        // Check if it's a DoNutSentry query
        if strings.HasSuffix(q.Name, ".q.ch.at.") {
            handleDoNutSentryQuery(w, r, q)
            return
        }
        
        // Handle traditional direct queries
        handleDirectQuery(w, r, q)
    }
}

func handleDoNutSentryQuery(w dns.ResponseWriter, r *dns.Msg, q dns.Question) {
    // Extract encoded query from subdomain
    fullName := strings.ToLower(q.Name)
    subdomain := strings.TrimSuffix(fullName, ".q.ch.at.")
    
    // Try decoding strategies in order
    var query string
    var err error
    
    // 1. Try simple hyphenated format
    if isSimpleFormat(subdomain) {
        query = strings.ReplaceAll(subdomain, "-", " ")
    } else {
        // 2. Try base32 decoding
        decoded, err := base32Decode(subdomain)
        if err == nil {
            // 3. Check if decoded data is compressed
            if isCompressed(decoded) {
                query, err = decompressLZSS(decoded)
            } else {
                query = string(decoded)
            }
        }
    }
    
    if err != nil {
        sendErrorResponse(w, r, "Invalid query encoding")
        return
    }
    
    // Process query through LLM
    response := processLLMQuery(query)
    
    // Send response
    sendTXTResponse(w, r, response)
}

func base32Decode(s string) ([]byte, error) {
    // Handle DNS case insensitivity
    s = strings.ToUpper(s)
    
    // Add padding if needed
    padding := (8 - len(s)%8) % 8
    s = s + strings.Repeat("=", padding)
    
    return base32.StdEncoding.DecodeString(s)
}

func isCompressed(data []byte) bool {
    // LZSS compressed data has identifiable patterns
    // Check for compression markers
    return len(data) > 2 && data[0] == 0x1F && data[1] == 0x8B
}

func decompressLZSS(data []byte) (string, error) {
    // Implement LZSS decompression
    // Return decompressed string
}
```

#### Response Handling
```go
func sendTXTResponse(w dns.ResponseWriter, r *dns.Msg, response string) {
    m := new(dns.Msg)
    m.SetReply(r)
    
    // Limit response for DNS compatibility
    if len(response) > 500 {
        response = response[:497] + "..."
    }
    
    // Split into 255-byte chunks for DNS TXT records
    chunks := splitIntoChunks(response, 255)
    
    txt := &dns.TXT{
        Hdr: dns.RR_Header{
            Name:   r.Question[0].Name,
            Rrtype: dns.TypeTXT,
            Class:  dns.ClassINET,
            Ttl:    60,
        },
        Txt: chunks,
    }
    
    m.Answer = append(m.Answer, txt)
    w.WriteMsg(m)
}
```

### 4. Client Implementation Examples

#### Python Client
```python
import base64
import dns.resolver
import lzss

class DoNutSentryClient:
    def __init__(self, domain="q.ch.at"):
        self.domain = domain
        self.resolver = dns.resolver.Resolver()
    
    def query(self, text):
        # Encode query
        encoded = self.encode_query(text)
        
        # DNS query
        qname = f"{encoded}.{self.domain}"
        try:
            answers = self.resolver.resolve(qname, "TXT")
            
            # Concatenate TXT record strings
            response = ""
            for rdata in answers:
                for string in rdata.strings:
                    response += string.decode('utf-8')
            
            return response
            
        except dns.resolver.NXDOMAIN:
            return "Query failed: Invalid encoding"
    
    def encode_query(self, text):
        # For simple queries
        if len(text) < 50 and text.replace(" ", "-").isalnum():
            return text.lower().replace(" ", "-")
        
        # For complex queries, try compression
        compressed = lzss.compress(text.encode())
        
        # If compression helps
        if len(compressed) < len(text.encode()) * 0.8:
            encoded = base64.b32encode(compressed).decode()
        else:
            # Just base32 encode
            encoded = base64.b32encode(text.encode()).decode()
        
        # Remove padding and lowercase
        return encoded.rstrip('=').lower()

# Usage
client = DoNutSentryClient()
response = client.query("What is the meaning of life?")
print(response)
```

#### Bash Client Script
```bash
#!/bin/bash
# donutsentry.sh - Query ch.at through DNS

query="$*"

# Simple encoding for basic queries
if [[ "$query" =~ ^[a-zA-Z0-9\ ]+$ ]] && [ ${#query} -lt 50 ]; then
    encoded=$(echo "$query" | tr ' ' '-' | tr '[:upper:]' '[:lower:]')
else
    # Base32 encode for complex queries
    encoded=$(echo -n "$query" | base32 | tr -d '=' | tr '[:upper:]' '[:lower:]')
fi

# Query DNS
response=$(dig +short "${encoded}.q.ch.at" TXT | sed 's/"//g')

echo "$response"
```

### 5. Compression Algorithm Details

LZSS (Lempel-Ziv-Storer-Szymanski) is chosen for its:
- No header overhead (pure compressed data)
- Good compression on English text (4-6:1 typical)
- Simple decompression (important for server performance)
- Deterministic output (same input always produces same output)

Implementation considerations:
- Window size: 4096 bytes (12-bit offsets)
- Minimum match length: 3 bytes
- Output format: bit-packed (offset, length) pairs

### 6. DNS Caching Considerations

DNS responses will be cached by recursive resolvers. Design considerations:

1. **TTL Settings**: Use short TTL (60 seconds) for dynamic responses
2. **Cache Busting**: Clients can add random suffixes for fresh queries
3. **Idempotency**: Same query should return consistent responses within TTL window

### 7. Security Considerations

#### Query Privacy
- Queries visible to all DNS resolvers in path
- Base32 encoding provides obfuscation, not encryption
- Compressed queries harder to read but not secure

#### Rate Limiting
- Implement per-IP rate limiting
- Use DNS RRL (Response Rate Limiting)
- Consider proof-of-work for expensive queries

#### Input Validation
- Strict base32 validation
- Limit decompressed size to prevent bombs
- Sanitize queries before LLM processing

### 8. Performance Optimization

#### Caching Strategy
```go
var queryCache = &sync.Map{} // thread-safe cache

func getCachedResponse(query string) (string, bool) {
    if val, ok := queryCache.Load(query); ok {
        if cached, ok := val.(*CachedResponse); ok {
            if time.Since(cached.Timestamp) < 5*time.Minute {
                return cached.Response, true
            }
        }
    }
    return "", false
}
```

#### Parallel Processing
- Handle multiple DNS queries concurrently
- Batch LLM requests when possible
- Pre-compute responses for common queries

### 9. Monitoring and Debugging

#### Query Logging
```go
func logQuery(subdomain, decodedQuery, response string) {
    log.Printf("DoNutSentry Query: subdomain=%s query=%q response_len=%d",
        subdomain, decodedQuery, len(response))
}
```

#### Health Checks
```bash
# Monitor query success rate
dig health.check.q.ch.at TXT

# Test encoding/decoding
dig test.base32.encode.q.ch.at TXT
```

### 10. Migration Path

1. **Phase 1**: Deploy wildcard DNS handling alongside existing direct queries
2. **Phase 2**: Update documentation to promote recursive queries
3. **Phase 3**: Monitor usage patterns and optimize
4. **Phase 4**: Potentially deprecate direct queries (keep for compatibility)

## Testing Plan

### Unit Tests
```go
func TestQueryEncoding(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"hello world", "hello-world"},
        {"What is AI?", "j5gxc4tfoqwwe33pnm"},
        // Long query that needs compression
        {"Explain quantum computing...", "compressed_base32..."},
    }
    
    for _, test := range tests {
        encoded := encodeQuery(test.input)
        if encoded != test.expected {
            t.Errorf("Expected %s, got %s", test.expected, encoded)
        }
    }
}
```

### Integration Tests
```bash
# Test simple query
result=$(dig what-is-dns.q.ch.at TXT +short)
[[ "$result" =~ "Domain Name System" ]] || exit 1

# Test base32 encoded
encoded=$(echo -n "hello" | base32 | tr -d '=' | tr '[:upper:]' '[:lower:]')
result=$(dig ${encoded}.q.ch.at TXT +short)
[[ "$result" =~ "Hello" ]] || exit 1

# Test compressed query
# ... compression test ...
```

### Load Testing
- Simulate 1000 queries/second through recursive resolvers
- Measure response times with various query complexities
- Test cache effectiveness

## Success Metrics

1. **Accessibility**: 95%+ of users can query through their default DNS
2. **Performance**: <200ms average response time
3. **Reliability**: 99.9% uptime for DNS service
4. **Adoption**: 50%+ of DNS queries use recursive format within 6 months

This is DoNutSentry v1: Making ch.at universally accessible through standard DNS infrastructure, with intelligent encoding and compression to maximize query expressiveness within DNS constraints.