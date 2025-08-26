package main

import (
	"log"
	"os"
	"path/filepath"
)

// Port configuration based on environment
var (
	HTTP_PORT  int
	HTTPS_PORT int
	SSH_PORT   int
	DNS_PORT   int
)

func init() {
	// Check for high-port development mode
	if os.Getenv("HIGH_PORT_MODE") == "true" {
		log.Println("Running in HIGH_PORT_MODE - using non-privileged ports")
		HTTP_PORT = 8080   // Instead of 80
		HTTPS_PORT = 8443  // Instead of 443
		SSH_PORT = 2222    // Instead of 22
		DNS_PORT = 8053    // Instead of 53
	} else {
		// Production mode - standard ports
		HTTP_PORT = 80
		HTTPS_PORT = 443
		SSH_PORT = 22
		DNS_PORT = 53
	}
	
	log.Printf("Port configuration: HTTP=%d, HTTPS=%d, SSH=%d, DNS=%d", 
		HTTP_PORT, HTTPS_PORT, SSH_PORT, DNS_PORT)
}

// findSSLCertificates looks for SSL certificates in common locations
func findSSLCertificates() (certPath, keyPath string, found bool) {
	// First, check working directory
	if fileExists("cert.pem") && fileExists("key.pem") {
		return "cert.pem", "key.pem", true
	}
	
	// Check for Let's Encrypt certificates
	domain := os.Getenv("BASE_DOMAIN")
	if domain == "" {
		domain = "chat.hypernym.ai" // fallback
	}
	
	letsEncryptPaths := []string{
		filepath.Join("/etc/letsencrypt/live", domain),
		filepath.Join("/etc/letsencrypt/live", "chat."+domain),
	}
	
	for _, basePath := range letsEncryptPaths {
		certFile := filepath.Join(basePath, "fullchain.pem")
		keyFile := filepath.Join(basePath, "privkey.pem")
		
		if fileExists(certFile) && fileExists(keyFile) {
			log.Printf("Found Let's Encrypt certificates at %s", basePath)
			return certFile, keyFile, true
		}
	}
	
	// Check common alternative locations
	alternativePaths := []struct {
		cert string
		key  string
	}{
		{"/etc/ssl/certs/cert.pem", "/etc/ssl/private/key.pem"},
		{"/etc/ssl/cert.pem", "/etc/ssl/key.pem"},
	}
	
	for _, paths := range alternativePaths {
		if fileExists(paths.cert) && fileExists(paths.key) {
			log.Printf("Found certificates at %s", filepath.Dir(paths.cert))
			return paths.cert, paths.key, true
		}
	}
	
	return "", "", false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}