package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// DoNutV2Session represents an encrypted session with bidirectional paging
type DoNutV2Session struct {
	ID                 string
	ClientEncPubKey    []byte // X25519 public key (32 bytes)
	ClientSigPubKey    []byte // Ed25519 public key (32 bytes)
	ServerKeys         *ECCKeyPair
	SharedSecret       []byte // Derived from ECDH for XOR keys
	QueryPages         map[int]string // Decrypted query pages
	ResponsePages      map[int][]byte // Encrypted response pages (client can decrypt)
	TotalQueryPages    int
	TotalResponsePages int
	CreatedAt          time.Time
	LastActivity       time.Time
	mu                 sync.Mutex // Protect concurrent access
}

var (
	v2Sessions    = &sync.Map{} // session_id -> *DoNutV2Session
	v2SessionTTL  = 4 * time.Hour
	v2PageSize    = 400 // Characters per response page
)

// Initialize v2 cleanup routine
func init() {
	go v2SessionCleanup()
}

// Main v2 handler for .qp.ch.at domain
func handleDoNutSentryV2Query(w dns.ResponseWriter, r *dns.Msg, m *dns.Msg, q dns.Question) {
	// Extract subdomain (everything before .qp.ch.at.)
	fullName := strings.ToLower(q.Name)
	subdomain := strings.TrimSuffix(fullName, ".qp.ch.at.")
	
	if debugMode {
		log.Printf("[DonutSentryV2] === V2 QUERY RECEIVED ===")
		log.Printf("[DonutSentryV2] Subdomain: %s", subdomain)
		log.Printf("[DonutSentryV2] Full query: %s", q.Name)
	}
	
	// Route based on operation type
	if strings.HasSuffix(subdomain, ".init") {
		handleV2Init(m, q, subdomain)
		w.WriteMsg(m)
	} else if strings.HasSuffix(subdomain, ".exec") {
		// Special handling for exec to support async
		handleV2ExecAsync(w, r, m, q, subdomain)
		// Note: handleV2ExecAsync writes response directly
	} else if strings.HasSuffix(subdomain, ".status") {
		handleV2Status(m, q, subdomain)
		w.WriteMsg(m)
	} else if strings.Contains(subdomain, ".page.") {
		handleV2Page(m, q, subdomain)
		w.WriteMsg(m)
	} else if countDots(subdomain) >= 2 {
		handleV2QueryPage(m, q, subdomain)
		w.WriteMsg(m)
	} else {
		if debugMode {
			log.Printf("[DonutSentryV2] ERROR: Unknown query type: %s", subdomain)
		}
		respondWithTXT(m, q, "ERROR: Invalid v2 query format")
		w.WriteMsg(m)
	}
}

