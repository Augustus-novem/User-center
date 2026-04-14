from __future__ import annotations

import logging
import re
from dataclasses import asdict, dataclass
from pathlib import Path
from uuid import NAMESPACE_URL, uuid4, uuid5

from qdrant_client import QdrantClient, models

from app.config import BASE_DIR, KnowledgeBaseConfig, QdrantConfig

logger = logging.getLogger(__name__)

SUPPORTED_EXTENSIONS = {".md", ".markdown", ".txt"}


@dataclass
class KnowledgeChunk:
    doc_id: str
    title: str
    source: str
    chunk_index: int
    content: str


@dataclass
class SearchHit:
    score: float
    doc_id: str
    title: str
    source: str
    chunk_index: int
    content: str


@dataclass
class DocumentRecord:
    doc_id: str
    title: str
    filename: str
    source: str
    chunk_count: int
    size_bytes: int


@dataclass
class UploadResult:
    document: DocumentRecord
    chunks: list[KnowledgeChunk]


def split_text(text: str, chunk_size: int, chunk_overlap: int) -> list[str]:
    text = text.strip()
    if not text:
        return []

    if chunk_size <= 0:
        raise ValueError("chunk_size must be greater than 0")

    if chunk_overlap < 0:
        raise ValueError("chunk_overlap must be greater than or equal to 0")

    if chunk_overlap >= chunk_size:
        raise ValueError("chunk_overlap must be less than chunk_size")

    chunks: list[str] = []
    start = 0
    text_len = len(text)

    while start < text_len:
        end = min(start + chunk_size, text_len)
        chunk = text[start:end].strip()
        if chunk:
            chunks.append(chunk)

        if end >= text_len:
            break

        start = end - chunk_overlap

    return chunks


def _iter_knowledge_files(knowledge_base_dir: Path) -> list[Path]:
    files: list[Path] = []
    for file_path in knowledge_base_dir.rglob("*"):
        if not file_path.is_file():
            continue
        if file_path.suffix.lower() not in SUPPORTED_EXTENSIONS:
            continue
        files.append(file_path)
    return sorted(files)


def _sanitize_filename(filename: str) -> str:
    cleaned = Path(filename).name.strip() or "document.txt"
    cleaned = re.sub(r"[^\w\-.\u4e00-\u9fff]+", "_", cleaned)
    return cleaned[:120] or "document.txt"


def _sanitize_doc_id(value: str) -> str:
    cleaned = value.strip().lower()
    cleaned = re.sub(r"[^a-z0-9\u4e00-\u9fff_-]+", "-", cleaned)
    cleaned = re.sub(r"-+", "-", cleaned).strip("-")
    return cleaned[:80] or "document"


def generate_upload_doc_id(filename: str) -> str:
    stem = _sanitize_doc_id(Path(filename).stem)
    return f"{stem}-{uuid4().hex[:8]}"


def build_source_path(knowledge_base_dir: Path, doc_id: str, filename: str) -> Path:
    uploads_dir = knowledge_base_dir / "uploads"
    uploads_dir.mkdir(parents=True, exist_ok=True)
    safe_name = _sanitize_filename(filename)
    return uploads_dir / f"{doc_id}__{safe_name}"


def parse_doc_id_from_path(file_path: Path) -> str:
    name = file_path.name
    if "__" in name:
        maybe_id, _ = name.split("__", 1)
        if maybe_id:
            return maybe_id
    return file_path.stem


def original_filename_from_path(file_path: Path) -> str:
    name = file_path.name
    if "__" in name:
        _, original_name = name.split("__", 1)
        return original_name
    return file_path.name


def parse_title_from_path(file_path: Path) -> str:
    return Path(original_filename_from_path(file_path)).stem


def create_chunks_for_document(
    doc_id: str,
    title: str,
    source: str,
    text: str,
    chunk_size: int,
    chunk_overlap: int,
) -> list[KnowledgeChunk]:
    file_chunks = split_text(text, chunk_size, chunk_overlap)
    return [
        KnowledgeChunk(
            doc_id=doc_id,
            title=title,
            source=source,
            chunk_index=idx,
            content=chunk,
        )
        for idx, chunk in enumerate(file_chunks)
    ]


