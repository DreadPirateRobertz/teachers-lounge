FROM golang:1.25-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /user-service ./cmd/server

# ── Runtime image ────────────────────────────────────────────
FROM alpine:3.19 AS runtime

RUN adduser -D -u 10001 nonroot
COPY --from=builder /user-service /user-service

EXPOSE 8080
USER nonroot

ENTRYPOINT ["/user-service"]
