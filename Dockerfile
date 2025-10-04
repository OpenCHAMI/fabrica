# SPDX-FileCopyrightText: Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT
#
# Dockerfile for Fabrica
# Used by GoReleaser - binary is pre-built

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

# Copy pre-built binary from GoReleaser
COPY fabrica /usr/local/bin/fabrica

# Note: GoReleaser only copies the binary by default
# Additional files must be explicitly included in the build context

# Set ownership
RUN chown -R fabrica:fabrica /home/fabrica

# Switch to non-root user
USER fabrica

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/fabrica"]
CMD ["--help"]
