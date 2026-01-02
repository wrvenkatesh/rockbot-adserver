# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies for CGO (required for sqlite3)
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=1 is required for sqlite3
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o adserver ./cmd/server

# Final stage
FROM alpine:latest

# Install runtime dependencies for sqlite3 and wget for healthcheck
RUN apk --no-cache add ca-certificates sqlite wget

# Create app directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/adserver .

# Copy web templates and static files
COPY --from=builder /build/web ./web

# Create directory for database (will be mounted as volume in production)
RUN mkdir -p /app/data

# Expose port
EXPOSE 8080

# Set environment variable for database path
ENV DB_PATH=/app/data/adserver.db

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/login || exit 1

# Run the application
CMD ["./adserver"]