// Handle session initialization with keypair exchange
func handleV2Init(m *dns.Msg, q dns.Question, subdomain string) {
	// Extract client public keys bundle (two labels)
	parts := strings.Split(subdomain, ".")
	if len(parts) < 3 { // Need encPub.sigPub.init
		respondWithTXT(m, q, "ERROR: Invalid init format")
		return
	}
	
	encPubEncoded := strings.ToUpper(parts[0])
	sigPubEncoded := strings.ToUpper(parts[1])
	
	// Decode encryption public key
	clientEncPub, err := Base32DecodeNoPad(encPubEncoded)
	if err != nil || len(clientEncPub) != 32 {
		if debugMode {
			log.Printf("[DonutSentryV2 Init] Failed to decode encryption key: %v", err)
		}
		respondWithTXT(m, q, "ERROR: Invalid encryption key")
		return
	}
	
	// Decode signing public key
	clientSigPub, err := Base32DecodeNoPad(sigPubEncoded)
	if err != nil || len(clientSigPub) != 32 {
		if debugMode {
			log.Printf("[DonutSentryV2 Init] Failed to decode signing key: %v", err)
		}
		respondWithTXT(m, q, "ERROR: Invalid signing key")
		return
	}
	
	// Generate server keypairs for this session
	serverKeys, err := GenerateECCKeyPair()
	if err != nil {
		respondWithTXT(m, q, "ERROR: Failed to generate server keys")
		return
	}
	
	// Generate session ID
	sessionIDBytes := make([]byte, 16)
	if _, err := rand.Read(sessionIDBytes); err != nil {
		respondWithTXT(m, q, "ERROR: Failed to generate session ID")
		return
	}
	sessionID := Base32EncodeNoPad(sessionIDBytes)
	
	// Derive shared secret via ECDH
	sharedSecret, err := DeriveSharedSecret(serverKeys.EncryptionPrivate, clientEncPub)
	if err != nil {
		respondWithTXT(m, q, "ERROR: Failed to derive shared secret")
		return
	}
	
	// Create session
	session := &DoNutV2Session{
		ID:              sessionID,
		ClientEncPubKey: clientEncPub,
		ClientSigPubKey: clientSigPub,
		ServerKeys:      serverKeys,
		SharedSecret:    sharedSecret,
		QueryPages:      make(map[int]string),
		ResponsePages:   make(map[int][]byte),
		CreatedAt:       time.Now(),
		LastActivity:    time.Now(),
	}
	
	v2Sessions.Store(sessionID, session)
	
	// Encrypt session ID with client's encryption key
	encryptedSessionID, err := NaClEncrypt(sessionIDBytes, serverKeys.EncryptionPrivate, clientEncPub)
	if err != nil {
		if debugMode {
			log.Printf("[DonutSentryV2 Init] Encryption failed: %v", err)
		}
		respondWithTXT(m, q, "ERROR: Failed to encrypt session ID")
		return
	}
	
	// Encode server public keys
	serverPubKeysEncoded := EncodePublicKeys(serverKeys.EncryptionPublic, serverKeys.SigningPublic)
	
	// Response format: length_prefix + encrypted_session_id[base64] + server_pubkeys[base32]
	// Use length prefix instead of dot separator since DNS might split the response
	encSessionB64 := Base64Encode(encryptedSessionID)
	response := fmt.Sprintf("%03d%s%s", len(encSessionB64), encSessionB64, serverPubKeysEncoded)
	
	// Always log for debugging
	log.Printf("[DonutSentryV2 Init] Session %s created, client enc key: %x...", sessionID, clientEncPub[:8])
	log.Printf("[DonutSentryV2 Init] Response length: %d chars", len(response))
	log.Printf("[DonutSentryV2 Init] Encrypted session ID length: %d", len(Base64Encode(encryptedSessionID)))
	log.Printf("[DonutSentryV2 Init] Server keys encoded length: %d", len(serverPubKeysEncoded))
	log.Printf("[DonutSentryV2 Init] Dot present at index: %d", strings.Index(response, "."))
	
	respondWithTXT(m, q, response)
}

// Handle encrypted query page
func handleV2QueryPage(m *dns.Msg, q dns.Question, subdomain string) {
	// Format: <session_id>.<page_num>.<encrypted_data>.qp.ch.at
	parts := strings.Split(subdomain, ".")
	if len(parts) < 3 {
		respondWithTXT(m, q, "ERROR: Invalid query page format")
		return
	}
	
	sessionID := strings.ToUpper(parts[0])
	pageNumEncoded := strings.ToUpper(parts[1])
	encryptedDataEncoded := strings.ToUpper(parts[2])
	
	// Get session
	sessionInterface, ok := v2Sessions.Load(sessionID)
	if !ok {
		respondWithTXT(m, q, "ERROR: Session not found")
		return
	}
	session := sessionInterface.(*DoNutV2Session)
	
	// Decode page number
	pageNumBytes, err := Base32DecodeNoPad(pageNumEncoded)
	if err != nil || len(pageNumBytes) == 0 {
		respondWithTXT(m, q, "ERROR: Invalid page number")
		return
	}
	pageNum := int(pageNumBytes[0])
	
	// Decode encrypted data
	encryptedData, err := Base32DecodeNoPad(encryptedDataEncoded)
	if err != nil {
		respondWithTXT(m, q, "ERROR: Invalid encrypted data")
		return
	}
	
	// Derive XOR key for this page
	context := fmt.Sprintf("query:page:%d", pageNum)
	xorKey := DeriveXORKey(session.SharedSecret, context, len(encryptedData))
	
	// Decrypt with XOR (zero overhead!)
	plaintext := XORDecrypt(encryptedData, xorKey)
	
	// Store decrypted page
	session.mu.Lock()
	session.QueryPages[pageNum] = string(plaintext)
	session.LastActivity = time.Now()
	session.mu.Unlock()
	
	if debugMode {
		log.Printf("[DonutSentryV2 QueryPage] Session %s received page %d (%d bytes decrypted)", sessionID, pageNum, len(plaintext))
	}
	respondWithTXT(m, q, "ACK")
}

