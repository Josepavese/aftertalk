# Quickstart: Aftertalk Core (Go)

**Feature**: 001-aftertalk-core  
**Date**: 2026-03-04  
**Purpose**: Guide for quickly setting up and running Aftertalk Core in development environment

## Prerequisites

### Required Software

- **Go**: 1.22 or later
- **Docker**: v24.x or later
- **Docker Compose**: v2.x or later
- **PostgreSQL**: v15.x (via Docker or local install)
- **Redis**: v7.x (optional, via Docker or local install)
- **Make**: For build automation (optional)

### Recommended Tools

- **golangci-lint**: Go linter
- **kubectl**: For Kubernetes deployment (optional)
- **k9s**: Kubernetes CLI dashboard (optional)
- **pgAdmin**: PostgreSQL GUI (optional)
- **Redis Insight**: Redis GUI (optional)

## Quick Start with Docker Compose

### 1. Clone and Setup

```bash
# Clone the repository
git clone https://github.com/your-org/aftertalk.git
cd aftertalk

# Copy environment variables
cp .env.example .env

# Edit .env with your settings
# Required: DATABASE_URL, JWT_PUBLIC_KEY, GOOGLE_API_KEY (or other STT), OPENAI_API_KEY (or other LLM)
vim .env
```

### 2. Start Infrastructure Services

```bash
# Start PostgreSQL (Redis optional for distributed cache)
docker-compose up -d postgres

# Wait for PostgreSQL to be ready
docker-compose ps

# Check PostgreSQL connection
docker-compose exec postgres pg_isready -U aftertalk

# (Optional) Start Redis for distributed cache
docker-compose up -d redis
docker-compose exec redis redis-cli ping
```

### 3. Run Database Migrations

```bash
# Using make
make migrate-up

# Or directly with psql
psql $DATABASE_URL < migrations/001_init.up.sql

# Or using migration tool (e.g., golang-migrate)
migrate -database $DATABASE_URL -path migrations up
```

### 4. Build and Run

#### Option A: Using Make (Recommended)

```bash
# Build
make build

# Run
make run

# Or build and run in one step
make dev
```

#### Option B: Using Go directly

```bash
# Build
go build -o aftertalk ./cmd/aftertalk

# Run
./aftertalk

# Or run directly (for development)
go run ./cmd/aftertalk
```

#### Option C: Using Docker

```bash
# Build Docker image
docker build -t aftertalk:latest .

# Run container
docker run -p 8080:8080 -p 8081:8081 \
  -e DATABASE_URL=postgresql://aftertalk:password@postgres:5432/aftertalk \
  -e GOOGLE_API_KEY=your-key \
  -e OPENAI_API_KEY=your-key \
  aftertalk:latest

# Or with docker-compose
docker-compose up aftertalk
```

### 5. Verify Services

```bash
# Check API health
curl http://localhost:8080/v1/health

# Expected response:
# {"status":"healthy","timestamp":"2026-03-04T12:00:00.000Z"}

# Check readiness
curl http://localhost:8080/v1/ready

# Check WebSocket endpoint (Bot Recorder)
wscat -c ws://localhost:8081/ws
# Should receive: {"type":"connection_required","message":"Please authenticate with JWT token"}
```

## Development Workflow

### 1. Create a Test Session

```bash
# Create session via API
curl -X POST http://localhost:8080/v1/sessions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-api-key" \
  -d '{
    "participants": [
      {"userId": "user-1", "role": "therapist"},
      {"userId": "user-2", "role": "patient"}
    ]
  }'

# Response includes sessionId and JWT tokens for each participant
# Example response:
# {
#   "id": "550e8400-e29b-41d4-a716-446655440000",
#   "status": "active",
#   "participants": [
#     {"userId": "user-1", "role": "therapist", "token": "eyJhbGc..."},
#     {"userId": "user-2", "role": "patient", "token": "eyJhbGc..."}
#   ]
# }
```

### 2. Simulate Audio Streaming

Use the provided test client or manually via WebSocket:

```bash
# Run test client
go run ./tests/integration/test_audio_stream.go \
  --session-id <SESSION_ID> \
  --token <JWT_TOKEN>

# Or use wscat manually
wscat -c ws://localhost:8081/ws
```

**WebSocket flow:**
```json
// 1. Authenticate
{"type":"authenticate","token":"your-jwt-token"}

// 2. Server responds
{"type":"connection_acknowledged","sessionId":"session-id","role":"therapist"}

// 3. Send audio chunks (base64-encoded Opus)
{"type":"audio_chunk","sequenceNumber":0,"timestamp":1709560000000,"duration":15000,"data":"//NkAAAA..."}

// 4. Server acknowledges
{"type":"audio_chunk_received","sequenceNumber":0,"timestamp":1709560015000}

// 5. End session
{"type":"end_session","reason":"normal","timestamp":1709560300000}

// 6. Server confirms
{"type":"session_ended","sessionId":"session-id","duration":300000,"totalChunks":20}
```

