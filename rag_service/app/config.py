from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml


BASE_DIR = Path(__file__).resolve().parent.parent
DEFAULT_CONFIG_PATH = BASE_DIR / "config" / "config.yaml"


@dataclass
class ServerConfig:
    name: str = "rag_service"
    host: str = "0.0.0.0"
    port: int = 18081
    mode: str = "dev"


@dataclass
class KnowledgeBaseConfig:
    dir: str = "knowledge_base"
    chunk_size: int = 200
    chunk_overlap: int = 50


@dataclass
class QdrantConfig:
    host: str = "127.0.0.1"
    port: int = 6333
    collection_name: str = "knowledge_chunks"
    embedding_model: str = "BAAI/bge-small-zh-v1.5"
    top_k: int = 5
    min_score: float = 0.5
    batch_size: int = 64
    timeout: int = 10
    recreate_on_startup: bool = False
    model_cache_dir: str = ".cache/fastembed"




@dataclass
class LLMConfig:
    enabled: bool = False
    base_url: str = ""
    api_key: str = ""
    model: str = ""
    timeout: int = 30
    temperature: float = 0.2
    max_tokens: int = 512


@dataclass
class AppConfig:
    server: ServerConfig
    knowledge_base: KnowledgeBaseConfig
    qdrant: QdrantConfig
    llm: LLMConfig


def _load_yaml(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}

    with path.open("r", encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}

    if not isinstance(data, dict):
        raise ValueError("config file must be a yaml mapping")

    return data


def _get_env_int(name: str, default: int) -> int:
    value = os.getenv(name)
    if value is None or value == "":
        return default
    return int(value)


def _get_env_bool(name: str, default: bool) -> bool:
    value = os.getenv(name)
    if value is None or value == "":
        return default
    return value.lower() in {"1", "true", "yes", "y", "on"}


def _get_env_float(name: str, default: float) -> float:
    value = os.getenv(name)
    if value is None or value == "":
        return default
    return float(value)


def load_config() -> AppConfig:
    config_path = Path(
        os.getenv("RAG_SERVICE_CONFIG", str(DEFAULT_CONFIG_PATH))
    ).resolve()

    raw = _load_yaml(config_path)

    server_raw = raw.get("server", {}) if isinstance(raw.get("server", {}), dict) else {}
    kb_raw = (
        raw.get("knowledge_base", {})
        if isinstance(raw.get("knowledge_base", {}), dict)
        else {}
    )
    qdrant_raw = (
        raw.get("qdrant", {})
        if isinstance(raw.get("qdrant", {}), dict)
        else {}
    )
    llm_raw = raw.get("llm", {}) if isinstance(raw.get("llm", {}), dict) else {}

    server = ServerConfig(
        name=os.getenv("RAG_SERVICE_NAME", server_raw.get("name", "rag_service")),
        host=os.getenv("RAG_SERVICE_HOST", server_raw.get("host", "0.0.0.0")),
        port=_get_env_int("RAG_SERVICE_PORT", int(server_raw.get("port", 18081))),
        mode=os.getenv("RAG_SERVICE_MODE", server_raw.get("mode", "dev")),
    )

    knowledge_base = KnowledgeBaseConfig(
        dir=kb_raw.get("dir", "knowledge_base"),
        chunk_size=int(kb_raw.get("chunk_size", 200)),
        chunk_overlap=int(kb_raw.get("chunk_overlap", 50)),
    )

    qdrant = QdrantConfig(
        host=os.getenv("QDRANT_HOST", qdrant_raw.get("host", "127.0.0.1")),
        port=_get_env_int("QDRANT_PORT", int(qdrant_raw.get("port", 6333))),
        collection_name=os.getenv(
            "QDRANT_COLLECTION_NAME",
            qdrant_raw.get("collection_name", "knowledge_chunks"),
        ),
        embedding_model=os.getenv(
            "QDRANT_EMBEDDING_MODEL",
            qdrant_raw.get("embedding_model", "BAAI/bge-small-zh-v1.5"),
        ),
        top_k=_get_env_int("QDRANT_TOP_K", int(qdrant_raw.get("top_k", 5))),
        min_score=_get_env_float("QDRANT_MIN_SCORE", float(qdrant_raw.get("min_score", 0.5))),
        batch_size=_get_env_int(
            "QDRANT_BATCH_SIZE",
            int(qdrant_raw.get("batch_size", 64)),
        ),
        timeout=_get_env_int("QDRANT_TIMEOUT", int(qdrant_raw.get("timeout", 10))),
        recreate_on_startup=_get_env_bool(
            "QDRANT_RECREATE_ON_STARTUP",
            bool(qdrant_raw.get("recreate_on_startup", False)),
        ),
        model_cache_dir=os.getenv(
            "QDRANT_MODEL_CACHE_DIR",
            qdrant_raw.get("model_cache_dir", ".cache/fastembed"),
        ),
    )

    llm = LLMConfig(
        enabled=_get_env_bool("LLM_ENABLED", bool(llm_raw.get("enabled", False))),
        base_url=os.getenv("LLM_BASE_URL", str(llm_raw.get("base_url", ""))),
        api_key=os.getenv("LLM_API_KEY", str(llm_raw.get("api_key", ""))),
        model=os.getenv("LLM_MODEL", str(llm_raw.get("model", ""))),
        timeout=_get_env_int("LLM_TIMEOUT", int(llm_raw.get("timeout", 30))),
        temperature=_get_env_float("LLM_TEMPERATURE", float(llm_raw.get("temperature", 0.2))),
        max_tokens=_get_env_int("LLM_MAX_TOKENS", int(llm_raw.get("max_tokens", 512))),
    )

    return AppConfig(server=server, knowledge_base=knowledge_base, qdrant=qdrant, llm=llm)


settings = load_config()