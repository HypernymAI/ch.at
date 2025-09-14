package main

import (
	"log"
	"github.com/joho/godotenv"
)

func init() {
	// Load .env file before anything else
	if err := godotenv.Load(); err != nil {
		// Only log errors - this is important to always see
		log.Printf("No .env file found: %v", err)
	}
}