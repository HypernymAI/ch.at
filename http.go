//go:build !with_ajaxish

package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Session tracking to prevent duplicate message processing
var (
	sessionSeqs = make(map[string]int)
	sessionMu   sync.RWMutex
	
	// BILLING PROTECTION: Track requests per IP to prevent runaway costs
	ipRequestCounts = make(map[string]int)
	ipRequestMu     sync.RWMutex
	lastResetTime   = time.Now()
)

const htmlPromptPrefix = "You are a helpful assistant. Use HTML formatting instead of markdown (no CSS or style attributes): "

// buildModelTable generates the model selection radio button table
func buildModelTable(selectedModel string) string {
	modelTable := "<table class='model-radio-table'><tr><th>Provider</th><th>Models</th></tr>"
	
	if selectedModel == "" {
		selectedModel = os.Getenv("BASIC_OPENAI_MODEL")
		if selectedModel == "" {
			selectedModel = "llama-8b"
		}
	}
	
	if modelRegistry != nil {
		type providerGroup struct{
			Emoji string
			Name string
			Models []string
		}
		providerGroups := map[string]*providerGroup{
			"meta": {"🔷", "Meta", []string{}},
			"openai": {"🟢", "OpenAI", []string{}},
			"anthropic": {"🟠", "Anthropic", []string{}},
			"google": {"🔵", "Google", []string{}},
		}
		
		allModels := modelRegistry.List()
		for _, m := range allModels {
			switch m.Family {
			case "gpt":
				providerGroups["openai"].Models = append(providerGroups["openai"].Models, m.ID)
			case "claude":
				providerGroups["anthropic"].Models = append(providerGroups["anthropic"].Models, m.ID)
			case "gemini":
				providerGroups["google"].Models = append(providerGroups["google"].Models, m.ID)
			case "llama":
				providerGroups["meta"].Models = append(providerGroups["meta"].Models, m.ID)
			}
		}
		
		for _, providerKey := range []string{"meta", "openai", "anthropic", "google"} {
			group := providerGroups[providerKey]
			if len(group.Models) > 0 {
				modelTable += fmt.Sprintf("<tr><td>%s %s</td><td>", group.Emoji, group.Name)
				for _, modelID := range group.Models {
					checked := ""
					if modelID == selectedModel {
						checked = "checked"
					}
					modelTable += fmt.Sprintf(`<label><input type="radio" name="model" value="%s" %s> %s</label> `,
						modelID, checked, modelID)
				}
				modelTable += "</td></tr>"
			}
		}
	} else {
		checked := ""
		if selectedModel == "llama-8b" {
			checked = "checked"
		}
		modelTable += fmt.Sprintf(`<tr><td>🔷 Meta</td><td><label><input type="radio" name="model" value="llama-8b" %s> llama-8b</label></td></tr>`, checked)
	}
	modelTable += "</table>"
	return modelTable
}

// RequestTelemetry holds comprehensive telemetry data for a request
type RequestTelemetry struct {
	RequestID       string
	Method          string
	Path            string
	UserAgent       string
	RemoteAddr      string
	Query           string
	InputHash       string
	OutputHash      string
	InputTokens     int
	OutputTokens    int
	Model           string
	FinishReason    string
	ContentFiltered bool
	ResponseType    string
	Status          int
	StartTime       time.Time
	Duration        time.Duration
}

// isBrowserUA checks if the user agent appears to be from a web browser
func isBrowserUA(ua string) bool {
	ua = strings.ToLower(ua)
	browserIndicators := []string{
		"mozilla", "msie", "trident", "edge", "chrome", "safari", 
		"firefox", "opera", "webkit", "gecko", "khtml",
	}
	for _, indicator := range browserIndicators {
		if strings.Contains(ua, indicator) {
			return true
		}
	}
	return false
}

// tierToModel maps tier names to model selection tags for the router
func tierToModel(tier string) string {
	// The router will select models based on tier tags
	// We use special model names that the router recognizes as tier selectors
	switch tier {
	case "fast":
		return "tier:fast"
	case "frontier":
		return "tier:frontier"
	default:
		return "tier:balanced"
	}
}

