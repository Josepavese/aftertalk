# Aftertalk Testing Documentation

## Overview

This document provides comprehensive guidance on all testing strategies for the Aftertalk application, covering **Unit Testing**, **Integration Testing**, **E2E Testing**, and **Performance Testing**.

## Table of Contents

1. [Testing Philosophy](#testing-philosophy)
2. [Test Types](#test-types)
   - [Unit Tests](#unit-tests)
   - [Integration Tests](#integration-tests)
   - [E2E Tests](#e2e-tests)
   - [Performance Tests](#performance-tests)
3. [Running Tests](#running-tests)
4. [Test Infrastructure](#test-infrastructure)
5. [CI/CD Pipeline](#cicd-pipeline)
6. [Code Quality](#code-quality)
7. [Real-World Testing](#real-world-testing)

---

## Testing Philosophy

### Test Pyramid

Aftertalk follows the test pyramid approach:

```
    /\
   /  \     E2E Tests (Few, expensive)
  /____\
 /      \   Integration Tests (Medium)
/________\
----------  Unit Tests (Many, fast)
```

- **70% Unit Tests**: Fast, isolated, test single functions
- **20% Integration Tests**: Test component interactions
- **10% E2E Tests**: Test complete workflows

### Testing Principles

1. **Isolation**: Each test should be independent
2. **Determinism**: Tests should produce same results every time
3. **Fast Feedback**: Unit tests run in < 100ms
4. **Coverage**: Aim for > 80% code coverage
5. **Documentation**: Tests serve as living documentation

---

## Test Types

### Unit Tests

Unit tests verify individual functions and methods in isolation.

#### Location
```
internal/
├── config/config_test.go
├── storage/cache/cache_test.go
├── storage/cache/sessions_test.go
├── storage/cache/queue_test.go
├── storage/sqlite/db_test.go
├── core/session/entity_test.go
├── core/session/repository_test.go
├── core/session/service_test.go
├── core/transcription/entity_transcription_test.go
├── core/transcription/repository_repository_test.go
├── core/transcription/service_service_test.go
├── core/minutes/entity_test.go
├── core/minutes/repository_test.go
├── core/minutes/service_test.go
├── api/handler/session_test.go
├── api/handler/transcription_test.go
├── api/handler/minutes_test.go
├── api/handler/health_test.go
├── api/middleware/middleware_test.go
├── ai/stt/tests/
├── ai/llm/tests/
├── logging/logger_test.go
├── metrics/metrics_test.go
├── bot/server_test.go
├── bot/session_test.go
├── bot/timestamp_test.go
pkg/
├── jwt/jwt_test.go
├── audio/pcm_test.go
├── webhook/client_test.go
cmd/aftertalk/main_test.go
```

#### Running Unit Tests

```bash
# Run all unit tests
make test-unit

# Run specific package
go test -v ./internal/config/...

# Run with coverage
go test -v -coverprofile=coverage.out ./internal/config/...
go tool cover -html=coverage.out -o coverage.html
```

#### Unit Test Patterns

**Table-Driven Tests**:
```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"valid input", "hello", "HELLO", false},
        {"empty input", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected, got)
            }
        })
    }
}
```

**Mocking with testify/mock**:
```go
type MockRepository struct {
    mock.Mock
}

func (m *MockRepository) GetByID(id string) (*Entity, error) {
    args := m.Called(id)
    return args.Get(0).(*Entity), args.Error(1)
}
```

---

### Integration Tests

Integration tests verify that multiple components work together correctly.

#### Location
```
internal/storage/sqlite/integration_test.go
```

#### What They Test

1. **Database Operations**: Real SQLite database with migrations
2. **Service Chains**: Session → Transcription → Minutes flow
3. **Cache Integration**: Cache with real database operations
4. **API Endpoints**: HTTP handlers with real services

#### Running Integration Tests

```bash
# Run all integration tests
make test-integration

# Run specific integration test
go test -v ./internal/storage/sqlite/... -run TestDB_

# Run with race detection
go test -v -race ./internal/storage/sqlite/...
```

#### Integration Test Example

```go
func TestDB_Migrations(t *testing.T) {
    ctx := context.Background()
    db, err := sqlite.New(ctx, ":memory:")
    require.NoError(t, err)
    defer db.Close()
    
    // Run migrations
    err = runMigrations(db)
    require.NoError(t, err)
    
    // Verify tables exist
    tables := []string{"sessions", "participants", "transcriptions", "minutes"}
    for _, table := range tables {
        var count int
        err := db.QueryRowContext(ctx, 
            "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", 
            table).Scan(&count)
        require.NoError(t, err)
        assert.Equal(t, 1, count, "Table %s should exist", table)
    }
}
```

---

### E2E Tests

E2E tests verify complete user workflows from start to finish.

#### Location
```
e2e/tests.go
e2e/run_tests.sh
```

#### What They Test

1. **Full Application Lifecycle**: Startup → Operations → Shutdown
2. **Session Workflow**: Create → Connect → End
3. **Transcription Workflow**: Upload → Process → Retrieve
4. **Minutes Workflow**: Generate → Edit → History
5. **WebSocket Communication**: Real-time audio streaming
6. **Error Scenarios**: Invalid inputs, timeouts, failures

#### Running E2E Tests

```bash
# Run E2E tests (automatically starts server)
make test-e2e

# Or manually
./e2e/run_tests.sh

# Or with custom server
go test -v ./e2e/... -timeout=5m
```

#### E2E Test Architecture

```
┌─────────────────────────────────────────────┐
│           E2E Test Suite                    │
├─────────────────────────────────────────────┤
│  1. Start Application                       │
│     ├── Load config                         │
│     ├── Initialize DB (SQLite in-memory)    │
│     ├── Start HTTP server                   │
│     └── Wait for readiness                  │
│                                             │
│  2. Execute Tests                           │
│     ├── Session API tests                   │
│     ├── Transcription API tests             │
│     ├── Minutes API tests                   │
│     └── WebSocket tests                     │
│                                             │
│  3. Cleanup                                 │
│     ├── Stop server                         │
│     ├── Close DB connections                │
│     └── Cleanup temp files                  │
└─────────────────────────────────────────────┘
```

#### E2E Test Example

```go
func TestSessionWorkflow(t *testing.T) {
    // Start application
    server, cleanup := startTestServer(t)
    defer cleanup()
    
    baseURL := "http://localhost:8080"
    
    // Create session
    reqBody := `{
        "participant_count": 2,
        "participants": [
            {"user_id": "user1", "role": "client"},
            {"user_id": "user2", "role": "professional"}
        ]
    }`
    
    resp, err := http.Post(
        baseURL+"/v1/sessions",
        "application/json",
        strings.NewReader(reqBody),
    )
    require.NoError(t, err)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
    
    // Parse response
    var sessionResp CreateSessionResponse
    err = json.NewDecoder(resp.Body).Decode(&sessionResp)
    require.NoError(t, err)
    
    // Verify session created
    assert.NotEmpty(t, sessionResp.SessionID)
    assert.Len(t, sessionResp.Participants, 2)
    
    // Retrieve session
    resp, err = http.Get(baseURL + "/v1/sessions/" + sessionResp.SessionID)
    require.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

---

### Performance Tests

Performance tests measure system behavior under load.

#### Location
```
internal/performance/benchmarks_test.go
internal/performance/load_test.go
internal/performance/stress_test.go
internal/performance/pprof_test.go
run_performance_tests.sh
```

#### Types of Performance Tests

1. **Benchmarks**: Micro-benchmarks for critical paths
2. **Load Tests**: System behavior under expected load
3. **Stress Tests**: System behavior under extreme load
4. **Profiles**: CPU, memory, goroutine analysis

#### Running Performance Tests

```bash
# Run all performance tests
make test-performance

# Run specific benchmarks
go test -bench=BenchmarkSessionCreation -benchmem ./internal/performance

# Run load tests
go test -v -run=TestConcurrentSessionCreation ./internal/performance -timeout=5m

# Run with profiling
go test -bench=BenchmarkCPUProfile -benchmem -cpuprofile=cpu.prof ./internal/performance
go tool pprof cpu.prof

# Start pprof server
go test -v -run=TestStartPprofServer ./internal/performance
# Access at: http://localhost:6060/debug/pprof/
```

#### Benchmark Example

```go
func BenchmarkSessionCreation1000(b *testing.B) {
    ctx := context.Background()
    db, _ := sqlite.New(ctx, ":memory:")
    defer db.Close()
    
    repo := session.NewRepository(db)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        s := session.NewSession(uuid.New().String(), 2)
        repo.Create(ctx, s)
    }
}
```

#### Performance Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Session Creation | < 5ms | p95 latency |
| Transcription Processing | < 2s | End-to-end |
| Minutes Generation | < 10s | End-to-end |
| Concurrent Sessions | > 100 | Simultaneous |
| Memory Usage | < 512MB | Peak |
| CPU Usage | < 50% | Average |

---

## Running Tests

### Quick Commands

```bash
# Run all tests
make test

# Run specific test type
make test-unit          # Unit tests only
make test-integration   # Integration tests only
make test-e2e          # E2E tests
make test-performance  # Performance tests

# Run with coverage
make test-coverage

# Run linter
make lint

# Format code
make fmt
```

### Test Options

```bash
# Run with race detection
go test -race ./...

# Run specific test
go test -v -run TestFunctionName ./package/...

# Run with verbose output
go test -v ./...

# Run with timeout
go test -timeout=10m ./...

# Skip slow tests
go test -short ./...

# Run benchmarks only
go test -bench=. ./...

# Run benchmarks with memory stats
go test -bench=. -benchmem ./...
```

---

## Test Infrastructure

### Test Database

Integration and E2E tests use an **in-memory SQLite database** (`:memory:`):

- Fresh database for each test
- Fast execution (< 10ms setup)
- No external dependencies
- Automatic cleanup

### Test Fixtures

Common test data in `test/fixtures/`:

```
test/fixtures/
├── sessions.json
├── transcriptions.json
├── minutes.json
└── audio/
    ├── sample_1.pcm
    └── sample_2.pcm
```

### Test Utilities

**Helper functions** in `test/helpers/`:

```go
// test/helpers/database.go
func SetupTestDB(t *testing.T) *sqlite.DB {
    db, err := sqlite.New(context.Background(), ":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { db.Close() })
    return db
}

// test/helpers/server.go
func StartTestServer(t *testing.T) (string, func()) {
    // Start server on random port
    // Return URL and cleanup function
}
```

---

## CI/CD Pipeline

### GitHub Actions Workflow

Located at `.github/workflows/ci.yml`:

```yaml
name: CI Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: golangci-lint run
      - run: go vet ./...

  test:
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test -v -race -coverprofile=coverage.out ./...
      - uses: codecov/codecov-action@v4

  build:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go build -v -o bin/aftertalk ./cmd/aftertalk
      - run: docker build -t aftertalk:latest .
```

### Pipeline Stages

1. **Lint**: Code quality checks
2. **Test**: Run all test suites
3. **Build**: Compile binary and Docker image
4. **Security**: Vulnerability scanning
5. **Deploy**: Push to registry (main branch only)

### Test Coverage Reporting

Coverage reports are uploaded to:
- **Codecov**: Historical coverage tracking
- **GitHub Artifacts**: Detailed HTML reports
- **PR Comments**: Coverage changes

---

## Code Quality

### Linter Configuration

Strict `.golangci.yml` configuration:

```yaml
linters:
  enable-all: true
  disable:
    - depguard
    - exhaustruct
    - ireturn
    - varnamelen
```

### Pre-commit Hooks

Install pre-commit hooks:

```bash
# Install pre-commit
pip install pre-commit

# Install hooks
pre-commit install

# Run manually
pre-commit run --all-files
```

### Code Review Checklist

- [ ] All tests pass
- [ ] Coverage > 80%
- [ ] No linter warnings
- [ ] Benchmarks show no regression
- [ ] Documentation updated

---

## Real-World Testing

### 2-Interlocutor Test Plan

See `docs/REAL_WORLD_TESTING.md` for the complete test plan using **nido** VM orchestration.

### Test Scenarios

1. **Basic Conversation**: 2 participants, 5-minute call
2. **Long Session**: 2 participants, 1-hour call
3. **Concurrent Sessions**: Multiple 2-participant calls simultaneously
4. **Network Interruptions**: Simulate packet loss, latency
5. **Audio Quality**: Varying sample rates, bit depths

### Test Environment

```bash
# Spawn VMs with nido
nido spawn --image ubuntu-22.04 --memory 2048 --cpus 2 test-vm-1
nido spawn --image ubuntu-22.04 --memory 2048 --cpus 2 test-vm-2

# Deploy application
nido ssh test-vm-1 "docker run -d -p 8080:8080 aftertalk:latest"

# Run tests from VM 2
nido ssh test-vm-2 "./run_interlocutor_tests.sh"
```

---

## Troubleshooting

### Common Issues

**Test Timeouts**:
```bash
# Increase timeout
go test -timeout=10m ./...
```

**Race Conditions**:
```bash
# Run with race detector
go test -race ./...
```

**Database Locks**:
```bash
# Use file-based SQLite instead of :memory:
go test -tags=integration ./...
```

**Flaky Tests**:
```bash
# Run multiple times
go test -count=10 ./...
```

### Debugging Tests

```bash
# Run single test with verbose output
go test -v -run TestSpecificFunction ./package

# Add debug prints
log.Printf("Debug: value=%v", value)

# Use debugger
dlv test ./package
```

---

## Best Practices

### Do's

✅ Write tests before or with code (TDD)  
✅ Use table-driven tests for multiple scenarios  
✅ Mock external dependencies  
✅ Test both happy paths and error cases  
✅ Keep tests fast (< 100ms for unit tests)  
✅ Use descriptive test names  
✅ Clean up resources in `t.Cleanup()`  

### Don'ts

❌ Skip tests without documenting why  
❌ Test implementation details  
❌ Use sleep/wait in tests  
❌ Share state between tests  
❌ Ignore flaky tests  
❌ Write tests that depend on external services  

---

## Resources

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Testify Documentation](https://github.com/stretchr/testify)
- [golangci-lint Documentation](https://golangci-lint.run/)
- [Go Test Coverage](https://go.dev/blog/cover)
- [PProf Documentation](https://github.com/google/pprof)

---

## Maintenance

This document should be updated when:
- New test types are added
- Testing patterns change
- CI/CD pipeline is modified
- New testing tools are adopted

Last updated: 2025-03-04
