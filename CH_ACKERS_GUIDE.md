# CH.acker's Guide to Ch.at

## The Four Protocols of Power

Ch.at isn't just another chatbot - it's a universal AI access layer that works when everything else is blocked. Here's how to wield it.

### 1. HTTP - The Gateway Drug
```bash
# Basic query
curl "http://chat.hypernym.ai/?q=How+do+I+hack+time"

# URL encoded for complex queries
curl "http://chat.hypernym.ai/?q=$(echo -n 'Explain quantum computing' | jq -sRr @uri)"

# Stream mode (coming soon)
curl "http://chat.hypernym.ai/stream?q=Write+me+a+story"
```

### 2. SSH - The Terminal Native
```bash
# Interactive mode (it's like a BBS from the future)
ssh chat.hypernym.ai

# Pipe mode for scripting
echo "What is the meaning of life?" | ssh chat.hypernym.ai

# Multi-line questions
cat << EOF | ssh chat.hypernym.ai
Analyze this code:
def factorial(n):
    return 1 if n <= 1 else n * factorial(n-1)
EOF
```

### 3. DNS - The Invisible Protocol

This is where it gets spicy. DoNutSentry tunnels AI through DNS queries.

```bash
# Basic DNS query (requires specifying the server)
dig @chat.hypernym.ai "$(echo -n 'Hello world' | base32 | tr -d '=' | tr '[:upper:]' '[:lower:]').q.chat.hypernym.ai" TXT +short

# Pro tip: Make an alias
alias askai='f() { dig @chat.hypernym.ai "$(echo -n "$1" | base32 | tr -d = | tr "[:upper:]" "[:lower:]").q.chat.hypernym.ai" TXT +short }; f'

# Now just:
askai "What is Bitcoin"
```

#### Why DNS is Magic
- Works behind corporate firewalls
- Bypasses most content filters  
- Looks like normal DNS traffic
- Can't be blocked without breaking the internet

### 4. API - The Developer's Delight
```bash
# OpenAI-compatible endpoint
curl -X POST http://chat.hypernym.ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{"role": "user", "content": "Hello"}],
    "model": "gpt-4o-mini"
  }'

# Works with any OpenAI SDK
# Just point base_url to http://chat.hypernym.ai/v1
```

## Advanced Hacks

### DNS Chaining for Long Queries
When base32 encoding makes your query too long:
```bash
# Split long text into chunks
QUERY="Explain the entire history of Unix in detail"
# This would be too long, so use session protocol (coming soon)
```

### Pre-configured DNS Resolution
Add to `/etc/resolver/chat` (macOS) or `/etc/dnsmasq.d/chat.conf` (Linux):
```
# Route all .q.chat.hypernym.ai queries to ch.at (optional - DNS delegation already works)
server=/q.chat.hypernym.ai/chat.hypernym.ai
```

### SSH Config for Quick Access
Add to `~/.ssh/config`:
```
Host ai
    HostName chat.hypernym.ai
    Port 22
    User chat
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
```
Now just: `ssh ai`

### Curl Function for Your Shell
```bash
# Add to ~/.bashrc or ~/.zshrc
chat() {
    curl -s "http://chat.hypernym.ai/?q=$(echo -n "$*" | jq -sRr @uri)"
}

# Usage
chat What is the speed of light
```

## Security & Privacy

- **No logs by default** - Ch.at doesn't store queries
- **No authentication** - Anonymous access
- **No tracking** - Your queries are yours alone
- **SSL available** - Use https:// for encrypted queries

## Bypassing Restrictions

### Corporate Firewall
- DNS queries often unrestricted
- SSH on port 22 usually allowed for "IT purposes"
- HTTP might be proxied but rarely blocked

### Coffee Shop WiFi
- DNS always works (they need it for captive portal)
- Use DNS queries until you authenticate

### Restricted Countries
- DNS is fundamental infrastructure
- Much harder to block than HTTPS
- Looks like normal computer behavior

## The Migration Plan

Currently using `q.chat.hypernym.ai` but will migrate to `ch.at`:
- HTTP: `http://ch.at/?q=hello`
- SSH: `ssh ch.at`
- DNS: `dig "query.q.ch.at" TXT`
- API: `https://ch.at/v1/chat/completions`

## Easter Eggs & Fun Stuff

```bash
# ASCII art mode
echo "Draw me ASCII art of a cat" | ssh chat.hypernym.ai

# Code golf
curl "http://chat.hypernym.ai/?q=shortest+python+quine"

# Wisdom mode
dig @chat.hypernym.ai "$(echo -n 'Tell me a zen koan' | base32 | tr -d '=' | tr '[:upper:]' '[:lower:]').q.chat.hypernym.ai" TXT +short
```

## Building Your Own Tools

Ch.at is just a protocol - build anything:

```python
# Python DNS client
import dns.resolver
import base64

def ask_dns(question, server='chat.hypernym.ai'):
    encoded = base64.b32encode(question.encode()).decode().lower().strip('=')
    query = f"{encoded}.q.chat.hypernym.ai"
    resolver = dns.resolver.Resolver()
    resolver.nameservers = [server]
    answers = resolver.resolve(query, 'TXT')
    return ' '.join([str(rdata).strip('"') for rdata in answers])
```

## The Philosophy

Ch.at exists because AI should be accessible everywhere, always. Not just when you have:
- The right browser
- JavaScript enabled  
- Cookies accepted
- A GUI available
- HTTPS unblocked

Information wants to be free. AI assistance should be too.

## Stay Updated

- GitHub: [coming soon]
- DNS: `dig changelog.q.chat.hypernym.ai TXT`
- SSH: `ssh chat.hypernym.ai --version`

Remember: With great protocols comes great accessibility. Use ch.at to learn, create, and solve problems - wherever you are, whatever your constraints.

Happy hacking! 🚀