const htmlHeader = `<!DOCTYPE html>
<html>
<head>
    <title>ch.at</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        html { height: 100%; scroll-behavior: smooth; }
        body { 
            margin: 0;
            padding: 0;
            font-family: system-ui, -apple-system, sans-serif; 
            background: #FFF8F0; 
            color: #2C1F3D;
            text-align: center;
            min-height: 100vh;
        }
        .main-container {
            display: flex;
            flex-direction: column;
            height: 100vh;
            overflow: hidden;
        }
        .page-content {
            flex: 1;
            display: flex;
            flex-direction: column;
            padding: 2.5rem;
            padding-bottom: 1rem;
            overflow-y: auto;
            min-height: 0;
        }
        .header-section {
            flex-shrink: 0;
        }
        .chat { 
            text-align: left; 
            max-width: 700px; 
            margin: 1.25rem auto;
            width: 100%;
            flex: 1;
            overflow-y: auto;
            min-height: 0;
        }
        /* Auto-scroll hack using pseudo-element */
        .chat::after {
            content: '';
            display: block;
            height: 0;
            clear: both;
        }
        .footer-section {
            flex-shrink: 0;
            background: #FFF8F0;
            border-top: 1px solid #E8DCC4;
            padding: 1rem 2.5rem 2.5rem;
        }
        .q { padding: 1.25rem; background: #E8DCC4; font-style: italic; font-size: large; border-left: 4px solid #6B4C8A; }
        .a { 
            padding: 1.5rem 1.25rem; 
            position: relative; 
            background: #FFFBF5; 
            margin: 1.5rem 0 0.5rem 0; 
            border-radius: 8px;
            border: 1px solid #E8DCC4;
        }
        form { max-width: 700px; margin: 0 auto; padding: 0.75rem 2.5rem; }
        .input-row { display: flex; gap: .5rem; margin-bottom: .5rem; }
        input[type="text"] { 
            width: 100%; 
            padding: 1rem 1.25rem;
            font-size: 1.1rem;
            border: 3px solid #6B4C8A;
            border-radius: 12px;
            background: #FFFBF5;
            transition: all 0.2s;
            outline: none;
        }
        input[type="text"]:focus {
            border-color: #5A3D79;
            background: white;
            box-shadow: 0 0 0 3px rgba(107, 76, 138, 0.1);
        }
        input[type="submit"] { 
            padding: 1rem 2rem;  /* Normal width */
            font-size: 1rem;
            font-weight: 600;
            background: #6B4C8A;
            color: white;
            border: none;
            border-radius: 10px;
            cursor: pointer;
            transition: all 0.2s;
            box-shadow: 0 2px 6px rgba(107, 76, 138, 0.2);
            min-width: 100px;
        }
        input[type="submit"]:hover {
            background: #5A3D79;
            transform: translateY(-1px);
            box-shadow: 0 3px 10px rgba(107, 76, 138, 0.25);
        }
        input[type="submit"]:active {
            transform: translateY(0);
            box-shadow: 0 1px 4px rgba(107, 76, 138, 0.2);
        }
        
        /* Mode toggle */
        .mode-toggle {
            display: flex;
            justify-content: flex-start;
            gap: 1rem;
            margin: 1rem 0;
            font-size: 0.9rem;
        }
        .mode-toggle label {
            padding: 6px 16px;
            border-radius: 20px;
            cursor: pointer;
            transition: all 0.2s;
            border: 1px solid #ddd;
        }
        .mode-toggle input[type="radio"] {
            display: none;
        }
        .mode-toggle input[type="radio"]:checked + label {
            background: #6B4C8A;
            color: white;
            border-color: #6B4C8A;
        }
        
        /* Provider dropdown */
        .provider-select {
            margin: 1rem 0;
            position: relative;
            max-width: 700px;
            margin: 1rem auto;
        }
        .provider-dropdown {
            width: 100%;
            padding: 5px 10px;  /* Half height */
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            background: white;
            cursor: pointer;
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 0.9rem;
            line-height: 1.2;
        }
        .provider-dropdown:hover {
            border-color: #999;
        }
        .provider-options {
            position: absolute;
            top: 100%;
            left: 0;
            right: 0;
            background: white;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            margin-top: 4px;
            max-height: 300px;
            overflow-y: auto;
            z-index: 100;
            display: none;
        }
        .provider-options.open {
            display: block;
        }
        .provider-option {
            padding: 5px 10px;  /* Half height */
            cursor: pointer;
            display: flex;
            align-items: center;
            gap: 8px;
            transition: background 0.2s;
        }
        .provider-option:hover {
            background: #f5f5f5;
        }
        
        /* Provider-specific styling */
        .provider-openai { border-left: 4px solid #10A37F; }
        .provider-anthropic { border-left: 4px solid #D97757; }
        .provider-google { border-left: 4px solid #4285F4; }
        .provider-meta { border-left: 4px solid #0668E1; }
        .provider-mistral { border-left: 4px solid #7C3AED; }
        .provider-cohere { border-left: 4px solid #6B46C1; }
        .provider-amazon { border-left: 4px solid #FF9900; }
        .provider-microsoft { border-left: 4px solid #0078D4; }
        
        /* Badge provider colors */
        .provider-openai .badge-toggle { 
            border-left: 3px solid #10A37F !important;
            background: linear-gradient(90deg, rgba(16,163,127,0.05) 0%, white 50%);
        }
        .provider-anthropic .badge-toggle { 
            border-left: 3px solid #D97757 !important;
            background: linear-gradient(90deg, rgba(217,119,87,0.05) 0%, white 50%);
        }
        .provider-google .badge-toggle { 
            border-left: 3px solid #4285F4 !important;
            background: linear-gradient(90deg, rgba(66,133,244,0.05) 0%, white 50%);
        }
        .provider-meta .badge-toggle { 
            border-left: 3px solid #0668E1 !important;
            background: linear-gradient(90deg, rgba(6,104,225,0.05) 0%, white 50%);
        }
        .provider-mistral .badge-toggle { 
            border-left: 3px solid #7C3AED !important;
            background: linear-gradient(90deg, rgba(124,58,237,0.05) 0%, white 50%);
        }
        
        /* Model selection */
        .model-select {
            margin: 1rem auto;
            max-width: 700px;
        }
        .model-dropdown {
            width: 100%;
            padding: 5px 10px;  /* Half height */
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-size: 0.9rem;
            background: white;
            cursor: pointer;
            line-height: 1.2;
        }
        .model-dropdown:hover {
            border-color: #999;
        }
        
        /* Model badge on responses */
        /* PROMINENT BADGE - TOP OF EACH ANSWER */
        .model-badge {
            position: absolute;
            top: 0;
            right: 0;
            z-index: 10;
        }
        .badge-toggle {
            padding: 4px 12px;  /* Reduced from 8px 16px - half the vertical padding */
            border: 1px solid #6B4C8A;  /* Thinner border */
            border-radius: 16px;  /* Smaller radius */
            background: linear-gradient(135deg, #FFFBF5 0%, #F5EDE1 100%);
            cursor: pointer;
            font-size: 0.75rem;  /* Smaller font */
            display: inline-flex;
            align-items: center;
            gap: 4px;  /* Smaller gap */
            transition: all 0.2s;
            font-family: inherit;
            line-height: 1;  /* Tight line height */
        }
        .badge-toggle:hover {
            opacity: 1;
            box-shadow: 0 1px 4px rgba(107, 76, 138, 0.15);
        }
        .provider-dot {
            font-size: 0.8rem;
        }
        .expand-icon {
            transition: transform 0.2s;
            font-size: 0.7rem;
        }
        .badge-toggle.expanded .expand-icon {
            transform: rotate(180deg);
        }
        
        /* Metadata panel */
        .metadata-panel {
            position: absolute;
            bottom: calc(100% + 8px);
            left: 0;
            background: white;
            border: 1px solid #e0e0e0;
            border-radius: 8px;
            padding: 12px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
            z-index: 100;
            min-width: 320px;
            display: none;
        }
        .metadata-panel.open {
            display: block;
        }
        .metadata-table {
            font-size: 0.8rem;
            width: 100%;
            border-collapse: collapse;
        }
        .metadata-table td {
            padding: 4px 8px;
        }
        .metadata-table td:first-child {
            font-weight: 600;
            color: #666;
            width: 40%;
        }
        
        /* Tier selection (simplified mode) */
        .tier-selection { display: flex; gap: 1rem; justify-content: center; font-size: 0.9rem; margin: 1rem 0; }
        .tier-selection label { cursor: pointer; }
        .tier-selection input[type="radio"] { cursor: pointer; }
        
        /* Model radio table */
        .model-radio-table {
            width: 100%;
            max-width: 700px;
            margin: 1rem auto;
            border-collapse: collapse;
            text-align: left;
        }
        
        /* Model selection INSIDE form but positioned below viewport */
        #model-selection {
            position: absolute;
            top: calc(100vh + 10rem);
            left: 0;
            right: 0;
            background: #FFF8F0;
            padding: 2rem;
            min-height: 100vh;
        }
        .model-radio-table th {
            text-align: left;
            padding: 0.5rem;
            border-bottom: 2px solid #6B4C8A;
            font-weight: 600;
        }
        .model-radio-table td {
            padding: 0.75rem;
            vertical-align: top;
            border-bottom: 1px solid #E8DCC4;
            text-align: left;
        }
        .model-radio-table td:first-child {
            width: 120px;
            font-weight: 500;
            text-align: left;
        }
        .model-radio-table label {
            display: inline-block;
            margin: 0.25rem 0.5rem 0.25rem 0;
            padding: 0.25rem 0.5rem;
            border: 1px solid #ddd;
            border-radius: 8px;
            cursor: pointer;
            transition: all 0.2s;
        }
        .model-radio-table label:hover {
            background: #f5f5f5;
            border-color: #6B4C8A;
        }
        .model-radio-table input[type="radio"] {
            margin-right: 0.25rem;
        }
        .model-radio-table input[type="radio"]:checked + label,
        .model-radio-table label:has(input:checked) {
            background: #6B4C8A;
            color: white;
            border-color: #6B4C8A;
        }
        
        /* Control visibility */
        #advanced-controls { display: block; }
        #simplified-controls { display: none; }
        #advanced-controls.hidden { display: none; }
        #simplified-controls.hidden { display: none; }
    </style>
</head>
<body onload="var c=document.querySelector('.chat');if(c)c.scrollTop=c.scrollHeight;">
    <div class="main-container">
    <div class="page-content">
        <div class="header-section">
            <h1>ch.at</h1>
            <p>Universal Basic Intelligence</p>
            <p><small><i>pronounced "ch-dot-at"</i></small></p>
        </div>
        <div class="chat">`

