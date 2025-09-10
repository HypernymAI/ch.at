package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Session storage for DoNutSentry
type DoNutSession struct {
	ID           string
	PublicKey    *rsa.PublicKey
	Chunks       map[int]string
	TotalChunks  int
	CreatedAt    time.Time
	LastActivity time.Time
}

var (
	sessions   = &sync.Map{} // session_id -> *DoNutSession
	sessionTTL = 5 * time.Minute
	
	// Domain configuration for DoNutSentry
	donutSentryDomain = getDoNutSentryDomain()
)

func getDoNutSentryDomain() string {
	// Allow override via environment variable
	if domain := os.Getenv("DONUTSENTRY_DOMAIN"); domain != "" {
		// Ensure it starts with a dot and ends with a dot
		if !strings.HasPrefix(domain, ".") {
			domain = "." + domain
		}
		if !strings.HasSuffix(domain, ".") {
			domain = domain + "."
		}
		return domain
	}
	// Default to the original .q.ch.at. domain
	return ".q.ch.at."
}

func handleDoNutSentryQuery(w dns.ResponseWriter, r *dns.Msg, m *dns.Msg, q dns.Question) {
	// Ensure we send the response at the end
	defer w.WriteMsg(m)
	
	// Extract subdomain (everything before the configured domain)
	fullName := strings.ToLower(q.Name)
	subdomain := strings.TrimSuffix(fullName, donutSentryDomain)
	
	if debugMode {
		log.Printf("[DonutSentry] Query received: %s", subdomain)
	}
	
	// Debug output
	if debugMode {
		log.Println("======= DONUTSENTRY DEBUG =======")
		log.Printf("DoNutSentry domain: %s", donutSentryDomain)
		log.Printf("Received query: %s", subdomain)
	}

	// Handle different query types
	if strings.HasSuffix(subdomain, ".init") {
		// Session initialization - implement RSA key exchange
		handleSessionInit(m, q, subdomain)
		return
	} else if strings.HasSuffix(subdomain, ".exec") {
		// Session execution - implement chunk assembly
		handleSessionExec(m, q, subdomain)
		return
	} else if countDots(subdomain) >= 2 {
		// Might be a session chunk - implement chunk handling
		handleSessionChunk(m, q, subdomain)
		return
	}

	// Simple query - decode and process
	var prompt string

	// Try base32 first
	decoded, err := decodeBase32Query(subdomain)
	if err == nil {
		prompt = decoded
		if debugMode {
			log.Printf("Successfully decoded base32: %s -> %s", subdomain, prompt)
		}
	} else {
		// Not base32, use simple encoding
		prompt = strings.ReplaceAll(subdomain, "-", " ")
		if debugMode {
			log.Printf("Using simple encoding: %s -> %s (base32 error: %v)", subdomain, prompt, err)
		}
	}

	// Get service configuration
	config := getServiceConfig("DONUTSENTRY")
	
	// Get LLM response using router
	dnsPrompt := "Answer in 2000 characters or less, no markdown formatting: " + prompt
	messages := []map[string]string{
		{"role": "user", "content": dnsPrompt},
	}
	params := &RouterParams{
		MaxTokens:   config.MaxTokens,
		Temperature: config.Temperature,
	}
	llmResp, err := LLMWithRouter(messages, config.Model, params, nil)
	var responseText string
	if err != nil {
		responseText = "Error: " + err.Error()
	} else {
		responseText = llmResp.Content
	}
	
	// Trim to DNS limits (allowing more room with EDNS0)
	if len(responseText) > 2000 {
		responseText = responseText[:1997] + "..."
	}
	
	if debugMode {
		log.Printf("LLM response length: %d chars", len(responseText))
		log.Println("======= END DEBUG =======")
	}
	
	respondWithTXT(m, q, responseText)
}


func decodeBase32Query(s string) (string, error) {
	// Strict Base32 validator for unpadded Base32 (DNS-safe)
	// Rules:
	// - Alphabet: A-Z and 2-7
	// - No '=' allowed (removed for DNS)
	// - Length must be valid without padding: len % 8 in {0, 2, 4, 5, 7}
	// - Fully decodable after adding back padding
	
	if s == "" {
		return "", fmt.Errorf("empty string")
	}
	
	// Check for valid base32 characters: A-Z, 2-7 only (no padding)
	upper := strings.ToUpper(s)
	
	for i := 0; i < len(upper); i++ {
		c := upper[i]
		if !((c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')) {
			return "", fmt.Errorf("invalid base32 character: %c", c)
		}
	}
	
	// Length must correspond to valid unpadded base32
	validLengths := map[int]bool{0: true, 2: true, 4: true, 5: true, 7: true}
	if !validLengths[len(upper)%8] {
		return "", fmt.Errorf("invalid base32 length for unpadded string")
	}
	
	// Add padding back so Go's decoder can handle it
	padding := (8 - len(upper)%8) % 8
	padded := upper + strings.Repeat("=", padding)
	
	// Try to decode
	decoded, err := base32.StdEncoding.DecodeString(padded)
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

func respondWithTXT(m *dns.Msg, q dns.Question, response string) {
	
	// Split response into 255-byte chunks for DNS TXT records
	var txtStrings []string
	for i := 0; i < len(response); i += 255 {
		end := i + 255
		if end > len(response) {
			end = len(response)
		}
		txtStrings = append(txtStrings, response[i:end])
	}

	txt := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   q.Name,
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		Txt: txtStrings,
	}
	m.Answer = append(m.Answer, txt)
}

// Handle session initialization
func handleSessionInit(m *dns.Msg, q dns.Question, subdomain string) {
	// Extract public key hash from subdomain
	// Format: <pubkey_hash>.init.q.ch.at
	parts := strings.Split(subdomain, ".")
	if len(parts) < 2 {
		respondWithTXT(m, q, "ERROR: Invalid init format")
		return
	}
	
	pubKeyHashEncoded := parts[0]
	
	// For v1, we'll generate a simple session ID
	// In a real implementation, we'd verify the public key and encrypt the session ID
	sessionID := make([]byte, 16)
	if _, err := rand.Read(sessionID); err != nil {
		respondWithTXT(m, q, "ERROR: Failed to generate session ID")
		return
	}
	
	// Create new session
	session := &DoNutSession{
		ID:           base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sessionID),
		Chunks:       make(map[int]string),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}
	
	// Store session
	sessions.Store(session.ID, session)
	
	// For now, return the session ID directly (in production, encrypt with client's public key)
	// The client expects base64 encoded encrypted session ID
	response := base64.StdEncoding.EncodeToString(sessionID)
	
	if debugMode {
		log.Printf("Session initialized: %s (pubkey hash: %s)", session.ID, pubKeyHashEncoded)
	}
	respondWithTXT(m, q, response)
}

