# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" \
    -o notification-service ./cmd/server

# Use a minimal base instead of scratch so wget is available for HEALTHCHECK
FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /app/notification-service /notification-service

EXPOSE 9000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:9000/health || exit 1

ENTRYPOINT ["/notification-service"]