const htmlFooterTemplate = `</div>
    </div>
    <div class="footer-section">
    <form method="POST" action="/" id="chat-form">
        <div class="input-row">
            <input type="text" name="q" placeholder="Type your message..." autofocus>
            <input type="submit" value="Send">
        </div>
        <textarea name="h" style="display:none">%s</textarea>
        <input type="hidden" name="session" value="%s">
        <input type="hidden" name="seq" value="%d">
        
        <!-- Model Selection INSIDE form but positioned below viewport -->
        <div id="model-selection">
            <div class="model-table">
                %s
            </div>
            <p style="text-align: center; margin: 2rem 0;">
                <a href="#" style="padding: 0.5rem 1rem; border: 1px solid #6B4C8A; border-radius: 8px; text-decoration: none; color: #6B4C8A; display: inline-block;">
                    ch.at now! ↑
                </a>
            </p>
        </div>
    </form>
    
    <p id="footer-top"><a href="/">New Chat</a></p>
    <p><small>
        Also available: ssh ch.at • curl ch.at/?q=hello • dig @ch.at "question" TXT<br>
        No logs • No accounts • Free software • <a href="https://github.com/Deep-ai-inc/ch.at">GitHub</a>
    </small></p>
    <p style="margin: 0.5rem 0;">
        <a href="#model-selection" style="padding: 0.4rem 0.8rem; border: 1px solid #6B4C8A; border-radius: 8px; text-decoration: none; color: #6B4C8A; display: inline-block; font-size: 0.9rem;">
            Model Selection ↓
        </a>
    </p>
    
    </div>
    </div>
</body>
</html>`

func StartHTTPServer(port int) error {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/v1/chat/completions", handleChatCompletions)
	http.HandleFunc("/health", handleHealth)
	
	// Model management endpoints
	http.HandleFunc("/v1/models", handleListModels)
	http.HandleFunc("/v1/models/", handleGetModel)
	http.HandleFunc("/v1/deployments", handleListDeployments)
	http.HandleFunc("/v1/deployments/", handleGetDeployment)
	http.HandleFunc("/routing_table", handleRoutingTable)
	http.HandleFunc("/terms_of_service", handleTermsOfService)

	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, nil)
}

