# Build stage
FROM golang:1.23.10 AS builder

# Security: Set non-root user for build
RUN useradd -u 10001 -m builder
USER builder

ENV GOOS=linux
ENV CGO_ENABLED=0

WORKDIR /app

# Copy dependency files first for better caching
COPY --chown=builder:builder go.mod go.sum ./
RUN make deps

# Copy source code
COPY --chown=builder:builder . .

# Run security checks and tests
RUN make test

# Build the application
RUN make build

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

# Security labels
LABEL \
    org.opencontainers.image.title="promgithub" \
    org.opencontainers.image.description="GitHub webhook handler for Prometheus metrics" \
    org.opencontainers.image.vendor="darthfork" \
    security.non-root="true" \
    security.no-shell="true"

# Use distroless nonroot user (uid=65532, gid=65532)
USER nonroot:nonroot

WORKDIR /app

# Copy only the necessary binary
COPY --from=builder --chown=nonroot:nonroot /app/build/promgithub /app/promgithub

# Security: Run on non-privileged port
EXPOSE 8080

# Security: Use exec form to avoid shell
CMD ["/app/promgithub"]
