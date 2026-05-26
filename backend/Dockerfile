# ── Stage 1: Build ─────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# Install git and ca-certificates (needed for go get over HTTPS).
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Cache dependency downloads separately from source compilation.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and compile.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /build/server ./cmd/server

# ── Stage 2: Runtime ────────────────────────────────────────────────────────
FROM alpine:3.19 AS runner

RUN apk add --no-cache ca-certificates tzdata

# Non-root user for security.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /build/server ./server

RUN chown appuser:appgroup /app/server
USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/server"]
