from urllib.parse import urlparse

import structlog
from bs4 import BeautifulSoup, Tag

log = structlog.get_logger()


def _extract_snippet(container: Tag | None, a_tag: Tag, title: str) -> str:
    """Extract snippet from a result container."""
    if not container:
        return ""

    # Known snippet classes (fast path)
    for sel in ["div.VwiC3b", "div[data-sncf]", "span.hgKElc"]:
        el = container.select_one(sel)
        if el:
            text = el.get_text(strip=True)
            if text and text != title:
                return text

    # Structural fallback — leaf text blocks outside the title link
    candidates = []
    for el in container.find_all(["div", "span"]):
        if el.find_parent("a") == a_tag or el.find(["div", "span"], recursive=False):
            continue
        text = el.get_text(strip=True)
        if len(text) >= 60 and text != title:
            candidates.append(text)

    return max(candidates, key=len) if candidates else ""


def parse_serp(html: str, limit: int = 10) -> list[dict]:
    """
    Future-proof SERP parser using structural patterns (a > h3).
    Works with both old (div.g) and new (div.MjjYud) Google layouts.
    """
    soup = BeautifulSoup(html, "lxml")
    results = []
    position = 0

    search_root = (
        soup.select_one("div#rso")
        or soup.select_one("div#search")
        or soup.body
    )
    if not search_root:
        return results

    for a_tag in search_root.find_all("a", href=True):
        h3 = a_tag.find("h3")
        if not h3:
            continue

        href = a_tag.get("href", "")
        if not href.startswith("http"):
            continue

        parsed = urlparse(href)
        domain = parsed.netloc.lower()
        if not domain or "google." in domain or "googleapis." in domain:
            continue

        # Skip ads
        is_ad = False
        parent = a_tag.parent
        for _ in range(10):
            if parent is None:
                break
            if parent.get("id", "") in ("tads", "bottomads"):
                is_ad = True
                break
            if parent.get("data-text-ad") is not None:
                is_ad = True
                break
            parent = parent.parent
        if is_ad:
            continue

        title = h3.get_text(strip=True)
        if not title:
            continue

        # Deduplicate by URL
        if any(r["url"] == href for r in results):
            continue

        # Find result container for snippet extraction
        container = (
            a_tag.find_parent("div", class_="MjjYud")
            or a_tag.find_parent("div", class_="g")
        )
        snippet = _extract_snippet(container, a_tag, title)

        position += 1
        results.append({
            "position": position,
            "url": href,
            "domain": domain,
            "title": title,
            "snippet": snippet,
        })

        if position >= limit:
            break

    if not results:
        log.warning("parse_serp: 0 results", html_size=len(html))

    return results
