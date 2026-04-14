import logging
from contextlib import asynccontextmanager
from pathlib import Path

import uvicorn
from fastapi import FastAPI, File, HTTPException, UploadFile
from pydantic import BaseModel, Field

from app.config import BASE_DIR, settings
from app.knowledge_base import (
    build_document_registry,
    build_qdrant_index,
    delete_document_points,
    load_knowledge_base,
    prepare_uploaded_document,
    search_knowledge,
    upsert_chunks,
)
from app.qa_service import answer_question


logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)s | %(name)s | %(message)s",
)

logger = logging.getLogger(__name__)


class KnowledgeSearchRequest(BaseModel):
    query: str = Field(..., description="检索问题")
    top_k: int | None = Field(default=None, ge=1, le=20, description="返回条数")


class AskRequest(BaseModel):
    query: str = Field(..., description="用户问题")
    top_k: int | None = Field(default=None, ge=1, le=20, description="召回片段数")
    use_llm: bool = Field(default=True, description="是否优先使用外部 LLM")


@asynccontextmanager
async def lifespan(app: FastAPI):
    kb_dir = Path(settings.knowledge_base.dir)
    if not kb_dir.is_absolute():
        kb_dir = (BASE_DIR / kb_dir).resolve()
    kb_dir.mkdir(parents=True, exist_ok=True)

    chunks = load_knowledge_base(
        knowledge_base_dir=kb_dir,
        chunk_size=settings.knowledge_base.chunk_size,
        chunk_overlap=settings.knowledge_base.chunk_overlap,
    )
    client = build_qdrant_index(chunks, settings.qdrant)
    documents = build_document_registry(kb_dir, chunks)

    app.state.knowledge_base_dir = kb_dir
    app.state.knowledge_chunks = chunks
    app.state.documents = documents
    app.state.qdrant_client = client

    logger.info(
        "rag_service startup completed, knowledge_chunks=%d, documents=%d, qdrant_collection=%s",
        len(chunks),
        len(documents),
        settings.qdrant.collection_name,
    )
    yield


app = FastAPI(
    title=settings.server.name,
    lifespan=lifespan,
)


@app.get("/health")
def health() -> dict:
    return {
        "status": "ok",
        "service": settings.server.name,
        "mode": settings.server.mode,
        "collection": settings.qdrant.collection_name,
        "embedding_model": settings.qdrant.embedding_model,
        "llm_enabled": settings.llm.enabled,
        "llm_model": settings.llm.model,
        "document_count": len(getattr(app.state, "documents", {})),
    }


@app.post("/knowledge/search")
def knowledge_search(req: KnowledgeSearchRequest) -> dict:
    query = req.query.strip()
    if not query:
        raise HTTPException(status_code=400, detail="query cannot be empty")

    top_k = req.top_k or settings.qdrant.top_k
    hits = search_knowledge(
        client=app.state.qdrant_client,
        qdrant_config=settings.qdrant,
        query=query,
        top_k=top_k,
    )

    return {
        "query": query,
        "top_k": top_k,
        "hits": [
            {
                "score": hit.score,
                "doc_id": hit.doc_id,
                "title": hit.title,
                "source": hit.source,
                "chunk_index": hit.chunk_index,
                "content": hit.content,
            }
            for hit in hits
        ],
    }


@app.post("/ask")
def ask(req: AskRequest) -> dict:
    query = req.query.strip()
    if not query:
        raise HTTPException(status_code=400, detail="query cannot be empty")

    top_k = req.top_k or settings.qdrant.top_k
    hits = search_knowledge(
        client=app.state.qdrant_client,
        qdrant_config=settings.qdrant,
        query=query,
        top_k=top_k,
    )

    result = answer_question(
        query=query,
        hits=hits,
        llm_config=settings.llm,
        use_llm=req.use_llm,
    )

    return {
        "query": query,
        "top_k": top_k,
        "mode": result.mode,
        "answer": result.answer,
        "references": [
            {
                "doc_id": ref.doc_id,
                "title": ref.title,
                "source": ref.source,
                "chunk_index": ref.chunk_index,
                "score": ref.score,
                "content": ref.content,
            }
            for ref in result.references
        ],
    }


@app.post("/knowledge/upload")
async def knowledge_upload(file: UploadFile = File(...)) -> dict:
    if not file.filename:
        raise HTTPException(status_code=400, detail="filename cannot be empty")

    content = await file.read()
    try:
        upload_result = prepare_uploaded_document(
            knowledge_base_dir=app.state.knowledge_base_dir,
            knowledge_base_config=settings.knowledge_base,
            filename=file.filename,
            content_bytes=content,
        )
        upsert_chunks(
            client=app.state.qdrant_client,
            qdrant_config=settings.qdrant,
            chunks=upload_result.chunks,
        )
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except Exception as exc:  # noqa: BLE001
        logger.exception("upload knowledge failed: %s", exc)
        if "upload_result" in locals():
            Path(upload_result.document.source).unlink(missing_ok=True)
        raise HTTPException(status_code=500, detail="upload knowledge failed") from exc

    app.state.knowledge_chunks.extend(upload_result.chunks)
    app.state.documents[upload_result.document.doc_id] = upload_result.document

    return {
        "message": "document uploaded and indexed successfully",
        "document": {
            "doc_id": upload_result.document.doc_id,
            "title": upload_result.document.title,
            "filename": upload_result.document.filename,
            "source": upload_result.document.source,
            "chunk_count": upload_result.document.chunk_count,
            "size_bytes": upload_result.document.size_bytes,
        },
    }


@app.get("/documents")
def list_documents() -> dict:
    documents = sorted(
        app.state.documents.values(),
        key=lambda item: item.doc_id,
    )
    return {
        "total": len(documents),
        "documents": [
            {
                "doc_id": doc.doc_id,
                "title": doc.title,
                "filename": doc.filename,
                "source": doc.source,
                "chunk_count": doc.chunk_count,
                "size_bytes": doc.size_bytes,
            }
            for doc in documents
        ],
    }


@app.delete("/documents/{doc_id}")
def delete_document(doc_id: str) -> dict:
    document = app.state.documents.get(doc_id)
    if document is None:
        raise HTTPException(status_code=404, detail="document not found")

    try:
        removed_points = delete_document_points(
            client=app.state.qdrant_client,
            qdrant_config=settings.qdrant,
            doc_id=doc_id,
        )
    except Exception as exc:  # noqa: BLE001
        logger.exception("delete document points failed: %s", exc)
        raise HTTPException(status_code=500, detail="delete document failed") from exc

    Path(document.source).unlink(missing_ok=True)
    app.state.documents.pop(doc_id, None)
    app.state.knowledge_chunks = [
        chunk for chunk in app.state.knowledge_chunks if chunk.doc_id != doc_id
    ]

    return {
        "message": "document deleted successfully",
        "doc_id": doc_id,
        "removed_points": removed_points,
        "document": {
            "doc_id": document.doc_id,
            "title": document.title,
            "filename": document.filename,
            "source": document.source,
            "chunk_count": document.chunk_count,
            "size_bytes": document.size_bytes,
        },
    }


if __name__ == "__main__":
    uvicorn.run(
        "app.main:app",
        host=settings.server.host,
        port=settings.server.port,
        reload=settings.server.mode == "dev",
    )
