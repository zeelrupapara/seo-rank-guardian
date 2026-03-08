package scraper

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"go.uber.org/zap"
)

type SearchResult struct {
	Position int    `json:"position"`
	URL      string `json:"url"`
	Domain   string `json:"domain"`
	Title    string `json:"title"`
	Snippet  string `json:"snippet"`
}

type CollySearcher struct {
	log         *zap.SugaredLogger
	resultLimit int
	minDelay    time.Duration
	randomDelay time.Duration
	maxRetries  int
}

func NewCollySearcher(cfg config.ScraperConfig, log *zap.SugaredLogger) *CollySearcher {
	randomDelay := time.Duration(cfg.MaxDelayMs-cfg.MinDelayMs) * time.Millisecond
	if randomDelay < 0 {
		randomDelay = 0
	}
	return &CollySearcher{
		log:         log,
		resultLimit: cfg.ResultLimit,
		minDelay:    time.Duration(cfg.MinDelayMs) * time.Millisecond,
		randomDelay: randomDelay,
		maxRetries:  cfg.MaxRetries,
	}
}

func (s *CollySearcher) Name() string  { return "colly-google" }
func (s *CollySearcher) Enabled() bool { return true }

func (s *CollySearcher) Search(opts SearchOptions) ([]SearchResult, error) {
	query := opts.Query
	region := opts.Region
	if region == "" {
		region = "us"
	}
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	resultLimit := opts.ResultLimit
	if resultLimit <= 0 {
		resultLimit = s.resultLimit
	}

	var results []SearchResult
	var scrapeErr error

	c := colly.NewCollector(
		colly.UserAgent(RandomUserAgent()),
		colly.MaxDepth(1),
		colly.AllowURLRevisit(),
	)

	c.SetRequestTimeout(30 * time.Second)

	if err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*google*",
		Delay:       s.minDelay,
		RandomDelay: s.randomDelay,
		Parallelism: 1,
	}); err != nil {
		return nil, fmt.Errorf("failed to set limit rule: %w", err)
	}

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
		r.Headers.Set("Accept-Encoding", "gzip, deflate, br")
		r.Headers.Set("Connection", "keep-alive")
		r.Headers.Set("Sec-Ch-Ua", `"Chromium";v="131", "Not_A Brand";v="24"`)
		r.Headers.Set("Sec-Ch-Ua-Mobile", "?0")
		r.Headers.Set("Sec-Fetch-Dest", "document")
		r.Headers.Set("Sec-Fetch-Mode", "navigate")
		r.Headers.Set("Sec-Fetch-Site", "none")
		s.log.Debugf("Scraping: %s", r.URL.String())
	})

	c.OnHTML("#captcha-form", func(_ *colly.HTMLElement) {
		scrapeErr = fmt.Errorf("captcha detected for query: %s", query)
		s.log.Warnf("Captcha detected for query: %s", query)
	})

	c.OnHTML("div.g", func(e *colly.HTMLElement) {
		if scrapeErr != nil || len(results) >= resultLimit {
			return
		}

		// Skip ads
		if e.Attr("data-text-ad") != "" {
			return
		}

		title := e.ChildText("h3")
		link := e.ChildAttr("a", "href")

		if link == "" || strings.HasPrefix(link, "/search") || strings.HasPrefix(link, "#") {
			return
		}

		// Extract snippet with fallback
		snippet := e.ChildText("div.VwiC3b")
		if snippet == "" {
			snippet = e.ChildText("div[data-sncf]")
		}

		parsedURL, err := url.Parse(link)
		if err != nil {
			return
		}

		domain := parsedURL.Hostname()
		if domain == "" || strings.Contains(domain, "google.") {
			return
		}

		results = append(results, SearchResult{
			Position: len(results) + 1,
			URL:      link,
			Domain:   domain,
			Title:    title,
			Snippet:  snippet,
		})
	})

	c.OnResponse(func(r *colly.Response) {
		if r.StatusCode != 200 {
			scrapeErr = fmt.Errorf("unexpected status code: %d for query: %s", r.StatusCode, query)
		}
		if len(r.Body) < 1000 {
			s.log.Warnf("Suspiciously small response (%d bytes) for query: %s", len(r.Body), query)
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		s.log.Errorf("Scrape error for query '%s': %v", query, err)
		scrapeErr = fmt.Errorf("scrape error: %w", err)
	})

	searchURL := fmt.Sprintf(
		"https://www.google.com/search?q=%s&num=%d&gl=%s&hl=%s",
		url.QueryEscape(query),
		resultLimit, region, lang,
	)

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			s.log.Infof("Retry %d/%d for query: %s", attempt, s.maxRetries, query)
			c.UserAgent = RandomUserAgent()
			time.Sleep(s.minDelay * time.Duration(attempt))
		}

		scrapeErr = nil
		results = nil

		if err := c.Visit(searchURL); err != nil {
			scrapeErr = fmt.Errorf("visit error: %w", err)
			continue
		}

		c.Wait()

		if scrapeErr == nil && len(results) > 0 {
			break
		}
	}

	if scrapeErr != nil {
		return nil, scrapeErr
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found for query: %s", query)
	}

	s.log.Infof("Found %d results for query: %s", len(results), query)
	return results, nil
}
