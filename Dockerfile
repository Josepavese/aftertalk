# Build stage
FROM golang:1.25.8-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG AFTERTALK_COMMIT=local
ARG AFTERTALK_TAG=dev
ARG AFTERTALK_BUILD_TIME=
ARG AFTERTALK_BUILD_SOURCE=docker
RUN LDFLAGS="-s -w" && \
    LDFLAGS="${LDFLAGS} -X github.com/Josepavese/aftertalk/internal/version.Commit=${AFTERTALK_COMMIT}" && \
    LDFLAGS="${LDFLAGS} -X github.com/Josepavese/aftertalk/internal/version.Tag=${AFTERTALK_TAG}" && \
    LDFLAGS="${LDFLAGS} -X github.com/Josepavese/aftertalk/internal/version.BuildTime=${AFTERTALK_BUILD_TIME}" && \
    LDFLAGS="${LDFLAGS} -X github.com/Josepavese/aftertalk/internal/version.BuildSource=${AFTERTALK_BUILD_SOURCE}" && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="${LDFLAGS}" -o /aftertalk ./cmd/aftertalk

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

RUN adduser -D -u 1000 aftertalk && \
    mkdir -p /opt/aftertalk /var/lib/aftertalk && \
    chown -R aftertalk:aftertalk /opt/aftertalk /var/lib/aftertalk

WORKDIR /opt/aftertalk

# Copy binary from builder
COPY --from=builder /aftertalk /opt/aftertalk/aftertalk

# Default writable DB location for containerized runtime
ENV AFTERTALK_DATABASE_PATH=/var/lib/aftertalk/aftertalk.db

# Create non-root user
USER aftertalk

# Expose ports
EXPOSE 8080 8081

# Run
ENTRYPOINT ["./aftertalk"]