// Handle query execution and response pagination (async version)
func handleV2ExecAsync(w dns.ResponseWriter, r *dns.Msg, m *dns.Msg, q dns.Question, subdomain string) {
	// Format: <session_id>.<total_pages>.exec.qp.ch.at
	parts := strings.Split(subdomain, ".")
	if len(parts) < 3 {
		respondWithTXT(m, q, "ERROR: Invalid exec format")
		w.WriteMsg(m)
		return
	}
	
	sessionID := strings.ToUpper(parts[0])
	totalPagesEncoded := strings.ToUpper(parts[1])
	
	// Get session
	sessionInterface, ok := v2Sessions.Load(sessionID)
	if !ok {
		respondWithTXT(m, q, "ERROR: Session not found")
		w.WriteMsg(m)
		return
	}
	session := sessionInterface.(*DoNutV2Session)
	
	// Decode total pages
	totalPagesBytes, err := Base32DecodeNoPad(totalPagesEncoded)
	if err != nil || len(totalPagesBytes) == 0 {
		respondWithTXT(m, q, "ERROR: Invalid total pages")
		w.WriteMsg(m)
		return
	}
	totalPages := int(totalPagesBytes[0])
	
	session.mu.Lock()
	// Verify all query pages received
	if len(session.QueryPages) != totalPages {
		session.mu.Unlock()
		respondWithTXT(m, q, fmt.Sprintf("ERROR: Missing pages (have %d, need %d)", len(session.QueryPages), totalPages))
		w.WriteMsg(m)
		return
	}
	
	// Reassemble query (for MVP, treat as plaintext)
	var query strings.Builder
	for i := 0; i < totalPages; i++ {
		page, ok := session.QueryPages[i]
		if !ok {
			session.mu.Unlock()
			respondWithTXT(m, q, fmt.Sprintf("ERROR: Missing page %d", i))
			w.WriteMsg(m)
			return
		}
		query.WriteString(page)
	}
	session.TotalQueryPages = totalPages
	
	// Get LLM response
	fullQuery := query.String()
	session.mu.Unlock()
	
	if debugMode {
		log.Printf("[DonutSentryV2 Exec] Session %s executing query: %s", sessionID, fullQuery)
		log.Printf("[DonutSentryV2 Exec] Reassembled query from %d pages: %s", totalPages, fullQuery)
		log.Printf("[DonutSentryV2 Exec] Calling LLM with prompt...")
	}
	
	// Mark session as processing and return immediately
	session.mu.Lock()
	session.TotalResponsePages = -1 // -1 means "processing"
	session.mu.Unlock()
	
	// Start async processing
	go func() {
		log.Printf("[DonutSentryV2 Async] Goroutine started for session %s", sessionID)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[DonutSentryV2 Async] PANIC in session %s: %v", sessionID, r)
			}
		}()
		
		// Generate response
		dnsPrompt := "Answer in detail, no markdown formatting: " + fullQuery
		log.Printf("[DonutSentryV2 Async] Calling LLM for session %s with %d char prompt", sessionID, len(dnsPrompt))
		if debugMode {
			log.Printf("[DonutSentryV2 Debug] About to call LLM - apiURL: %s, modelName: %s", apiURL, modelName)
		}
		
		llmStart := time.Now()
		llmResp, err := LLM(dnsPrompt, nil)
		llmDuration := time.Since(llmStart)
		
		var responseText string
		if err != nil {
			log.Printf("[DonutSentryV2 Async] LLM ERROR for session %s after %v: %v", sessionID, llmDuration, err)
			if debugMode {
				log.Printf("[DonutSentryV2] Error type: %T", err)
			}
			responseText = "Error: " + err.Error()
		} else {
			log.Printf("[DonutSentryV2 Async] LLM SUCCESS for session %s after %v: Got %d chars", sessionID, llmDuration, len(llmResp.Content))
			if debugMode {
				log.Printf("[DonutSentryV2] LLM SUCCESS: Got response with %d chars", len(llmResp.Content))
			}
			responseText = llmResp.Content
		}
		
		// Paginate response
		pages := paginateResponse(responseText, v2PageSize)
		
		session.mu.Lock()
		session.TotalResponsePages = len(pages)
		
		// Store each page with XOR encryption (zero overhead!)
		for i, pageContent := range pages {
			metadata := fmt.Sprintf("[Page %d/%d]", i+1, len(pages))
			fullContent := metadata + pageContent
			plaintext := []byte(fullContent)
			
			// Derive XOR key for this response page
			context := fmt.Sprintf("response:page:%d", i)
			xorKey := DeriveXORKey(session.SharedSecret, context, len(plaintext))
			
			// Encrypt with XOR - same size as plaintext!
			encrypted := XOREncrypt(plaintext, xorKey)
			
			// Store encrypted page
			session.ResponsePages[i] = encrypted
		}
		session.LastActivity = time.Now()
		session.mu.Unlock()
		
		log.Printf("[DonutSentryV2 Async] Processing COMPLETE for session %s: %d response pages ready", sessionID, len(pages))
		if debugMode {
			log.Printf("[DonutSentryV2 Exec] Async processing complete for session %s, generated %d pages", sessionID, len(pages))
		}
	}()
	
	// Return processing status immediately
	if debugMode {
		log.Printf("[DonutSentryV2 Exec] Returning PROCESSING status for session %s", sessionID)
	}
	respondWithTXT(m, q, "PROCESSING")
	w.WriteMsg(m) // Send response immediately!
}

