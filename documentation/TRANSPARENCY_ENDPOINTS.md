# ch.at Transparency Endpoints Documentation

## Overview

ch.at provides three critical transparency endpoints that allow users to verify system behavior, understand data handling practices, and review terms of service. These endpoints are LIVE and reflect real-time system state - they can and should be checked before any transaction.

## Critical Future Requirement: Authentication Integration

**⚠️ IMPORTANT**: When authentication/login functionality is implemented, the system MUST:
1. Present the Terms of Service as the FIRST response upon sign-in
2. Include links to these live transparency endpoints
3. Require explicit TOS acceptance before granting service access
4. Provide the auth token ONLY after agreement

Example auth flow:
```json
// Initial auth response (before agreement)
{
  "tos": {
    "version": "1.0.0",
    "status": "active",
    "agreement_required": true,
    "endpoints": {
      "terms": "/terms_of_service",
      "routing": "/routing_table"
    },
    "message": "These are LIVE endpoints - verify before every transaction"
  },
  "auth_token": null
}

// After explicit agreement
{
  "auth_token": "jwt_token_here",
  "tos_accepted": {
    "version": "1.0.0",
    "timestamp": "2025-09-08T10:00:00Z"
  }
}
```

## Endpoint Reference

### 1. `/routing_table` - Model Routing and Privacy Status

**Purpose**: Provides complete visibility into model routing, system health, and privacy configuration.

**Formats**:
- HTML: Human-readable dashboard (default)
- JSON: Machine-readable data (`Accept: application/json`)

#### Features

##### Privacy Status Section
- **Prominent Display**: First major element after title
- **Visual Indicators**: 
  - 🔴 Red border/text when logging ENABLED
  - 🟢 Green border/text when logging DISABLED
- **Clear Messaging**: Explains exactly what's being logged
- **Configuration Instructions**: How to change settings
- **TOS Link**: Direct link to terms of service

##### System Statistics
- Total models configured
- Healthy deployments count
- Router initialization status
- Audit system status

##### Model Routing Table
Shows for each model:
- Model ID and name
- Family (Claude, GPT, Llama, etc.)
- Deployment name (e.g., `claude-3.5-haiku-oneapi-anthropic`)
- Provider and channel
- Health status (✅ Healthy / ❌ Unhealthy)
- Test command for verification

##### Tier Routing Information
- Available models in each tier (fast/balanced/frontier)
- Description of each tier
- Test commands for tier-based routing

##### Channel Mapping
- Channel numbers and their providers
- Which models are available on each channel

#### Usage Examples

```bash
# HTML view (for browsers)
curl http://localhost:8080/routing_table

# JSON format (for programs)
curl http://localhost:8080/routing_table -H "Accept: application/json"

# Check privacy status programmatically
curl -s http://localhost:8080/routing_table -H "Accept: application/json" | jq .audit_enabled
```

#### JSON Response Structure
```json
{
  "timestamp": 1234567890,
  "router_initialized": true,
  "audit_enabled": true,  // Critical privacy indicator
  "total_models": 18,
  "healthy_deployments": 19,
  "models": [
    {
      "id": "claude-3.5-haiku",
      "name": "Claude 3.5 Haiku",
      "family": "claude",
      "deployments": ["claude-3.5-haiku-oneapi-anthropic"],
      "healthy": true,
      "channel": "2"
    }
    // ... more models
  ],
  "tiers": {
    "fast": ["gpt-nano", "claude-haiku", "llama-8b"],
    "balanced": ["gpt-mini", "claude-sonnet", "llama-70b"],
    "frontier": ["gpt-5", "claude-opus", "llama-405b"]
  }
}
```

### 2. `/terms_of_service` - Terms and Privacy Policy

**Purpose**: Provides legally binding terms of service with real-time privacy status.

**Formats**:
- HTML: Styled, readable format (default)
- JSON: Structured data (`Accept: application/json` or `?format=json`)

#### Key Components

##### Version Control
- Version number (e.g., "1.0.0")
- Effective date
- Last modified timestamp
- Status field ("active" indicates current TOS)

##### Agreement Statement
```
⚠️ BY USING THIS API, YOU AGREE TO THESE TERMS OF SERVICE
```

