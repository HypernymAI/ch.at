package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	auditDB     *sql.DB
	auditDBOnce sync.Once
	auditEnabled bool = true  // Can be set to false to disable all logging
)

// DisableAudit turns off all audit logging
func DisableAudit() {
	auditEnabled = false
	log.Println("[AUDIT] Audit logging DISABLED")
}

// EnableAudit turns on audit logging
func EnableAudit() {
	auditEnabled = true
	log.Println("[AUDIT] Audit logging ENABLED")
}

// LLMAuditEntry represents a complete LLM interaction
type LLMAuditEntry struct {
	ID             int64
	ConversationID string
	Timestamp      time.Time
	Model          string
	Deployment     string
	Provider       string
	FullInput      string // JSON encoded
	FullOutput     string
	InputTokens    int
	OutputTokens   int
	Error          string
}

// InitAuditDB initializes the SQLite database for LLM audit logging
func InitAuditDB() error {
	// Check if audit is enabled via environment variable (default: enabled)
	if os.Getenv("ENABLE_LLM_AUDIT") == "false" {
		DisableAudit()
		return nil
	}
	
	var err error
	auditDBOnce.Do(func() {
		auditDB, err = sql.Open("sqlite3", "llm_audit.db")
		if err != nil {
			log.Printf("Failed to open audit database: %v", err)
			return
		}

		// Create tables if they don't exist
		schema := `
		CREATE TABLE IF NOT EXISTS llm_audit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			model TEXT NOT NULL,
			deployment TEXT,
			provider TEXT,
			full_input TEXT NOT NULL,
			full_output TEXT NOT NULL,
			input_tokens INTEGER,
			output_tokens INTEGER,
			error TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_conversation_id ON llm_audit(conversation_id);
		CREATE INDEX IF NOT EXISTS idx_timestamp ON llm_audit(timestamp);
		CREATE INDEX IF NOT EXISTS idx_model ON llm_audit(model);
		`

		_, err = auditDB.Exec(schema)
		if err != nil {
			log.Printf("Failed to create audit schema: %v", err)
			return
		}

		log.Println("[AUDIT] LLM audit database initialized")
	})

	return err
}

// LogLLMInteraction logs a complete LLM interaction to the audit database
func LogLLMInteraction(conversationID string, model string, deployment string, provider string, input interface{}, output string, inputTokens int, outputTokens int, err error) {
	// Skip if audit is disabled
	if !auditEnabled {
		return
	}
	
	if auditDB == nil {
		// Silently skip if DB not initialized
		return
	}

	// Convert input to JSON for storage
	inputJSON, jsonErr := json.Marshal(input)
	if jsonErr != nil {
		log.Printf("[AUDIT] Failed to marshal input: %v", jsonErr)
		inputJSON = []byte(fmt.Sprintf("Error marshaling input: %v", jsonErr))
	}

	errorStr := ""
	if err != nil {
		errorStr = err.Error()
	}

	// Insert into database
	query := `
		INSERT INTO llm_audit (
			conversation_id, model, deployment, provider,
			full_input, full_output, input_tokens, output_tokens, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, dbErr := auditDB.Exec(query,
		conversationID, model, deployment, provider,
		string(inputJSON), output, inputTokens, outputTokens, errorStr)

	if dbErr != nil {
		log.Printf("[AUDIT] Failed to log LLM interaction: %v", dbErr)
		return
	}

	id, _ := result.LastInsertId()
	log.Printf("[AUDIT] Logged LLM interaction ID=%d, ConvID=%s, Model=%s, InputLen=%d, OutputLen=%d",
		id, conversationID, model, len(inputJSON), len(output))
}

// GetConversationHistory retrieves all interactions for a conversation
func GetConversationHistory(conversationID string) ([]LLMAuditEntry, error) {
	if auditDB == nil {
		return nil, fmt.Errorf("audit database not initialized")
	}

	query := `
		SELECT id, conversation_id, timestamp, model, deployment, provider,
		       full_input, full_output, input_tokens, output_tokens, error
		FROM llm_audit
		WHERE conversation_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := auditDB.Query(query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LLMAuditEntry
	for rows.Next() {
		var entry LLMAuditEntry
		err := rows.Scan(
			&entry.ID, &entry.ConversationID, &entry.Timestamp,
			&entry.Model, &entry.Deployment, &entry.Provider,
			&entry.FullInput, &entry.FullOutput,
			&entry.InputTokens, &entry.OutputTokens, &entry.Error,
		)
		if err != nil {
			log.Printf("[AUDIT] Error scanning row: %v", err)
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}