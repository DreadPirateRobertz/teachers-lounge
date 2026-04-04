# LiteLLM AI Gateway
# Using the official LiteLLM image — no custom build needed for Phase 1
FROM ghcr.io/berriai/litellm:main-latest

WORKDIR /app
COPY config.yaml ./config.yaml

EXPOSE 4000
HEALTHCHECK --interval=30s --timeout=10s --start-period=20s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:4000/health/liveliness')"

CMD ["--config", "/app/config.yaml", "--port", "4000", "--detailed_debug"]
