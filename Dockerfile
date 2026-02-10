# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version injection
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown

# Build the application
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s \
    -X git.uhomes.net/uhs-go/wechat-subscription-svc/internal/version.Version=${VERSION} \
    -X git.uhomes.net/uhs-go/wechat-subscription-svc/internal/version.BuildTime=${BUILD_TIME} \
    -X git.uhomes.net/uhs-go/wechat-subscription-svc/internal/version.GitCommit=${GIT_COMMIT}" \
    -o /app/bin/server ./cmd/server

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Set timezone
ENV TZ=Asia/Shanghai

# Create non-root user
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/server /app/server

# Copy config files
COPY --from=builder /app/configs /app/configs

# Copy web files
COPY --from=builder /app/web /app/web

# Copy docs
COPY --from=builder /app/docs /app/docs

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8090 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/ || exit 1

# Run the application
CMD ["/app/server"]
