from __future__ import annotations

import asyncio
import time
from typing import List, Tuple
from urllib.parse import quote_plus, urlparse

import structlog
from patchright.async_api import async_playwright, Playwright, Browser

from scraper.parser import parse_serp

log = structlog.get_logger()

SOCS_COOKIE_VALUE = "CAISHAgBEhJnd3NfMjAyNTAzMTAtMAEiAmVuKAMaBgiA2e68Bg"

STEALTH_JS = """
Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
window.chrome = { runtime: {} };
"""


class GoogleSearchResult:
    __slots__ = ("position", "url", "domain", "title", "snippet")

    def __init__(self, position: int, url: str, domain: str, title: str, snippet: str):
        self.position = position
        self.url = url
        self.domain = domain
        self.title = title
        self.snippet = snippet


COUNTRY_TO_GL = {
    "united states": "us", "united kingdom": "gb", "canada": "ca",
    "australia": "au", "india": "in", "germany": "de", "france": "fr",
    "brazil": "br", "japan": "jp", "mexico": "mx", "spain": "es",
    "italy": "it", "netherlands": "nl", "russia": "ru", "south korea": "kr",
    "indonesia": "id", "turkey": "tr", "saudi arabia": "sa", "sweden": "se",
    "switzerland": "ch", "poland": "pl", "belgium": "be", "austria": "at",
    "norway": "no", "denmark": "dk", "finland": "fi", "ireland": "ie",
    "new zealand": "nz", "singapore": "sg", "malaysia": "my",
    "philippines": "ph", "thailand": "th", "vietnam": "vn",
    "south africa": "za", "egypt": "eg", "nigeria": "ng",
    "argentina": "ar", "colombia": "co", "chile": "cl", "peru": "pe",
    "portugal": "pt", "czech republic": "cz", "romania": "ro",
    "hungary": "hu", "greece": "gr", "israel": "il", "ukraine": "ua",
    "pakistan": "pk", "bangladesh": "bd", "hong kong": "hk", "taiwan": "tw",
    "china": "cn", "uae": "ae", "united arab emirates": "ae",
}


def _normalize_region(region: str) -> str:
    """Convert full country name to ISO 3166-1 alpha-2 code for Google's gl= param."""
    region = region.strip().lower()
    if len(region) <= 3:
        return region  # already a code like "us", "gb"
    return COUNTRY_TO_GL.get(region, "us")


