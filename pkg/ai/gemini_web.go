package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"go.uber.org/zap"
)

const (
	geminiURL             = "https://gemini.google.com/app"
	selectorChatInput     = `div[contenteditable="true"], rich-textarea`
	selectorSendButton    = `button[aria-label="Send message"]`
	selectorResponseBlock = `.model-response-text, .response-container, .message-content`
	selectorStopButton    = `button[aria-label="Stop generating"]`
)

// GeminiWebConfig holds configuration for the web scraper mode.
type GeminiWebConfig struct {
	TimeoutSec int
	Logger     *zap.SugaredLogger
}

// GeminiWebScraper implements AIClient by scraping gemini.google.com.
type GeminiWebScraper struct {
	timeout time.Duration
	log     *zap.SugaredLogger
}

// NewGeminiWebScraper creates a new web scraper AI client.
func NewGeminiWebScraper(cfg GeminiWebConfig) (*GeminiWebScraper, error) {
	timeout := cfg.TimeoutSec
	if timeout <= 0 {
		timeout = 300
	}
	return &GeminiWebScraper{
		timeout: time.Duration(timeout) * time.Second,
		log:     cfg.Logger,
	}, nil
}

func (g *GeminiWebScraper) Analyze(prompt string) (string, error) {
	result, err := g.AnalyzeStructured(AnalyzeOptions{UserContent: prompt})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

func (g *GeminiWebScraper) AnalyzeStructured(opts AnalyzeOptions) (*AnalyzeResult, error) {
	prompt := WrapPromptForWebMode(opts.SystemInstruction, opts.UserContent, opts.ResponseSchema)

	g.log.Info("gemini-web: launching browser")

	path, found := launcher.LookPath()
	if !found {
		return nil, fmt.Errorf("gemini-web: Chrome/Chromium not found in PATH")
	}

	u, err := launcher.New().Bin(path).
		Set("disable-blink-features", "AutomationControlled").
		Set("no-first-run").
		Set("no-default-browser-check").
		Headless(true).
		Launch()
	if err != nil {
		return nil, fmt.Errorf("gemini-web: failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("gemini-web: failed to connect to browser: %w", err)
	}
	defer browser.Close()

	// Open incognito context for clean state
	incognito, err := browser.Incognito()
	if err != nil {
		return nil, fmt.Errorf("gemini-web: failed to create incognito context: %w", err)
	}
	defer incognito.Close()

	page, err := incognito.Page(proto.TargetCreateTarget{URL: geminiURL})
	if err != nil {
		return nil, fmt.Errorf("gemini-web: failed to open page: %w", err)
	}
	defer page.Close()

	g.log.Info("gemini-web: navigated to gemini.google.com, waiting for chat input")

	// Wait for chat input to appear
	err = page.Timeout(30 * time.Second).WaitStable(2 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("gemini-web: page did not stabilize: %w", err)
	}

	inputEl, err := page.Timeout(30 * time.Second).Element(selectorChatInput)
	if err != nil {
		return nil, fmt.Errorf("gemini-web: chat input not found: %w", err)
	}

	// Use Rod's Input method to type the prompt — more reliable than JS eval
	// on custom elements like rich-textarea
	if err := inputEl.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, fmt.Errorf("gemini-web: failed to focus input: %w", err)
	}
	time.Sleep(300 * time.Millisecond)

	if err := inputEl.Input(prompt); err != nil {
		return nil, fmt.Errorf("gemini-web: failed to input prompt: %w", err)
	}

	g.log.Info("gemini-web: prompt entered, clicking send")

	// Wait for UI to register input
	time.Sleep(1 * time.Second)

	sendBtn, err := page.Timeout(10 * time.Second).Element(selectorSendButton)
	if err != nil {
		return nil, fmt.Errorf("gemini-web: send button not found: %w", err)
	}

	if err := sendBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, fmt.Errorf("gemini-web: failed to click send: %w", err)
	}

	g.log.Info("gemini-web: waiting for response generation")

	// Wait for generation to complete by polling
	responseText, err := g.waitForResponse(page)
	if err != nil {
		return nil, fmt.Errorf("gemini-web: %w", err)
	}

	g.log.Infof("gemini-web: response received (%d chars)", len(responseText))

	// Extract JSON from response
	jsonStr, err := extractJSON(responseText)
	if err != nil {
		return nil, fmt.Errorf("gemini-web: %w", err)
	}

	return &AnalyzeResult{Content: jsonStr}, nil
}

// waitForResponse polls until the response is complete (stop button disappears and DOM is stable).
func (g *GeminiWebScraper) waitForResponse(page *rod.Page) (string, error) {
	deadline := time.Now().Add(g.timeout)
	pollInterval := 2 * time.Second
	var lastText string
	stableCount := 0

	// First wait a bit for response to start
	time.Sleep(3 * time.Second)

	for time.Now().Before(deadline) {
		// Check if stop button is still present (means still generating)
		stopBtn, err := page.Timeout(1 * time.Second).Element(selectorStopButton)
		stillGenerating := err == nil && stopBtn != nil

		// Get current response text
		responseEls, err := page.Elements(selectorResponseBlock)
		if err != nil || len(responseEls) == 0 {
			time.Sleep(pollInterval)
			continue
		}

		// Get the last response element's text (the AI's reply)
		currentText, err := responseEls[len(responseEls)-1].Text()
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if !stillGenerating && currentText == lastText && len(currentText) > 0 {
			stableCount++
			if stableCount >= 2 {
				return currentText, nil
			}
		} else {
			stableCount = 0
		}

		lastText = currentText
		time.Sleep(pollInterval)
	}

	if lastText != "" {
		return lastText, nil
	}
	return "", fmt.Errorf("timed out waiting for response after %v", g.timeout)
}

var codeFenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")

// extractJSON strips markdown code fences and extracts a JSON object from text.
func extractJSON(text string) (string, error) {
	// Try to extract from code fence first
	if matches := codeFenceRe.FindStringSubmatch(text); len(matches) > 1 {
		candidate := strings.TrimSpace(matches[1])
		if json.Valid([]byte(candidate)) {
			return candidate, nil
		}
	}

	// Fallback: find first { to last }
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		candidate := text[start : end+1]
		if json.Valid([]byte(candidate)) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no valid JSON object found in response (length: %d)", len(text))
}
