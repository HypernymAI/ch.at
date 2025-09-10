package main

import (
	"flag"
	"log"
	"os"
)

// Note: Port configuration has moved to config.go
// Use HIGH_PORT_MODE=true environment variable for development

var debugMode bool

func main() {
	// Parse command line flags
	flag.BoolVar(&debugMode, "debug", false, "Enable debug logging")
	flag.Parse()
	
	// Initialize audit database FIRST
	if err := InitAuditDB(); err != nil {
		log.Printf("WARNING: Audit database initialization failed: %v", err)
		log.Println("LLM interactions will not be logged")
	}
	
	// Initialize model router (non-blocking, falls back to legacy if fails)
	if err := InitializeModelRouter(); err != nil {
		log.Printf("Model router initialization failed: %v", err)
		log.Println("Using legacy LLM mode")
	}
	
	// Beacon application startup
	beacon("chat_startup", map[string]interface{}{
		"http_port":  HTTP_PORT,
		"https_port": HTTPS_PORT,
		"ssh_port":   SSH_PORT,
		"dns_port":   DNS_PORT,
		"high_port_mode": os.Getenv("HIGH_PORT_MODE") == "true",
		"debug_mode": debugMode,
		"router_enabled": modelRouter != nil,
	})

	// SSH Server
	if SSH_PORT > 0 {
		go func() {
			StartSSHServer(SSH_PORT)
		}()
	}

	// DNS Server
	if DNS_PORT > 0 {
		go func() {
			StartDNSServer(DNS_PORT)
		}()
	}

	// HTTP/HTTPS Server
	// TODO: Implement graceful shutdown with signal handling
	if HTTP_PORT > 0 || HTTPS_PORT > 0 {
		if HTTPS_PORT > 0 {
			go func() {
				certPath, keyPath, found := findSSLCertificates()
				if !found {
					log.Printf("WARNING: SSL certificates not found, HTTPS disabled")
					log.Printf("Expected cert.pem and key.pem in working directory")
					log.Printf("Or valid Let's Encrypt certificates")
					return
				}
				StartHTTPSServer(HTTPS_PORT, certPath, keyPath)
			}()
		}

		if HTTP_PORT > 0 {
			StartHTTPServer(HTTP_PORT)
		} else {
			// If only HTTPS is enabled, block forever
			select {}
		}
	} else {
		// If no servers enabled, block forever
		select {}
	}
}
