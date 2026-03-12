"""
Patchright (headed) + DataImpulse proxy — production-ready test.
Browser is properly closed after each search to avoid resource leaks.
"""

import asyncio
from urllib.parse import urlparse, quote_plus

from bs4 import BeautifulSoup, Tag
from patchright.async_api import async_playwright, Playwright, Browser

PROXY_URL = "http://00554b628aa45b1d6f2c:790e3120cfdc8a8d@gw.dataimpulse.com:823"
SOCS_COOKIE = "CAISHAgBEhJnd3NfMjAyNTAzMTAtMAEiAmVuKAMaBgiA2e68Bg"

STEALTH_JS = """
Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
window.chrome = { runtime: {} };
"""


def parse_proxy_url(proxy_url: str) -> dict:
    parsed = urlparse(proxy_url)
    proxy = {"server": f"{parsed.scheme}://{parsed.hostname}:{parsed.port}"}
    if parsed.username:
        proxy["username"] = parsed.username
    if parsed.password:
        proxy["password"] = parsed.password
    return proxy


# ── Future-proof structural SERP parser ────────────────────────────────


def _extract_snippet(container: Tag | None, a_tag: Tag, title: str) -> str:
    if not container:
        return ""
    for sel in ["div.VwiC3b", "div[data-sncf]", "span.hgKElc"]:
        el = container.select_one(sel)
        if el:
            text = el.get_text(strip=True)
            if text and text != title:
                return text
    candidates = []
    for el in container.find_all(["div", "span"]):
        if el.find_parent("a") == a_tag or el.find(["div", "span"], recursive=False):
            continue
        text = el.get_text(strip=True)
        if len(text) >= 60 and text != title:
            candidates.append(text)
    return max(candidates, key=len) if candidates else ""


def parse_serp(html: str, limit: int = 10) -> list[dict]:
    soup = BeautifulSoup(html, "lxml")
    results = []
    position = 0
    search_root = soup.select_one("div#rso") or soup.select_one("div#search") or soup.body
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
        is_ad = False
        parent = a_tag.parent
        for _ in range(10):
            if parent is None:
                break
            if parent.get("id", "") in ("tads", "bottomads") or parent.get("data-text-ad") is not None:
                is_ad = True
                break
            parent = parent.parent
        if is_ad:
            continue
        title = h3.get_text(strip=True)
        if not title or any(r["url"] == href for r in results):
            continue
        container = a_tag.find_parent("div", class_="MjjYud") or a_tag.find_parent("div", class_="g")
        snippet = _extract_snippet(container, a_tag, title)
        position += 1
        results.append({
            "position": position, "url": href, "domain": domain,
            "title": title, "snippet": snippet,
        })
        if position >= limit:
            break
    return results


def is_captcha(html: str) -> bool:
    html_lower = html.lower()
    return any(s in html_lower for s in [
        'id="captcha-form"', "unusual traffic",
        "our systems have detected unusual traffic",
    ])


# ── Scraper class (mirrors production usage) ──────────────────────────


class GoogleScraper:
    """Patchright-based Google scraper. Launches browser per search, closes after."""

    def __init__(self, proxy_url: str = ""):
        self.proxy_url = proxy_url
        self._pw: Playwright | None = None
        self._browser: Browser | None = None

    async def _launch(self):
        """Launch Patchright browser."""
        self._pw = await async_playwright().start()
        launch_opts = {"headless": False}
        if self.proxy_url:
            launch_opts["proxy"] = parse_proxy_url(self.proxy_url)
        self._browser = await self._pw.chromium.launch(**launch_opts)

    async def search(self, query: str, region: str = "us", language: str = "en", limit: int = 10) -> list[dict]:
        """Run a single search. Opens browser, searches, closes browser."""
        await self._launch()
        try:
            return await self._do_search(query, region, language, limit)
        finally:
            await self.close()

    async def _do_search(self, query: str, region: str, language: str, limit: int) -> list[dict]:
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
                {"name": "SOCS", "value": SOCS_COOKIE, "domain": ".google.com", "path": "/"},
                {"name": "CONSENT", "value": "YES+cb", "domain": ".google.com", "path": "/"},
            ])

            page = await context.new_page()
            url = (
                f"https://www.google.com/search"
                f"?q={quote_plus(query)}&num={limit}&gl={region}&hl={language}"
            )

            resp = await page.goto(url, wait_until="domcontentloaded", timeout=30000)
            status = resp.status if resp else 0

            if status == 429:
                raise Exception("Rate limited (429)")

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

            if is_captcha(html):
                raise Exception("CAPTCHA detected")

            return parse_serp(html, limit)
        finally:
            await context.close()

    async def close(self):
        """Close browser and Playwright — frees all resources."""
        if self._browser:
            await self._browser.close()
            self._browser = None
        if self._pw:
            await self._pw.stop()
            self._pw = None


# ── Test Runner ────────────────────────────────────────────────────────


async def test_search(query: str) -> bool:
    print(f"\n{'='*60}")
    print(f"Query: '{query}'")
    print(f"{'='*60}")

    scraper = GoogleScraper(proxy_url=PROXY_URL)
    try:
        results = await scraper.search(query)
        print(f"  Results: {len(results)}")
        for r in results:
            print(f"    #{r['position']} [{r['domain']}] {r['title'][:55]}")
            if r['snippet']:
                print(f"           {r['snippet'][:80]}")
        if results:
            print("  SUCCESS!")
        return len(results) > 0
    except Exception as e:
        print(f"  FAILED: {e}")
        return False


async def main():
    queries = [
        "google in California",
        "best seo tools for small business",
        "python programming tutorial",
    ]

    results = []
    for i, query in enumerate(queries):
        r = await test_search(query)
        results.append((query, r))
        if i < len(queries) - 1:
            print("\n  Waiting 8s between queries...")
            await asyncio.sleep(8)

    print(f"\n\n{'='*60}")
    print("FINAL RESULTS: Patchright (headed) + Proxy + Proper Cleanup")
    print(f"{'='*60}")
    for query, ok in results:
        print(f"  [{'PASS' if ok else 'FAIL'}] {query}")
    passed = sum(1 for _, ok in results if ok)
    print(f"\n  {passed}/{len(results)} passed")
    print(f"{'='*60}")


if __name__ == "__main__":
    asyncio.run(main())
