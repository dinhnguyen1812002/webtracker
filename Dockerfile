# Multi-stage build for minimal image size
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Install templ for template compilation
RUN go install github.com/a-h/templ/cmd/templ@latest

# Generate templ templates
RUN templ generate

# Build the application with optimizations for production
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o main ./cmd/main.go

# Final stage - minimal runtime image
FROM scratch

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /app/main /main

# Copy configuration files
COPY --from=builder /app/config.docker.yaml /config.yaml
COPY --from=builder /app/config.dev.yaml /config.dev.yaml

# Copy migration files
COPY --from=builder /app/infrastructure/postgres/migrations /migrations

# Copy static files
COPY --from=builder /app/static /static

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/main", "-health-check"] || exit 1

# Run the application
ENTRYPOINT ["/main"]