func StartHTTPSServer(port int, certFile, keyFile string) error {
	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServeTLS(addr, certFile, keyFile, nil)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	// Initialize telemetry
	telemetry := &RequestTelemetry{
		RequestID:   generateRequestID(),
		Method:      r.Method,
		Path:        r.URL.Path,
		RemoteAddr:  r.RemoteAddr,
		UserAgent:   r.Header.Get("User-Agent"),
		StartTime:   time.Now(),
	}
	
	// Note: Removed auto-redirect to anchor as it was breaking page loads
	// Browsers will still respect the anchor if manually navigated to /#footer-top
	
	// Beacon request start
	beacon("request_start", map[string]interface{}{
		"request_id": telemetry.RequestID,
		"method":     telemetry.Method,
		"path":       telemetry.Path,
		"remote_addr": telemetry.RemoteAddr,
		"user_agent": telemetry.UserAgent,
	})

	w.Header().Set("Access-Control-Allow-Origin", "*")
	if !rateLimitAllow(r.RemoteAddr) {
		beacon("rate_limit_exceeded", map[string]interface{}{
			"remote_addr": r.RemoteAddr,
		})
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	var query, history, prompt, tier, sessionID, seqStr string
	content := ""
	jsonResponse := ""

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}
		query = r.FormValue("q")
		history = r.FormValue("h")
		tier = r.FormValue("tier")
		sessionID = r.FormValue("session")
		seqStr = r.FormValue("seq")
		
		// Default to balanced tier if not specified
		if tier == "" {
			tier = "balanced"
		}

		// Limit history size to ensure compatibility
		if len(history) > 65536 {
			history = history[len(history)-65536:]
		}

		if query == "" {
			body, err := io.ReadAll(io.LimitReader(r.Body, 65536)) // Limit body size
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}
			query = string(body)
		}
	} else {
		query = r.URL.Query().Get("q")
		// Support path-based queries like /what-is-go
		if query == "" && r.URL.Path != "/" {
			query = strings.ReplaceAll(strings.TrimPrefix(r.URL.Path, "/"), "-", " ")
		}
	}

	accept := r.Header.Get("Accept")
	userAgent := strings.ToLower(r.Header.Get("User-Agent"))
	wantsJSON := strings.Contains(accept, "application/json")
	wantsHTML := isBrowserUA(userAgent) || strings.Contains(accept, "text/html")
	wantsStream := strings.Contains(accept, "text/event-stream")

	if query != "" {
		// BILLING PROTECTION: Rate limit per IP to prevent $450 disasters
		ipRequestMu.Lock()
		// Reset counts every hour
		if time.Since(lastResetTime) > time.Hour {
			ipRequestCounts = make(map[string]int)
			lastResetTime = time.Now()
		}
		
		ipAddr := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ipAddr = forwarded
		}
		
		requestCount := ipRequestCounts[ipAddr]
		if requestCount >= 50 { // Max 50 LLM calls per hour per IP
			ipRequestMu.Unlock()
			// Rate limit exceeded
			http.Error(w, "Rate limit exceeded - too many requests. Please wait before trying again.", http.StatusTooManyRequests)
			return
		}
		ipRequestCounts[ipAddr]++
		ipRequestMu.Unlock()
		
		// Check for duplicate submission using session/sequence
		isDuplicate := false
		// Session check
		if sessionID != "" && seqStr != "" {
			if seq, err := strconv.Atoi(seqStr); err == nil {
				sessionMu.RLock()
				lastSeq, exists := sessionSeqs[sessionID]
				sessionMu.RUnlock()
				
				if exists && seq <= lastSeq {
					// This is a duplicate submission (refresh/back button)
					isDuplicate = true
					// Duplicate detected
					beacon("duplicate_submission_blocked", map[string]interface{}{
						"session_id": sessionID,
						"seq": seq,
						"last_seq": lastSeq,
					})
				} else if !exists {
					// CRITICAL: Store the sequence IMMEDIATELY to prevent race conditions
					// This prevents multiple refreshes from all making LLM calls
					sessionMu.Lock()
					sessionSeqs[sessionID] = seq
					sessionMu.Unlock()
					// Preemptively stored
				}
			}
		}
		
		// Build message array from history for full conversation context
		var messages []map[string]string
		
		// Parse the Q:/A: history into messages
		if history != "" {
			histParts := strings.Split("\n"+history, "\nQ: ")
			for _, part := range histParts[1:] {
				if i := strings.Index(part, "\nA: "); i >= 0 {
					question := part[:i]
					answer := part[i+4:]
					
					// Strip model metadata marker if present
					if modelIdx := strings.Index(answer, "§MODEL:"); modelIdx >= 0 {
						answer = answer[:modelIdx]
					}
					answer = strings.TrimSpace(answer)
					
					// Add to messages array
					messages = append(messages, map[string]string{
						"role": "user",
						"content": question,
					})
					messages = append(messages, map[string]string{
						"role": "assistant",
						"content": answer,
					})
				}
			}
		}
		
		// Add current query to messages
		messages = append(messages, map[string]string{
			"role": "user",
			"content": query,
		})
		
		// Keep prompt for non-message uses (backward compatibility)
		prompt = query

		if wantsHTML && r.Header.Get("Accept") != "application/json" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			
			// If duplicate, just show existing history without calling LLM
			if isDuplicate {
				// Just render the existing history
				w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; object-src 'none'; base-uri 'none'; style-src 'unsafe-inline'")
				fmt.Fprint(w, htmlHeader)
				
				// Render existing history
				if history != "" {
					histParts := strings.Split("\n"+history, "\nQ: ")
					for _, part := range histParts[1:] {
						if i := strings.Index(part, "\nA: "); i >= 0 {
							question := part[:i]
							answer := part[i+4:]
							
							// Extract model metadata if present
							modelName := "llama-8b"
							if modelIdx := strings.Index(answer, "§MODEL:"); modelIdx >= 0 {
								modelStart := modelIdx + len("§MODEL:")
								if endIdx := strings.Index(answer[modelStart:], "§"); endIdx >= 0 {
									modelName = answer[modelStart : modelStart+endIdx]
									answer = answer[:modelIdx]
								}
							}
							answer = strings.TrimSpace(answer)
							
							fmt.Fprintf(w, "<div class=\"q\">%s</div>\n", html.EscapeString(question))
							fmt.Fprintf(w, "<div class=\"a\">%s", answer)
							
							// Generate badge
							providerEmoji := "⚫"
							providerName := "Unknown"
							
							if strings.Contains(modelName, "gpt") {
								providerEmoji = "🟢"
								providerName = "OpenAI"
							} else if strings.Contains(modelName, "claude") {
								providerEmoji = "🟠"
								providerName = "Anthropic"
							} else if strings.Contains(modelName, "gemini") {
								providerEmoji = "🔵"
								providerName = "Google"
							} else if strings.Contains(modelName, "llama") {
								providerEmoji = "🔷"
								providerName = "Meta"
							} else if strings.Contains(modelName, "mistral") || strings.Contains(modelName, "mixtral") {
								providerEmoji = "🟣"
								providerName = "Mistral"
							}
							
							fmt.Fprintf(w, `<div class="model-badge provider-%s">
								<div class="badge-toggle">
									<span class="provider-dot">%s</span>
									<span class="model-name">%s</span>
								</div>
							</div>`,
								strings.ToLower(providerName),
								providerEmoji,
								modelName,
							)
							fmt.Fprintf(w, "</div>\n")
						}
					}
				}
				
				// Generate session/seq for the form
				if sessionID == "" {
					sessionID = fmt.Sprintf("sess_%d_%s", time.Now().Unix(), generateRequestID()[:8])
				}
				nextSeq := 1
				if seqStr != "" {
					if seq, err := strconv.Atoi(seqStr); err == nil {
						nextSeq = seq // Keep same seq since it was duplicate
					}
				}
				
				// Build model table and footer
				modelTable := buildModelTable(r.FormValue("model"))
				safeHistory := strings.ReplaceAll(history, "</textarea>", "&lt;/textarea&gt;")
				
				fmt.Fprintf(w, htmlFooterTemplate,
					safeHistory,
					sessionID,
					nextSeq,
					modelTable,
				)
				
				return
			}
			
			// Not duplicate, continue with normal processing
			w.Header().Set("Transfer-Encoding", "chunked")
			w.Header().Set("X-Accel-Buffering", "no")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; object-src 'none'; base-uri 'none'; style-src 'unsafe-inline'")
			flusher := w.(http.Flusher)

			headerSize := len(htmlHeader)
			historySize := len(html.EscapeString(history))
			querySize := len(html.EscapeString(query))
			currentSize := headerSize + historySize + querySize + 10

			const minThreshold = 6144

			// Calculate total height for auto-scroll positioning
			messageHeight := 0
			if history != "" {
				histParts := strings.Split("\n"+history, "\nQ: ")
				messageHeight = (len(histParts) - 1) * 290 // Approximate per Q/A pair
			}
			messageHeight += 80 // For the new question we're about to add
			
			// If we have enough content, add spacer to start scrolled
			if messageHeight > 600 {
				scrollOffset := messageHeight - 350 // Leave 350px for incoming answer
				headerWithScroll := strings.Replace(htmlHeader, "</style>", fmt.Sprintf(`
					.chat::before {
						content: '';
						display: block;
						height: %dpx;
						margin-bottom: -%dpx;
					}
				</style>`, scrollOffset, scrollOffset), 1)
				fmt.Fprint(w, headerWithScroll)
			} else {
				fmt.Fprint(w, htmlHeader)
			}
			
			if currentSize < minThreshold {
				paddingNeeded := (minThreshold - currentSize) / 3
				if paddingNeeded > 0 {
					padding := strings.Repeat("\u200B", paddingNeeded)
					fmt.Fprint(w, padding)
				}
			}
			
			if history != "" {
				histParts := strings.Split("\n"+history, "\nQ: ")
				for _, part := range histParts[1:] {
					if i := strings.Index(part, "\nA: "); i >= 0 {
						question := part[:i]
						answer := part[i+4:]
						
						// Extract model metadata if present (same as initial page load)
						modelName := "llama-8b" // default
						if modelIdx := strings.Index(answer, "§MODEL:"); modelIdx >= 0 {
							modelStart := modelIdx + len("§MODEL:")
							if endIdx := strings.Index(answer[modelStart:], "§"); endIdx >= 0 {
								modelName = answer[modelStart : modelStart+endIdx]
								// Remove the metadata from the answer
								answer = answer[:modelIdx]
							}
						}
						answer = strings.TrimSpace(answer)
						
						fmt.Fprintf(w, "<div class=\"q\">%s</div>\n", html.EscapeString(question))
						// History answers contain HTML, render them as-is
						fmt.Fprintf(w, "<div class=\"a\">%s", answer)
						
						// Generate badge for historical response
						providerEmoji := "⚫"
						providerName := "Unknown"
						
						if strings.Contains(modelName, "gpt") {
							providerEmoji = "🟢"
							providerName = "OpenAI"
						} else if strings.Contains(modelName, "claude") {
							providerEmoji = "🟠"
							providerName = "Anthropic"
						} else if strings.Contains(modelName, "gemini") {
							providerEmoji = "🔵"
							providerName = "Google"
						} else if strings.Contains(modelName, "llama") {
							providerEmoji = "🔷"
							providerName = "Meta"
						} else if strings.Contains(modelName, "mistral") || strings.Contains(modelName, "mixtral") {
							providerEmoji = "🟣"
							providerName = "Mistral"
						}
						
						// Add the badge (no JavaScript onclick)
						fmt.Fprintf(w, `<div class="model-badge provider-%s">
							<div class="badge-toggle">
								<span class="provider-dot">%s</span>
								<span class="model-name">%s</span>
							</div>
						</div>`,
							strings.ToLower(providerName),
							providerEmoji,
							modelName,
						)
						
						fmt.Fprintf(w, "</div>\n")
					}
				}
			}
			fmt.Fprintf(w, "<div class=\"q\">%s</div>\n<div class=\"a\">", html.EscapeString(query))
			flusher.Flush()

			ch := make(chan string)
			var llmResp *LLMResponse
			go func() {
				var resp *LLMResponse
				var err error
				
				// Use the selected model from the form, or default
				modelToUse := r.FormValue("model")
				if modelToUse == "" {
					// Fallback to environment default or llama-8b
					modelToUse = os.Getenv("BASIC_OPENAI_MODEL")
					if modelToUse == "" {
						modelToUse = "llama-8b"
					}
				}
				
				// Build messages with HTML prompt prefix for assistant
				// Need to add the HTML instruction to the first user message
				if len(messages) > 0 {
					messages[0]["content"] = htmlPromptPrefix + messages[0]["content"]
				}
				
				// Use router if available
				if modelRouter != nil {
					// Send full message array with conversation context!
					resp, err = LLMWithRouter(messages, modelToUse, nil, ch)
				} else {
					err = fmt.Errorf("model router not initialized")
				}
				if err != nil {
					// Log the error but don't try to send it
					// The channel is managed by LLM/LLMWithRouter
					// LLM error
				} else {
					llmResp = resp
				}
			}()

			response := ""
			for chunk := range ch {
				// Don't escape HTML since we asked for HTML format
				if _, err := fmt.Fprint(w, chunk); err != nil {
					return
				}
				response += chunk
				flusher.Flush()
			}
			
			// Update telemetry with LLM response data if available
			if llmResp != nil {
				telemetry.InputHash = llmResp.InputHash
				telemetry.OutputHash = llmResp.OutputHash
				telemetry.InputTokens = llmResp.InputTokens
				telemetry.OutputTokens = llmResp.OutputTokens
				telemetry.Model = llmResp.Model
				telemetry.FinishReason = llmResp.FinishReason
				telemetry.ContentFiltered = llmResp.ContentFiltered
			}
			
			// ALWAYS add model badge - every response gets one!
			
			// Get model name from response or use what was requested
			modelName := ""
			if llmResp != nil && llmResp.Model != "" {
				modelName = llmResp.Model
			} else {
				// Fallback to what was requested
				modelName = r.FormValue("model")
				if modelName == "" {
					modelName = os.Getenv("BASIC_OPENAI_MODEL")
					if modelName == "" {
						modelName = "llama-8b"
					}
				}
			}
			
			// Detect provider from model name
			providerEmoji := "⚫"
			providerName := "Unknown"
			
			if strings.Contains(modelName, "gpt") {
				providerEmoji = "🟢"
				providerName = "OpenAI"
			} else if strings.Contains(modelName, "claude") {
				providerEmoji = "🟠"
				providerName = "Anthropic"
			} else if strings.Contains(modelName, "gemini") {
				providerEmoji = "🔵"
				providerName = "Google"
			} else if strings.Contains(modelName, "llama") {
				providerEmoji = "🔷"
				providerName = "Meta"
			} else if strings.Contains(modelName, "mistral") || strings.Contains(modelName, "mixtral") {
				providerEmoji = "🟣"
				providerName = "Mistral"
			}
				
				// Add the badge HTML (no JavaScript onclick)
				fmt.Fprintf(w, `<div class="model-badge provider-%s">
					<div class="badge-toggle">
						<span class="provider-dot">%s</span>
						<span class="model-name">%s</span>
					</div>
				</div>`,
					strings.ToLower(providerName),
					providerEmoji,
					modelName,
				)
			
			fmt.Fprint(w, "</div>\n")

			// Keep the full HTML response in history with model metadata in a special format
			modelInfo := ""
			if llmResp != nil && llmResp.Model != "" {
				modelInfo = llmResp.Model
			} else {
				// Get model from form to use as fallback
				requestedModel := r.FormValue("model")
				if requestedModel != "" {
					modelInfo = requestedModel
				} else {
					modelInfo = "llama-8b"
				}
			}
			// Use a delimiter that won't appear in normal text
			finalHistory := history + fmt.Sprintf("Q: %s\nA: %s\n§MODEL:%s§\n\n", query, response, modelInfo)
			
			// sessionID already extracted at the top of the function
			
			// Get provider and model selections
			provider := r.FormValue("provider")
			if provider == "" {
				provider = "meta" // Default provider
			}
			model := r.FormValue("model")
			if model == "" {
				model = "llama-8b" // Default model
			}
			
			// No mode toggle needed for no-JS version
			
			// Build providers JSON and model options from actual registry
			var modelOptions string
			providerEmoji = "🔷"
			providerName = "Meta"
			// var providersJSON string = "{}" // Not needed in no-JS version
			
			if modelRegistry != nil {
				allModels := modelRegistry.List()
				
				// Build providers data structure
				type providerInfo struct {
					Name    string   `json:"name"`
					Color   string   `json:"color"`
					Emoji   string   `json:"emoji"`
					Models  []string `json:"models"`
					Enabled bool     `json:"enabled"`
				}
				
				// Use pointers to allow mutation
				providersData := map[string]*providerInfo{
					"openai": {
						Name:    "OpenAI",
						Color:   "#10A37F",
						Emoji:   "🟢",
						Models:  []string{},
						Enabled: true,
					},
					"anthropic": {
						Name:    "Anthropic",
						Color:   "#D97757",
						Emoji:   "🟠",
						Models:  []string{},
						Enabled: true,
					},
					"google": {
						Name:    "Google",
						Color:   "#4285F4",
						Emoji:   "🔵",
						Models:  []string{},
						Enabled: true,
					},
					"meta": {
						Name:    "Meta",
						Color:   "#0668E1",
						Emoji:   "🔷",
						Models:  []string{},
						Enabled: true,
					},
				}
				
				// Populate models for each provider
				for _, m := range allModels {
					switch m.Family {
					case "gpt":
						providersData["openai"].Models = append(providersData["openai"].Models, m.ID)
					case "claude":
						providersData["anthropic"].Models = append(providersData["anthropic"].Models, m.ID)
					case "gemini":
						providersData["google"].Models = append(providersData["google"].Models, m.ID)
					case "llama":
						providersData["meta"].Models = append(providersData["meta"].Models, m.ID)
					// Note: mixtral family exists but has no valid deployments, so it won't show up
					}
				}
				
				// Convert to JSON
				// Not needed in no-JS version
				// if jsonBytes, err := json.Marshal(providersData); err == nil {
				// 	providersJSON = string(jsonBytes)
				// }
				
				// Build model options for current provider
				var modelsForProvider []string
				
				switch provider {
				case "openai":
					providerEmoji = "🟢"
					providerName = "OpenAI"
					modelsForProvider = providersData["openai"].Models
				case "anthropic":
					providerEmoji = "🟠"
					providerName = "Anthropic"
					modelsForProvider = providersData["anthropic"].Models
				case "google":
					providerEmoji = "🔵"
					providerName = "Google"
					modelsForProvider = providersData["google"].Models
				case "meta":
					providerEmoji = "🔷"
					providerName = "Meta"
					modelsForProvider = providersData["meta"].Models
				default:
					// Default to Meta/Llama
					providerEmoji = "🔷"
					providerName = "Meta"
					modelsForProvider = providersData["meta"].Models
				}
				
				// Build options HTML
				for _, modelID := range modelsForProvider {
					modelOptions += fmt.Sprintf(`<option value="%s">%s</option>`, modelID, modelID)
				}
				
				// Fallback if no models found
				if modelOptions == "" {
					modelOptions = `<option value="llama-8b">llama-8b</option>`
				}
			} else {
				// Fallback if registry not available
				modelOptions = `<option value="llama-8b">llama-8b</option>`
				// providersJSON = `{"meta": {"name": "Meta", "color": "#0668E1", "emoji": "🔷", "models": ["llama-8b"], "enabled": true}}` // Not needed
			}
			
			// No tier selection for no-JS version
			
			// Escape only the minimal necessary for textarea safety
			safeHistory := strings.ReplaceAll(finalHistory, "</textarea>", "&lt;/textarea&gt;")
			
			// Build radio button table for models
			modelTable := "<table class='model-radio-table'><tr><th>Provider</th><th>Models</th></tr>"
			
			// Determine which model was actually used (from response)
			actualModel := ""
			if llmResp != nil && llmResp.Model != "" {
				actualModel = llmResp.Model
			} else if model != "" {
				actualModel = model
			} else {
				actualModel = "llama-8b"
			}
			
			if modelRegistry != nil {
				// Group models by provider
				type providerGroup struct{
					Emoji string
					Name string
					Models []string
				}
				providerGroups := map[string]*providerGroup{
					"meta": {"🔷", "Meta", []string{}},
					"openai": {"🟢", "OpenAI", []string{}},
					"anthropic": {"🟠", "Anthropic", []string{}},
					"google": {"🔵", "Google", []string{}},
				}
				
				allModels := modelRegistry.List()
				for _, m := range allModels {
					switch m.Family {
					case "gpt":
						providerGroups["openai"].Models = append(providerGroups["openai"].Models, m.ID)
					case "claude":
						providerGroups["anthropic"].Models = append(providerGroups["anthropic"].Models, m.ID)
					case "gemini":
						providerGroups["google"].Models = append(providerGroups["google"].Models, m.ID)
					case "llama":
						providerGroups["meta"].Models = append(providerGroups["meta"].Models, m.ID)
					}
				}
				
				// Build table rows
				for _, providerKey := range []string{"meta", "openai", "anthropic", "google"} {
					group := providerGroups[providerKey]
					if len(group.Models) > 0 {
						modelTable += fmt.Sprintf("<tr><td>%s %s</td><td>", group.Emoji, group.Name)
						for _, modelID := range group.Models {
							checked := ""
							if modelID == actualModel {
								checked = "checked"
							}
							modelTable += fmt.Sprintf(`<label><input type="radio" name="model" value="%s" %s> %s</label> `,
								modelID, checked, modelID)
						}
						modelTable += "</td></tr>"
					}
				}
			} else {
				// Fallback if registry not available
				checked := ""
				if actualModel == "llama-8b" {
					checked = "checked"
				}
				modelTable += fmt.Sprintf(`<tr><td>🔷 Meta</td><td><label><input type="radio" name="model" value="llama-8b" %s> llama-8b</label></td></tr>`, checked)
			}
			modelTable += "</table>"
			
			
			// Update session sequence after successful processing
			if sessionID == "" {
				sessionID = fmt.Sprintf("sess_%d_%s", time.Now().Unix(), generateRequestID()[:8])
			}
			nextSeq := 1
			if seqStr != "" {
				if seq, err := strconv.Atoi(seqStr); err == nil {
					nextSeq = seq + 1
					// Store the new sequence number
					sessionMu.Lock()
					sessionSeqs[sessionID] = seq
					sessionMu.Unlock()
					// Stored sequence
				}
			}
			
			// Format footer with session tracking
			fmt.Fprintf(w, htmlFooterTemplate,
				safeHistory,  // conversation history
				sessionID,    // session ID
				nextSeq,      // next sequence number
				modelTable,   // model radio button table
			)
			
			// Calculate final telemetry
			telemetry.Duration = time.Since(telemetry.StartTime)
			telemetry.Status = 200
			telemetry.Query = query
			telemetry.ResponseType = "html_stream"
			
			// Beacon comprehensive request telemetry
			beacon("request_complete", map[string]interface{}{
				"request_id":       telemetry.RequestID,
				"status":           telemetry.Status,
				"duration_ms":      telemetry.Duration.Milliseconds(),
				"has_query":        true,
				"query_hash":       generateSignature(query),
				"response_type":    telemetry.ResponseType,
				"input_hash":       telemetry.InputHash,
				"output_hash":      telemetry.OutputHash,
				"input_tokens":     telemetry.InputTokens,
				"output_tokens":    telemetry.OutputTokens,
				"total_tokens":     telemetry.InputTokens + telemetry.OutputTokens,
				"model":            telemetry.Model,
				"finish_reason":    telemetry.FinishReason,
				"content_filtered": telemetry.ContentFiltered,
			})
			return
		}

		// More strict curl detection: only exact match or curl/ prefix
		isCurl := (userAgent == "curl" || strings.HasPrefix(userAgent, "curl/")) && !wantsHTML && !wantsJSON && !wantsStream
		if isCurl {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Transfer-Encoding", "chunked")
			w.Header().Set("X-Accel-Buffering", "no")
			flusher := w.(http.Flusher)

			// No Q: A: format - just stream the response
			// flusher.Flush()

			ch := make(chan string)
			var llmResp *LLMResponse
			go func() {
				var resp *LLMResponse
				var err error
				
				// Router MUST be available - no fallback!
				if modelRouter != nil {
					resp, err = LLMWithRouter(prompt, tierToModel(tier), nil, ch)
				} else {
					err = fmt.Errorf("model router not initialized")
				}
				if err != nil {
					// Log the error but don't try to send it
					// The channel is managed by LLM/LLMWithRouter
					// LLM error
				} else {
					llmResp = resp
				}
			}()

			response := ""
			for chunk := range ch {
				fmt.Fprint(w, chunk)
				response += chunk
				flusher.Flush()
			}
			
			// Update telemetry with LLM response data if available
			if llmResp != nil {
				telemetry.InputHash = llmResp.InputHash
				telemetry.OutputHash = llmResp.OutputHash
				telemetry.InputTokens = llmResp.InputTokens
				telemetry.OutputTokens = llmResp.OutputTokens
				telemetry.Model = llmResp.Model
				telemetry.FinishReason = llmResp.FinishReason
				telemetry.ContentFiltered = llmResp.ContentFiltered
			}
			fmt.Fprint(w, "\n")
			
			// Calculate final telemetry
			telemetry.Duration = time.Since(telemetry.StartTime)
			telemetry.Status = 200
			telemetry.Query = query
			telemetry.ResponseType = "curl"
			
			// Beacon comprehensive request telemetry
			beacon("request_complete", map[string]interface{}{
				"request_id":       telemetry.RequestID,
				"status":           telemetry.Status,
				"duration_ms":      telemetry.Duration.Milliseconds(),
				"has_query":        true,
				"query_hash":       generateSignature(query),
				"response_type":    telemetry.ResponseType,
				"input_hash":       telemetry.InputHash,
				"output_hash":      telemetry.OutputHash,
				"input_tokens":     telemetry.InputTokens,
				"output_tokens":    telemetry.OutputTokens,
				"total_tokens":     telemetry.InputTokens + telemetry.OutputTokens,
				"model":            telemetry.Model,
				"finish_reason":    telemetry.FinishReason,
				"content_filtered": telemetry.ContentFiltered,
			})
			return
		}

		promptToUse := prompt
		if wantsHTML {
			promptToUse = htmlPromptPrefix + prompt
		}
		
		var llmResp *LLMResponse
		var err error
		
		// Router MUST be available - no fallback!
		if modelRouter != nil {
			llmResp, err = LLMWithRouter(promptToUse, tierToModel(tier), nil, nil)
		} else {
			err = fmt.Errorf("model router not initialized")
		}
		if err != nil {
			content = err.Error()
			errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
			jsonResponse = string(errJSON)
		} else {
			// Update telemetry with LLM response data
			telemetry.InputHash = llmResp.InputHash
			telemetry.OutputHash = llmResp.OutputHash
			telemetry.InputTokens = llmResp.InputTokens
			telemetry.OutputTokens = llmResp.OutputTokens
			telemetry.Model = llmResp.Model
			telemetry.FinishReason = llmResp.FinishReason
			telemetry.ContentFiltered = llmResp.ContentFiltered
			
			respJSON, _ := json.Marshal(map[string]string{
				"question": query,
				"answer":   llmResp.Content,
			})
			jsonResponse = string(respJSON)

			// Just return the response content, NO Q: A: FORMAT!
			content = llmResp.Content
			if len(content) > 65536 {
				content = content[:65536]
			}
		}
	} else if history != "" {
		content = history
	}

	if wantsStream && query != "" {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		ch := make(chan string)
		var llmResp *LLMResponse
		go func() {
			var resp *LLMResponse
			var err error
			
			// Router MUST be available - no fallback!
			if modelRouter != nil {
				resp, err = LLMWithRouter(prompt, tierToModel(tier), nil, ch)
			} else {
				err = fmt.Errorf("model router not initialized")
			}
			if err != nil {
				fmt.Fprintf(w, "data: Error: %s\n\n", err.Error())
				flusher.Flush()
			} else {
				llmResp = resp
			}
		}()

		for chunk := range ch {
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		
		// Update telemetry with LLM response data if available
		if llmResp != nil {
			telemetry.InputHash = llmResp.InputHash
			telemetry.OutputHash = llmResp.OutputHash
			telemetry.InputTokens = llmResp.InputTokens
			telemetry.OutputTokens = llmResp.OutputTokens
			telemetry.Model = llmResp.Model
			telemetry.FinishReason = llmResp.FinishReason
			telemetry.ContentFiltered = llmResp.ContentFiltered
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		
		// Calculate final telemetry
		telemetry.Duration = time.Since(telemetry.StartTime)
		telemetry.Status = 200
		telemetry.Query = query
		telemetry.ResponseType = "event-stream"
		
		// Note: For streaming, we don't have token counts unless we track the response
		beacon("request_complete", map[string]interface{}{
			"request_id":       telemetry.RequestID,
			"status":           telemetry.Status,
			"duration_ms":      telemetry.Duration.Milliseconds(),
			"has_query":        true,
			"query_hash":       generateSignature(query),
			"response_type":    telemetry.ResponseType,
			"input_hash":       telemetry.InputHash,
			"output_hash":      telemetry.OutputHash,
			"input_tokens":     telemetry.InputTokens,
			"output_tokens":    telemetry.OutputTokens,
			"total_tokens":     telemetry.InputTokens + telemetry.OutputTokens,
			"model":            telemetry.Model,
			"finish_reason":    telemetry.FinishReason,
			"content_filtered": telemetry.ContentFiltered,
		})
		return
	}

	if wantsJSON && jsonResponse != "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, jsonResponse)
	} else if wantsHTML && query == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; object-src 'none'; base-uri 'none'; style-src 'unsafe-inline'")
		
		// EXACT pixel calculation - no estimates
		parts := strings.Split("\n"+content, "\nQ: ")
		
		// Known constants:
		// Container width: 700px
		// Font: 16px, ~8px per char average (system-ui)
		// Chars per line: 700px / 8px = ~87 chars
		// Line height: 24px
		// Q padding: 20px (1.25rem)
		// A padding: 24px (1.5rem top/bottom)
		// Badge: 32px total height
		// Margins: 24px between messages
		
		const charsPerLine = 87
		const lineHeight = 24
		totalPixels := 0
		
		for _, part := range parts[1:] {
			if i := strings.Index(part, "\nA: "); i >= 0 {
				question := part[:i]
				answer := part[i+4:]
				
				// Strip metadata for accurate char count
				if modelIdx := strings.Index(answer, "§MODEL:"); modelIdx >= 0 {
					answer = answer[:modelIdx]
				}
				
				// Q: lines * lineHeight + padding
				qLines := (len(question) + charsPerLine - 1) / charsPerLine
				qHeight := qLines*lineHeight + 20 + 20 // top+bottom padding
				
				// A: lines * lineHeight + padding  
				aLines := (len(answer) + charsPerLine - 1) / charsPerLine
				aHeight := aLines*lineHeight + 24 + 24 // top+bottom padding
				
				// Total: Q + A + badge + margin
				totalPixels += qHeight + aHeight + 32 + 24
			}
		}
		
		fmt.Fprint(w, htmlHeader)
		
		// Add spacer to scroll to bottom if needed
		// Chat container starts ~200px from top (header+padding)
		// Viewport is ~600px for chat area
		if totalPixels > 600 {
			spacerHeight := totalPixels - 400 // Leave some visible at top
			fmt.Fprintf(w, `<div style="height:%dpx;margin-bottom:-%dpx;"></div>`, spacerHeight, spacerHeight)
		}
		
		for _, part := range parts[1:] {
			if i := strings.Index(part, "\nA: "); i >= 0 {
				question := part[:i]
				answer := part[i+4:]
				
				// Extract model metadata if present (can be at end of answer)
				modelName := "llama-8b" // default
				if modelIdx := strings.Index(answer, "§MODEL:"); modelIdx >= 0 {
					modelStart := modelIdx + 7
					if endIdx := strings.Index(answer[modelStart:], "§"); endIdx >= 0 {
						modelName = answer[modelStart : modelStart+endIdx]
						// Remove the metadata line from the answer
						answer = answer[:modelIdx] + answer[modelStart+endIdx+1:]
					}
				}
				
				answer = strings.TrimRight(answer, "\n")
				fmt.Fprintf(w, "<div class=\"q\">%s</div>\n", html.EscapeString(question))
				
				// Add answer with badge for ALL responses
				fmt.Fprintf(w, "<div class=\"a\">%s", answer)
				
				// Generate badge for historical response
				
				// Detect provider from model name
				providerEmoji := "⚫"
				providerName := "Unknown"
				
				if strings.Contains(modelName, "gpt") {
					providerEmoji = "🟢"
					providerName = "OpenAI"
				} else if strings.Contains(modelName, "claude") {
					providerEmoji = "🟠"
					providerName = "Anthropic"
				} else if strings.Contains(modelName, "gemini") {
					providerEmoji = "🔵"
					providerName = "Google"
				} else if strings.Contains(modelName, "llama") {
					providerEmoji = "🔷"
					providerName = "Meta"
				} else if strings.Contains(modelName, "mistral") || strings.Contains(modelName, "mixtral") {
					providerEmoji = "🟣"
					providerName = "Mistral"
				}
				
				// Add the badge (no JavaScript onclick)
				fmt.Fprintf(w, `<div class="model-badge provider-%s">
					<div class="badge-toggle">
						<span class="provider-dot">%s</span>
						<span class="model-name">%s</span>
					</div>
				</div>`,
					strings.ToLower(providerName),
					providerEmoji,
					modelName,
				)
				
				fmt.Fprintf(w, "</div>\n")
			}
		}

		// Default settings for initial page load
		// Escape only </textarea> to prevent breaking out
		safeContent := strings.ReplaceAll(content, "</textarea>", "&lt;/textarea&gt;")
		
		// Generate new session ID for new chat
		// sessionID := fmt.Sprintf("sess_%d_%s", time.Now().Unix(), generateRequestID()[:8]) // Not needed in no-JS version
		
		// Build radio button table for initial page load
		modelTable := "<table class='model-radio-table'><tr><th>Provider</th><th>Models</th></tr>"
		
		// Get system's configured default model from environment (loaded via godotenv)
		defaultModel := os.Getenv("BASIC_OPENAI_MODEL")
		if defaultModel == "" {
			// If not set, try to use first available model
			if modelRegistry != nil {
				allModels := modelRegistry.List()
				if len(allModels) > 0 {
					defaultModel = allModels[0].ID
				}
			}
		}
		
		if modelRegistry != nil {
			// Group models by provider
			type providerGroup struct{
				Emoji string
				Name string
				Models []string
			}
			providerGroups := map[string]*providerGroup{
				"meta": {"🔷", "Meta", []string{}},
				"openai": {"🟢", "OpenAI", []string{}},
				"anthropic": {"🟠", "Anthropic", []string{}},
				"google": {"🔵", "Google", []string{}},
			}
			
			allModels := modelRegistry.List()
			
			for _, m := range allModels {
				switch m.Family {
				case "gpt":
					providerGroups["openai"].Models = append(providerGroups["openai"].Models, m.ID)
				case "claude":
					providerGroups["anthropic"].Models = append(providerGroups["anthropic"].Models, m.ID)
				case "gemini":
					providerGroups["google"].Models = append(providerGroups["google"].Models, m.ID)
				case "llama":
					providerGroups["meta"].Models = append(providerGroups["meta"].Models, m.ID)
				}
			}
			
			// Build table rows with default selected (first available model)
			for _, providerKey := range []string{"meta", "openai", "anthropic", "google"} {
				group := providerGroups[providerKey]
				if len(group.Models) > 0 {
					modelTable += fmt.Sprintf("<tr><td>%s %s</td><td>", group.Emoji, group.Name)
					for _, modelID := range group.Models {
						checked := ""
						if modelID == defaultModel {
							checked = "checked"
						}
						modelTable += fmt.Sprintf(`<label><input type="radio" name="model" value="%s" %s> %s</label>`,
							modelID, checked, modelID)
					}
					modelTable += "</td></tr>"
				}
			}
		} else {
			// Fallback if registry not available - no models
			modelTable += `<tr><td colspan="2">No models available</td></tr>`
		}
		modelTable += "</table>"
		
		
		// Generate new session for initial page
		newSessionID := fmt.Sprintf("sess_%d_%s", time.Now().Unix(), generateRequestID()[:8])
		
		// Format footer for initial page
		fmt.Fprintf(w, htmlFooterTemplate,
			safeContent,  // history
			newSessionID, // new session ID
			1,            // starting sequence number
			modelTable,   // model radio button table
		)
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, content)
	}
	
	// Calculate final telemetry
	telemetry.Duration = time.Since(telemetry.StartTime)
	telemetry.Status = 200
	telemetry.Query = query
	telemetry.ResponseType = func() string {
		if wantsJSON { return "json" }
		if wantsHTML { return "html" }
		if wantsStream { return "stream" }
		return "plain"
	}()
	
	// Beacon comprehensive request telemetry
	beacon("request_complete", map[string]interface{}{
		"request_id":       telemetry.RequestID,
		"status":           telemetry.Status,
		"duration_ms":      telemetry.Duration.Milliseconds(),
		"has_query":        query != "",
		"query_hash":       generateInputSignature(query),
		"response_type":    telemetry.ResponseType,
		"input_hash":       telemetry.InputHash,
		"output_hash":      telemetry.OutputHash,
		"input_tokens":     telemetry.InputTokens,
		"output_tokens":    telemetry.OutputTokens,
		"total_tokens":     telemetry.InputTokens + telemetry.OutputTokens,
		"model":            telemetry.Model,
		"finish_reason":    telemetry.FinishReason,
		"content_filtered": telemetry.ContentFiltered,
	})
}

