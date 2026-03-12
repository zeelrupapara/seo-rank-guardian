from __future__ import annotations

from typing import Optional, List, Dict


SEO_SYSTEM_INSTRUCTION = """You are a world-class SEO strategist and content analyst. You combine technical SEO expertise with deep content strategy knowledge to produce reports that directly drive ranking improvements.

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
Your response MUST be a single valid JSON object and NOTHING else. No markdown, no code fences, no explanatory text, no comments, no preamble, no summary outside the JSON. Do not wrap the JSON in backticks or any other formatting. The very first character of your response must be { and the very last character must be }. Any text outside the JSON object will cause a system failure."""


REPORT_SCHEMA = {
    "type": "object",
    "properties": {
        "summary": {
            "type": "string",
            "description": "2-3 sentence overview of ranking performance and key changes",
        },
        "health_score": {
            "type": "integer",
            "description": "Overall SEO health score from 0-100",
        },
        "critical_alerts": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "keyword": {"type": "string"},
                    "state": {"type": "string"},
                    "severity": {"type": "string", "enum": ["high", "medium", "low"]},
                    "current_position": {"type": "integer"},
                    "previous_position": {"type": "integer"},
                    "message": {
                        "type": "string",
                        "description": "Clear explanation of the issue and its impact",
                    },
                },
                "required": ["keyword", "severity", "message"],
            },
        },
        "keyword_rankings": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "keyword": {"type": "string"},
                    "state": {"type": "string"},
                    "your_position": {"type": "integer"},
                    "prev_position": {"type": "integer"},
                    "change": {
                        "type": "string",
                        "enum": ["improved", "dropped", "new", "lost", "stable"],
                    },
                    "top_3": {"type": "array", "items": {"type": "string"}},
                    "competitors_in_top_10": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "domain": {"type": "string"},
                                "position": {"type": "integer"},
                            },
                            "required": ["domain", "position"],
                        },
                    },
                },
                "required": ["keyword", "your_position", "change"],
            },
        },
        "content_gaps": {
            "type": "array",
            "description": "Per-keyword content gap analysis comparing target page vs higher-ranking competitors",
            "items": {
                "type": "object",
                "properties": {
                    "keyword": {"type": "string"},
                    "state": {"type": "string"},
                    "target_url": {
                        "type": "string",
                        "description": "The target page URL ranking for this keyword",
                    },
                    "target_position": {"type": "integer"},
                    "gaps": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "competitor_domain": {"type": "string"},
                                "competitor_position": {"type": "integer"},
                                "competitor_url": {"type": "string"},
                                "missing_topics": {
                                    "type": "array",
                                    "items": {"type": "string"},
                                    "description": "Topics/subtopics the competitor covers that the target page lacks",
                                },
                                "content_advantage": {
                                    "type": "string",
                                    "description": "What the competitor page does better (depth, format, freshness, structure)",
                                },
                            },
                            "required": [
                                "competitor_domain",
                                "competitor_position",
                                "competitor_url",
                                "missing_topics",
                                "content_advantage",
                            ],
                        },
                    },
                },
                "required": ["keyword", "target_url", "target_position", "gaps"],
            },
        },
        "recommendations": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "priority": {
                        "type": "integer",
                        "description": "1 = highest priority",
                    },
                    "keyword": {"type": "string"},
                    "state": {"type": "string"},
                    "action": {"type": "string", "description": "Short action title"},
                    "details": {
                        "type": "string",
                        "description": "Detailed recommendation with specific steps",
                    },
                    "target_url": {
                        "type": "string",
                        "description": "Which page to improve",
                    },
                    "content_changes": {
                        "type": "string",
                        "description": "Specific content to add or modify on the target page",
                    },
                },
                "required": ["priority", "action", "details"],
            },
        },
        "competitor_insights": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "domain": {"type": "string"},
                    "avg_position": {"type": "number"},
                    "keywords_beating_us": {"type": "integer"},
                    "trend": {
                        "type": "string",
                        "enum": ["improving", "stable", "declining"],
                    },
                },
                "required": ["domain", "avg_position", "trend"],
            },
        },
    },
    "required": [
        "summary",
        "health_score",
        "critical_alerts",
        "keyword_rankings",
        "content_gaps",
        "recommendations",
        "competitor_insights",
    ],
}


def build_report_content(
    domain: str,
    competitors: list[str],
    results: list[dict],
    diffs: list[dict],
) -> str:
    lines = []
    lines.append(f"Domain: {domain}")
    lines.append(f"Competitors: [{', '.join(competitors)}]")
    lines.append("")
    lines.append("=== Current SERP Results ===")

    for r in results:
        marker = ""
        if r.get("is_target"):
            marker = " [TARGET]"
        elif r.get("is_competitor"):
            marker = " [COMPETITOR]"

        lines.append(
            f'- Keyword: "{r["keyword"]}" | Region: {r["state"]} | '
            f'Position: {r["position"]} | Domain: {r["domain"]}{marker}'
        )
        if r.get("url"):
            lines.append(f'  URL: {r["url"]}')
        if r.get("title"):
            lines.append(f'  Title: "{r["title"]}"')
        if r.get("snippet"):
            lines.append(f'  Snippet: "{r["snippet"]}"')

    if diffs:
        lines.append("")
        lines.append("=== Rank Changes vs Previous Run ===")
        for d in diffs:
            lines.append(
                f'- {d["domain"]} | Keyword: "{d["keyword"]}" | Region: {d["state"]} | '
                f'{d["change_type"]} (pos {d["prev_position"]} → {d["curr_position"]}, delta: {d["delta"]})'
            )

    lines.append("")
    lines.append("=== Instructions ===")
    lines.append(
        "1. Use web search to research the actual ranking URLs above — "
        "understand what each page covers and how well it satisfies search intent."
    )
    lines.append(
        "2. For every keyword where a competitor outranks the target, "
        "identify the specific content gaps and structural advantages."
    )
    lines.append(
        "3. Generate the full SEO intelligence report with content gap analysis "
        "and page-specific recommendations."
    )

    return "\n".join(lines)
