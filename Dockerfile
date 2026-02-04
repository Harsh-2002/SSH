# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o ssh-mcp \
    ./cmd/server

# Runtime stage - distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary
COPY --from=builder /build/ssh-mcp /ssh-mcp

# Create data directory (for SSH keys)
VOLUME ["/data"]

# Default to HTTP mode
ENV SSH_MCP_MODE=http
EXPOSE 8000

ENTRYPOINT ["/ssh-mcp"]
CMD ["-mode", "http", "-port", "8000"]