def load_knowledge_base(
    knowledge_base_dir: Path,
    chunk_size: int,
    chunk_overlap: int,
) -> list[KnowledgeChunk]:
    if not knowledge_base_dir.exists():
        raise FileNotFoundError(
            f"knowledge base directory not found: {knowledge_base_dir}"
        )

    chunks: list[KnowledgeChunk] = []
    files = _iter_knowledge_files(knowledge_base_dir)

    logger.info(
        "start loading knowledge base, dir=%s, file_count=%d, chunk_size=%d, chunk_overlap=%d",
        knowledge_base_dir,
        len(files),
        chunk_size,
        chunk_overlap,
    )

    for file_path in files:
        content = file_path.read_text(encoding="utf-8").strip()
        doc_id = parse_doc_id_from_path(file_path)
        title = parse_title_from_path(file_path)
        file_chunks = create_chunks_for_document(
            doc_id=doc_id,
            title=title,
            source=str(file_path),
            text=content,
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap,
        )
        chunks.extend(file_chunks)

        logger.info(
            "loaded document, file=%s, doc_id=%s, chars=%d, chunks=%d",
            file_path.name,
            doc_id,
            len(content),
            len(file_chunks),
        )

    logger.info(
        "knowledge base loaded successfully, documents=%d, total_chunks=%d",
        len(files),
        len(chunks),
    )

    return chunks


def build_document_registry(
    knowledge_base_dir: Path,
    chunks: list[KnowledgeChunk],
) -> dict[str, DocumentRecord]:
    chunk_count_map: dict[str, int] = {}
    source_map: dict[str, str] = {}
    title_map: dict[str, str] = {}

    for chunk in chunks:
        chunk_count_map[chunk.doc_id] = chunk_count_map.get(chunk.doc_id, 0) + 1
        source_map[chunk.doc_id] = chunk.source
        title_map[chunk.doc_id] = chunk.title

    documents: dict[str, DocumentRecord] = {}
    for file_path in _iter_knowledge_files(knowledge_base_dir):
        doc_id = parse_doc_id_from_path(file_path)
        documents[doc_id] = DocumentRecord(
            doc_id=doc_id,
            title=title_map.get(doc_id, parse_title_from_path(file_path)),
            filename=original_filename_from_path(file_path),
            source=source_map.get(doc_id, str(file_path)),
            chunk_count=chunk_count_map.get(doc_id, 0),
            size_bytes=file_path.stat().st_size,
        )

    return documents


def prepare_uploaded_document(
    knowledge_base_dir: Path,
    knowledge_base_config: KnowledgeBaseConfig,
    filename: str,
    content_bytes: bytes,
) -> UploadResult:
    if not filename:
        raise ValueError("filename cannot be empty")

    suffix = Path(filename).suffix.lower()
    if suffix not in SUPPORTED_EXTENSIONS:
        raise ValueError(
            f"unsupported file type: {suffix or 'unknown'}, supported: {', '.join(sorted(SUPPORTED_EXTENSIONS))}"
        )

    if not content_bytes:
        raise ValueError("file content cannot be empty")

    try:
        text = content_bytes.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ValueError("file must be utf-8 encoded text") from exc

    doc_id = generate_upload_doc_id(filename)
    target_path = build_source_path(knowledge_base_dir, doc_id, filename)
    target_path.write_bytes(content_bytes)

    chunks = create_chunks_for_document(
        doc_id=doc_id,
        title=Path(filename).stem,
        source=str(target_path),
        text=text,
        chunk_size=knowledge_base_config.chunk_size,
        chunk_overlap=knowledge_base_config.chunk_overlap,
    )
    if not chunks:
        target_path.unlink(missing_ok=True)
        raise ValueError("uploaded document is empty after trimming")

    document = DocumentRecord(
        doc_id=doc_id,
        title=Path(filename).stem,
        filename=Path(filename).name,
        source=str(target_path),
        chunk_count=len(chunks),
        size_bytes=len(content_bytes),
    )
    return UploadResult(document=document, chunks=chunks)


def _resolve_cache_dir(path_str: str) -> str:
    cache_dir = Path(path_str)
    if not cache_dir.is_absolute():
        cache_dir = (BASE_DIR / cache_dir).resolve()
    cache_dir.mkdir(parents=True, exist_ok=True)
    return str(cache_dir)


def _chunk_point_id(chunk: KnowledgeChunk) -> str:
    return str(uuid5(NAMESPACE_URL, f"{chunk.source}:{chunk.chunk_index}"))


