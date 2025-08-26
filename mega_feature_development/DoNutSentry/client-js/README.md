# DoNutSentry JavaScript Client

JavaScript/TypeScript client for querying ch.at through DNS using the DoNutSentry protocol.

## Installation

```bash
npm install @ch.at/donutsentry-client
```

## Quick Start

```javascript
import { DoNutSentryClient } from '@ch.at/donutsentry-client';

const client = new DoNutSentryClient();

// Simple query
const result = await client.query('what is dns');
console.log(result.response);

// The client automatically selects the best encoding:
// - Simple: for basic alphanumeric queries
// - Base32: for queries with special characters  
// - Compressed: for long queries exceeding DNS limits
```

## API

### `new DoNutSentryClient(options?)`

Create a new client instance.

```typescript
const client = new DoNutSentryClient({
  domain: 'q.ch.at',              // DNS domain (default: 'q.ch.at')
  dnsServers: ['8.8.8.8'],        // Custom DNS servers
  timeout: 5000,                  // Query timeout in ms
  retries: 3,                     // Number of retries
  defaultEncoding: 'base32'       // Default encoding strategy
});
```

### `client.query(text, options?)`

Query ch.at with automatic encoding selection.

```typescript
const result = await client.query('What is machine learning?', {
  encoding: 'compressed',  // Force specific encoding
  cacheBust: true,        // Add random suffix to bypass cache
  timeout: 10000          // Custom timeout for this query
});

console.log(result.response);        // The LLM response
console.log(result.metadata);        // Query metadata
```

### `client.getEncodingStats(text)`

Get encoding statistics for a query to see how different strategies perform.

```typescript
const stats = await client.getEncodingStats('Your query here');

console.log(stats.simple);      // Simple encoding stats
console.log(stats.base32);      // Base32 encoding stats  
console.log(stats.compressed);  // Compressed encoding stats
```

## Encoding Strategies

### Simple Encoding
- For queries with only letters, numbers, and spaces
- Replaces spaces with hyphens
- Most readable: `what-is-rust.q.ch.at`

### Base32 Encoding
- For queries with special characters or punctuation
- DNS-safe encoding of any UTF-8 text
- Example: `nfxgc4tfoqwwc3tt.q.ch.at`

### Compressed Encoding
- For long queries exceeding DNS label limits
- Uses LZSS compression before base32 encoding
- Achieves 4-6x compression on typical English text

## Examples

### Basic Queries

```javascript
// Simple query
const result1 = await client.query('what is dns');

// Query with special characters
const result2 = await client.query('What is AI? Explain briefly.');

// Long query (automatically compressed)
const result3 = await client.query(
  'Explain how neural networks learn through backpropagation...'
);
```

### Check Encoding Performance

```javascript
const stats = await client.getEncodingStats(
  'Explain quantum computing in detail'
);

if (!stats.simple.valid && stats.compressed.valid) {
  console.log('This query needs compression to fit in DNS');
}
```

### Custom DNS Configuration

```javascript
// Use specific DNS servers
const client = new DoNutSentryClient({
  dnsServers: ['1.1.1.1', '1.0.0.1'],
  timeout: 3000
});

// Use system DNS
const defaultClient = new DoNutSentryClient();
```

## Testing

```bash
# Run tests
npm test

# Run tests with coverage
npm run test:coverage

# Run example
npm run example
```

## How It Works

### Simple Mode (Short Queries)
1. **Query Encoding**: The client analyzes your query and selects the optimal encoding strategy
2. **DNS Query**: Sends query as subdomain to `*.q.ch.at` through your DNS resolver
3. **Response**: ch.at responds with LLM-generated text in DNS TXT records
4. **Decoding**: Client concatenates and returns the response

### Session Mode (Large Queries)
For queries that exceed DNS limits, the client automatically uses session mode:

1. **Session Init**: Client generates RSA keypair and sends public key
2. **Receive Session ID**: Server returns encrypted session ID
3. **Send Chunks**: Query is compressed and sent in multiple DNS queries
4. **Execute**: Final DNS query triggers processing
5. **Response**: Full response returned in TXT records (up to 4KB)

The client handles this automatically - just use `query()` as normal.

## Limitations

- DNS label limit: 63 characters per label
- Total domain limit: 255 characters
- Response limit: ~500 characters (DNS compatibility)
- Queries are visible to DNS resolvers (use HTTPS for privacy)

## Development

```bash
# Install dependencies
npm install

# Build TypeScript
npm run build

# Run tests
npm test

# Lint code
npm run lint
```

## License

MIT