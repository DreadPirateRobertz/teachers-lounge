FROM golang:1.23-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /user-service ./cmd/server

# ── Runtime image ────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12 AS runtime

COPY --from=builder /user-service /user-service

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/user-service"]
