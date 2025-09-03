package main

import (
	"flag"
	"log"
)

// Note: Port configuration has moved to config.go
// Use HIGH_PORT_MODE=true environment variable for development

var debugMode bool

func main() {
	// Parse command line flags
	flag.BoolVar(&debugMode, "debug", false, "Enable debug logging")
	flag.Parse()
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
				StartHTTPSServer(HTTPS_PORT, "cert.pem", "key.pem")
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
