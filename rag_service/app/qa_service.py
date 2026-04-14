from __future__ import annotations

import json
import logging
import re
from dataclasses import dataclass
from typing import Iterable
from urllib import error, request

from app.config import LLMConfig
from app.knowledge_base import SearchHit

logger = logging.getLogger(__name__)

_DEFAULT_SYSTEM_PROMPT = (
    "你是固定知识库问答助手。"
    "你只能依据给定的知识库上下文回答，不允许编造。"
    "如果上下文不足以回答，请明确说明“根据当前知识库无法确认”。"
    "回答请使用中文，尽量简洁，并优先给出结论。"
)


@dataclass
class AskReference:
    doc_id: str
    title: str
    source: str
    chunk_index: int
    score: float
    content: str


@dataclass
class AskResult:
    answer: str
    references: list[AskReference]
    mode: str
    context: str


def _clean_text(text: str) -> str:
    cleaned = text.replace("\r", "\n")
    cleaned = re.sub(r"^#+\s*", "", cleaned, flags=re.MULTILINE)
    cleaned = re.sub(r"^\s*[-*]\s+", "", cleaned, flags=re.MULTILINE)
    cleaned = re.sub(r"^\s*\d+[.)、]\s*", "", cleaned, flags=re.MULTILINE)
    cleaned = re.sub(r"\n{2,}", "\n", cleaned)
    return cleaned.strip()


def build_context(hits: Iterable[SearchHit], max_context_chars: int = 4000) -> str:
    parts: list[str] = []
    current_len = 0

    for idx, hit in enumerate(hits, start=1):
        block = (
            f"[参考片段 {idx}]\n"
            f"标题: {hit.title}\n"
            f"文档ID: {hit.doc_id}\n"
            f"分片序号: {hit.chunk_index}\n"
            f"内容:\n{hit.content.strip()}\n"
        )
        if current_len + len(block) > max_context_chars:
            break
        parts.append(block)
        current_len += len(block)

    return "\n".join(parts).strip()


def build_references(hits: Iterable[SearchHit]) -> list[AskReference]:
    references: list[AskReference] = []
    for hit in hits:
        references.append(
            AskReference(
                doc_id=hit.doc_id,
                title=hit.title,
                source=hit.source,
                chunk_index=hit.chunk_index,
                score=hit.score,
                content=hit.content,
            )
        )
    return references


def llm_is_configured(config: LLMConfig) -> bool:
    return bool(config.enabled and config.base_url.strip() and config.model.strip())


def _no_hit_answer() -> str:
    return "根据当前知识库未检索到达到阈值的相关内容，暂时无法回答该问题。"


def _fallback_refusal_answer() -> str:
    return "已检索到相关片段，但当前未使用可用的 LLM 生成答案。为避免基于片段硬答，暂不直接给出结论，请结合参考片段人工确认。"


def _call_openai_compatible_llm(
    config: LLMConfig,
    query: str,
    context: str,
) -> str:
    endpoint = f"{config.base_url.rstrip('/')}/chat/completions"
    payload = {
        "model": config.model,
        "temperature": config.temperature,
        "max_tokens": config.max_tokens,
        "messages": [
            {"role": "system", "content": _DEFAULT_SYSTEM_PROMPT},
            {
                "role": "user",
                "content": (
                    f"问题：{query}\n\n"
                    f"知识库上下文：\n{context}\n\n"
                    "请基于以上上下文回答，并在答案结尾简要说明依据来自哪些片段。"
                ),
            },
        ],
    }

    headers = {
        "Content-Type": "application/json",
    }
    if config.api_key.strip():
        headers["Authorization"] = f"Bearer {config.api_key.strip()}"

    req = request.Request(
        endpoint,
        data=json.dumps(payload).encode("utf-8"),
        headers=headers,
        method="POST",
    )

    try:
        with request.urlopen(req, timeout=config.timeout) as resp:
            body = resp.read().decode("utf-8")
    except error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="ignore")
        raise RuntimeError(
            f"llm request failed, status={exc.code}, detail={detail}"
        ) from exc
    except error.URLError as exc:
        raise RuntimeError(f"llm request failed: {exc}") from exc

    try:
        data = json.loads(body)
        return data["choices"][0]["message"]["content"].strip()
    except (KeyError, IndexError, TypeError, json.JSONDecodeError) as exc:
        raise RuntimeError(f"invalid llm response: {body}") from exc


def answer_question(
    query: str,
    hits: list[SearchHit],
    llm_config: LLMConfig,
    use_llm: bool = True,
    max_context_chars: int = 4000,
) -> AskResult:
    references = build_references(hits)
    context = build_context(hits, max_context_chars=max_context_chars)

    if not hits:
        return AskResult(
            answer=_no_hit_answer(),
            references=[],
            mode="no_hit",
            context="",
        )

    if use_llm and llm_is_configured(llm_config):
        try:
            answer = _call_openai_compatible_llm(llm_config, query=query, context=context)
            return AskResult(
                answer=answer,
                references=references,
                mode="llm",
                context=context,
            )
        except Exception as exc:  # noqa: BLE001
            logger.exception("llm answering failed, fallback to extractive mode: %s", exc)

    return AskResult(
        answer=_fallback_refusal_answer(),
        references=references,
        mode="fallback_refusal",
        context=context,
    )
