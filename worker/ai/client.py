from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional, Protocol


@dataclass
class AnalyzeOptions:
    system_instruction: str = ""
    user_content: str = ""
    response_schema: Optional[dict] = None
    enable_search: bool = False


@dataclass
class AnalyzeResult:
    content: str = ""
    grounding_meta: Optional[dict] = None


class AIClient(Protocol):
    async def analyze(self, prompt: str) -> str: ...
    async def analyze_structured(self, opts: AnalyzeOptions) -> AnalyzeResult: ...


def create_ai_client(provider: str, api_key: str = "", model: str = "gemini-2.0-flash", search_grounding: bool = True) -> Optional[AIClient]:
    if provider == "gemini":
        if not api_key:
            return None

        from ai.gemini_api import GeminiAPIClient
        return GeminiAPIClient(
            api_key=api_key,
            model=model,
            search_grounding=search_grounding,
        )

    return None
