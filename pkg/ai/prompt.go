package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

const SEOSystemInstruction = `You are a world-class SEO strategist and content analyst. You combine technical SEO expertise with deep content strategy knowledge to produce reports that directly drive ranking improvements.

You will receive SERP ranking data that includes the actual URLs, page titles, and snippets of every ranking result — both the target site and competitors. This is your primary evidence.

## Your analysis process (follow this order):

### Step 1: Research the ranking pages
Use web search/grounding to visit and study the actual URLs provided in the data. For each keyword+region:
- Read the target's ranking page — note its content structure, topics covered, word count, content freshness, and how well it satisfies search intent
- Read the top-ranking competitor pages — note what they cover that the target does not
- Identify the dominant search intent (informational, transactional, navigational, commercial investigation) and whether each page aligns with it

### Step 2: Content gap analysis (most critical output)
For each keyword where competitors outrank the target:
- List specific topics, subtopics, questions, and content angles that competitor pages cover but the target page lacks
- Assess content depth: does the competitor go deeper on key subtopics? Do they use data, examples, case studies, visuals?
- Check content format alignment: are top results using listicles, how-to guides, comparison tables, video embeds, FAQ sections?
- Note page structure advantages: better heading hierarchy, table of contents, featured snippet optimization, schema markup
- Evaluate E-E-A-T signals: author credentials, citations, original research, user-generated content

### Step 3: Ranking movement analysis
- Flag any position drops > 3, any keyword falling off page 1, any new competitor entering top 3
- Correlate rank changes with possible causes: content updates by competitors, algorithm patterns, seasonal trends
- When multiple regions are tracked, surface regional disparities and possible local SEO factors

### Step 4: Actionable recommendations
Every recommendation must:
- Reference the specific target URL to improve
- Describe exactly what content to add, modify, or restructure on that page
- Explain the expected impact (which competitor it would help overtake, estimated position improvement)
- Be prioritized by: (1) traffic potential of the keyword, (2) size of the gap, (3) implementation effort

## Health score criteria
- 90-100: Most keywords on page 1, stable or improving, strong content coverage
- 70-89: Some page 1 presence, minor drops, moderate content gaps
- 50-69: Significant drops or few page 1 rankings, notable content gaps vs competitors
- 0-49: Major losses, off page 1 for important keywords, critical content deficiencies

## Important rules
- Be strictly evidence-based: every claim must cite specific data from the SERP results or your web research
- Never give generic SEO advice — every recommendation must be specific to the actual pages and keywords in the data
- If no rank changes exist (first run), focus entirely on current competitive positioning and content gap analysis
- Prioritize depth over breadth: a thorough analysis of the most important keywords beats shallow coverage of all keywords

## CRITICAL — Output format
Your response MUST be a single valid JSON object and NOTHING else. No markdown, no code fences, no explanatory text, no comments, no preamble, no summary outside the JSON. Do not wrap the JSON in backticks or any other formatting. The very first character of your response must be { and the very last character must be }. Any text outside the JSON object will cause a system failure.`

var ReportSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"summary": map[string]any{
			"type":        "string",
			"description": "2-3 sentence overview of ranking performance and key changes",
		},
		"health_score": map[string]any{
			"type":        "integer",
			"description": "Overall SEO health score from 0-100",
		},
		"critical_alerts": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"keyword":           map[string]any{"type": "string"},
					"state":             map[string]any{"type": "string"},
					"severity":          map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
					"current_position":  map[string]any{"type": "integer"},
					"previous_position": map[string]any{"type": "integer"},
					"message":           map[string]any{"type": "string", "description": "Clear explanation of the issue and its impact"},
				},
				"required": []string{"keyword", "severity", "message"},
			},
		},
		"keyword_rankings": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"keyword":       map[string]any{"type": "string"},
					"state":         map[string]any{"type": "string"},
					"your_position": map[string]any{"type": "integer"},
					"prev_position": map[string]any{"type": "integer"},
					"change":        map[string]any{"type": "string", "enum": []string{"improved", "dropped", "new", "lost", "stable"}},
					"top_3":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"competitors_in_top_10": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"domain":   map[string]any{"type": "string"},
								"position": map[string]any{"type": "integer"},
							},
							"required": []string{"domain", "position"},
						},
					},
				},
				"required": []string{"keyword", "your_position", "change"},
			},
		},
		"content_gaps": map[string]any{
			"type":        "array",
			"description": "Per-keyword content gap analysis comparing target page vs higher-ranking competitors",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"keyword":         map[string]any{"type": "string"},
					"state":           map[string]any{"type": "string"},
					"target_url":      map[string]any{"type": "string", "description": "The target page URL ranking for this keyword"},
					"target_position": map[string]any{"type": "integer"},
					"gaps": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"competitor_domain":   map[string]any{"type": "string"},
								"competitor_position": map[string]any{"type": "integer"},
								"competitor_url":      map[string]any{"type": "string"},
								"missing_topics":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Topics/subtopics the competitor covers that the target page lacks"},
								"content_advantage":   map[string]any{"type": "string", "description": "What the competitor page does better (depth, format, freshness, structure)"},
							},
							"required": []string{"competitor_domain", "competitor_position", "competitor_url", "missing_topics", "content_advantage"},
						},
					},
				},
				"required": []string{"keyword", "target_url", "target_position", "gaps"},
			},
		},
		"recommendations": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"priority":        map[string]any{"type": "integer", "description": "1 = highest priority"},
					"keyword":         map[string]any{"type": "string"},
					"state":           map[string]any{"type": "string"},
					"action":          map[string]any{"type": "string", "description": "Short action title"},
					"details":         map[string]any{"type": "string", "description": "Detailed recommendation with specific steps"},
					"target_url":      map[string]any{"type": "string", "description": "Which page to improve"},
					"content_changes": map[string]any{"type": "string", "description": "Specific content to add or modify on the target page"},
				},
				"required": []string{"priority", "action", "details"},
			},
		},
		"competitor_insights": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"domain":              map[string]any{"type": "string"},
					"avg_position":        map[string]any{"type": "number"},
					"keywords_beating_us": map[string]any{"type": "integer"},
					"trend":               map[string]any{"type": "string", "enum": []string{"improving", "stable", "declining"}},
				},
				"required": []string{"domain", "avg_position", "trend"},
			},
		},
	},
	"required": []string{"summary", "health_score", "critical_alerts", "keyword_rankings", "content_gaps", "recommendations", "competitor_insights"},
}

