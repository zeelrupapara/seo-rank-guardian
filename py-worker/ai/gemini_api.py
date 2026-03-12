import json
import re

import httpx
import structlog

from ai.client import AnalyzeOptions, AnalyzeResult

log = structlog.get_logger()

CODE_FENCE_RE = re.compile(r"```(?:json)?\s*\n?(.*?)\n?\s*```", re.DOTALL)


class GeminiAPIClient:
    def __init__(self, api_key: str, model: str = "gemini-2.0-flash", search_grounding: bool = True):
        self.api_key = api_key
        self.model = model
        self.search_grounding = search_grounding
        self._client = httpx.AsyncClient(timeout=180.0)

    async def analyze(self, prompt: str) -> str:
        result = await self.analyze_structured(AnalyzeOptions(user_content=prompt))
        return result.content

    async def analyze_structured(self, opts: AnalyzeOptions) -> AnalyzeResult:
        req_body: dict = {
            "contents": [{"parts": [{"text": opts.user_content}]}],
        }

        if opts.system_instruction:
            req_body["system_instruction"] = {"parts": [{"text": opts.system_instruction}]}

        use_search = opts.enable_search and self.search_grounding

        if use_search:
            req_body["tools"] = [{"google_search": {}}]

        # Gemini 2.5+ doesn't support responseSchema with google_search tool,
        # so only use structured output when search is disabled.
        if opts.response_schema and not use_search:
            req_body["generationConfig"] = {
                "responseMimeType": "application/json",
                "responseSchema": opts.response_schema,
            }
        elif opts.response_schema and use_search:
            # Inject schema into the prompt text so Gemini follows it even without
            # structured output enforcement (which is incompatible with google_search).
            schema_text = json.dumps(opts.response_schema, indent=2)
            schema_instruction = (
                "\n\n## REQUIRED JSON SCHEMA\n"
                "Your response MUST conform exactly to this JSON schema. "
                "Use these exact field names, types, and structure. "
                "Do NOT add extra fields or use a different structure.\n\n"
                f"```json\n{schema_text}\n```"
            )
            # Append to user content so it's the last thing the model sees
            req_body["contents"][0]["parts"][0]["text"] += schema_instruction
            req_body["generationConfig"] = {
                "temperature": 0.2,
            }
        else:
            req_body["generationConfig"] = {
                "temperature": 0.2,
            }

        url = (
            f"https://generativelanguage.googleapis.com/v1beta/models/"
            f"{self.model}:generateContent?key={self.api_key}"
        )

        resp = await self._client.post(url, json=req_body)

        if resp.status_code != 200:
            detail = ""
            try:
                detail = resp.json().get("error", {}).get("message", resp.text[:200])
            except Exception:
                detail = resp.text[:200]
            raise Exception(f"Gemini API error (status {resp.status_code}, model {self.model}): {detail}")

        data = resp.json()

        if "error" in data and data["error"]:
            raise Exception(f"Gemini error: {data['error'].get('message', 'unknown')}")

        candidates = data.get("candidates", [])
        if not candidates:
            log.error("Gemini empty candidates", raw_response=json.dumps(data)[:500])
            raise Exception("Empty response from Gemini")

        candidate = candidates[0]
        parts = candidate.get("content", {}).get("parts", [])
        if not parts:
            finish_reason = candidate.get("finishReason", "unknown")
            log.error("Gemini no parts", finish_reason=finish_reason, candidate=json.dumps(candidate)[:500])
            raise Exception(f"Empty response from Gemini (finishReason={finish_reason})")

        # Collect text from all parts (search grounding may split across multiple parts)
        text_parts = [p["text"] for p in parts if "text" in p]
        content = "\n".join(text_parts) if text_parts else ""
        if not content:
            log.error("Gemini no text in parts", parts=json.dumps(parts)[:500])
            raise Exception("No text content in Gemini response")

        grounding_meta = None
        gm = candidates[0].get("groundingMetadata")
        if gm:
            grounding_meta = gm

        return AnalyzeResult(content=content, grounding_meta=grounding_meta)


def extract_json(text: str) -> dict:
    """Extract a JSON object from AI response text.

    Handles: raw JSON, markdown code fences, text before/after JSON.
    Raises ValueError if no valid JSON found.
    """
    # 1) Try parsing the entire text as JSON directly
    stripped = text.strip()
    if stripped.startswith("{"):
        try:
            return json.loads(stripped)
        except json.JSONDecodeError:
            pass

    # 2) Try extracting from markdown code fences
    match = CODE_FENCE_RE.search(text)
    if match:
        candidate = match.group(1).strip()
        try:
            return json.loads(candidate)
        except json.JSONDecodeError:
            pass

    # 3) Find first { to last } (greedy brace matching)
    start = text.find("{")
    end = text.rfind("}")
    if start >= 0 and end > start:
        candidate = text[start : end + 1]
        try:
            return json.loads(candidate)
        except json.JSONDecodeError:
            pass

    raise ValueError(f"No valid JSON object found in AI response (length: {len(text)})")
