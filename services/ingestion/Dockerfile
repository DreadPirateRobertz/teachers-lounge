FROM python:3.12

WORKDIR /app

# Install system deps for PDF processing (poppler for pdftotext, tesseract for OCR)
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev \
    poppler-utils \
    tesseract-ocr \
    libmagic1 \
    ffmpeg \
    libxml2-dev libxslt1-dev \
    cmake \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app/ ./app/

# Non-root user
RUN useradd --no-create-home --shell /bin/false appuser
USER appuser

EXPOSE 8080

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080", "--workers", "2"]