### 3. Monitor Processing

```bash
# Watch logs (structured JSON)
tail -f logs/aftertalk.log | jq

# Check processing status
curl http://localhost:8080/v1/sessions/<SESSION_ID>

# Response shows status: "processing", "completed", or "error"
```

### 4. Retrieve Minutes

```bash
# Get minutes via API
curl http://localhost:8080/v1/sessions/<SESSION_ID>/minutes \
  -H "X-API-Key: dev-api-key"

# Expected response:
# {
#   "id": "minutes-id",
#   "sessionId": "session-id",
#   "themes": ["Theme 1", "Theme 2"],
#   "contentsReported": [...],
#   "professionalInterventions": [...],
#   "progressIssues": {"progress": [...], "issues": [...]},
#   "nextSteps": ["Step 1", "Step 2"],
#   "citations": [
#     {"text": "Quote", "timestampMs": 12345, "role": "patient"}
#   ],
#   "status": "ready"
# }
```

## Testing

### Unit Tests

```bash
# Run all unit tests
make test

# Or with go test
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run specific package
go test ./internal/ai/...

# Run with verbose output
go test -v ./internal/core/...
```

### Integration Tests

```bash
# Run integration tests (requires database)
make test-integration

# Or directly
go test -tags=integration ./tests/integration/...

# Run specific test
go test -tags=integration -run TestFullPipeline ./tests/integration/...
```

### Load Testing

```bash
# Install k6
# https://k6.io/docs/getting-started/installation/

# Run load test
k6 run tests/load/scenarios/100_concurrent_sessions.js

# Or using make
make load-test
```

### Benchmarks

```bash
# Run Go benchmarks
go test -bench=. ./internal/ai/...

# Compare benchmarks
go test -bench=. -benchmem ./internal/ai/... > old.txt
# Make changes
go test -bench=. -benchmem ./internal/ai/... > new.txt
# Compare
benchstat old.txt new.txt
```

## Configuration

### Environment Variables

Create `.env` file in repository root:

```bash
# Server
HTTP_PORT=8080
WS_PORT=8081

# Database
DATABASE_URL=postgresql://aftertalk:password@localhost:5432/aftertalk?sslmode=disable

# Redis (optional)
REDIS_URL=redis://localhost:6379

# JWT Authentication
JWT_PUBLIC_KEY="-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----"
JWT_ISSUER=aftertalk

# STT Provider
STT_PROVIDER=google
GOOGLE_API_KEY=your-google-api-key

# Alternative STT providers:
# STT_PROVIDER=aws
# AWS_ACCESS_KEY_ID=xxx
# AWS_SECRET_ACCESS_KEY=xxx
# AWS_REGION=eu-west-1

# LLM Provider
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-xxx

# Alternative LLM providers:
# LLM_PROVIDER=anthropic
# ANTHROPIC_API_KEY=xxx

# Webhook (optional, for backend notification)
WEBHOOK_URL=https://backend.mondopsicologi.it/webhooks/minutes

# Performance
MAX_CONCURRENT_SESSIONS=200
PROCESSING_WORKERS=10

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# Development
ENVIRONMENT=development
```

### Config File (YAML)

Alternatively, use `config.yaml`:

```yaml
# config.yaml
server:
  http_port: 8080
  ws_port: 8081

database:
  url: postgresql://aftertalk:password@localhost:5432/aftertalk

redis:
  url: redis://localhost:6379
  enabled: false

auth:
  jwt_public_key: |
    -----BEGIN PUBLIC KEY-----
    ...
    -----END PUBLIC KEY-----

stt:
  provider: google
  google_api_key: ${GOOGLE_API_KEY}
  language: it-IT

llm:
  provider: openai
  openai_api_key: ${OPENAI_API_KEY}
  model: gpt-4
  temperature: 0.3

performance:
  max_concurrent_sessions: 200
  processing_workers: 10

logging:
  level: info
  format: json
```

## Development Tips

### 1. Hot Reload (Development Mode)

```bash
# Install air (Go hot reload tool)
go install github.com/cosmtrek/air@latest

# Run with hot reload
air

# Or use make
make watch
```

### 2. Debugging with Delve

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/aftertalk

# Or attach to running process
dlv attach <pid>
```

### 3. Profiling

```bash
# CPU profile
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Memory profile
curl http://localhost:8080/debug/pprof/heap > mem.prof
go tool pprof mem.prof

# Or use pprof web UI
go tool pprof -http=:8081 cpu.prof
```

### 4. Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
make lint

# Or directly
golangci-lint run

# Fix issues
golangci-lint run --fix
```