class GoogleScraper:
    """Patchright-based Google scraper with Serper.dev API fallback.

    Uses headed Chromium via Patchright (patched Playwright) to bypass
    Google SearchGuard. Browser is launched per search and closed after
    to avoid resource leaks on the server.
    """

    def __init__(
        self,
        proxy_url: str = "",
        serper_api_key: str = "",
    ):
        self.proxy_url = proxy_url
        self.serper_api_key = serper_api_key
        self._pw: Playwright | None = None
        self._browser: Browser | None = None

    async def _launch_browser(self):
        """Launch Patchright browser."""
        self._pw = await async_playwright().start()

        launch_opts = {"headless": False}
        if self.proxy_url:
            parsed = urlparse(self.proxy_url)
            proxy = {"server": f"{parsed.scheme}://{parsed.hostname}:{parsed.port}"}
            if parsed.username:
                proxy["username"] = parsed.username
            if parsed.password:
                proxy["password"] = parsed.password
            launch_opts["proxy"] = proxy

        self._browser = await self._pw.chromium.launch(**launch_opts)

    async def close(self):
        """Close browser and Playwright — frees all resources."""
        if self._browser:
            await self._browser.close()
            self._browser = None
        if self._pw:
            await self._pw.stop()
            self._pw = None

    async def search_async(
        self,
        query: str,
        region: str = "us",
        language: str = "en",
        result_limit: int = 10,
    ) -> Tuple[List[GoogleSearchResult], str]:
        attempted_methods = []

        gl = _normalize_region(region)
        url = (
            f"https://www.google.com/search"
            f"?q={quote_plus(query)}&num={result_limit}&gl={gl}&hl={language}"
        )

        # Step 1: Patchright browser (retry up to 5 times — each launch gets a fresh proxy IP)
        method = "patchright:chromium"
        attempted_methods.append(method)
        max_retries = 5
        for attempt in range(1, max_retries + 1):
            log.info("search.attempt", method=method, query=query, gl=gl, attempt=attempt)
            t0 = time.monotonic()
            try:
                results = await self._search_browser(url, result_limit)
                elapsed_ms = round((time.monotonic() - t0) * 1000)
                if results:
                    log.info("search.success", method=method, result_count=len(results), elapsed_ms=elapsed_ms)
                    return results, method
                log.warning("search.empty", method=method, elapsed_ms=elapsed_ms)
                break  # got 200 but 0 results — no point retrying
            except Exception as e:
                elapsed_ms = round((time.monotonic() - t0) * 1000)
                err_str = str(e).lower()
                is_retryable = "429" in err_str or "rate limit" in err_str or "captcha" in err_str
                log.warning("search.failed", method=method, error=str(e), elapsed_ms=elapsed_ms, attempt=attempt)
                if is_retryable and attempt < max_retries:
                    delay = 2 + attempt  # increasing backoff: 3s, 4s, 5s, 6s
                    log.info("search.retry", reason="retryable_error_rotating_ip", next_attempt=attempt + 1, delay_s=delay)
                    await asyncio.sleep(delay)
                    continue
                break

        # Step 2: Serper.dev API (fallback)
        if self.serper_api_key:
            method = "serper_api"
            attempted_methods.append(method)
            log.info("search.attempt", method=method, query=query, region=region)
            t0 = time.monotonic()
            try:
                results = await self._search_serper(query, region, language, result_limit)
                elapsed_ms = round((time.monotonic() - t0) * 1000)
                if results:
                    log.info("search.success", method=method, result_count=len(results), elapsed_ms=elapsed_ms)
                    return results, method
            except Exception as e:
                elapsed_ms = round((time.monotonic() - t0) * 1000)
                log.warning("search.failed", method=method, error=str(e), elapsed_ms=elapsed_ms)

        log.error("search.all_failed", query=query, attempts=attempted_methods)
        raise Exception("All search methods failed")

    # ── Patchright browser ─────────────────────────────────────────────

    async def _search_browser(self, url: str, result_limit: int) -> List[GoogleSearchResult]:
        await self._launch_browser()
        try:
            return await self._do_search(url, result_limit)
        finally:
            await self.close()

    async def _do_search(self, url: str, result_limit: int) -> List[GoogleSearchResult]:
        context = await self._browser.new_context(
            user_agent=(
                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                "AppleWebKit/537.36 (KHTML, like Gecko) "
                "Chrome/136.0.0.0 Safari/537.36"
            ),
            viewport={"width": 1280, "height": 800},
            locale="en-US",
        )
        try:
            await context.add_init_script(STEALTH_JS)
            await context.add_cookies([
                {"name": "SOCS", "value": SOCS_COOKIE_VALUE, "domain": ".google.com", "path": "/"},
                {"name": "CONSENT", "value": "YES+cb", "domain": ".google.com", "path": "/"},
            ])

            page = await context.new_page()
            resp = await page.goto(url, wait_until="domcontentloaded", timeout=30000)

            if resp and resp.status == 429:
                raise Exception("HTTP 429 — rate limited")

            # Wait for results container
            for sel in ["div#rso", "div#search"]:
                try:
                    await page.wait_for_selector(sel, timeout=10000)
                    break
                except Exception:
                    continue

            # Dismiss consent banner if present
            try:
                btn = page.locator("button#L2AGLb")
                if await btn.count() > 0:
                    await btn.click(timeout=3000)
                    await page.wait_for_timeout(2000)
            except Exception:
                pass

            html = await page.content()

            if _is_captcha(html):
                raise Exception("CAPTCHA detected")

            parsed = parse_serp(html, result_limit)
            return _to_search_results(parsed)
        finally:
            await context.close()

    # ── Serper.dev API ─────────────────────────────────────────────────

    async def _search_serper(
        self, query: str, region: str, language: str, result_limit: int
    ) -> List[GoogleSearchResult]:
        import httpx

        async with httpx.AsyncClient(timeout=15) as client:
            resp = await client.post(
                "https://google.serper.dev/search",
                headers={
                    "X-API-KEY": self.serper_api_key,
                    "Content-Type": "application/json",
                },
                json={
                    "q": query,
                    "gl": region,
                    "hl": language,
                    "num": result_limit,
                },
            )
            resp.raise_for_status()
            data = resp.json()

        results = []
        organic = data.get("organic", [])
        for i, item in enumerate(organic[:result_limit], start=1):
            link = item.get("link", "")
            domain = _extract_domain(link)
            results.append(GoogleSearchResult(
                position=i,
                url=link,
                domain=domain,
                title=item.get("title", ""),
                snippet=item.get("snippet", ""),
            ))
        return results


def _to_search_results(parsed: list) -> List[GoogleSearchResult]:
    return [
        GoogleSearchResult(
            position=r["position"],
            url=r["url"],
            domain=r["domain"],
            title=r["title"],
            snippet=r["snippet"],
        )
        for r in parsed
    ] if parsed else []


def _extract_domain(url: str) -> str:
    try:
        parsed = urlparse(url)
        return parsed.netloc or ""
    except Exception:
        return ""


def _is_captcha(html: str) -> bool:
    captcha_signals = [
        'id="captcha-form"',
        "unusual traffic",
        "our systems have detected unusual traffic",
    ]
    html_lower = html.lower()
    return any(signal in html_lower for signal in captcha_signals)
