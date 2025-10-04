# SPDX-FileCopyrightText: Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT
# Dockerfile for Fabrica
# Multi-stage build for minimal final image

# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o fabrica \
    ./cmd/fabrica

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    git \
    bash

# Create non-root user
RUN addgroup -g 1000 fabrica && \
    adduser -D -u 1000 -G fabrica fabrica

WORKDIR /home/fabrica

# Copy binary from builder
COPY --from=builder /build/fabrica /usr/local/bin/fabrica

# Copy templates and documentation
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/docs ./docs
COPY --from=builder /build/examples ./examples
COPY --from=builder /build/README.md ./README.md
COPY --from=builder /build/LICENSE ./LICENSE

# Set ownership
RUN chown -R fabrica:fabrica /home/fabrica

# Switch to non-root user
USER fabrica

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/fabrica"]
CMD ["--help"]
