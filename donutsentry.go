package main

import (
	"encoding/base32"
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

	// First try base32 decoding if it looks like base32
	if looksLikeBase32(subdomain) {
		decoded, err := decodeBase32Query(subdomain)
		if err == nil {
			prompt = decoded
		} else if isSimpleQuery(subdomain) {
			// Fall back to simple encoding if base32 fails
			prompt = strings.ReplaceAll(subdomain, "-", " ")
		} else {
			respondWithTXT(m, q, "Error: Invalid query encoding")
			return
		}
	} else if isSimpleQuery(subdomain) {
		// Simple encoding: just replace hyphens with spaces
		prompt = strings.ReplaceAll(subdomain, "-", " ")
	} else {
		respondWithTXT(m, q, "Error: Invalid query encoding")
		return
	}

	// For testing, return a simple response without LLM
	testResponse := "DoNutSentry Test Response: You queried '" + prompt + "' via " + subdomain + ".q.ch.at"
	
	respondWithTXT(m, q, testResponse)
}

func isSimpleQuery(s string) bool {
	// Simple queries only contain lowercase letters, numbers, and hyphens
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}

func looksLikeBase32(s string) bool {
	// Base32 strings are typically:
	// - All lowercase letters/numbers
	// - No hyphens
	// - Length is multiple of 8 (without padding) or close to it
	// - Contains base32 alphabet characters
	
	if strings.Contains(s, "-") {
		return false // Simple queries use hyphens
	}
	
	// Check if it's mostly base32 alphabet (a-z, 2-7)
	base32Chars := 0
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '2' && c <= '7') {
			base32Chars++
		}
	}
	
	// If more than 80% are valid base32 chars and no hyphens, probably base32
	return float64(base32Chars) / float64(len(s)) > 0.8
}

func decodeBase32Query(s string) (string, error) {
	// DoNutSentry uses lowercase base32 without padding
	// Convert to uppercase and add padding if needed
	s = strings.ToUpper(s)
	
	// Add padding if necessary
	padding := (8 - len(s)%8) % 8
	if padding > 0 {
		s += strings.Repeat("=", padding)
	}

	decoded, err := base32.StdEncoding.DecodeString(s)
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