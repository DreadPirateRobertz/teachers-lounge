FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o notification-service ./cmd/server

# ---- runtime ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /app/notification-service /notification-service

EXPOSE 8084

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8084/health || exit 1

ENTRYPOINT ["/notification-service"]
