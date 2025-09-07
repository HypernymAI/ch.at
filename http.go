package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const htmlPromptPrefix = "You are a helpful assistant. Use HTML formatting instead of markdown (no CSS or style attributes): "

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
        body { text-align: center; margin: 2.5rem; }
        .chat { text-align: left; max-width: 600px; margin: 1.25rem auto; }
        .q { padding: 1.25rem; background: #EEE; font-style: italic; font-size: large; }
        .a { padding: 0.5rem 1.25rem; }
        form { max-width: 568px; margin: 0 auto 3rem; }
        .input-row { display: flex; gap: .5rem; margin-bottom: .5rem; }
        input[type="text"] { width: 100%; padding: .5rem; }
        input[type="submit"] { padding: .5rem; }
        .tier-selection { display: flex; gap: 1rem; justify-content: center; font-size: 0.9rem; }
        .tier-selection label { cursor: pointer; }
        .tier-selection input[type="radio"] { cursor: pointer; }
		@media (prefers-color-scheme: dark) {
			body { background: #181a1b; color: #e8e6e3; }
			.chat { background: #222326; }
			.q { background: #23262a; color: #c9d1d9; }
			.a { color: #e8e6e3; }
			input[type="text"], input[type="submit"] { background: #23262a; color: #e8e6e3; border: 1px solid #444; }
			form { background: #181a1b; }
			a { color: #58a6ff; }
		}
    </style>
</head>
<body>
    <h1>ch.at</h1>
    <p>Universal Basic Intelligence</p>
    <p><small><i>pronounced "ch-dot-at"</i></small></p>
    <div class="chat">`

const htmlFooterTemplate = `</div>
    <form method="POST" action="/">
        <div class="tier-selection">
            <label><input type="radio" name="tier" value="fast" %s> Fast</label>
            <label><input type="radio" name="tier" value="balanced" %s> Balanced</label>
            <label><input type="radio" name="tier" value="frontier" %s> Frontier</label>
        </div>
        <div class="input-row">
            <input type="text" name="q" placeholder="Type your message..." autofocus>
            <input type="submit" value="Send">
        </div>
        <textarea name="h" style="display:none">%s</textarea>
    </form>
    <p><a href="/">New Chat</a></p>
    <p><small>
        Also available: ssh ch.at • curl ch.at/?q=hello • dig @ch.at "question" TXT<br>
        No logs • No accounts • Free software • <a href="https://github.com/Deep-ai-inc/ch.at">GitHub</a>
    </small></p>
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

	var query, history, prompt, tier string
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
		prompt = query
		if history != "" {
			prompt = history + "Q: " + query
		}

		if wantsHTML && r.Header.Get("Accept") != "application/json" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Transfer-Encoding", "chunked")
			w.Header().Set("X-Accel-Buffering", "no")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'none'; object-src 'none'; base-uri 'none'; style-src 'unsafe-inline'")
			flusher := w.(http.Flusher)

			headerSize := len(htmlHeader)
			historySize := len(html.EscapeString(history))
			querySize := len(html.EscapeString(query))
			currentSize := headerSize + historySize + querySize + 10

			const minThreshold = 6144

			fmt.Fprint(w, htmlHeader)
			
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
						answer = strings.TrimRight(answer, "\n")
						fmt.Fprintf(w, "<div class=\"q\">%s</div>\n", html.EscapeString(question))
						// History answers contain HTML, render them as-is
						fmt.Fprintf(w, "<div class=\"a\">%s</div>\n", answer)
					}
				}
			}
			fmt.Fprintf(w, "<div class=\"q\">%s</div>\n<div class=\"a\">", html.EscapeString(query))
			flusher.Flush()

			ch := make(chan string)
			var llmResp *LLMResponse
			go func() {
				htmlPrompt := htmlPromptPrefix + prompt
				var resp *LLMResponse
				var err error
				
				// Use router if available with tier selection
				if modelRouter != nil {
					resp, err = LLMWithRouter(htmlPrompt, tierToModel(tier), ch)
				} else {
					resp, err = LLM(htmlPrompt, ch)
				}
				if err != nil {
					// Log the error but don't try to send it
					// The channel is managed by LLM/LLMWithRouter
					log.Printf("LLM error: %v", err)
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
			fmt.Fprint(w, "</div>\n")

			// Keep the full HTML response in history for consistent rendering
			finalHistory := history + fmt.Sprintf("Q: %s\nA: %s\n\n", query, response)
			// Format the radio buttons with current tier selection
			fastChecked := ""
			balancedChecked := "checked"
			frontierChecked := ""
			if tier == "fast" {
				fastChecked = "checked"
				balancedChecked = ""
			} else if tier == "frontier" {
				frontierChecked = "checked"
				balancedChecked = ""
			}
			// Escape only the minimal necessary for textarea safety
			// Replace </textarea> to prevent breaking out of the textarea
			safeHistory := strings.ReplaceAll(finalHistory, "</textarea>", "&lt;/textarea&gt;")
			fmt.Fprintf(w, htmlFooterTemplate, fastChecked, balancedChecked, frontierChecked, safeHistory)
			
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

			fmt.Fprintf(w, "Q: %s\nA: ", query)
			flusher.Flush()

			ch := make(chan string)
			var llmResp *LLMResponse
			go func() {
				var resp *LLMResponse
				var err error
				
				// Use router if available with tier selection
				if modelRouter != nil {
					resp, err = LLMWithRouter(prompt, tierToModel(tier), ch)
				} else {
					resp, err = LLM(prompt, ch)
				}
				if err != nil {
					// Log the error but don't try to send it
					// The channel is managed by LLM/LLMWithRouter
					log.Printf("LLM error: %v", err)
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
		
		// Use router if available with tier selection
		if modelRouter != nil {
			llmResp, err = LLMWithRouter(promptToUse, tierToModel(tier), nil)
		} else {
			llmResp, err = LLM(promptToUse, nil)
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

			newExchange := fmt.Sprintf("Q: %s\nA: %s\n\n", query, llmResp.Content)
			if history != "" {
				content = history + newExchange
			} else {
				content = newExchange
			}
			if len(content) > 65536 {
				newExchangeLen := len(newExchange)
				if newExchangeLen > 65536 {
					content = newExchange[:65536]
				} else {
					maxHistory := 65536 - newExchangeLen
					if len(history) > maxHistory {
						content = history[len(history)-maxHistory:] + newExchange
					}
				}
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
			
			// Use router if available with tier selection
			if modelRouter != nil {
				resp, err = LLMWithRouter(prompt, tierToModel(tier), ch)
			} else {
				resp, err = LLM(prompt, ch)
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
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'none'; object-src 'none'; base-uri 'none'; style-src 'unsafe-inline'")
		fmt.Fprint(w, htmlHeader)
		parts := strings.Split("\n"+content, "\nQ: ")
		for _, part := range parts[1:] {
			if i := strings.Index(part, "\nA: "); i >= 0 {
				question := part[:i]
				answer := part[i+4:]
				answer = strings.TrimRight(answer, "\n")
				fmt.Fprintf(w, "<div class=\"q\">%s</div>\n", html.EscapeString(question))
				fmt.Fprintf(w, "<div class=\"a\">%s</div>\n", answer)
			}
		}

		// Default tier selection for initial page load
		// Escape only </textarea> to prevent breaking out
		safeContent := strings.ReplaceAll(content, "</textarea>", "&lt;/textarea&gt;")
		fmt.Fprintf(w, htmlFooterTemplate, "", "checked", "", safeContent)
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
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
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
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

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
			log.Printf("[handleChatCompletions] Module processing error: %v", err)
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

	// Determine which LLM function to use
	var llmFunc func(interface{}, chan<- string) (*LLMResponse, error)
	
	// Check if router is available and model is specified
	if modelRouter != nil && req.Model != "" {
		log.Printf("[handleChatCompletions] Using router for model: %s", req.Model)
		// Use new router
		llmFunc = func(input interface{}, stream chan<- string) (*LLMResponse, error) {
			return LLMWithRouter(input, req.Model, stream)
		}
	} else {
		log.Printf("[handleChatCompletions] Falling back to legacy LLM (router=%v, model=%s)", modelRouter != nil, req.Model)
		// Fall back to legacy LLM
		llmFunc = LLM
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
	
	// Check if LLM is configured
	if apiKey != "" && apiURL != "" && modelName != "" {
		health["llm_configured"] = true
		health["llm_model"] = modelName
	} else {
		health["llm_configured"] = false
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
