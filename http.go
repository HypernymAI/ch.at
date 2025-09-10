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
        body { text-align: center; margin: 2.5rem; font-family: system-ui, -apple-system, sans-serif; background: #FFF8F0; color: #2C1F3D; }
        .chat { text-align: left; max-width: 700px; margin: 1.25rem auto; }
        .q { padding: 1.25rem; background: #E8DCC4; font-style: italic; font-size: large; border-left: 4px solid #6B4C8A; }
        .a { 
            padding: 1.5rem 1.25rem; 
            position: relative; 
            background: #FFFBF5; 
            margin: 1.5rem 0 0.5rem 0; 
            border-radius: 8px;
            border: 1px solid #E8DCC4;
        }
        form { max-width: 700px; margin: 0 auto 3rem; }
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
            justify-content: center;
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
        
        /* Control visibility */
        #advanced-controls { display: block; }
        #simplified-controls { display: none; }
        #advanced-controls.hidden { display: none; }
        #simplified-controls.hidden { display: none; }
        
        /* Dark mode */
        @media (prefers-color-scheme: dark) {
            body { background: #181a1b; color: #e8e6e3; }
            .chat { background: #222326; }
            .q { background: #23262a; color: #c9d1d9; }
            .a { color: #e8e6e3; }
            input[type="text"], input[type="submit"] { background: #23262a; color: #e8e6e3; border: 1px solid #444; }
            form { background: #181a1b; }
            a { color: #58a6ff; }
            .provider-dropdown, .provider-options, .model-dropdown, .badge-toggle, .metadata-panel {
                background: #2a2a2a;
                border-color: #444;
                color: #e8e6e3;
            }
            .provider-option:hover { background: #333; }
            .mode-toggle label { background: #2a2a2a; border-color: #444; color: #e8e6e3; }
            .mode-toggle input[type="radio"]:checked + label { background: #58a6ff; color: #000; border-color: #58a6ff; }
        }
    </style>
</head>
<body>
    <h1>ch.at</h1>
    <p>Universal Basic Intelligence</p>
    <p><small><i>pronounced "ch-dot-at"</i></small></p>
    <div class="chat">`

const htmlFooterTemplate = `</div>
    <!-- Mode Toggle -->
    <div class="mode-toggle">
        <input type="radio" id="mode-advanced" name="ui-mode" value="advanced" %s>
        <label for="mode-advanced">⚙️ Advanced</label>
        
        <input type="radio" id="mode-simplified" name="ui-mode" value="simplified" %s>
        <label for="mode-simplified">⚡ Simplified</label>
    </div>
    
    <!-- Advanced Mode Controls -->
    <div id="advanced-controls" class="%s">
        <div class="provider-select">
            <div class="provider-dropdown" onclick="toggleProviderDropdown()">
                <span class="provider-dot">%s</span>
                <span class="provider-name">%s</span>
                <span style="margin-left: auto;">▼</span>
            </div>
            <div class="provider-options" id="provider-options"></div>
        </div>
        
        <div class="model-select">
            <select id="model-dropdown" class="model-dropdown" name="adv_model">
                %s
            </select>
        </div>
    </div>
    
    <!-- Simplified Mode Controls -->
    <div id="simplified-controls" class="%s">
        <div class="tier-selection">
            <label><input type="radio" name="tier" value="fast" %s> ⚡ Fast</label>
            <label><input type="radio" name="tier" value="balanced" %s> ⚖️ Balanced</label>
            <label><input type="radio" name="tier" value="frontier" %s> 🚀 Frontier</label>
        </div>
    </div>
    
    <form id="chat-form" onsubmit="sendMessage(event); return false;">
        <input type="hidden" name="mode" id="mode-input" value="%s">
        <input type="hidden" name="provider" id="provider-input" value="%s">
        <input type="hidden" name="model" id="model-input" value="%s">
        <input type="hidden" name="session_id" id="session-input" value="%s">
        <div class="input-row">
            <input type="text" name="q" id="query-input" placeholder="Type your message..." autofocus>
            <input type="submit" value="Send" id="send-button">
        </div>
        <textarea name="h" id="history-input" style="display:none">%s</textarea>
    </form>
    
    <p><a href="#" onclick="localStorage.removeItem('chat_conversation'); localStorage.removeItem('conversation_id'); conversationMetadata.conversationId = null; document.querySelector('.chat').innerHTML = ''; return false;">New Chat</a></p>
    <p><small>
        Also available: ssh ch.at • curl ch.at/?q=hello • dig @ch.at "question" TXT<br>
        No logs • No accounts • Free software • <a href="https://github.com/Deep-ai-inc/ch.at">GitHub</a>
    </small></p>
    
    <script>
        // Provider data with branding - dynamically generated from registry
        const providers = %s;
        
        // Response metadata storage
        const conversationMetadata = {
            sessionId: document.getElementById('session-input').value || generateSessionId(),
            conversationId: null,  // Will be set on first message
            responses: {},
            currentMode: document.getElementById('mode-input').value || 'advanced'
        };
        
        // Load conversation from localStorage
        function loadConversation() {
            const saved = localStorage.getItem('chat_conversation');
            if (saved) {
                const conversation = JSON.parse(saved);
                const chatDiv = document.querySelector('.chat');
                chatDiv.innerHTML = ''; // Clear existing
                
                conversation.messages.forEach(msg => {
                    if (msg.type === 'question') {
                        const qDiv = document.createElement('div');
                        qDiv.className = 'q';
                        qDiv.textContent = msg.content;
                        chatDiv.appendChild(qDiv);
                    } else if (msg.type === 'answer') {
                        const aDiv = document.createElement('div');
                        aDiv.className = 'a';
                        aDiv.innerHTML = msg.content;
                        
                        // Add badge
                        const badgeHTML = '<div class="model-badge provider-' + msg.provider.toLowerCase() + '">' +
                            '<button class="badge-toggle" onclick="toggleMetadata(\'' + msg.id + '\')">' +
                                '<span class="provider-dot">' + msg.providerEmoji + '</span>' +
                                '<span class="model-name">' + msg.model + '</span>' +
                                '<span class="expand-icon">▼</span>' +
                            '</button>' +
                            '<div class="metadata-panel" id="metadata-' + msg.id + '">' +
                                '<table class="metadata-table">' +
                                    '<tr><td>Model:</td><td>' + msg.model + '</td></tr>' +
                                    '<tr><td>Provider:</td><td>' + msg.provider + '</td></tr>' +
                                    '<tr><td>Time:</td><td>' + msg.timestamp + '</td></tr>' +
                                '</table>' +
                            '</div>' +
                        '</div>';
                        
                        aDiv.innerHTML = msg.content + badgeHTML;
                        chatDiv.appendChild(aDiv);
                    }
                });
            }
        }
        
        // Save conversation to localStorage
        function saveConversation() {
            const messages = [];
            const chatDiv = document.querySelector('.chat');
            const questions = chatDiv.querySelectorAll('.q');
            const answers = chatDiv.querySelectorAll('.a');
            
            questions.forEach((q, i) => {
                messages.push({
                    type: 'question',
                    content: q.textContent
                });
                
                if (answers[i]) {
                    // Extract model info from badge
                    const badge = answers[i].querySelector('.model-badge');
                    const modelName = answers[i].querySelector('.model-name');
                    const providerDot = answers[i].querySelector('.provider-dot');
                    
                    // Get clean content without badge HTML
                    const content = answers[i].innerHTML.split('<div class="model-badge')[0];
                    
                    messages.push({
                        type: 'answer',
                        content: content,
                        model: modelName ? modelName.textContent : 'llama-8b',
                        provider: badge ? badge.className.replace('model-badge provider-', '') : 'meta',
                        providerEmoji: providerDot ? providerDot.textContent : '🔷',
                        timestamp: new Date().toLocaleTimeString(),
                        id: 'msg_' + Date.now() + '_' + i
                    });
                }
            });
            
            localStorage.setItem('chat_conversation', JSON.stringify({
                sessionId: conversationMetadata.sessionId,
                messages: messages
            }));
        }
        
        // Load on page load ONLY if chat is empty (no server-rendered content)
        window.addEventListener('DOMContentLoaded', function() {
            const chatDiv = document.querySelector('.chat');
            if (!chatDiv || chatDiv.children.length === 0) {
                loadConversation();
            }
        });
        
        // AJAX message sending
        async function sendMessage(event) {
            event.preventDefault();
            
            const queryInput = document.getElementById('query-input');
            const query = queryInput.value.trim();
            if (!query) return;
            
            // Disable form while sending
            queryInput.disabled = true;
            document.getElementById('send-button').disabled = true;
            
            // Add question to chat
            const chatDiv = document.querySelector('.chat');
            const questionDiv = document.createElement('div');
            questionDiv.className = 'q';
            questionDiv.textContent = query;
            chatDiv.appendChild(questionDiv);
            
            // Create answer div
            const answerDiv = document.createElement('div');
            answerDiv.className = 'a';
            chatDiv.appendChild(answerDiv);
            
            // Get current settings
            const mode = document.getElementById('mode-input').value;
            const provider = document.getElementById('provider-input').value;
            const model = document.getElementById('model-input').value;
            const history = document.getElementById('history-input').value;
            
            try {
                // Generate conversation ID on first message
                if (!conversationMetadata.conversationId) {
                    const historyCount = document.querySelectorAll('.q').length;
                    if (historyCount <= 1) {  // This is the first message
                        conversationMetadata.conversationId = 'conv_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
                        localStorage.setItem('conversation_id', conversationMetadata.conversationId);
                    } else {
                        // Try to get existing conversation ID from localStorage
                        conversationMetadata.conversationId = localStorage.getItem('conversation_id') || 
                            'conv_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
                    }
                }
                
                // Make AJAX request with URL-encoded data
                const params = new URLSearchParams();
                params.append('q', query);
                params.append('mode', mode);
                params.append('provider', provider);
                params.append('model', model);
                params.append('h', history);
                params.append('conversation_id', conversationMetadata.conversationId);
                
                const response = await fetch('/', {
                    method: 'POST',
                    headers: {
                        'X-Requested-With': 'XMLHttpRequest',  // Tell server this is AJAX
                        'Content-Type': 'application/x-www-form-urlencoded'
                    },
                    body: params.toString()
                });
                
                if (!response.ok) throw new Error('Network response was not ok');
                
                // Stream the response
                const reader = response.body.getReader();
                const decoder = new TextDecoder();
                let buffer = '';
                let responseText = '';
                
                while (true) {
                    const {done, value} = await reader.read();
                    if (done) break;
                    
                    buffer += decoder.decode(value, {stream: true});
                }
                
                // Just use the response directly - NO Q: A: parsing needed
                responseText = buffer.trim();
                answerDiv.innerHTML = responseText;
                
                // Add badge for this response
                const responseId = 'resp_' + Date.now();
                const modelName = model || 'llama-8b';
                const providerInfo = detectProvider(modelName);
                
                const badgeHTML = '<div class="model-badge provider-' + providerInfo.name.toLowerCase() + '">' +
                    '<button class="badge-toggle" onclick="toggleMetadata(\'' + responseId + '\')">' +
                        '<span class="provider-dot">' + providerInfo.emoji + '</span>' +
                        '<span class="model-name">' + modelName + '</span>' +
                        '<span class="expand-icon">▼</span>' +
                    '</button>' +
                    '<div class="metadata-panel" id="metadata-' + responseId + '">' +
                        '<table class="metadata-table">' +
                            '<tr><td>Model:</td><td>' + modelName + '</td></tr>' +
                            '<tr><td>Provider:</td><td>' + providerInfo.name + '</td></tr>' +
                            '<tr><td>Time:</td><td>' + new Date().toLocaleTimeString() + '</td></tr>' +
                        '</table>' +
                    '</div>' +
                '</div>';
                
                answerDiv.innerHTML = responseText + badgeHTML;
                
                // Save to localStorage
                saveConversation();
                
                // Build history ARRAY for next request
                const historyMessages = [];
                const allQuestions = document.querySelectorAll('.q');
                const allAnswers = document.querySelectorAll('.a');
                
                allQuestions.forEach((q, i) => {
                    historyMessages.push({
                        role: 'user',
                        content: q.textContent
                    });
                    if (allAnswers[i]) {
                        // Get answer text without badge
                        const answerClone = allAnswers[i].cloneNode(true);
                        const badge = answerClone.querySelector('.model-badge');
                        if (badge) badge.remove();
                        historyMessages.push({
                            role: 'assistant',
                            content: answerClone.textContent.trim()
                        });
                    }
                });
                
                // Store history as JSON array
                document.getElementById('history-input').value = JSON.stringify(historyMessages);
                
            } catch (error) {
                answerDiv.innerHTML = '<p style="color: red;">Error: ' + error.message + '</p>';
            } finally {
                // Re-enable form
                queryInput.value = '';
                queryInput.disabled = false;
                queryInput.focus();
                document.getElementById('send-button').disabled = false;
            }
        }
        
        // Helper function to detect provider from model name
        function detectProvider(modelName) {
            if (modelName.includes('gpt')) return {name: 'OpenAI', emoji: '🟢'};
            if (modelName.includes('claude')) return {name: 'Anthropic', emoji: '🟠'};
            if (modelName.includes('gemini')) return {name: 'Google', emoji: '🔵'};
            if (modelName.includes('llama')) return {name: 'Meta', emoji: '🔷'};
            if (modelName.includes('mistral')) return {name: 'Mistral', emoji: '🟣'};
            return {name: 'Unknown', emoji: '⚫'};
        }
        
        // Initialize session ID if not set
        if (!document.getElementById('session-input').value) {
            document.getElementById('session-input').value = conversationMetadata.sessionId;
        }
        
        // Mode toggle handler
        document.querySelectorAll('input[name="ui-mode"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                const mode = e.target.value;
                conversationMetadata.currentMode = mode;
                document.getElementById('mode-input').value = mode;
                
                if (mode === 'advanced') {
                    document.getElementById('advanced-controls').classList.remove('hidden');
                    document.getElementById('simplified-controls').classList.add('hidden');
                } else {
                    document.getElementById('advanced-controls').classList.add('hidden');
                    document.getElementById('simplified-controls').classList.remove('hidden');
                }
            });
        });
        
        // Provider dropdown toggle
        function toggleProviderDropdown() {
            const options = document.getElementById('provider-options');
            options.classList.toggle('open');
            
            // Populate if empty
            if (options.children.length === 0) {
                populateProviders();
            }
        }
        
        // Close dropdown when clicking outside
        document.addEventListener('click', (e) => {
            if (!e.target.closest('.provider-select')) {
                document.getElementById('provider-options').classList.remove('open');
            }
        });
        
        // Populate provider options
        function populateProviders() {
            const container = document.getElementById('provider-options');
            container.innerHTML = '';
            
            Object.entries(providers).forEach(([key, provider]) => {
                const option = document.createElement('div');
                option.className = 'provider-option provider-' + key;
                option.innerHTML = 
                    '<span class="provider-dot">' + provider.emoji + '</span>' +
                    '<span>' + provider.name + '</span>' +
                    '<span style="margin-left: auto; color: #999; font-size: 0.8rem;">' + 
                    provider.models.length + ' models</span>';
                option.onclick = () => selectProvider(key);
                container.appendChild(option);
            });
        }
        
        // Select provider
        function selectProvider(providerId) {
            const provider = providers[providerId];
            if (!provider) return;
            
            // Update dropdown display
            const dropdown = document.querySelector('.provider-dropdown');
            dropdown.innerHTML = 
                '<span class="provider-dot">' + provider.emoji + '</span>' +
                '<span class="provider-name">' + provider.name + '</span>' +
                '<span style="margin-left: auto;">▼</span>';
            dropdown.className = 'provider-dropdown provider-' + providerId;
            
            // Update hidden input
            document.getElementById('provider-input').value = providerId;
            
            // Update model dropdown
            populateModels(provider.models);
            
            // Close dropdown
            document.getElementById('provider-options').classList.remove('open');
        }
        
        // Populate model dropdown
        function populateModels(models) {
            const select = document.getElementById('model-dropdown');
            select.innerHTML = '';
            
            models.forEach(model => {
                const option = document.createElement('option');
                option.value = model;
                option.textContent = model;
                select.appendChild(option);
            });
            
            // Update hidden input
            if (models.length > 0) {
                document.getElementById('model-input').value = models[0];
            }
        }
        
        // Update model input when dropdown changes
        document.getElementById('model-dropdown').addEventListener('change', (e) => {
            document.getElementById('model-input').value = e.target.value;
        });
        
        // Model badge toggle
        function toggleMetadata(responseId) {
            const panel = document.getElementById('metadata-' + responseId);
            const badge = document.querySelector('[data-response-id="' + responseId + '"] .badge-toggle');
            
            if (panel) {
                panel.classList.toggle('open');
                badge.classList.toggle('expanded');
            }
        }
        
        // Add response metadata
        function addResponseMetadata(metadata) {
            if (metadata && metadata.response_id) {
                conversationMetadata.responses[metadata.response_id] = metadata;
            }
        }
        
        // Generate session ID
        function generateSessionId() {
            return 'sess_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
        }
    </script>
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
	isAJAX := r.Header.Get("X-Requested-With") == "XMLHttpRequest"
	wantsHTML := (isBrowserUA(userAgent) || strings.Contains(accept, "text/html")) && !isAJAX
	wantsStream := strings.Contains(accept, "text/event-stream")

	if query != "" {
		prompt = query
		// Don't send history as part of the prompt - just the query!
		// History is for display only, not for the LLM

		// Handle AJAX requests
		if isAJAX {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			
			// Get the model from the request
			modelToUse := r.FormValue("model")
			if modelToUse == "" {
				modelToUse = "llama-8b"
			}
			
			// Get conversation ID from request
			conversationID := r.FormValue("conversation_id")
			if conversationID == "" {
				// Generate new conversation ID if not provided
				conversationID = fmt.Sprintf("conv_%d_%s", time.Now().Unix(), generateSignature(prompt)[:8])
			}
			
			// Parse JSON message history
			var messages []Message
			if history != "" {
				// Parse the JSON array of messages
				var historyMessages []map[string]string
				if err := json.Unmarshal([]byte(history), &historyMessages); err == nil {
					for _, msg := range historyMessages {
						messages = append(messages, Message{
							Role: msg["role"],
							Content: msg["content"],
						})
					}
				}
			}
			// Add current query
			messages = append(messages, Message{Role: "user", Content: prompt})
			
			// Build conversation prompt with full history
			contextPrompt := ""
			for _, msg := range messages {
				if msg.Role == "user" {
					contextPrompt += "User: " + msg.Content + "\n"
				} else {
					contextPrompt += "Assistant: " + msg.Content + "\n"
				}
			}
			
			// Convert Message array to map format for LLMWithRouter
			var messagesMap []map[string]string
			for _, msg := range messages {
				messagesMap = append(messagesMap, map[string]string{
					"role": msg.Role,
					"content": msg.Content,
				})
			}
			
			// Call LLM with message array and conversation ID
			var llmResp *LLMResponse
			var err error
			if modelRouter != nil {
				// LLMWithRouterConv accepts conversation ID!
				llmResp, err = LLMWithRouterConv(messagesMap, modelToUse, conversationID, nil, nil)
			} else {
				// NO FALLBACK! Router must be initialized!
				err = fmt.Errorf("model router not initialized - cannot process request")
			}
			
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			
			// Return just the response content
			fmt.Fprint(w, llmResp.Content)
			return
		}

		if wantsHTML && r.Header.Get("Accept") != "application/json" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
				
				// Determine which model to use based on UI mode
				var modelToUse string
				uiMode := r.FormValue("mode")
				if uiMode == "advanced" {
					// In advanced mode, use the selected model
					modelToUse = r.FormValue("model")
					if modelToUse == "" {
						modelToUse = "llama-8b" // Default if not specified
					}
				} else {
					// In simplified mode, use tier-based selection
					modelToUse = tierToModel(tier)
				}
				
				// Use router if available
				if modelRouter != nil {
					resp, err = LLMWithRouter(htmlPrompt, modelToUse, nil, ch)
				} else {
					err = fmt.Errorf("model router not initialized")
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
			
			// ALWAYS add model badge - every response gets one!
			// Generate response ID for metadata tracking
			responseID := fmt.Sprintf("resp_%d_%s", time.Now().Unix(), generateRequestID()[:8])
			
			// Get model name from response or use what was requested
			modelName := ""
			if llmResp != nil && llmResp.Model != "" {
				modelName = llmResp.Model
			} else {
				// Fallback to what was requested
				uiMode := r.FormValue("mode")
				if uiMode == "advanced" {
					modelName = r.FormValue("model")
					if modelName == "" {
						modelName = "llama-8b"
					}
				} else {
					// Simplified mode - use tier to determine model
					tier := r.FormValue("tier")
					switch tier {
					case "fast":
						modelName = "llama-8b"
					case "frontier":
						modelName = "claude-opus"
					default:
						modelName = "llama-70b"
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
				
				// Add the badge HTML
				fmt.Fprintf(w, `<div class="model-badge provider-%s">
					<button class="badge-toggle" onclick="toggleMetadata('%s')">
						<span class="provider-dot">%s</span>
						<span class="model-name">%s</span>
						<span class="expand-icon">▼</span>
					</button>
					<div class="metadata-panel" id="metadata-%s">
						<table class="metadata-table">
							<tr><td>Model:</td><td>%s</td></tr>
							<tr><td>Provider:</td><td>%s</td></tr>
							<tr><td>Tokens:</td><td>In: %d | Out: %d</td></tr>
							<tr><td>Total:</td><td>%d tokens</td></tr>
							<tr><td>Time:</td><td>%s</td></tr>
						</table>
					</div>
				</div>`,
					strings.ToLower(providerName),
					responseID,
					providerEmoji,
					modelName,
					responseID,
					modelName,
					providerName,
					func() int {
						if llmResp != nil {
							return llmResp.InputTokens
						}
						return 0
					}(),
					func() int {
						if llmResp != nil {
							return llmResp.OutputTokens
						}
						return 0
					}(),
					func() int {
						if llmResp != nil {
							return llmResp.InputTokens + llmResp.OutputTokens
						}
						return 0
					}(),
					time.Now().Format("15:04:05"),
				)
				
				// Add JavaScript to store metadata
				fmt.Fprintf(w, `<script>
					addResponseMetadata({
						response_id: '%s',
						model_name: '%s',
						provider: '%s',
						input_tokens: %d,
						output_tokens: %d,
						timestamp: '%s'
					});
				</script>`,
					responseID,
					modelName,
					strings.ToLower(providerName),
					func() int {
						if llmResp != nil {
							return llmResp.InputTokens
						}
						return 0
					}(),
					func() int {
						if llmResp != nil {
							return llmResp.OutputTokens
						}
						return 0
					}(),
					time.Now().Format(time.RFC3339),
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
			
			// Get mode from form or default to advanced
			uiMode := r.FormValue("mode")
			if uiMode == "" {
				uiMode = "advanced"
			}
			
			// Get session ID or generate new one
			sessionID := r.FormValue("session_id")
			if sessionID == "" {
				sessionID = fmt.Sprintf("sess_%d_%s", time.Now().Unix(), generateRequestID()[:8])
			}
			
			// Get provider and model selections
			provider := r.FormValue("provider")
			if provider == "" {
				provider = "meta" // Default provider
			}
			model := r.FormValue("model")
			if model == "" {
				model = "llama-8b" // Default model
			}
			
			// Mode toggle states
			advancedChecked := ""
			simplifiedChecked := ""
			advancedClass := ""
			simplifiedClass := "hidden"
			if uiMode == "simplified" {
				simplifiedChecked = "checked"
				advancedClass = "hidden"
				simplifiedClass = ""
			} else {
				advancedChecked = "checked"
			}
			
			// Build providers JSON and model options from actual registry
			var modelOptions string
			providerEmoji = "🔷"
			providerName = "Meta"
			var providersJSON string = "{}"
			
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
				if jsonBytes, err := json.Marshal(providersData); err == nil {
					providersJSON = string(jsonBytes)
				}
				
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
				providersJSON = `{"meta": {"name": "Meta", "color": "#0668E1", "emoji": "🔷", "models": ["llama-8b"], "enabled": true}}`
			}
			
			// Format the tier radio buttons
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
			safeHistory := strings.ReplaceAll(finalHistory, "</textarea>", "&lt;/textarea&gt;")
			
			// Format footer with all parameters
			fmt.Fprintf(w, htmlFooterTemplate,
				advancedChecked,    // advanced mode radio
				simplifiedChecked,  // simplified mode radio
				advancedClass,      // advanced controls visibility
				providerEmoji,      // provider emoji
				providerName,       // provider name
				modelOptions,       // model dropdown options
				simplifiedClass,    // simplified controls visibility
				fastChecked,        // fast tier
				balancedChecked,    // balanced tier
				frontierChecked,    // frontier tier
				uiMode,            // current mode
				provider,          // current provider
				model,             // current model
				sessionID,         // session ID
				safeHistory,       // conversation history
				providersJSON,      // providers JSON for JavaScript
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
		fmt.Fprint(w, htmlHeader)
		parts := strings.Split("\n"+content, "\nQ: ")
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
				responseID := fmt.Sprintf("hist_%d_%d", time.Now().Unix(), len(parts))
				
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
				
				// Add the badge
				fmt.Fprintf(w, `<div class="model-badge provider-%s">
					<button class="badge-toggle" onclick="toggleMetadata('%s')">
						<span class="provider-dot">%s</span>
						<span class="model-name">%s</span>
						<span class="expand-icon">▼</span>
					</button>
					<div class="metadata-panel" id="metadata-%s">
						<table class="metadata-table">
							<tr><td>Model:</td><td>%s</td></tr>
							<tr><td>Provider:</td><td>%s</td></tr>
							<tr><td>Time:</td><td>Historical</td></tr>
						</table>
					</div>
				</div>`,
					strings.ToLower(providerName),
					responseID,
					providerEmoji,
					modelName,
					responseID,
					modelName,
					providerName,
				)
				
				fmt.Fprintf(w, "</div>\n")
			}
		}

		// Default settings for initial page load
		// Escape only </textarea> to prevent breaking out
		safeContent := strings.ReplaceAll(content, "</textarea>", "&lt;/textarea&gt;")
		
		// Generate new session ID for new chat
		sessionID := fmt.Sprintf("sess_%d_%s", time.Now().Unix(), generateRequestID()[:8])
		
		// Build model options and providers JSON from registry
		var modelOptions string
		var providersJSON string = "{}"
		
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
			if jsonBytes, err := json.Marshal(providersData); err == nil {
				providersJSON = string(jsonBytes)
			}
			
			// Build options HTML for Meta (default)
			for _, modelID := range providersData["meta"].Models {
				modelOptions += fmt.Sprintf(`<option value="%s">%s</option>`, modelID, modelID)
			}
		}
		
		// Fallback if no models found
		if modelOptions == "" {
			modelOptions = `<option value="llama-8b">llama-8b</option>`
			providersJSON = `{"meta": {"name": "Meta", "color": "#0668E1", "emoji": "🔷", "models": ["llama-8b"], "enabled": true}}`
		}
		
		// Default to advanced mode with Meta provider
		fmt.Fprintf(w, htmlFooterTemplate,
			"checked",   // advanced mode radio (default)
			"",          // simplified mode radio
			"",          // advanced controls visible
			"🔷",        // Meta emoji
			"Meta",      // Meta name
			modelOptions, // Dynamic model options from registry
			"hidden",    // simplified controls hidden
			"",          // fast tier
			"checked",   // balanced tier (default)
			"",          // frontier tier
			"advanced",  // current mode
			"meta",      // current provider
			"llama-8b",  // current model (default per user)
			sessionID,   // session ID
			safeContent, // history
			providersJSON, // providers JSON for JavaScript
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
	log.Printf("[handleChatCompletions] START - Method: %s", r.Method)
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
		log.Printf("[handleChatCompletions] Failed to decode JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	log.Printf("[handleChatCompletions] Request - Model: %s, MaxTokens: %d, Temperature: %f", 
		req.Model, req.MaxTokens, req.Temperature)

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
	
	log.Printf("[handleChatCompletions] Using router for model: %s", req.Model)
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
