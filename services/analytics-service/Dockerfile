FROM python:3.12-slim

WORKDIR /service

# tl-logging is copied here by CI before docker build (see ci.yml docker-build job)
COPY tl-logging/ ./tl-logging/
COPY requirements.txt .
# Install shared logging package first (local path ../tl-logging works for
# local dev; Docker uses ./tl-logging via COPY above).
RUN pip install --no-cache-dir ./tl-logging
RUN pip install --no-cache-dir -r requirements.txt

COPY app/ ./app/

EXPOSE 8085

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8085", "--workers", "1"]
