package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
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
	ClientPubKey       *rsa.PublicKey
	ServerPubKey       *rsa.PublicKey
	ServerPrivKey      *rsa.PrivateKey
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
	defer w.WriteMsg(m)
	
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
	} else if strings.HasSuffix(subdomain, ".exec") {
		handleV2Exec(m, q, subdomain)
	} else if strings.Contains(subdomain, ".page.") {
		handleV2Page(m, q, subdomain)
	} else if countDots(subdomain) >= 2 {
		handleV2QueryPage(m, q, subdomain)
	} else {
		if debugMode {
			log.Printf("[DonutSentryV2] ERROR: Unknown query type: %s", subdomain)
		}
		respondWithTXT(m, q, "ERROR: Invalid v2 query format")
	}
}

// Handle session initialization with keypair exchange
func handleV2Init(m *dns.Msg, q dns.Question, subdomain string) {
	// Extract client public key hash
	parts := strings.Split(subdomain, ".")
	if len(parts) < 2 {
		respondWithTXT(m, q, "ERROR: Invalid init format")
		return
	}
	
	pubKeyHashEncoded := strings.ToUpper(parts[0])
	
	// Decode client public key hash (first 20 bytes of SHA-256)
	pubKeyHash, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(pubKeyHashEncoded)
	if err != nil || len(pubKeyHash) < 20 {
		respondWithTXT(m, q, "ERROR: Invalid public key hash")
		return
	}
	
	// Generate server keypair for this session
	serverPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
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
	sessionID := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sessionIDBytes)
	
	// Create session (client pubkey will be set when they send first encrypted message)
	session := &DoNutV2Session{
		ID:            sessionID,
		ServerPubKey:  &serverPrivKey.PublicKey,
		ServerPrivKey: serverPrivKey,
		QueryPages:    make(map[int]string),
		ResponsePages: make(map[int][]byte),
		CreatedAt:     time.Now(),
		LastActivity:  time.Now(),
	}
	
	v2Sessions.Store(sessionID, session)
	
	// Encode server public key to DER
	serverPubKeyDER, err := x509.MarshalPKIXPublicKey(&serverPrivKey.PublicKey)
	if err != nil {
		respondWithTXT(m, q, "ERROR: Failed to encode server public key")
		return
	}
	
	// Response: session_id_bytes + server_pubkey_der
	// (In production, encrypt session_id with client's public key)
	response := append(sessionIDBytes, serverPubKeyDER...)
	responseB64 := base64.StdEncoding.EncodeToString(response)
	
	if debugMode {
		log.Printf("[DonutSentryV2 Init] Session %s created, pubkey hash: %x", sessionID, pubKeyHash)
	}
	respondWithTXT(m, q, responseB64)
}

// Handle encrypted query page
func handleV2QueryPage(m *dns.Msg, q dns.Question, subdomain string) {
	// Format: <session_id>.<page_num>.<encrypted_signed_data>.qp.ch.at
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
	pageNumBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(pageNumEncoded)
	if err != nil || len(pageNumBytes) == 0 {
		respondWithTXT(m, q, "ERROR: Invalid page number")
		return
	}
	pageNum := int(pageNumBytes[0])
	
	// Decode encrypted data
	encryptedData, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(encryptedDataEncoded)
	if err != nil {
		respondWithTXT(m, q, "ERROR: Invalid encrypted data")
		return
	}
	
	// For v2 MVP, store the encrypted data as-is (signature verification and decryption TODO)
	session.mu.Lock()
	session.QueryPages[pageNum] = string(encryptedData)
	session.LastActivity = time.Now()
	session.mu.Unlock()
	
	if debugMode {
		log.Printf("[DonutSentryV2 QueryPage] Session %s received page %d (%d bytes)", sessionID, pageNum, len(encryptedData))
	}
	respondWithTXT(m, q, "ACK")
}

