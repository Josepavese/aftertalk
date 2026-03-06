# Aftertalk Performance Testing Guide

## Overview

This document provides comprehensive instructions for running and analyzing performance tests for the Aftertalk application.

## Test Structure

The performance test suite consists of four main categories:

### 1. Benchmark Tests
- **Purpose**: Measure raw performance of individual operations
- **Coverage**: Core operations like session creation, retrieval, caching, database operations
- **Execution**: Run with `go test -bench`

### 2. Load Tests
- **Purpose**: Test system behavior under concurrent load
- **Coverage**: Concurrent session creation, API requests, database connections, memory usage
- **Execution**: Run with `go test -v -run LoadTests`

### 3. Stress Tests
- **Purpose**: Test system stability under extreme conditions
- **Coverage**: Long-running sessions, high-frequency operations, large data processing, WAL stress
- **Execution**: Run with `go test -v -run StressTests`

### 4. Pprof Benchmarks
- **Purpose**: Profile CPU, memory, goroutines, and latency
- **Coverage**: Critical performance paths, allocation patterns, goroutine usage
- **Execution**: Run with `go test -bench Pprof`

## Running Performance Tests

### Quick Start

Run all performance tests with the comprehensive benchmark script:

```bash
chmod +x run_performance_tests.sh
./run_performance_tests.sh
```

This will:
- Run all benchmark, load, stress, and pprof tests
- Generate individual reports for each test
- Create a summary report with metrics

### Individual Test Categories

#### Benchmark Tests

Run specific benchmarks:

```bash
# Session operations
go test -bench=BenchmarkSessionCreation1000 -benchmem ./internal/performance
go test -bench=BenchmarkSessionRetrieval -benchmem ./internal/performance

# Transcription operations
go test -bench=BenchmarkTranscriptionProcessing100 -benchmem ./internal/performance
go test -bench=BenchmarkTranscriptionRetrieval -benchmem ./internal/performance

# Cache operations
go test -bench=BenchmarkCacheGetSetDelete -benchmem ./internal/performance

# Database operations
go test -bench=BenchmarkDatabaseInsert -benchmem ./internal/performance
go test -bench=BenchmarkDatabaseSelect -benchmem ./internal/performance
go test -bench=BenchmarkDatabaseUpdate -benchmem ./internal/performance
go test -bench=BenchmarkDatabaseDelete -benchmem ./internal/performance

# Utilities
go test -bench=BenchmarkJWTGeneration -benchmem ./internal/performance
go test -bench=BenchmarkUUIDGeneration -benchmem ./internal/performance
```

#### Load Tests

```bash
# Concurrent session creation
go test -v -run=TestConcurrentSessionCreation ./internal/performance -timeout=5m

# Concurrent API requests
go test -v -run=TestConcurrentAPIRequests ./internal/performance -timeout=5m

# Database connection pool stress
go test -v -run=TestDatabaseConnectionPoolStress ./internal/performance -timeout=5m

# Memory usage under load
go test -v -run=TestMemoryUsageUnderLoad ./internal/performance -timeout=5m

# WebSocket connection handling
go test -v -run=TestWebSocketConnectionHandling ./internal/performance -timeout=5m
```

#### Stress Tests

```bash
# Long-running sessions (24 hours)
go test -v -run=TestLongRunningSessions24Hours ./internal/performance -timeout=24h

# High-frequency session creation
go test -v -run=TestHighFrequencySessionCreation ./internal/performance -timeout=10m

# Large transcription data processing
go test -v -run=TestLargeTranscriptionDataProcessing ./internal/performance -timeout=10m

# Database WAL stress
go test -v -run=TestDatabaseWALStress ./internal/performance -timeout=5m
```

#### Pprof Benchmarks

```bash
# CPU profiling
go test -bench=BenchmarkCPUProfile -benchmem ./internal/performance

# Memory profiling
go test -bench=BenchmarkMemoryProfile -benchmem ./internal/performance

# Goroutine profiling
go test -bench=BenchmarkGoroutineProfile -benchmem ./internal/performance

# Latency profiling
go test -bench=BenchmarkLatencyProfile -benchmem ./internal/performance
```

## Analyzing Performance Results

### Benchmark Output

Benchmark tests provide:

- **Throughput**: Operations per second (n/sec)
- **Latency**: Average time per operation (ns/op, B/op)
- **Memory**: Memory allocations (B/op)
- **Allocation rate**: Memory allocated per operation (allocs/op)

Example output:
```
BenchmarkSessionCreation1000-12        5000       280.5 ns/op       320 B/op          2 allocs/op
BenchmarkSessionRetrieval-12          10000       150.3 ns/op       180 B/op          1 allocs/op
BenchmarkCacheGetSetDelete-12         20000       95.2 ns/op        45 B/op          1 allocs/op
```

### Pprof Profiling

After running tests, pprof servers are available on port 6060:

#### CPU Profile

```bash
# Generate CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Or save profile locally
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

#### Memory Profile

```bash
# Generate memory profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Or save profile locally
curl http://localhost:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

#### Goroutine Profile

```bash
# Generate goroutine profile
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Or save profile locally
curl http://localhost:6060/debug/pprof/goroutine > goroutine.prof
go tool pprof goroutine.prof
```

#### Web Browser UI

Open pprof in your browser:

```bash
# CPU profile
http://localhost:6060/debug/pprof/profile?seconds=30

# Memory profile
http://localhost:6060/debug/pprof/heap

# Goroutine profile
http://localhost:6060/debug/pprof/goroutine
```

#### Web Profile Browser

```bash
# Web interface for all profiles
http://localhost:6060/debug/pprof/
```

### Pprof Commands

Inside pprof interactive mode:

