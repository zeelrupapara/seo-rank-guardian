package scraper

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"go.uber.org/zap"
)

type RodSearcher struct {
	log        *zap.SugaredLogger
	enabled    bool
	maxRetries int
	minDelay   time.Duration
	maxDelay   time.Duration
}

func NewRodSearcher(cfg config.ScraperConfig, log *zap.SugaredLogger) *RodSearcher {
	return &RodSearcher{
		log:        log,
		enabled:    cfg.RodEnabled,
		maxRetries: cfg.MaxRetries,
		minDelay:   time.Duration(cfg.MinDelayMs) * time.Millisecond,
		maxDelay:   time.Duration(cfg.MaxDelayMs) * time.Millisecond,
	}
}

func (r *RodSearcher) Name() string    { return "rod-google" }
func (r *RodSearcher) Enabled() bool   { return r.enabled }

func (r *RodSearcher) Search(opts SearchOptions) ([]SearchResult, error) {
	region := opts.Region
	if region == "" {
		region = "us"
	}
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	limit := opts.ResultLimit
	if limit <= 0 {
		limit = 10
	}

	searchURL := fmt.Sprintf(
		"https://www.google.com/search?q=%s&num=%d&gl=%s&hl=%s",
		url.QueryEscape(opts.Query), limit, region, lang,
	)

	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := r.minDelay * time.Duration(attempt)
			if backoff > r.maxDelay {
				backoff = r.maxDelay
			}
			jitter := time.Duration(rand.Int63n(int64(r.maxDelay - r.minDelay + 1)))
			r.log.Infof("[rod] Retry %d/%d for query: %s (waiting %v)", attempt, r.maxRetries, opts.Query, backoff+jitter)
			time.Sleep(backoff + jitter)
		}

		results, err := r.scrapeOnce(searchURL, opts.Query, limit)
		if err != nil {
			lastErr = err
			continue
		}
		return results, nil
	}

	return nil, fmt.Errorf("rod: all %d attempts failed: %w", r.maxRetries+1, lastErr)
}

func (r *RodSearcher) scrapeOnce(searchURL, query string, limit int) ([]SearchResult, error) {
	path, found := launcher.LookPath()
	if !found {
		return nil, fmt.Errorf("rod: Chrome/Chromium not found in PATH")
	}

	u, err := launcher.New().Bin(path).
		Set("disable-blink-features", "AutomationControlled").
		Set("no-first-run").
		Set("no-default-browser-check").
		Headless(true).
		Launch()
	if err != nil {
		return nil, fmt.Errorf("rod: failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("rod: failed to connect to browser: %w", err)
	}
	defer browser.Close()

	ua := RandomUserAgent()
	page := browser.MustPage("")

	// Set user agent
	_ = proto.NetworkSetUserAgentOverride{UserAgent: ua}.Call(page)

	if err := page.Navigate(searchURL); err != nil {
		return nil, fmt.Errorf("rod navigate error: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("rod wait load error: %w", err)
	}

	// Check for CAPTCHA
	captcha, err := page.Element("#captcha-form")
	if err == nil && captcha != nil {
		return nil, fmt.Errorf("captcha detected for query: %s", query)
	}

	// Wait for search results
	_, err = page.Timeout(10 * time.Second).Element("div.g")
	if err != nil {
		return nil, fmt.Errorf("rod: no search results rendered for query: %s", query)
	}

	elements, err := page.Elements("div.g")
	if err != nil {
		return nil, fmt.Errorf("rod: failed to get result elements: %w", err)
	}

	var results []SearchResult
	for _, el := range elements {
		if len(results) >= limit {
			break
		}

		// Skip ads
		dataAd, _ := el.Attribute("data-text-ad")
		if dataAd != nil && *dataAd != "" {
			continue
		}

		titleEl, err := el.Element("h3")
		if err != nil {
			continue
		}
		title, _ := titleEl.Text()

		linkEl, err := el.Element("a")
		if err != nil {
			continue
		}
		link, err := linkEl.Property("href")
		if err != nil || link.String() == "" {
			continue
		}
		linkStr := link.String()

		if strings.HasPrefix(linkStr, "/search") || strings.HasPrefix(linkStr, "#") {
			continue
		}

		// Extract snippet
		var snippet string
		snippetEl, err := el.Element("div.VwiC3b")
		if err == nil && snippetEl != nil {
			snippet, _ = snippetEl.Text()
		}
		if snippet == "" {
			snippetEl, err = el.Element("div[data-sncf]")
			if err == nil && snippetEl != nil {
				snippet, _ = snippetEl.Text()
			}
		}

		parsedURL, err := url.Parse(linkStr)
		if err != nil {
			continue
		}
		domain := parsedURL.Hostname()
		if domain == "" || strings.Contains(domain, "google.") {
			continue
		}

		results = append(results, SearchResult{
			Position: len(results) + 1,
			URL:      linkStr,
			Domain:   domain,
			Title:    title,
			Snippet:  snippet,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("rod: no results found for query: %s", query)
	}

	r.log.Infof("[rod] Found %d results for query: %s", len(results), query)
	return results, nil
}
