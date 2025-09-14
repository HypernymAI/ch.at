package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const htmlPromptPrefix = "You are a helpful assistant. Use HTML formatting instead of markdown (no CSS or style attributes): "

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
        form { max-width: 568px; margin: 0 auto 3rem; display: flex; gap: .5rem; }
        input[type="text"] { width: 100%; padding: .5rem; }
        input[type="submit"] { padding: .5rem; }
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
        <input type="text" name="q" placeholder="Type your message..." autofocus>
        <input type="submit" value="Send">
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

	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, nil)
}

func StartHTTPSServer(port int, certFile, keyFile string) error {
	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServeTLS(addr, certFile, keyFile, nil)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if !rateLimitAllow(r.RemoteAddr) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	var query, history, prompt string
	content := ""
	jsonResponse := ""

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}
		query = r.FormValue("q")
		history = r.FormValue("h")

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
						fmt.Fprintf(w, "<div class=\"a\">%s</div>\n", html.EscapeString(answer))
					}
				}
			}
			fmt.Fprintf(w, "<div class=\"q\">%s</div>\n<div class=\"a\">", html.EscapeString(query))
			flusher.Flush()

			ch := make(chan string)
			go func() {
				htmlPrompt := htmlPromptPrefix + prompt
				if _, err := LLM(htmlPrompt, ch); err != nil {
					ch <- err.Error()
					close(ch)
				}
			}()

			response := ""
			for chunk := range ch {
				escapedChunk := html.EscapeString(chunk)
				if _, err := fmt.Fprint(w, escapedChunk); err != nil {
					return
				}
				response += escapedChunk
				flusher.Flush()
			}
			fmt.Fprint(w, "</div>\n")

			finalHistory := history + fmt.Sprintf("Q: %s\nA: %s\n\n", query, response)
			fmt.Fprintf(w, htmlFooterTemplate, html.EscapeString(finalHistory))
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
			go func() {
				if _, err := LLM(prompt, ch); err != nil {
					ch <- err.Error()
					close(ch)
				}
			}()

			response := ""
			for chunk := range ch {
				fmt.Fprint(w, chunk)
				response += chunk
				flusher.Flush()
			}
			fmt.Fprint(w, "\n")
			return
		}

		promptToUse := prompt
		if wantsHTML {
			promptToUse = htmlPromptPrefix + prompt
		}
		llmResp, err := LLM(promptToUse, nil)
		if err != nil {
			content = err.Error()
			errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
			jsonResponse = string(errJSON)
		} else {
			response := llmResp.Content
			respJSON, _ := json.Marshal(map[string]string{
				"question": query,
				"answer":   response,
			})
			jsonResponse = string(respJSON)

			newExchange := fmt.Sprintf("Q: %s\nA: %s\n\n", query, response)
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
		go func() {
			if _, err := LLM(prompt, ch); err != nil {
				fmt.Fprintf(w, "data: Error: %s\n\n", err.Error())
				flusher.Flush()
			}
		}()

		for chunk := range ch {
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
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

		fmt.Fprintf(w, htmlFooterTemplate, html.EscapeString(content))
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, content)
	}
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
	Index   int     `json:"index"`
	Message Message `json:"message"`
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
	for i, msg := range req.Messages {
		messages[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
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
		go LLM(messages, ch)

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
		llmResp, err := LLM(messages, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		response := llmResp.Content

		chatResp := ChatResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []Choice{{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: response,
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