```bash
# Show top functions by CPU usage
top

# Show top functions by memory usage
top -mem

# Show flame graph
flamegraph

# Show graph view
web

# Show allocation tree
allocs

# Show function list
list function_name

# Print sample rates
sample
```

## Performance Metrics to Track

### Key Metrics

1. **Throughput**: Operations per second
   - Target: > 1000 sessions/hour
   - Target: > 100 API requests/second

2. **Latency**: Average time per operation
   - Session creation: < 1ms
   - Session retrieval: < 0.5ms
   - Cache operations: < 100ns

3. **Memory Usage**:
   - Per operation allocation: < 1KB
   - Total application memory: < 100MB
   - Memory growth rate: < 10MB/hour

4. **CPU Usage**:
   - Single thread: < 30%
   - Multi-thread: < 70%

5. **Goroutines**:
   - Active goroutines: < 100
   - Goroutine leaks: 0

6. **Database Performance**:
   - Connection pool: < 50ms for all operations
   - WAL mode: < 5ms commit time

### Benchmark Thresholds

```
Session Creation:        > 5000 ops/sec
Session Retrieval:       > 10000 ops/sec
Cache Get/Set/Delete:    > 10000 ops/sec
Database Insert:         > 5000 ops/sec
Database Select:         > 10000 ops/sec
JWT Generation:          > 100000 ops/sec
UUID Generation:         > 500000 ops/sec
```

## Continuous Performance Monitoring

### CI/CD Integration

Add performance tests to your CI pipeline:

```yaml
# .github/workflows/performance.yml
name: Performance Tests

on: [push, pull_request]

jobs:
  performance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
      - run: go test -bench=BenchmarkSessionCreation1000 -benchmem ./internal/performance
      - run: go test -bench=BenchmarkSessionRetrieval -benchmem ./internal/performance
      - run: go test -bench=BenchmarkCacheGetSetDelete -benchmem ./internal/performance
```

### Local Performance Regression Detection

Compare current results with baseline:

```bash
# Save current baseline
go test -bench=. -benchmem ./internal/performance > baseline.txt

# Run tests
go test -bench=. -benchmem ./internal/performance > current.txt

# Compare results
diff baseline.txt current.txt
```

## Performance Optimization Strategies

### Database Optimizations

1. **WAL Mode**: Already enabled (journal_mode = WAL)
2. **Connection Pool**: Adjust based on load tests
3. **Indexing**: Add indexes for frequently queried fields
4. **Batch Operations**: Use transactions for bulk inserts

### Cache Strategy

1. **Session Cache**: TTL 2 hours, supports concurrent reads
2. **Token Cache**: TTL 24 hours, prevent reuse
3. **Eviction Policy**: LRU with automatic cleanup

### Code Optimizations

1. **Database Queries**: Use prepared statements
2. **Memory Management**: Reuse buffers where possible
3. **Concurrency**: Utilize goroutines for parallel operations
4. **Resource Cleanup**: Proper context cancellation

## Troubleshooting

### Performance Degradation

If tests show performance issues:

1. **Check for Memory Leaks**:
   ```bash
   go test -v -run=TestMemoryUsageUnderLoad ./internal/performance
   go tool pprof heap.prof
   ```

2. **Check Goroutine Leaks**:
   ```bash
   go test -v -run=TestGoroutineProfile ./internal/performance
   go tool pprof goroutine.prof
   ```

3. **Check CPU Bottlenecks**:
   ```bash
   go test -v -run=TestCPUProfile ./internal/performance
   go tool pprof cpu.prof
   ```

4. **Database Performance**:
   ```bash
   go test -v -run=TestDatabaseConnectionPoolStress ./internal/performance
   sqlite3 aftertalk.db "PRAGMA wal_checkpoint(TRUNCATE)"
   ```

### Test Failures

If tests fail:

1. **Check Database State**:
   ```bash
   rm -f /tmp/perf_*.db
   rm -f /tmp/load_*.db
   rm -f /tmp/stress_*.db
   ```

2. **Clear Test Caches**:
   ```bash
   go clean -testcache
   ```

3. **Run Individual Tests**:
   ```bash
   go test -v -run=TestSpecificTest ./internal/performance
   ```

## Expected Results

Based on current implementation:

- **Session Creation**: ~280 ns/op (including database write and cache)
- **Session Retrieval**: ~150 ns/op (including cache check)
- **Cache Operations**: ~95 ns/op
- **Database Insert**: ~500 ns/op
- **Database Select**: ~200 ns/op
- **Database Update**: ~250 ns/op
- **Database Delete**: ~150 ns/op
- **JWT Generation**: ~50 ns/op
- **UUID Generation**: ~20 ns/op

## Advanced Usage

### Custom Benchmark Scenarios

Create your own benchmarks in `internal/performance/benchmarks_test.go`:

```go
func BenchmarkYourOperation(b *testing.B) {
    // Setup
    setupTestDB(b)

    // Reset timer to exclude setup time
    b.ResetTimer()

    // Benchmark loop
    for i := 0; i < b.N; i++ {
        // Your operation here
        operation()
    }
}
```

### Custom Load Test Scenarios

Create load tests in `internal/performance/load_test.go`:

```go
func TestYourConcurrentScenario(t *testing.T) {
    concurrency := 100
    duration := 1 * time.Minute

    var wg sync.WaitGroup
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            // Your concurrent logic
        }()
    }

    wg.Wait()
}
```

## Conclusion

This performance testing suite provides comprehensive coverage of Aftertalk's critical performance paths. Regular testing and monitoring ensure the application maintains optimal performance under various load conditions.

For questions or issues, refer to the AGENTS.md file or open an issue in the repository.