// WrapPromptForWebMode merges system instruction, user content, and JSON schema
// into a single prompt string for the Gemini web UI (which has no separate
// system instruction or schema enforcement fields).
func WrapPromptForWebMode(systemInstruction, userContent string, schema map[string]any) string {
	var b strings.Builder

	if systemInstruction != "" {
		fmt.Fprintf(&b, "=== SYSTEM INSTRUCTION ===\n%s\n\n", systemInstruction)
	}

	fmt.Fprintf(&b, "=== DATA ===\n%s\n\n", userContent)

	if schema != nil {
		schemaJSON, err := json.MarshalIndent(schema, "", "  ")
		if err == nil {
			fmt.Fprintf(&b, "=== RESPONSE JSON SCHEMA ===\n%s\n\n", string(schemaJSON))
		}
	}

	b.WriteString("Respond with ONLY a valid JSON object matching the schema above. No markdown fences, no extra text. The first character must be { and the last must be }.")

	return b.String()
}

// ReportResult is a simplified view of model.SearchResult for prompt building.
type ReportResult struct {
	Keyword      string
	State        string
	Position     int
	Domain       string
	URL          string
	Title        string
	Snippet      string
	IsTarget     bool
	IsCompetitor bool
}

// ReportDiff is a simplified view of model.RankDiff for prompt building.
type ReportDiff struct {
	Domain       string
	Keyword      string
	State        string
	ChangeType   string
	PrevPosition int
	CurrPosition int
	Delta        int
}

// BuildReportContent builds the user content for the AI report prompt.
func BuildReportContent(domain string, competitors []string, results []ReportResult, diffs []ReportDiff) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Domain: %s\n", domain)
	fmt.Fprintf(&b, "Competitors: [%s]\n", strings.Join(competitors, ", "))

	b.WriteString("\n=== Current SERP Results ===\n")
	for _, r := range results {
		marker := ""
		if r.IsTarget {
			marker = " [TARGET]"
		} else if r.IsCompetitor {
			marker = " [COMPETITOR]"
		}
		fmt.Fprintf(&b, "- Keyword: %q | Region: %s | Position: %d | Domain: %s%s\n",
			r.Keyword, r.State, r.Position, r.Domain, marker)
		if r.URL != "" {
			fmt.Fprintf(&b, "  URL: %s\n", r.URL)
		}
		if r.Title != "" {
			fmt.Fprintf(&b, "  Title: %q\n", r.Title)
		}
		if r.Snippet != "" {
			fmt.Fprintf(&b, "  Snippet: %q\n", r.Snippet)
		}
	}

	if len(diffs) > 0 {
		b.WriteString("\n=== Rank Changes vs Previous Run ===\n")
		for _, d := range diffs {
			fmt.Fprintf(&b, "- %s | Keyword: %q | Region: %s | %s (pos %d → %d, delta: %d)\n",
				d.Domain, d.Keyword, d.State, d.ChangeType, d.PrevPosition, d.CurrPosition, d.Delta)
		}
	}

	b.WriteString("\n=== Instructions ===\n")
	b.WriteString("1. Use web search to research the actual ranking URLs above — understand what each page covers and how well it satisfies search intent.\n")
	b.WriteString("2. For every keyword where a competitor outranks the target, identify the specific content gaps and structural advantages.\n")
	b.WriteString("3. Generate the full SEO intelligence report with content gap analysis and page-specific recommendations.\n")

	return b.String()
}