def _chunk_payload(chunk: KnowledgeChunk) -> dict:
    payload = asdict(chunk)
    payload["document"] = chunk.content
    return payload


def ensure_collection(client: QdrantClient, qdrant_config: QdrantConfig) -> None:
    if client.collection_exists(qdrant_config.collection_name):
        return

    client.create_collection(
        collection_name=qdrant_config.collection_name,
        vectors_config=models.VectorParams(
            size=client.get_embedding_size(qdrant_config.embedding_model),
            distance=models.Distance.COSINE,
        ),
    )


def build_qdrant_index(
    chunks: list[KnowledgeChunk],
    qdrant_config: QdrantConfig,
) -> QdrantClient:
    client = QdrantClient(
        host=qdrant_config.host,
        port=qdrant_config.port,
        timeout=qdrant_config.timeout,
    )

    client.set_model(
        qdrant_config.embedding_model,
        cache_dir=_resolve_cache_dir(qdrant_config.model_cache_dir),
    )

    if qdrant_config.recreate_on_startup:
        try:
            if client.collection_exists(qdrant_config.collection_name):
                client.delete_collection(qdrant_config.collection_name)
                logger.info(
                    "deleted existing qdrant collection, collection=%s",
                    qdrant_config.collection_name,
                )
        except Exception:
            logger.info(
                "skip deleting qdrant collection, collection may not exist yet, collection=%s",
                qdrant_config.collection_name,
            )

    ensure_collection(client, qdrant_config)
    upsert_chunks(client, qdrant_config, chunks)

    count_result = client.count(
        collection_name=qdrant_config.collection_name,
        exact=True,
    )

    logger.info(
        "qdrant index ready, collection=%s, points=%d, embedding_model=%s",
        qdrant_config.collection_name,
        count_result.count,
        qdrant_config.embedding_model,
    )

    return client


def upsert_chunks(
    client: QdrantClient,
    qdrant_config: QdrantConfig,
    chunks: list[KnowledgeChunk],
) -> None:
    if not chunks:
        return

    ensure_collection(client, qdrant_config)
    client.upload_collection(
        collection_name=qdrant_config.collection_name,
        vectors=[
            models.Document(
                text=chunk.content,
                model=qdrant_config.embedding_model,
            )
            for chunk in chunks
        ],
        payload=[_chunk_payload(chunk) for chunk in chunks],
        ids=[_chunk_point_id(chunk) for chunk in chunks],
        batch_size=qdrant_config.batch_size,
        wait=True,
    )


def delete_document_points(
    client: QdrantClient,
    qdrant_config: QdrantConfig,
    doc_id: str,
) -> int:
    doc_filter = models.Filter(
        must=[
            models.FieldCondition(
                key="doc_id",
                match=models.MatchValue(value=doc_id),
            )
        ]
    )
    count_result = client.count(
        collection_name=qdrant_config.collection_name,
        count_filter=doc_filter,
        exact=True,
    )
    client.delete(
        collection_name=qdrant_config.collection_name,
        points_selector=models.FilterSelector(filter=doc_filter),
        wait=True,
    )
    return int(count_result.count)


def search_knowledge(
    client: QdrantClient,
    qdrant_config: QdrantConfig,
    query: str,
    top_k: int,
) -> list[SearchHit]:
    if not query.strip():
        raise ValueError("query cannot be empty")

    result = client.query_points(
        collection_name=qdrant_config.collection_name,
        query=models.Document(
            text=query,
            model=qdrant_config.embedding_model,
        ),
        limit=top_k,
        with_payload=True,
    )

    hits: list[SearchHit] = []
    filtered_count = 0
    for point in result.points:
        score = float(point.score)
        if score < qdrant_config.min_score:
            filtered_count += 1
            continue

        payload = point.payload or {}
        hits.append(
            SearchHit(
                score=score,
                doc_id=str(payload.get("doc_id", "")),
                title=str(payload.get("title", "")),
                source=str(payload.get("source", "")),
                chunk_index=int(payload.get("chunk_index", 0)),
                content=str(payload.get("content", "")),
            )
        )

    logger.info(
        "knowledge search completed, query=%s, requested_top_k=%d, returned_hits=%d, filtered_hits=%d, min_score=%.4f",
        query,
        top_k,
        len(hits),
        filtered_count,
        qdrant_config.min_score,
    )

    return hits