// Handle session chunk upload
func handleSessionChunk(m *dns.Msg, q dns.Question, subdomain string) {
	// Format: <session_id>.<chunk_num>.<chunk_data>.q.ch.at
	parts := strings.Split(subdomain, ".")
	if len(parts) < 3 {
		respondWithTXT(m, q, "ERROR: Invalid chunk format")
		return
	}
	
	sessionID := strings.ToUpper(parts[0])
	chunkNumEncoded := parts[1]
	chunkDataEncoded := parts[2]
	
	// Decode chunk number
	chunkNumBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(chunkNumEncoded))
	if err != nil || len(chunkNumBytes) == 0 {
		respondWithTXT(m, q, "ERROR: Invalid chunk number")
		return
	}
	chunkNum := int(chunkNumBytes[0])
	
	// Decode chunk data
	chunkData, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(chunkDataEncoded))
	if err != nil {
		respondWithTXT(m, q, "ERROR: Invalid chunk data")
		return
	}
	
	// Get session
	sessionInterface, ok := sessions.Load(sessionID)
	if !ok {
		respondWithTXT(m, q, "ERROR: Session not found")
		return
	}
	session := sessionInterface.(*DoNutSession)
	
	// Store chunk
	session.Chunks[chunkNum] = string(chunkData)
	session.LastActivity = time.Now()
	
	if debugMode {
		log.Printf("Received chunk %d for session %s (%d bytes)", chunkNum, sessionID, len(chunkData))
	}
	respondWithTXT(m, q, "ACK")
}

// Handle session execution
func handleSessionExec(m *dns.Msg, q dns.Question, subdomain string) {
	// Format: <session_id>.<total_chunks>.exec.q.ch.at
	parts := strings.Split(subdomain, ".")
	if len(parts) < 3 {
		respondWithTXT(m, q, "ERROR: Invalid exec format")
		return
	}
	
	sessionID := strings.ToUpper(parts[0])
	totalChunksEncoded := parts[1]
	
	// Decode total chunks
	totalChunksBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(totalChunksEncoded))
	if err != nil || len(totalChunksBytes) == 0 {
		respondWithTXT(m, q, "ERROR: Invalid total chunks")
		return
	}
	totalChunks := int(totalChunksBytes[0])
	
	// Get session
	sessionInterface, ok := sessions.Load(sessionID)
	if !ok {
		respondWithTXT(m, q, "ERROR: Session not found")
		return
	}
	session := sessionInterface.(*DoNutSession)
	
	// Check if we have all chunks
	if len(session.Chunks) != totalChunks {
		respondWithTXT(m, q, fmt.Sprintf("ERROR: Missing chunks (have %d, need %d)", len(session.Chunks), totalChunks))
		return
	}
	
	// Reassemble the query
	var reassembled strings.Builder
	for i := 0; i < totalChunks; i++ {
		chunk, ok := session.Chunks[i]
		if !ok {
			respondWithTXT(m, q, fmt.Sprintf("ERROR: Missing chunk %d", i))
			return
		}
		reassembled.WriteString(chunk)
	}
	
	// Clean up session
	sessions.Delete(sessionID)
	
	query := reassembled.String()
	if debugMode {
		log.Printf("Executed session %s: reassembled %d chunks into query: %s", sessionID, totalChunks, query)
	}
	
	// Get service configuration
	config := getServiceConfig("DONUTSENTRY")
	
	// Get LLM response for the reassembled query using router
	dnsPrompt := "Answer in 2000 characters or less, no markdown formatting: " + query
	messages := []map[string]string{
		{"role": "user", "content": dnsPrompt},
	}
	params := &RouterParams{
		MaxTokens:   config.MaxTokens,
		Temperature: config.Temperature,
	}
	llmResp, err := LLMWithRouter(messages, config.Model, params, nil)
	var responseText string
	if err != nil {
		responseText = fmt.Sprintf("Error processing %d chunks: %s", totalChunks, err.Error())
	} else {
		responseText = llmResp.Content
	}
	
	// Trim to DNS limits (allowing more room with EDNS0)
	if len(responseText) > 2000 {
		responseText = responseText[:1997] + "..."
	}
	
	respondWithTXT(m, q, responseText)
}

// Count dots in a string
func countDots(s string) int {
	count := 0
	for _, c := range s {
		if c == '.' {
			count++
		}
	}
	return count
}