# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# gcc + musl required for CGO (blevesearch/go-faiss, bbolt mmap)
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev

WORKDIR /app

# Cache module downloads separately from source
COPY go.mod go.sum ./
RUN go mod download

# Copy source and compile
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /app/bin/server ./cmd/server

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.21 AS runtime

RUN apk add --no-cache ca-certificates tzdata

# Non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Binary + config
COPY --from=builder /app/bin/server          ./server
COPY --from=builder /app/config/config.yaml  ./config/config.yaml

# Bleve file-backed index storage (Phase 5)
RUN mkdir -p /app/data/bleve && chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

ENTRYPOINT ["./server"]
