FROM python:3.11

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

COPY rag_service/requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt

COPY rag_service /app

RUN mkdir -p /app/.cache/fastembed /app/knowledge_base

EXPOSE 18081

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "18081"]