// Handle query execution and response pagination
func handleV2Exec(m *dns.Msg, q dns.Question, subdomain string) {
	// Format: <session_id>.<total_pages>.exec.qp.ch.at
	parts := strings.Split(subdomain, ".")
	if len(parts) < 3 {
		respondWithTXT(m, q, "ERROR: Invalid exec format")
		return
	}
	
	sessionID := strings.ToUpper(parts[0])
	totalPagesEncoded := strings.ToUpper(parts[1])
	
	// Get session
	sessionInterface, ok := v2Sessions.Load(sessionID)
	if !ok {
		respondWithTXT(m, q, "ERROR: Session not found")
		return
	}
	session := sessionInterface.(*DoNutV2Session)
	
	// Decode total pages
	totalPagesBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(totalPagesEncoded)
	if err != nil || len(totalPagesBytes) == 0 {
		respondWithTXT(m, q, "ERROR: Invalid total pages")
		return
	}
	totalPages := int(totalPagesBytes[0])
	
	session.mu.Lock()
	// Verify all query pages received
	if len(session.QueryPages) != totalPages {
		session.mu.Unlock()
		respondWithTXT(m, q, fmt.Sprintf("ERROR: Missing pages (have %d, need %d)", len(session.QueryPages), totalPages))
		return
	}
	
	// Reassemble query (for MVP, treat as plaintext)
	var query strings.Builder
	for i := 0; i < totalPages; i++ {
		page, ok := session.QueryPages[i]
		if !ok {
			session.mu.Unlock()
			respondWithTXT(m, q, fmt.Sprintf("ERROR: Missing page %d", i))
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
	
	// Generate response
	dnsPrompt := "Answer in detail, no markdown formatting: " + fullQuery
	if debugMode {
		log.Printf("[DonutSentryV2 Debug] About to call LLM - apiURL: %s, modelName: %s", apiURL, modelName)
	}
	llmResp, err := LLM(dnsPrompt, nil)
	var responseText string
	if err != nil {
		if debugMode {
			log.Printf("[DonutSentryV2] LLM ERROR: %v", err)
			log.Printf("[DonutSentryV2] Error type: %T", err)
		}
		responseText = "Error: " + err.Error()
	} else {
		if debugMode {
			log.Printf("[DonutSentryV2] LLM SUCCESS: Got response with %d chars", len(llmResp.Content))
		}
		responseText = llmResp.Content
	}
	
	// Paginate response
	pages := paginateResponse(responseText, v2PageSize)
	
	session.mu.Lock()
	session.TotalResponsePages = len(pages)
	
	// Store each page (for MVP, store plaintext with metadata)
	for i, pageContent := range pages {
		metadata := fmt.Sprintf("[Page %d/%d]", i+1, len(pages))
		fullContent := metadata + pageContent
		
		// TODO: Encrypt with client's public key and sign with server's private key
		// For now, just base64 encode
		session.ResponsePages[i] = []byte(fullContent)
	}
	
	// Get first page
	firstPage := session.ResponsePages[0]
	session.LastActivity = time.Now()
	session.mu.Unlock()
	
	if debugMode {
		log.Printf("[DonutSentryV2 Exec] Returning page 1/%d", len(pages))
	}
	respondWithTXT(m, q, base64.StdEncoding.EncodeToString(firstPage))
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
		log.Printf("[DonutSentryV2 Page] Session %s returning page %d/%d", sessionID, pageNum, session.TotalResponsePages)
	}
	respondWithTXT(m, q, base64.StdEncoding.EncodeToString(page))
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

// RSA Crypto functions (TODO: Implement these properly)

func rsaEncrypt(plaintext []byte, pubKey *rsa.PublicKey) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, plaintext, nil)
}

func rsaDecrypt(ciphertext []byte, privKey *rsa.PrivateKey) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, ciphertext, nil)
}

func rsaSign(data []byte, privKey *rsa.PrivateKey) ([]byte, error) {
	hash := sha256.Sum256(data)
	return rsa.SignPSS(rand.Reader, privKey, rsa.PSSSaltLengthAuto, hash[:], nil)
}

func rsaVerify(data, signature []byte, pubKey *rsa.PublicKey) error {
	hash := sha256.Sum256(data)
	return rsa.VerifyPSS(pubKey, rsa.PSSSaltLengthAuto, hash[:], signature, nil)
}