// Handle status check
func handleV2Status(m *dns.Msg, q dns.Question, subdomain string) {
	// Format: <session_id>.status.qp.ch.at
	parts := strings.Split(subdomain, ".")
	if len(parts) < 2 {
		respondWithTXT(m, q, "ERROR: Invalid status format")
		return
	}
	
	sessionID := strings.ToUpper(parts[0])
	log.Printf("[DonutSentryV2 Status] Checking status for session %s", sessionID)
	
	// Get session
	sessionInterface, ok := v2Sessions.Load(sessionID)
	if !ok {
		log.Printf("[DonutSentryV2 Status] Session %s not found", sessionID)
		respondWithTXT(m, q, "ERROR: Session not found")
		return
	}
	session := sessionInterface.(*DoNutV2Session)
	
	session.mu.Lock()
	totalPages := session.TotalResponsePages
	session.mu.Unlock()
	
	log.Printf("[DonutSentryV2 Status] Session %s has TotalResponsePages=%d", sessionID, totalPages)
	
	if totalPages == -1 {
		// Still processing
		respondWithTXT(m, q, "PROCESSING")
	} else if totalPages == 0 {
		// Not started yet
		respondWithTXT(m, q, "NOT_STARTED")
	} else {
		// Ready with N pages
		// Return first page directly when ready
		session.mu.Lock()
		firstPage := session.ResponsePages[0]
		session.mu.Unlock()
		
		log.Printf("[DonutSentryV2 Status] Session %s ready with %d pages, returning first page (%d bytes)", sessionID, totalPages, len(firstPage))
		if debugMode {
			log.Printf("[DonutSentryV2 Status] Session %s ready with %d pages, returning first page", sessionID, totalPages)
		}
		respondWithTXT(m, q, Base64Encode(firstPage))
	}
}

// Handle response page requests
func handleV2Page(m *dns.Msg, q dns.Question, subdomain string) {
	// Format: <session_id>.page.N.qp.ch.at
	parts := strings.Split(subdomain, ".")
	if len(parts) < 3 || parts[1] != "page" {
		respondWithTXT(m, q, "ERROR: Invalid page request format")
		return
	}
	
	sessionID := strings.ToUpper(parts[0])
	pageNumStr := parts[2]
	
	// Parse page number
	var pageNum int
	if _, err := fmt.Sscanf(pageNumStr, "%d", &pageNum); err != nil {
		respondWithTXT(m, q, "ERROR: Invalid page number")
		return
	}
	
	// Get session
	sessionInterface, ok := v2Sessions.Load(sessionID)
	if !ok {
		respondWithTXT(m, q, "ERROR: Session not found")
		return
	}
	session := sessionInterface.(*DoNutV2Session)
	
	session.mu.Lock()
	// Check if page exists (0-indexed internally, 1-indexed in protocol)
	page, exists := session.ResponsePages[pageNum-1]
	if !exists || pageNum > session.TotalResponsePages {
		session.mu.Unlock()
		respondWithTXT(m, q, "ERROR: Page not found")
		return
	}
	session.LastActivity = time.Now()
	session.mu.Unlock()
	
	if debugMode {
		log.Printf("[DonutSentryV2 Page] Session %s returning page %d/%d (encrypted %d bytes)", sessionID, pageNum, session.TotalResponsePages, len(page))
	}
	respondWithTXT(m, q, Base64Encode(page))
}

// Paginate response into chunks
func paginateResponse(text string, pageSize int) []string {
	var pages []string
	runes := []rune(text) // Handle Unicode properly
	
	for i := 0; i < len(runes); i += pageSize {
		end := i + pageSize
		if end > len(runes) {
			end = len(runes)
		}
		pages = append(pages, string(runes[i:end]))
	}
	
	return pages
}

// Clean up expired sessions
func v2SessionCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		now := time.Now()
		var toDelete []string
		
		v2Sessions.Range(func(key, value interface{}) bool {
			session := value.(*DoNutV2Session)
			if now.Sub(session.LastActivity) > v2SessionTTL {
				toDelete = append(toDelete, key.(string))
			}
			return true
		})
		
		for _, sessionID := range toDelete {
			v2Sessions.Delete(sessionID)
			if debugMode {
				log.Printf("[DonutSentryV2 Cleanup] Deleted expired session: %s", sessionID)
			}
		}
	}
}

