# DoNutSentry v1 Fixes - What Actually Worked
Date: 2025-01-17
Status: FINALLY WORKING

## The Problem
- Base32 decoding wasn't working
- Server was returning the encoded string instead of decoded value
- Old processes were blocking port 5354

## What DIDN'T Work (Wasted Time)
- Trying to use `looksLikeBase32()` function from old code
- Complex base32 detection logic
- Compression attempts (LZSS)
- Trying to support padded base32 in DNS labels (= characters are invalid)
- Debug logging to files that never appeared
- Not realizing the server is a blocking function

## What Actually Worked

### 1. Fixed Base32 Encoding/Decoding
Client sends base32 WITHOUT padding:
```javascript
// client-js/src/index.ts
private encodeBase32(text: string): string {
  // Remove padding for DNS compatibility
  return base32.encode(text).toLowerCase().replace(/=/g, '');
}
```

Server adds padding back before decoding:
```go
// donutsentry.go
func decodeBase32Query(s string) (string, error) {
  // Check valid base32 characters only
  upper := strings.ToUpper(s)
  
  // Validate length - must be valid for unpadded base32
  validLengths := map[int]bool{0: true, 2: true, 4: true, 5: true, 7: true}
  if !validLengths[len(upper)%8] {
    return "", fmt.Errorf("invalid base32 length for unpadded string")
  }
  
  // Add padding back
  padding := (8 - len(upper)%8) % 8
  padded := upper + strings.Repeat("=", padding)
  
  // Decode
  decoded, err := base32.StdEncoding.DecodeString(padded)
  if err != nil {
    return "", err
  }
  
  return string(decoded), nil
}
```

### 2. Process Management
The CORRECT way to manage the server:
```bash
# Kill old process properly
lsof -i :5354  # Find what's using the port
kill -9 <PID>  # Kill it

# Use screen for background running
screen -X -S donut quit  # Kill existing screen
go build .
screen -dmS donut ./ch.at -http 8080 -dns 5354
```

### 3. Testing
```bash
# Simple encoding test
dig @localhost -p 5354 what-is-dns.q.ch.at TXT +short
# Returns: "DoNutSentry v1.0.2: You queried 'what is dns' via what-is-dns.q.ch.at"

# Base32 encoding test  
dig @localhost -p 5354 jbswy3dpfqqho33snrsfyii.q.ch.at TXT +short
# Returns: "DoNutSentry v1.0.2: You queried 'Hello, world!' via jbswy3dpfqqho33snrsfyii.q.ch.at"
```

## Key Lessons
1. DNS labels cannot contain `=` characters - must use unpadded base32
2. The padding calculation is: `(8 - len(s)%8) % 8`
3. Old processes hold onto ports - use `lsof -i :PORT` to find them
4. The server runs in a blocking loop - use screen or background it
5. Debug with simple print statements, not complex logging

## Final Working State
- Client strips padding from base32
- Server adds padding back before decoding
- Both simple and base32 encoding work correctly
- Tests pass
- Server runs in screen session named "donut"