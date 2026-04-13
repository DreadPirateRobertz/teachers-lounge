FROM python:3.12-slim

WORKDIR /service

# Install deps first for layer caching
# tl-logging is copied here by CI before docker build (see ci.yml docker-build job)
COPY tl-logging/ ./tl-logging/
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app/ ./app/

EXPOSE 8080

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080", "--workers", "1"]
