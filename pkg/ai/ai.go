package ai

import "fmt"

type Provider string

const (
	ProviderGemini Provider = "gemini"
	ProviderClaude Provider = "claude"
	ProviderOpenAI Provider = "openai"
)

type AIClient interface {
	Analyze(prompt string) (string, error)
}

func NewAIClient(provider, apiKey string) (AIClient, error) {
	switch Provider(provider) {
	case ProviderGemini:
		return &stubClient{provider: provider}, nil
	case ProviderClaude:
		return &stubClient{provider: provider}, nil
	case ProviderOpenAI:
		return &stubClient{provider: provider}, nil
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", provider)
	}
}

type stubClient struct {
	provider string
}

func (s *stubClient) Analyze(prompt string) (string, error) {
	return "", fmt.Errorf("%s client not yet implemented", s.provider)
}