## Troubleshooting

### Common Issues

**1. Database connection failed**

```bash
# Check if PostgreSQL is running
docker-compose ps postgres

# Check logs
docker-compose logs postgres

# Test connection
psql $DATABASE_URL

# Restart service
docker-compose restart postgres
```

**2. Go module issues**

```bash
# Clean module cache
go clean -modcache

# Download dependencies
go mod download

# Verify dependencies
go mod verify

# Tidy dependencies
go mod tidy
```

**3. STT/LLM API errors**

```bash
# Check API credentials
echo $GOOGLE_API_KEY
echo $OPENAI_API_KEY

# Test API connection manually
curl -X POST https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}'
```

**4. WebSocket connection refused**

```bash
# Check if server is running
lsof -i :8081

# Check logs
tail -f logs/aftertalk.log

# Test WebSocket
wscat -c ws://localhost:8081/ws
```

**5. Processing stuck**

```bash
# Check processing status
curl http://localhost:8080/v1/sessions/<SESSION_ID>

# Check logs for errors
tail -f logs/aftertalk.log | grep -i error

# Check worker pool status
curl http://localhost:8080/debug/vars | jq
```

### Debug Mode

Enable debug logging:

```bash
# Set log level to debug
export LOG_LEVEL=debug

# Or in .env
LOG_LEVEL=debug

# Restart server
make dev
```

### Health Checks

```bash
# Core health
curl http://localhost:8080/v1/health

# Readiness (checks database, redis, providers)
curl http://localhost:8080/v1/ready

# Detailed health
curl http://localhost:8080/v1/health/detailed
```

## Production Deployment

### Docker

```bash
# Build production image
docker build -t aftertalk:production .

# Run with production config
docker run -d \
  --name aftertalk \
  -p 8080:8080 \
  -p 8081:8081 \
  -e ENVIRONMENT=production \
  -e DATABASE_URL=postgresql://... \
  -e GOOGLE_API_KEY=... \
  -e OPENAI_API_KEY=... \
  --restart unless-stopped \
  aftertalk:production
```

### Kubernetes

```bash
# Apply Kubernetes manifests
kubectl apply -f infra/kubernetes/

# Check deployment
kubectl get deployments

# Check pods
kubectl get pods

# Check logs
kubectl logs -f deployment/aftertalk

# Port forward for local testing
kubectl port-forward svc/aftertalk 8080:80 8081:81
```

### Scaling

```bash
# Scale horizontally
kubectl scale deployment aftertalk --replicas=5

# Or use HPA (Horizontal Pod Autoscaler)
kubectl apply -f infra/kubernetes/hpa.yaml

# Check HPA status
kubectl get hpa
```

### Monitoring

```bash
# Port forward Prometheus (if deployed)
kubectl port-forward svc/prometheus 9090:9090

# Port forward Grafana
kubectl port-forward svc/grafana 3001:80

# Access metrics endpoint
curl http://localhost:8080/metrics
```

## Performance Tuning

### Go Runtime

```bash
# Set GOMAXPROCS (default: number of CPUs)
export GOMAXPROCS=4

# Set memory limit (Go 1.19+)
export GOMEMLIMIT=1GiB

# Run with optimized flags
go run -ldflags="-s -w" ./cmd/aftertalk
```

### Database

```sql
-- Optimize PostgreSQL for Aftertalk workload
ALTER SYSTEM SET max_connections = 200;
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
ALTER SYSTEM SET work_mem = '16MB';
ALTER SYSTEM SET maintenance_work_mem = '256MB';

-- Restart PostgreSQL to apply
SELECT pg_reload_conf();
```

### Profiling in Production

```bash
# Enable pprof endpoint (behind auth in production!)
# Add to config:
# profiling:
#   enabled: true
#   endpoint: /debug/pprof

# Capture profile
curl http://aftertalk:8080/debug/pprof/profile?seconds=30 > profile.prof

# Analyze locally
go tool pprof profile.prof
```

## Next Steps

1. **Configure Providers**: Set up STT and LLM provider credentials in `.env`
2. **Generate JWT Keys**: Create RSA key pair for JWT signing
3. **Integrate with Backend**: Configure webhook URL for your backend application
4. **Customize Prompts**: Edit prompt templates in `internal/ai/llm/prompts.go`
5. **Add Monitoring**: Set up Prometheus and Grafana dashboards
6. **Security Review**: Review and harden security configurations before production

## Additional Resources

- [Architecture Overview](./plan.md)
- [Data Model](./data-model.md)
- [API Contracts](./contracts/)
- [Research & Decisions](./research.md)
- [Go Documentation](https://golang.org/doc/)
- [Pion WebRTC](https://github.com/pion/webrtc)
