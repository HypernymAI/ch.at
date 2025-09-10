package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// handleRoutingTable provides a comprehensive view of model routing
func handleRoutingTable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Check if JSON response requested
	if r.Header.Get("Accept") == "application/json" {
		handleRoutingTableJSON(w, r)
		return
	}
	
	// HTML response with dark theme
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>ch.at Routing Table</title>
    <style>
        body { font-family: monospace; background: #0a0a0a; color: #00ff41; padding: 20px; }
        h1 { color: #ffcc00; border-bottom: 2px solid #00ff41; padding-bottom: 10px; }
        h2 { color: #00ccff; margin-top: 30px; }
        table { width: 100%%; border-collapse: collapse; margin: 20px 0; }
        th { background: #1a1a1a; color: #00ff41; padding: 10px; text-align: left; border: 1px solid #00ff41; }
        td { padding: 8px; border: 1px solid #333; }
        tr:hover { background: #1a1a1a; }
        .success { color: #00ff41; }
        .error { color: #ff3333; }
        .warning { color: #ffcc00; }
        .info { color: #00ccff; }
        .deployment { color: #ff00ff; font-family: monospace; }
        .channel { color: #ffcc00; font-weight: bold; }
        .healthy { background: #00ff41; color: #000; padding: 2px 6px; border-radius: 3px; }
        .unhealthy { background: #ff3333; color: #fff; padding: 2px 6px; border-radius: 3px; }
        .provider-anthropic { color: #D97757; }
        .provider-openai { color: #10A37F; }
        .provider-google { color: #4285F4; }
        .provider-meta { color: #0668E1; }
        pre { background: #1a1a1a; padding: 10px; border: 1px solid #333; overflow-x: auto; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 20px 0; }
        .stat-card { background: #1a1a1a; padding: 15px; border: 1px solid #00ff41; }
        .stat-value { font-size: 2em; color: #ffcc00; }
        .stat-label { color: #888; margin-top: 5px; }
    </style>
</head>
<body>
    <h1>🗺️ Model Routing Table</h1>
`)
	
	// Privacy Notice
	privacyStatus := "🔴 LOGGING ENABLED"
	privacyClass := "error"
	privacyMessage := "All LLM interactions are being logged to audit database"
	if !auditEnabled {
		privacyStatus = "🟢 LOGGING DISABLED"
		privacyClass = "success"
		privacyMessage = "LLM interactions are NOT being logged"
	}
	
	fmt.Fprintf(w, `
    <div style="background: #1a1a1a; border: 2px solid #%s; padding: 15px; margin: 20px 0;">
        <h2 style="margin-top: 0;">🔐 Privacy Status</h2>
        <div style="font-size: 1.5em; margin: 10px 0;" class="%s">%s</div>
        <p class="info">%s</p>
        <p style="color: #888; font-size: 0.9em;">
            To change logging settings, set ENABLE_LLM_AUDIT in .env file and restart server.<br>
            View our <a href="/terms_of_service" style="color: #00ff41;">Terms of Service</a> for complete privacy policy.
        </p>
    </div>
`,
		ternary(auditEnabled, "ff3333", "00ff41"),
		privacyClass,
		privacyStatus,
		privacyMessage,
	)
	
	// Check system status
	if modelRegistry == nil || deploymentRegistry == nil {
		fmt.Fprintf(w, `<div class="error">❌ System not initialized!</div></body></html>`)
		return
	}
	
	// Get statistics
	allModels := modelRegistry.List()
	healthyDeployments := deploymentRegistry.GetHealthy()
	
	// Stats cards
	fmt.Fprintf(w, `
    <div class="stats">
        <div class="stat-card">
            <div class="stat-value">%d</div>
            <div class="stat-label">Total Models</div>
        </div>
        <div class="stat-card">
            <div class="stat-value">%d</div>
            <div class="stat-label">Healthy Deployments</div>
        </div>
        <div class="stat-card">
            <div class="stat-value">%s</div>
            <div class="stat-label">Router Status</div>
        </div>
        <div class="stat-card">
            <div class="stat-value">%s</div>
            <div class="stat-label">Audit System</div>
        </div>
    </div>
`,
		len(allModels),
		len(healthyDeployments),
		ternary(modelRouter != nil, "✅ ACTIVE", "❌ INACTIVE"),
		ternary(auditEnabled, "✅ ENABLED", "❌ DISABLED"),
	)
	
	// Model routing table
	fmt.Fprintf(w, `
    <h2>📊 Direct Model Routing</h2>
    <table>
        <tr>
            <th>Model ID</th>
            <th>Name</th>
            <th>Family</th>
            <th>Deployment</th>
            <th>Provider</th>
            <th>Channel</th>
            <th>Status</th>
            <th>Test Command</th>
        </tr>
`)
	
	// Sort models by family and ID
	sort.Slice(allModels, func(i, j int) bool {
		if allModels[i].Family != allModels[j].Family {
			return allModels[i].Family < allModels[j].Family
		}
		return allModels[i].ID < allModels[j].ID
	})
	
	// Display each model
	for _, model := range allModels {
		var deploymentID, provider, channel, status string
		var healthClass string
		
		if len(model.Deployments) > 0 {
			deploymentID = model.Deployments[0]
			if deployment, exists := deploymentRegistry.Get(deploymentID); exists {
				provider = string(deployment.Provider)
				channel = deployment.Tags["channel"]
				if deployment.Status.Healthy {
					status = "✅ Healthy"
					healthClass = "healthy"
				} else {
					status = "❌ Unhealthy"
					healthClass = "unhealthy"
				}
			} else {
				status = "⚠️ Not Found"
				healthClass = "warning"
			}
		} else {
			deploymentID = "No deployments"
			status = "❌ No Config"
			healthClass = "error"
		}
		
		providerClass := fmt.Sprintf("provider-%s", strings.ToLower(model.Family))
		
		testCmd := fmt.Sprintf(`curl -X POST http://localhost:8080/ \
  -H "X-Requested-With: XMLHttpRequest" \
  -d "q=test&model=%s"`, model.ID)
		
		fmt.Fprintf(w, `
        <tr>
            <td><strong class="%s">%s</strong></td>
            <td>%s</td>
            <td>%s</td>
            <td class="deployment">%s</td>
            <td>%s</td>
            <td class="channel">%s</td>
            <td class="%s">%s</td>
            <td><code style="font-size: 0.8em;">%s</code></td>
        </tr>
`,
			providerClass, model.ID,
			model.Name,
			model.Family,
			deploymentID,
			provider,
			channel,
			healthClass, status,
			testCmd,
		)
	}
	
	fmt.Fprintf(w, `</table>`)
	
	// Tier routing
	fmt.Fprintf(w, `
    <h2>🎯 Tier-Based Routing</h2>
    <table>
        <tr>
            <th>Tier</th>
            <th>Description</th>
            <th>Available Models</th>
            <th>Test Command</th>
        </tr>
`)
	
	tiers := []struct {
		Name        string
		Description string
	}{
		{"fast", "Quick, economical responses"},
		{"balanced", "Good performance/cost ratio"},
		{"frontier", "Maximum capability models"},
	}
	
	for _, tier := range tiers {
		// Find models in this tier
		var tierModels []string
		for _, deployment := range healthyDeployments {
			if deployment.Tags["tier"] == tier.Name {
				tierModels = append(tierModels, deployment.ModelID)
			}
		}
		
		// Deduplicate
		uniqueModels := make(map[string]bool)
		for _, m := range tierModels {
			uniqueModels[m] = true
		}
		
		var modelList []string
		for m := range uniqueModels {
			modelList = append(modelList, m)
		}
		sort.Strings(modelList)
		
		testCmd := fmt.Sprintf(`curl -X POST http://localhost:8080/ \
  -H "X-Requested-With: XMLHttpRequest" \
  -d "q=test&model=tier:%s"`, tier.Name)
		
		fmt.Fprintf(w, `
        <tr>
            <td><strong class="info">tier:%s</strong></td>
            <td>%s</td>
            <td>%s</td>
            <td><code style="font-size: 0.8em;">%s</code></td>
        </tr>
`,
			tier.Name,
			tier.Description,
			strings.Join(modelList, ", "),
			testCmd,
		)
	}
	
	fmt.Fprintf(w, `</table>`)
	
	// Channel information
	fmt.Fprintf(w, `
    <h2>📡 Channel Mapping</h2>
    <table>
        <tr>
            <th>Channel</th>
            <th>Provider</th>
            <th>Models Available</th>
        </tr>
        <tr>
            <td class="channel">2</td>
            <td>Anthropic Direct</td>
            <td>Claude models</td>
        </tr>
        <tr>
            <td class="channel">3</td>
            <td>Google</td>
            <td>Gemini models</td>
        </tr>
        <tr>
            <td class="channel">4</td>
            <td>Azure</td>
            <td>Llama models</td>
        </tr>
        <tr>
            <td class="channel">8</td>
            <td>OpenAI Direct</td>
            <td>GPT models (except nano)</td>
        </tr>
        <tr>
            <td class="channel">10</td>
            <td>AWS Bedrock</td>
            <td>Claude, Llama, various</td>
        </tr>
        <tr>
            <td class="channel">11</td>
            <td>Azure OpenAI</td>
            <td>GPT-4.1-nano only</td>
        </tr>
    </table>
`)
	
	// Footer with timestamp
	fmt.Fprintf(w, `
    <hr style="border-color: #333; margin-top: 40px;">
    <p style="color: #666; text-align: center;">
        Generated at %s | 
        <a href="/routing_table" style="color: #00ff41;">Refresh</a> | 
        <a href="/routing_table" style="color: #00ff41;" onclick="event.preventDefault(); fetch('/routing_table', {headers: {'Accept': 'application/json'}}).then(r => r.json()).then(d => console.log(d));">Get JSON</a>
    </p>
</body>
</html>
`, time.Now().Format("2006-01-02 15:04:05"))
}

// handleRoutingTableJSON returns JSON routing information
func handleRoutingTableJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	result := map[string]interface{}{
		"timestamp":          time.Now().Unix(),
		"router_initialized": modelRouter != nil,
		"audit_enabled":      auditEnabled,
		"models":             []map[string]interface{}{},
		"deployments":        []map[string]interface{}{},
		"tiers":              map[string][]string{},
	}
	
	if modelRegistry != nil {
		models := modelRegistry.List()
		result["total_models"] = len(models)
		
		for _, model := range models {
			modelInfo := map[string]interface{}{
				"id":          model.ID,
				"name":        model.Name,
				"family":      model.Family,
				"deployments": model.Deployments,
			}
			
			// Add deployment status
			if len(model.Deployments) > 0 {
				if deployment, exists := deploymentRegistry.Get(model.Deployments[0]); exists {
					modelInfo["healthy"] = deployment.Status.Healthy
					modelInfo["channel"] = deployment.Tags["channel"]
				}
			}
			
			modelsList := result["models"].([]map[string]interface{})
			modelsList = append(modelsList, modelInfo)
			result["models"] = modelsList
		}
	}
	
	if deploymentRegistry != nil {
		healthyDeps := deploymentRegistry.GetHealthy()
		result["healthy_deployments"] = len(healthyDeps)
		
		// Group by tier
		tiers := make(map[string][]string)
		for _, dep := range healthyDeps {
			tier := dep.Tags["tier"]
			if tier != "" {
				tiers[tier] = append(tiers[tier], dep.ModelID)
			}
		}
		result["tiers"] = tiers
	}
	
	json.NewEncoder(w).Encode(result)
}

// Helper function
func ternary(condition bool, ifTrue, ifFalse string) string {
	if condition {
		return ifTrue
	}
	return ifFalse
}