package main

import (
	"encoding/base32"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Session storage for DoNutSentry
type DoNutSession struct {
	ID        string
	Chunks    map[int]string
	CreatedAt time.Time
}

var (
	sessions   = &sync.Map{} // session_id -> *DoNutSession
	sessionTTL = 5 * time.Minute
)

func handleDoNutSentryQuery(w dns.ResponseWriter, r *dns.Msg, m *dns.Msg, q dns.Question) {
	// Ensure we send the response at the end
	defer w.WriteMsg(m)
	
	// Extract subdomain (everything before .q.ch.at.)
	fullName := strings.ToLower(q.Name)
	subdomain := strings.TrimSuffix(fullName, ".q.ch.at.")
	
	// Debug output to stderr
	fmt.Println("======= DONUTSENTRY DEBUG =======")
	fmt.Printf("Received query: %s\n", subdomain)

	// Handle different query types
	if strings.HasSuffix(subdomain, ".init") {
		// Session initialization - TODO: implement RSA key exchange
		respondWithTXT(m, q, "SESSION_NOT_IMPLEMENTED")
		return
	} else if strings.HasSuffix(subdomain, ".exec") {
		// Session execution - TODO: implement chunk assembly
		respondWithTXT(m, q, "SESSION_NOT_IMPLEMENTED")
		return
	} else if strings.Contains(subdomain, ".") {
		// Might be a session chunk - TODO: implement chunk handling
		respondWithTXT(m, q, "SESSION_NOT_IMPLEMENTED")
		return
	}

	// Simple query - decode and process
	var prompt string

	// Try base32 first
	decoded, err := decodeBase32Query(subdomain)
	if err == nil {
		prompt = decoded
		fmt.Printf("Successfully decoded base32: %s -> %s\n", subdomain, prompt)
	} else {
		// Not base32, use simple encoding
		prompt = strings.ReplaceAll(subdomain, "-", " ")
		fmt.Printf("Using simple encoding: %s -> %s (base32 error: %v)\n", subdomain, prompt, err)
	}

	// For testing, return a simple response without LLM
	testResponse := "DoNutSentry v1.0.2: You queried '" + prompt + "' via " + subdomain + ".q.ch.at"
	
	// Debug: also log the final response
	fmt.Printf("Final response: %s\n", testResponse)
	fmt.Println("======= END DEBUG =======")
	
	respondWithTXT(m, q, testResponse)
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