##### Privacy Configuration (Real-time)
- Current audit logging status
- What data is collected when enabled
- Data retention policies
- Configuration instructions

##### Terms Sections
1. **Service Description**
2. **Privacy and Data Collection**
   - Audit logging details
   - Data handling practices
3. **Acceptable Use**
4. **Prohibited Uses**
5. **Service Limitations**
6. **Disclaimer**
7. **Changes to Terms**
8. **Model Routing Transparency**
9. **API Access Information**

#### Usage Examples

```bash
# HTML view
curl http://localhost:8080/terms_of_service

# JSON format
curl http://localhost:8080/terms_of_service -H "Accept: application/json"

# Alternative JSON access
curl http://localhost:8080/terms_of_service?format=json

# Check version programmatically
curl -s http://localhost:8080/terms_of_service -H "Accept: application/json" | jq .version

# Check if TOS has changed
CURRENT_VERSION=$(curl -s http://localhost:8080/terms_of_service -H "Accept: application/json" | jq -r .version)
if [ "$CURRENT_VERSION" != "$EXPECTED_VERSION" ]; then
  echo "TOS has changed! Review required."
fi
```

#### JSON Response Structure
```json
{
  "version": "1.0.0",
  "effective_date": "2025-09-08",
  "last_modified": "2025-09-08T10:00:00Z",
  "status": "active",
  "agreement": "By using this API, you agree to these terms of service",
  "privacy": {
    "audit_logging": {
      "status": "enabled",  // or "disabled"
      "configurable": true,
      "description": "Audit logging can be enabled/disabled via ENABLE_LLM_AUDIT",
      "data_stored": [
        "Conversation IDs",
        "Timestamps",
        "Full request/response content",
        "Token counts",
        "Model and deployment information",
        "Error messages"
      ],
      "retention": "No automatic deletion - data persists until manually removed"
    },
    "data_handling": {
      "encryption": "TLS when HTTPS enabled",
      "storage": "Local SQLite database only",
      "third_party": "Queries routed to configured model providers"
    }
  },
  "terms": {
    "acceptable_use": [...],
    "prohibited": [...],
    "disclaimer": "Service provided AS IS without warranty",
    "limitations": [...],
    "changes": "Terms may be updated at any time"
  }
}
```

### 3. `/v1/chat/completions` - Main API Endpoint

While not a transparency endpoint itself, this is the primary service endpoint that is governed by the terms of service.

**Important**: Using this endpoint constitutes agreement to the current Terms of Service.

## Privacy Configuration

### Audit Logging Control

The system's audit logging can be configured via environment variable:

```bash
# In .env file
ENABLE_LLM_AUDIT=true   # Enable logging
ENABLE_LLM_AUDIT=false  # Disable logging
```

**When ENABLED**, the system logs:
- Conversation IDs
- Timestamps
- Full request and response content
- Token counts
- Model and deployment information
- Error messages

**When DISABLED**:
- No audit logging occurs
- Interactions are ephemeral
- No data persistence

### Verifying Privacy Status

Users can verify the current privacy configuration in multiple ways:

1. **Visual Check** - Visit `/routing_table` and look for the Privacy Status box
2. **JSON API** - `curl -s http://localhost:8080/routing_table -H "Accept: application/json" | jq .audit_enabled`
3. **Terms Check** - `curl -s http://localhost:8080/terms_of_service -H "Accept: application/json" | jq .privacy.audit_logging.status`

## Use Cases

### For End Users
- Verify what data is being collected
- Check which model will handle requests
- Review terms before using service
- Confirm no hidden logging

### For Developers
- Programmatically check TOS version
- Monitor system health
- Verify routing configuration
- Test model availability

### For Compliance
- Document user agreement to terms
- Track TOS version changes
- Demonstrate transparency
- Show user control over data

## Best Practices

### Before First Use
1. Check `/terms_of_service` to understand the agreement
2. Verify privacy status at `/routing_table`
3. Configure `ENABLE_LLM_AUDIT` as desired
4. Test model routing with provided commands

### Regular Verification
- Check TOS version periodically for changes
- Verify privacy settings match expectations
- Confirm model routing before sensitive queries

