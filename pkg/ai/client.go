package ai

import (
	"encoding/json"
	"fmt"
)

type Provider string

const (
	ProviderGemini Provider = "gemini"
	ProviderClaude Provider = "claude"
	ProviderOpenAI Provider = "openai"
)

// AnalyzeOptions is provider-agnostic. Each provider maps these
// to its own API format internally.
type AnalyzeOptions struct {
	SystemInstruction string         // Role/persona (Gemini: system_instruction, OpenAI: system message, Claude: system param)
	UserContent       string         // The user message containing data
	ResponseSchema    map[string]any // JSON Schema — each provider enforces this differently
	EnableSearch      bool           // Web search grounding (Gemini: google_search tool, others: provider-specific)
}

// AnalyzeResult holds the AI response plus optional metadata.
type AnalyzeResult struct {
	Content       string          // The text/JSON response body
	GroundingMeta json.RawMessage // Provider-specific grounding/citation data (nil if not used)
}

// AIClient is the provider-agnostic interface.
type AIClient interface {
	Analyze(prompt string) (string, error)
	AnalyzeStructured(opts AnalyzeOptions) (*AnalyzeResult, error)
}

// AIClientConfig holds all config needed to create any provider's client.
type AIClientConfig struct {
	Provider        string
	APIKey          string
	Model           string
	SearchGrounding bool
}

// NewAIClient creates a provider-specific client.
func NewAIClient(cfg AIClientConfig) (AIClient, error) {
	switch Provider(cfg.Provider) {
	case ProviderGemini:
		return NewGeminiClient(cfg)
	case ProviderClaude:
		return nil, fmt.Errorf("claude client not yet implemented")
	case ProviderOpenAI:
		return nil, fmt.Errorf("openai client not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", cfg.Provider)
	}
}
