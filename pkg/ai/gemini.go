package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GeminiClient struct {
	apiKey          string
	model           string
	searchGrounding bool
	client          *http.Client
}

func NewGeminiClient(cfg AIClientConfig) (*GeminiClient, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("gemini API key is required")
	}
	model := cfg.Model
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiClient{
		apiKey:          cfg.APIKey,
		model:           model,
		searchGrounding: cfg.SearchGrounding,
		client:          &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Request types

type geminiRequest struct {
	SystemInstruction *geminiContent    `json:"system_instruction,omitempty"`
	Contents          []geminiContent   `json:"contents"`
	Tools             []geminiTool      `json:"tools,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiTool struct {
	GoogleSearch *struct{} `json:"google_search,omitempty"`
}

type generationConfig struct {
	ResponseMimeType string         `json:"responseMimeType,omitempty"`
	ResponseSchema   map[string]any `json:"responseSchema,omitempty"`
}

// Response types

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Error      *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content           geminiContent      `json:"content"`
	GroundingMetadata *groundingMetadata `json:"groundingMetadata,omitempty"`
}

type groundingMetadata struct {
	WebSearchQueries  []string        `json:"webSearchQueries,omitempty"`
	GroundingChunks   json.RawMessage `json:"groundingChunks,omitempty"`
	GroundingSupports json.RawMessage `json:"groundingSupports,omitempty"`
	SearchEntryPoint  json.RawMessage `json:"searchEntryPoint,omitempty"`
}

func (g *GeminiClient) Analyze(prompt string) (string, error) {
	result, err := g.AnalyzeStructured(AnalyzeOptions{UserContent: prompt})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

func (g *GeminiClient) AnalyzeStructured(opts AnalyzeOptions) (*AnalyzeResult, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: opts.UserContent}}},
		},
	}

	if opts.SystemInstruction != "" {
		reqBody.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: opts.SystemInstruction}},
		}
	}

	if opts.EnableSearch && g.searchGrounding {
		reqBody.Tools = []geminiTool{{GoogleSearch: &struct{}{}}}
	}

	if opts.ResponseSchema != nil {
		reqBody.GenerationConfig = &generationConfig{
			ResponseMimeType: "application/json",
			ResponseSchema:   opts.ResponseSchema,
		}
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.model, g.apiKey,
	)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Avoid logging full response body which may contain request URL with API key
		return nil, fmt.Errorf("gemini API error (status %d, model %s)", resp.StatusCode, g.model)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("gemini error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	result := &AnalyzeResult{
		Content: geminiResp.Candidates[0].Content.Parts[0].Text,
	}

	if geminiResp.Candidates[0].GroundingMetadata != nil {
		groundingData, err := json.Marshal(geminiResp.Candidates[0].GroundingMetadata)
		if err == nil {
			result.GroundingMeta = groundingData
		}
	}

	return result, nil
}