### Programmatic Integration
```python
import requests
import json

def check_tos_and_privacy():
    # Get current TOS
    tos_response = requests.get(
        "http://localhost:8080/terms_of_service",
        headers={"Accept": "application/json"}
    )
    tos = tos_response.json()
    
    # Check version
    if tos["version"] != expected_version:
        raise Exception("TOS has changed - review required")
    
    # Verify privacy settings
    routing_response = requests.get(
        "http://localhost:8080/routing_table",
        headers={"Accept": "application/json"}
    )
    routing = routing_response.json()
    
    if routing["audit_enabled"] and not user_consents_to_logging:
        raise Exception("Audit logging enabled - user consent required")
    
    return True

# Check before sensitive operations
if check_tos_and_privacy():
    # Proceed with API calls
    response = requests.post("http://localhost:8080/v1/chat/completions", ...)
```

## Security Considerations

### No Hidden Mechanisms
- All logging is transparent and configurable
- Status is verifiable in real-time
- No background data collection

### Data Minimization
- Only configured data is collected
- Users control logging state
- Local storage only (no cloud)

### Audit Trail
When enabled, provides complete audit trail for:
- Debugging issues
- Performance analysis
- Usage monitoring
- Security investigations

## Troubleshooting

### Privacy Status Mismatch
If the privacy status shown doesn't match your configuration:
1. Verify `.env` file contains correct `ENABLE_LLM_AUDIT` value
2. Restart the server for changes to take effect
3. Check both `/routing_table` and `/terms_of_service` for consistency

### Model Routing Issues
Use `/routing_table` to:
- See which models are healthy
- Get test commands for each model
- Verify channel configurations
- Check tier assignments

### TOS Version Changes
To handle version changes:
1. Store the accepted version
2. Check current version before operations
3. Re-present TOS if version changed
4. Require re-acceptance for new version

## Integration Examples

### Bash Script
```bash
#!/bin/bash

# Check TOS version
CURRENT_VERSION=$(curl -s http://localhost:8080/terms_of_service \
  -H "Accept: application/json" | jq -r .version)

if [ "$CURRENT_VERSION" != "$ACCEPTED_VERSION" ]; then
  echo "Terms of Service updated to version $CURRENT_VERSION"
  echo "Please review: http://localhost:8080/terms_of_service"
  exit 1
fi

# Check privacy status
AUDIT_ENABLED=$(curl -s http://localhost:8080/routing_table \
  -H "Accept: application/json" | jq -r .audit_enabled)

echo "Audit logging is: $AUDIT_ENABLED"

# Proceed with API call
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"llama-8b","messages":[{"role":"user","content":"Hello"}]}'
```

### JavaScript/Node.js
```javascript
const axios = require('axios');

async function verifyTermsAndPrivacy() {
  // Check TOS
  const tosResponse = await axios.get('http://localhost:8080/terms_of_service', {
    headers: { 'Accept': 'application/json' }
  });
  
  if (tosResponse.data.version !== process.env.ACCEPTED_TOS_VERSION) {
    throw new Error(`TOS updated to ${tosResponse.data.version} - review required`);
  }
  
  // Check privacy
  const routingResponse = await axios.get('http://localhost:8080/routing_table', {
    headers: { 'Accept': 'application/json' }
  });
  
  console.log(`Audit logging: ${routingResponse.data.audit_enabled ? 'ENABLED' : 'DISABLED'}`);
  
  return true;
}

// Use in application
verifyTermsAndPrivacy()
  .then(() => {
    // Make API calls
  })
  .catch(err => {
    console.error('Verification failed:', err.message);
  });
```

## Summary

These transparency endpoints provide:
1. **Complete visibility** into system behavior
2. **Real-time status** of privacy settings
3. **Version-controlled** terms of service
4. **User control** over data collection
5. **Legal compliance** framework

They are LIVE endpoints that reflect current system state and can be checked before every transaction to ensure:
- Terms haven't changed
- Privacy settings match expectations
- Models are available and healthy
- Routing will work as expected

When authentication is implemented, these endpoints will be integrated into the sign-in flow to ensure explicit agreement before service access.

---

*These endpoints embody ch.at's commitment to transparency, user control, and ethical AI service delivery.*