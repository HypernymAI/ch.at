package main

import (
	"log"
	"os"
	"strings"
)

// Module represents a processing module that can handle specific types of requests
type Module interface {
	// Name returns the module identifier
	Name() string
	
	// ShouldHandle analyzes input to determine if this module should process it
	ShouldHandle(input string) bool
	
	// Process handles the input and returns a response
	Process(input string, messages []map[string]string) (string, error)
}

// Discriminator manages routing to different modules based on input analysis
type Discriminator struct {
	modules []Module
}

// NewDiscriminator creates a new discriminator with registered modules
func NewDiscriminator() *Discriminator {
	d := &Discriminator{
		modules: []Module{},
	}
	
	// Register modules based on environment variables
	// Each module can be enabled/disabled via ENABLE_MODULE_<NAME>=true/false
	
	if os.Getenv("ENABLE_MODULE_CODE") != "false" { // Default enabled
		d.RegisterModule(&CodeAnalysisModule{})
	}
	
	if os.Getenv("ENABLE_MODULE_RESEARCH") != "false" { // Default enabled
		d.RegisterModule(&ResearchModule{})
	}
	
	if os.Getenv("ENABLE_MODULE_CREATIVE") != "false" { // Default enabled
		d.RegisterModule(&CreativeWritingModule{})
	}
	
	if os.Getenv("ENABLE_MODULE_CHAOS") != "false" { // Default enabled
		d.RegisterModule(&ChaosModule{})
	}
	
	// Check for discriminator disable flag
	if os.Getenv("DISABLE_DISCRIMINATOR") == "true" {
		log.Println("[Discriminator] DISABLED via environment variable")
		return nil
	}
	
	return d
}

// RegisterModule adds a new module to the discriminator
func (d *Discriminator) RegisterModule(m Module) {
	d.modules = append(d.modules, m)
	log.Printf("[Discriminator] Registered module: %s", m.Name())
}

// Analyze determines which module should handle the input
func (d *Discriminator) Analyze(input string) Module {
	// Check each module in priority order
	for _, module := range d.modules {
		if module.ShouldHandle(input) {
			log.Printf("[Discriminator] Selected module: %s", module.Name())
			return module
		}
	}
	
	// No specific module matched
	return nil
}

// Process routes the input to the appropriate module
func (d *Discriminator) Process(input string, messages []map[string]string) (string, error) {
	module := d.Analyze(input)
	if module != nil {
		beacon("discriminator_route", map[string]interface{}{
			"module": module.Name(),
			"input_length": len(input),
		})
		return module.Process(input, messages)
	}
	
	// No module matched - use default LLM
	beacon("discriminator_default", map[string]interface{}{
		"input_length": len(input),
	})
	return "", nil // Signal to use default processing
}

// --- Example Modules ---

// CodeAnalysisModule handles code-related queries
type CodeAnalysisModule struct{}

func (m *CodeAnalysisModule) Name() string { return "code_analysis" }

func (m *CodeAnalysisModule) ShouldHandle(input string) bool {
	lower := strings.ToLower(input)
	codeKeywords := []string{
		"debug", "error", "bug", "code", "function", "class", 
		"compile", "syntax", "refactor", "implement", "algorithm",
	}
	
	for _, keyword := range codeKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func (m *CodeAnalysisModule) Process(input string, messages []map[string]string) (string, error) {
	// Enhance the prompt for code analysis
	enhancedPrompt := "You are an expert programmer and debugger. Analyze the following carefully:\n\n" + input
	
	// Use ch.at's own routing internally
	enhancedMessages := []map[string]string{
		{"role": "system", "content": "You are an expert software engineer."},
		{"role": "user", "content": enhancedPrompt},
	}
	
	response, err := LLMWithRouter(enhancedMessages, "llama-70b", nil)
	if err != nil {
		return "", err
	}
	
	return response.Content, nil
}

// ResearchModule handles research and information queries
type ResearchModule struct{}

func (m *ResearchModule) Name() string { return "research" }

func (m *ResearchModule) ShouldHandle(input string) bool {
	lower := strings.ToLower(input)
	researchKeywords := []string{
		"research", "explain", "what is", "how does", "why does",
		"compare", "difference between", "analyze", "study",
	}
	
	for _, keyword := range researchKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func (m *ResearchModule) Process(input string, messages []map[string]string) (string, error) {
	// Add research context
	enhancedPrompt := "Provide a comprehensive, well-researched response to:\n\n" + input
	
	enhancedMessages := []map[string]string{
		{"role": "system", "content": "You are a research assistant. Provide detailed, accurate information."},
		{"role": "user", "content": enhancedPrompt},
	}
	
	response, err := LLMWithRouter(enhancedMessages, "llama-70b", nil)
	if err != nil {
		return "", err
	}
	
	return response.Content, nil
}

// CreativeWritingModule handles creative writing tasks
type CreativeWritingModule struct{}

func (m *CreativeWritingModule) Name() string { return "creative_writing" }

func (m *CreativeWritingModule) ShouldHandle(input string) bool {
	lower := strings.ToLower(input)
	creativeKeywords := []string{
		"write", "story", "poem", "creative", "imagine",
		"fiction", "narrative", "character", "plot",
	}
	
	for _, keyword := range creativeKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func (m *CreativeWritingModule) Process(input string, messages []map[string]string) (string, error) {
	enhancedMessages := []map[string]string{
		{"role": "system", "content": "You are a creative writer with vivid imagination."},
		{"role": "user", "content": input},
	}
	
	response, err := LLMWithRouter(enhancedMessages, "llama-70b", nil)
	if err != nil {
		return "", err
	}
	
	return response.Content, nil
}

// ChaosModule wraps existing chaos rectification
type ChaosModule struct{}

func (m *ChaosModule) Name() string { return "chaos" }

func (m *ChaosModule) ShouldHandle(input string) bool {
	return strings.Contains(strings.ToLower(input), "magic")
}

func (m *ChaosModule) Process(input string, messages []map[string]string) (string, error) {
	// Use existing chaos rectification
	return processChaosRectification(input), nil
}

// Global discriminator instance
var discriminator *Discriminator

func init() {
	discriminator = NewDiscriminator()
	log.Println("[Discriminator] Initialized with modules")
}