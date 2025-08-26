package main

import (
	"log"
)

// Note: Port configuration has moved to config.go
// Use HIGH_PORT_MODE=true environment variable for development

func main() {
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