type ChatRequest struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	Stream           bool      `json:"stream,omitempty"`
	MaxTokens        int       `json:"max_tokens,omitempty"`
	Temperature      float64   `json:"temperature,omitempty"`
	TopP             float64   `json:"top_p,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
	FrequencyPenalty float64   `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64   `json:"presence_penalty,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	// Handle chat completions
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "86400")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !rateLimitAllow(r.RemoteAddr) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	if r.Method != "POST" {
		w.Header().Set("Allow", "POST, OPTIONS")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Failed to decode JSON
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Process request

	messages := make([]map[string]string, len(req.Messages))
	var fullContent string
	for i, msg := range req.Messages {
		messages[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
		fullContent += msg.Content + " "
	}
	
	// Use discriminator to analyze and potentially route to specialized modules
	if discriminator != nil {
		moduleResponse, err := discriminator.Process(fullContent, messages)
		if err != nil {
			// Module processing error
			// Fall through to default processing
		} else if moduleResponse != "" {
			// Module handled the request
			resp := ChatResponse{
				ID:      "chatcmpl-module-" + generateRequestID(),
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []Choice{{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: moduleResponse,
					},
					FinishReason: "stop",
				}},
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		// If moduleResponse is empty, fall through to default processing
	}

	// Router MUST be available - no fallback!
	if modelRouter == nil {
		http.Error(w, "Model router not initialized", http.StatusServiceUnavailable)
		return
	}
	
	if req.Model == "" {
		req.Model = "llama-8b" // Default model if not specified
	}
	
	// Build router parameters from request
	routerParams := &RouterParams{
		MaxTokens:        req.MaxTokens,
		Temperature:      req.Temperature,
		TopP:             req.TopP,
		Stop:             req.Stop,
		FrequencyPenalty: req.FrequencyPenalty,
		PresencePenalty:  req.PresencePenalty,
	}
	
	// Using router for model
	llmFunc := func(input interface{}, stream chan<- string) (*LLMResponse, error) {
		return LLMWithRouter(input, req.Model, routerParams, stream)
	}

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		ch := make(chan string)
		go llmFunc(messages, ch)

		for chunk := range ch {
			resp := map[string]interface{}{
				"id":      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   req.Model,
				"choices": []map[string]interface{}{{
					"index": 0,
					"delta": map[string]string{"content": chunk},
				}},
			}
			data, err := json.Marshal(resp)
			if err != nil {
				fmt.Fprintf(w, "data: Failed to marshal response\n\n")
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")

	} else {
		llmResp, err := llmFunc(messages, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		chatResp := ChatResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []Choice{{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: llmResp.Content,
				},
			}},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResp)
	}
}

// handleHealth provides a health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "healthy",
		"services": map[string]bool{
			"http":  HTTP_PORT > 0,
			"https": HTTPS_PORT > 0,
			"ssh":   SSH_PORT > 0,
			"dns":   DNS_PORT > 0,
		},
		"ports": map[string]int{
			"http":  HTTP_PORT,
			"https": HTTPS_PORT,
			"ssh":   SSH_PORT,
			"dns":   DNS_PORT,
		},
		"mode": "production",
	}
	
	if os.Getenv("HIGH_PORT_MODE") == "true" {
		health["mode"] = "development"
	}
	
	// Check if model router is configured
	if modelRouter != nil {
		health["llm_configured"] = true
		health["router_active"] = true
		health["default_model"] = "llama-8b" // Default when no model specified
		if modelRegistry != nil {
			allModels := modelRegistry.List()
			health["available_models"] = len(allModels)
		}
		if deploymentRegistry != nil {
			healthyDeps := deploymentRegistry.GetHealthy()
			health["healthy_deployments"] = len(healthyDeps)
		}
	} else {
		health["llm_configured"] = false
		health["router_active"] = false
	}
	
	// Add endpoints information
	baseURL := fmt.Sprintf("http://localhost:%d", HTTP_PORT)
	health["endpoints"] = map[string]interface{}{
		"chat_completions": map[string]string{
			"url":         baseURL + "/v1/chat/completions",
			"method":      "POST",
			"description": "OpenAI-compatible chat completions API",
		},
		"models_list": map[string]string{
			"url":         baseURL + "/v1/models",
			"method":      "GET",
			"description": "List all available models",
		},
		"routing_table": map[string]string{
			"url":         baseURL + "/routing_table",
			"method":      "GET",
			"description": "View complete model routing table and health status",
		},
		"terms_of_service": map[string]string{
			"url":         baseURL + "/terms_of_service",
			"method":      "GET",
			"description": "View terms of service and privacy policy",
		},
		"health": map[string]string{
			"url":         baseURL + "/health",
			"method":      "GET",
			"description": "This endpoint - system health and configuration",
		},
	}
	
	// Add privacy and terms information
	health["privacy"] = map[string]interface{}{
		"audit_logging": auditEnabled,
		"terms_url":     baseURL + "/terms_of_service",
		"policy":        "View full terms at /terms_of_service endpoint",
	}
	
	// Check SSL certificates for HTTPS
	if HTTPS_PORT > 0 {
		_, _, found := findSSLCertificates()
		health["ssl_certificates"] = found
	}
	
	// Check DoNutSentry configuration
	if donutSentryDomain != "" {
		health["donutsentry_domain"] = donutSentryDomain
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}