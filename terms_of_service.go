package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	TOS_VERSION = "1.0.0"
	TOS_DATE    = "2025-09-08"
)

// Provider TOS URLs
var providerTOSMap = map[string]map[string]string{
	"openai": {
		"name": "OpenAI Terms of Service",
		"url": "https://openai.com/policies/terms-of-use",
		"description": "Applies when using GPT models",
	},
	"anthropic": {
		"name": "Anthropic Terms of Service", 
		"url": "https://www.anthropic.com/legal/consumer-terms",
		"description": "Applies when using Claude models",
	},
	"google": {
		"name": "Google Gemini Terms",
		"url": "https://ai.google.dev/gemini-api/terms",
		"description": "Applies when using Gemini models",
	},
	"meta": {
		"name": "Meta Llama License",
		"url": "https://ai.meta.com/llama/license/",
		"description": "Applies when using Llama models",
	},
	"azure": {
		"name": "Microsoft Azure Terms",
		"url": "https://azure.microsoft.com/en-us/support/legal/",
		"description": "Applies when using Azure-hosted models",
	},
	"bedrock": {
		"name": "AWS Service Terms",
		"url": "https://aws.amazon.com/service-terms/",
		"description": "Applies when using AWS Bedrock models",
	},
}

// TOSDocument represents the structure of the terms of service JSON
type TOSDocument struct {
	Version       string `json:"version"`
	EffectiveDate string `json:"effective_date"`
	Status        string `json:"status"`
	Summary       struct {
		Title       string `json:"title"`
		Agreement   string `json:"agreement"`
		Description string `json:"description"`
	} `json:"summary"`
	Body struct {
		Sections []TOSSection `json:"sections"`
	} `json:"body"`
	References struct {
		ProviderTerms []TOSReference `json:"provider_terms"`
		Project       struct {
			Name        string `json:"name"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"project"`
		Endpoints []TOSReference `json:"endpoints"`
	} `json:"references"`
	Appendix map[string]interface{} `json:"appendix"`
}

type TOSSection struct {
	Title       string       `json:"title"`
	Content     string       `json:"content,omitempty"`
	Items       []string     `json:"items,omitempty"`
	Subsections []TOSSection `json:"subsections,omitempty"`
}

type TOSReference struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

var tosDocument *TOSDocument

// getActiveProviders returns a list of currently active/healthy providers
func getActiveProviders() []string {
	providersMap := make(map[string]bool)
	
	// Check deployment registry
	if deploymentRegistry != nil {
		healthyDeps := deploymentRegistry.GetHealthy()
		for _, dep := range healthyDeps {
			// Extract provider type from deployment
			providerStr := strings.ToLower(string(dep.Provider))
			
			// Map deployment provider to TOS provider
			if strings.Contains(providerStr, "oneapi") {
				// Check the channel to determine actual provider
				channel := dep.Tags["channel"]
				switch channel {
				case "2":
					providersMap["anthropic"] = true
				case "3":
					providersMap["google"] = true
				case "4", "11":
					providersMap["azure"] = true
				case "8":
					providersMap["openai"] = true
				case "10":
					providersMap["bedrock"] = true
				default:
					// Check model family
					if modelRegistry != nil {
						if model, exists := modelRegistry.Get(dep.ModelID); exists {
							switch model.Family {
							case "gpt":
								providersMap["openai"] = true
							case "claude":
								providersMap["anthropic"] = true
							case "gemini":
								providersMap["google"] = true
							case "llama":
								providersMap["meta"] = true
							}
						}
					}
				}
			} else {
				// Direct provider mapping
				switch providerStr {
				case "openai":
					providersMap["openai"] = true
				case "anthropic":
					providersMap["anthropic"] = true
				case "google", "vertex":
					providersMap["google"] = true
				case "azure":
					providersMap["azure"] = true
				case "bedrock":
					providersMap["bedrock"] = true
				}
			}
		}
	}
	
	// Convert map to slice
	var providers []string
	for provider := range providersMap {
		providers = append(providers, provider)
	}
	
	return providers
}

// loadTOS loads the terms of service from JSON file or uses defaults
func loadTOS() *TOSDocument {
	// Try to load from file
	data, err := ioutil.ReadFile("terms_of_service.json")
	if err == nil {
		var doc TOSDocument
		if err := json.Unmarshal(data, &doc); err == nil {
			log.Println("[TOS] Loaded terms of service from terms_of_service.json")
			
			// Add active provider terms dynamically
			doc.References.ProviderTerms = []TOSReference{}
			activeProviders := getActiveProviders()
			for _, provider := range activeProviders {
				if tosInfo, exists := providerTOSMap[provider]; exists {
					doc.References.ProviderTerms = append(doc.References.ProviderTerms, TOSReference{
						Name:        tosInfo["name"],
						URL:         tosInfo["url"],
						Description: tosInfo["description"],
					})
				}
			}
			
			// Update audit status dynamically
			if appendix, ok := doc.Appendix["audit_status"].(map[string]interface{}); ok {
				if auditEnabled {
					appendix["current"] = "ENABLED"
				} else {
					appendix["current"] = "DISABLED"
				}
			}
			
			// Update conversation logging status
			if dataCollection, ok := doc.Appendix["data_collection_summary"].(map[string]interface{}); ok {
				if convLogging, ok := dataCollection["conversation_logging"].(map[string]interface{}); ok {
					if auditEnabled {
						convLogging["status"] = "ENABLED"
					} else {
						convLogging["status"] = "DISABLED"
					}
				}
			}
			
			return &doc
		}
		log.Printf("[TOS] Failed to parse terms_of_service.json: %v", err)
	}
	
	// Return default if file doesn't exist or parse fails
	log.Println("[TOS] Using default terms of service")
	return getDefaultTOS()
}

// getDefaultTOS returns the default terms of service
func getDefaultTOS() *TOSDocument {
	// This would be a fallback - for now just return empty structure
	return &TOSDocument{
		Version:       TOS_VERSION,
		EffectiveDate: TOS_DATE,
		Status:        "active",
		Summary: struct {
			Title       string `json:"title"`
			Agreement   string `json:"agreement"`
			Description string `json:"description"`
		}{
			Title:       "ch.at Terms of Service",
			Agreement:   "By using this API, you agree to these terms of service",
			Description: "ch.at provides access to various Large Language Models (LLMs) through a unified routing interface.",
		},
	}
}

func init() {
	// Load TOS at startup
	tosDocument = loadTOS()
}

// handleTermsOfService provides TOS endpoint
func handleTermsOfService(w http.ResponseWriter, r *http.Request) {
	// Reload TOS to get current state
	tosDocument = loadTOS()
	
	// Check if JSON requested
	acceptHeader := r.Header.Get("Accept")
	isJSON := acceptHeader == "application/json" || r.URL.Query().Get("format") == "json"
	
	if isJSON {
		// Return the JSON structure with current status
		w.Header().Set("Content-Type", "application/json")
		
		// Add runtime information
		response := map[string]interface{}{
			"version":        tosDocument.Version,
			"effective_date": tosDocument.EffectiveDate,
			"last_modified":  time.Now().Format(time.RFC3339),
			"status":         tosDocument.Status,
			"current_configuration": map[string]interface{}{
				"audit_logging_enabled": auditEnabled,
				"active_providers":      getActiveProviders(),
				"total_models":          0,
				"healthy_deployments":   0,
			},
		}
		
		// Add model/deployment counts if available
		if modelRegistry != nil {
			response["current_configuration"].(map[string]interface{})["total_models"] = len(modelRegistry.List())
		}
		if deploymentRegistry != nil {
			response["current_configuration"].(map[string]interface{})["healthy_deployments"] = len(deploymentRegistry.GetHealthy())
		}
		
		// Add full document structure
		response["summary"] = tosDocument.Summary
		response["terms"] = tosDocument.Body
		response["references"] = tosDocument.References
		response["appendix"] = tosDocument.Appendix
		
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// HTML response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	auditStatus := "🔴 ENABLED - All conversations logged"
	auditColor := "#ff3333"
	statusText := "ON"
	if !auditEnabled {
		auditStatus = "🟢 DISABLED - No conversation logging"
		auditColor = "#00ff41"
		statusText = "OFF"
	}
	
	// Get active providers for display
	activeProviders := getActiveProviders()
	providersList := "None configured"
	if len(activeProviders) > 0 {
		providersList = strings.Join(activeProviders, ", ")
	}
	
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>ch.at Terms of Service</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #0a0a0a; 
            color: #e0e0e0; 
            padding: 20px;
            max-width: 900px;
            margin: 0 auto;
            line-height: 1.6;
        }
        h1 { color: #00ff41; border-bottom: 2px solid #00ff41; padding-bottom: 10px; }
        h2 { color: #00ccff; margin-top: 30px; }
        h3 { color: #ffcc00; }
        .version-box {
            background: #1a1a1a;
            border: 1px solid #333;
            padding: 15px;
            margin: 20px 0;
            border-radius: 5px;
        }
        .status-active { color: #00ff41; font-weight: bold; }
        .status-inactive { color: #ff3333; font-weight: bold; }
        .privacy-notice {
            background: #1a1a1a;
            border: 2px solid %s;
            padding: 20px;
            margin: 30px 0;
            border-radius: 5px;
        }
        .agreement-box {
            background: #0d1117;
            border-left: 4px solid #ffcc00;
            padding: 15px;
            margin: 20px 0;
            font-weight: bold;
        }
        ul { color: #ccc; }
        li { margin: 8px 0; }
        .warning { color: #ff3333; }
        .info { color: #00ccff; }
        .success { color: #00ff41; }
        code {
            background: #1a1a1a;
            padding: 2px 6px;
            border-radius: 3px;
            color: #ffcc00;
        }
        a { color: #00ff41; text-decoration: none; }
        a:hover { text-decoration: underline; }
        .footer {
            margin-top: 50px;
            padding-top: 20px;
            border-top: 1px solid #333;
            color: #666;
            text-align: center;
        }
    </style>
</head>
<body>
    <h1>📜 %s</h1>
    
    <div class="version-box">
        <div>Version: <strong>%s</strong></div>
        <div>Effective Date: <strong>%s</strong></div>
        <div>Status: <span class="status-active">%s</span></div>
        <div>Last Modified: <strong>%s</strong></div>
        <div>Active Providers: <strong>%s</strong></div>
    </div>
    
    <div class="agreement-box">
        ⚠️ %s
    </div>
    
    <div class="privacy-notice" style="border-color: %s;">
        <h2 style="margin-top: 0;">🔐 Current Privacy Configuration</h2>
        <div style="font-size: 1.2em; margin: 15px 0; color: %s;">
            Conversation Logging: <strong>%s (%s)</strong>
        </div>
        <p>This setting is configurable via the <code>ENABLE_LLM_AUDIT</code> environment variable.</p>
    </div>
    
`, 
		auditColor,
		tosDocument.Summary.Title,
		tosDocument.Version,
		tosDocument.EffectiveDate,
		strings.ToUpper(tosDocument.Status),
		time.Now().Format("2006-01-02 15:04:05"),
		providersList,
		tosDocument.Summary.Agreement,
		auditColor,
		auditColor,
		auditStatus,
		statusText,
	)
	
	// Render body sections from JSON
	for _, section := range tosDocument.Body.Sections {
		fmt.Fprintf(w, "    <h2>%s</h2>\n", section.Title)
		if section.Content != "" {
			fmt.Fprintf(w, "    <p>%s</p>\n", section.Content)
		}
		if len(section.Items) > 0 {
			fmt.Fprintf(w, "    <ul>\n")
			for _, item := range section.Items {
				fmt.Fprintf(w, "        <li>%s</li>\n", item)
			}
			fmt.Fprintf(w, "    </ul>\n")
		}
		// Handle subsections
		for _, subsection := range section.Subsections {
			fmt.Fprintf(w, "    <h3>%s</h3>\n", subsection.Title)
			if subsection.Content != "" {
				fmt.Fprintf(w, "    <p>%s</p>\n", subsection.Content)
			}
			if len(subsection.Items) > 0 {
				fmt.Fprintf(w, "    <ul>\n")
				for _, item := range subsection.Items {
					fmt.Fprintf(w, "        <li>%s</li>\n", item)
				}
				fmt.Fprintf(w, "    </ul>\n")
			}
		}
	}
	
	// Add references section if present
	if len(tosDocument.References.ProviderTerms) > 0 || len(tosDocument.References.Endpoints) > 0 {
		fmt.Fprintf(w, `    <h2>References</h2>`)
		
		// Active provider terms
		if len(tosDocument.References.ProviderTerms) > 0 {
			fmt.Fprintf(w, `    <h3>Active Provider Terms</h3><ul>`)
			for _, ref := range tosDocument.References.ProviderTerms {
				fmt.Fprintf(w, `        <li><a href="%s" target="_blank">%s</a> - %s</li>`, ref.URL, ref.Name, ref.Description)
			}
			fmt.Fprintf(w, `    </ul>`)
		}
		
		// API endpoints
		if len(tosDocument.References.Endpoints) > 0 {
			fmt.Fprintf(w, `    <h3>API Endpoints</h3><ul>`)
			for _, ref := range tosDocument.References.Endpoints {
				fmt.Fprintf(w, `        <li><a href="%s">%s</a> - %s</li>`, ref.URL, ref.Name, ref.Description)
			}
			fmt.Fprintf(w, `    </ul>`)
		}
		
		// Project link
		if tosDocument.References.Project.URL != "" {
			fmt.Fprintf(w, `    <h3>Project</h3><ul>`)
			fmt.Fprintf(w, `        <li><a href="%s" target="_blank">%s</a> - %s</li>`, 
				tosDocument.References.Project.URL,
				tosDocument.References.Project.Name,
				tosDocument.References.Project.Description)
			fmt.Fprintf(w, `    </ul>`)
		}
	}
	
	// Add footer
	fmt.Fprintf(w, `
    <div class="footer">
        <p>
            Generated at %s | 
            <a href="/routing_table">View Routing Table</a> | 
            <a href="/terms_of_service?format=json">Get JSON</a> |
            <a href="/health">Health Status</a> |
            <a href="/">Home</a>
        </p>
    </div>
</body>
</html>`, time.Now().Format("2006-01-02 15:04:05"))
}