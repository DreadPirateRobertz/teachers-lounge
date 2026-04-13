# ── Builder ───────────────────────────────────────────────────────────────────
# Full image provides cmake/gcc needed to compile onnx, lxml, asyncpg, etc.
FROM python:3.12 AS builder

WORKDIR /build

# Build-time system deps (not carried into the runtime layer).
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev \
    libxml2-dev \
    libxslt1-dev \
    cmake \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# tl-logging is copied here by CI before docker build (see ci.yml docker-build job)
COPY tl-logging/ ./tl-logging/
COPY requirements.txt .

# Pre-install modern onnx wheel so transitive deps (effdet via
# unstructured-inference) don't fall back to onnx==1.10.0 whose sdist
# setup.py breaks with "Unable to find requirements.txt".
RUN pip install --no-cache-dir setuptools wheel grpcio-tools \
    "onnx>=1.16.0" "onnxruntime>=1.18.0"

RUN printf "onnx>=1.16.0\nonnxruntime>=1.18.0\n" > /tmp/constraints.txt

# Install shared logging package first (path reference in requirements.txt
# uses ../tl-logging for local dev; Docker uses ./tl-logging via COPY above).
RUN pip install --no-cache-dir ./tl-logging

# Install all Python deps into the system site-packages so they can be
# cleanly COPY'd into the slim runtime stage.
RUN pip install --no-cache-dir --no-build-isolation --prefer-binary \
    -c /tmp/constraints.txt -r requirements.txt

# ── Runtime ───────────────────────────────────────────────────────────────────
FROM python:3.12-slim AS runtime

WORKDIR /app

# Runtime-only system deps — no build tools, no *-dev headers.
# - poppler-utils : pdftotext / pdf2image for PDF extraction
# - tesseract-ocr : OCR for image-in-PDF / image processor
# - libmagic1     : python-magic file type detection
# - ffmpeg        : audio/video transcription pipeline
# - libxml2 / libxslt1.1 : lxml runtime (headers not needed at runtime)
# - libpq5        : asyncpg PostgreSQL client runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
    poppler-utils \
    tesseract-ocr \
    libmagic1 \
    ffmpeg \
    libxml2 \
    libxslt1.1 \
    libpq5 \
    && rm -rf /var/lib/apt/lists/*

# Copy the compiled Python packages from the builder stage.
COPY --from=builder /usr/local/lib/python3.12/site-packages \
                    /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

COPY app/ ./app/

# Non-root user
RUN useradd --no-create-home --shell /bin/false appuser
USER appuser

EXPOSE 8080

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080", "--workers", "2"]
