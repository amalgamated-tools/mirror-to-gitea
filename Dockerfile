# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o mirror-to-gitea .

# Production stage
FROM alpine:latest AS production

# Add Docker Alpine packages and remove cache in the same layer
RUN apk --no-cache add ca-certificates tini && \
    rm -rf /var/cache/apk/*

# Set non-root user for better security
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

USER appuser

# Set working directory owned by appuser
WORKDIR /app

# Copy only the built application and entry point from builder
COPY --from=builder --chown=appuser:appuser /app/mirror-to-gitea .
COPY --chown=appuser:appuser docker-entrypoint.sh .

# Make entry point executable
USER root
RUN chmod +x /app/docker-entrypoint.sh
USER appuser

# Set environment to production to disable development features
ENV NODE_ENV=production

# Use tini as init system to properly handle signals
ENTRYPOINT ["/sbin/tini", "--"]

# The command to run
CMD [ "/app/docker-entrypoint.sh" ]
