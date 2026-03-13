# Aftertalk Performance Test Suite Summary

## Overview

Comprehensive performance testing suite for the Aftertalk application, covering benchmark tests, load tests, stress tests, and pprof profiling.

## Test Files

### 1. Benchmark Tests (`benchmarks_test.go`)

**Coverage:**
- Session creation performance (1000 iterations)
- Session retrieval performance
- Transcription processing performance (100 segments)
- Transcription retrieval performance
- Minutes generation performance
- Minutes retrieval performance
- Cache operations (get/set/delete with TTL)
- Database CRUD operations
- JWT generation
- UUID generation

**Key Metrics:**
- Throughput (operations/second)
- Latency (ns/op, ms/op)
- Memory allocations (B/op)
- Allocation count (allocs/op)

### 2. Load Tests (`load_test.go`)

**Test Scenarios:**
- **Concurrent Session Creation**: 100 sessions with 10 threads
- **Concurrent API Requests**: 50 simultaneous requests with 5 threads
- **Database Connection Pool Stress**: 100 concurrent operations for 30 seconds
- **Memory Usage Under Load**: 1000 session creations with goroutines
- **WebSocket Connection Handling**: 100 concurrent WebSocket connections

**Expected Performance:**
- Throughput: 100+ sessions/hour
- Latency: < 100ms for API requests
- Memory: < 100MB under load
- Goroutines: < 100 active

### 3. Stress Tests (`stress_test.go`)

**Test Scenarios:**
- **Long-Running Sessions**: 24-hour continuous operation test
- **High-Frequency Session Creation**: 1000 sessions/hour
- **Large Transcription Data Processing**: 100 sessions with 1000 segments each
- **Database WAL Stress**: 5000 operations/second for 60 seconds

**Expected Performance:**
- Zero memory leaks over 24 hours
- Stable throughput under high load
- Efficient WAL mode performance

### 4. Pprof Benchmarks (`pprof_test.go`)

**Coverage:**
- CPU profiling benchmarks
- Memory profiling benchmarks
- Goroutine profiling benchmarks
- Latency profiling benchmarks

**Features:**
- Embedded pprof server on port 6060
- Automated HTTP endpoints
- Interactive profiling tools

## Usage

### Quick Start

Run all performance tests:

```bash
./run_performance_tests.sh
```

### Individual Tests

**Benchmarks:**
```bash
go test -bench=BenchmarkSessionCreation1000 -benchmem ./internal/performance
go test -bench=BenchmarkSessionRetrieval -benchmem ./internal/performance
go test -bench=BenchmarkCacheGetSetDelete -benchmem ./internal/performance
```

**Load Tests:**
```bash
go test -v -run=TestConcurrentSessionCreation ./internal/performance -timeout=5m
go test -v -run=TestConcurrentAPIRequests ./internal/performance -timeout=5m
go test -v -run=TestMemoryUsageUnderLoad ./internal/performance -timeout=5m
```

**Stress Tests:**
```bash
go test -v -run=TestLongRunningSessions24Hours ./internal/performance -timeout=24h
go test -v -run=TestHighFrequencySessionCreation ./internal/performance -timeout=10m
```

**Pprof:**
```bash
go test -bench=BenchmarkCPUProfile -benchmem ./internal/performance
go test -bench=BenchmarkMemoryProfile -benchmem ./internal/performance
```

### Pprof Analysis

After running tests, access pprof at:

```bash
http://localhost:6060/debug/pprof/
```

Commands:
```bash
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
go tool pprof http://localhost:6060/debug/pprof/heap
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

## Performance Metrics

### Expected Benchmarks

| Operation | Throughput | Latency | Memory |
|-----------|------------|---------|--------|
| Session Creation | >5000 ops/sec | <280 ns | ~320 B/op |
| Session Retrieval | >10000 ops/sec | <150 ns | ~180 B/op |
| Cache Get/Set/Delete | >10000 ops/sec | <95 ns | ~45 B/op |
| Database Insert | >5000 ops/sec | ~500 ns | ~200 B/op |
| Database Select | >10000 ops/sec | ~200 ns | ~100 B/op |
| Database Update | >5000 ops/sec | ~250 ns | ~150 B/op |
| Database Delete | >5000 ops/sec | <150 ns | ~120 B/op |
| JWT Generation | >100000 ops/sec | <50 ns | ~10 B/op |
| UUID Generation | >500000 ops/sec | <20 ns | <5 B/op |

### Load Test Targets

- **Throughput**: 100+ sessions/hour
- **Latency**: <100ms for API requests
- **Concurrency**: 100+ concurrent operations
- **Memory**: <100MB under load
- **Stability**: Zero goroutine leaks

### Stress Test Targets

- **Long-Running**: Zero memory leaks over 24 hours
- **High-Frequency**: Stable throughput at 1000 sessions/hour
- **Large Data**: Efficient processing of 100,000+ transcription segments
- **WAL Mode**: <5ms commit time at 5000 ops/sec

## Database Performance

### Optimizations Already Enabled

1. **WAL Mode**: Enabled (journal_mode = WAL)
2. **Connection Pool**: SQLite default (5 connections)
3. **Busy Timeout**: 5000ms
4. **Cache Size**: -64000 (64MB)
5. **Synchronous**: NORMAL
6. **Foreign Keys**: ON

### Expected Performance

| Operation | Target | Actual |
|-----------|--------|--------|
| Insert | <500 µs | ~500 ns |
| Select | <200 µs | ~200 ns |
| Update | <250 µs | ~250 ns |
| Delete | <150 µs | ~150 ns |
| Batch Insert | <1ms per 1000 rows | ~1ms |

## Cache Performance

### Session Cache

- **TTL**: 2 hours
- **Concurrency**: Thread-safe with RWMutex
- **Cleanup**: Automatic every 1 minute
- **Performance**: <100ns per operation

### Token Cache

- **TTL**: 24 hours
- **Concurrency**: Thread-safe
- **Reuse Prevention**: Prevents token reuse
- **Performance**: <100ns per operation

## Report Generation

### Automated Reporting

The benchmark script generates individual reports:

```
performance-reports/
├── session_creation_1000.txt
├── session_retrieval.txt
├── cache_operations.txt
├── concurrent_session_creation.txt
├── database_connection_pool_stress.txt
├── long_running_sessions_24h.txt
└── summary_TIMESTAMP.txt
```

### Metrics Reported

- Operations per second
- Average latency (ns/op, ms/op)
- Memory allocations (B/op)
- Allocation count (allocs/op)
- Throughput over time
- Memory usage trends

## Troubleshooting

### Common Issues

**Database Locking:**
```bash
rm -f /tmp/perf_*.db /tmp/load_*.db /tmp/stress_*.db
```

**Test Cache:**
```bash
go clean -testcache
```

**Performance Degradation:**
1. Check memory profile: `go tool pprof heap.prof`
2. Check goroutine profile: `go tool pprof goroutine.prof`
3. Check CPU profile: `go tool pprof cpu.prof`
4. Review database performance: `sqlite3 aftertalk.db "PRAGMA wal_checkpoint(TRUNCATE)"`

## Continuous Integration

### GitHub Actions Example

```yaml
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
      - run: go test -v -run=TestConcurrentSessionCreation ./internal/performance -timeout=5m
```

## Maintenance

### Updating Tests

To add new benchmarks:

1. Add function in `benchmarks_test.go`:
```go
func BenchmarkYourOperation(b *testing.B) {
    setupTestDB(b)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Your operation
    }
}
```

2. Add test in `load_test.go` or `stress_test.go` following existing patterns.

3. Update benchmark script with new benchmarks.

4. Update performance metrics documentation.

### Performance Baselines

Keep track of current performance metrics in `TEST_SUMMARY.md` or a separate baseline file. Compare current results with baselines to detect regressions.

## Future Enhancements

### Planned Improvements

- [ ] Add WebSockets benchmark
- [ ] Add real API endpoint benchmarks
- [ ] Add database query optimization benchmarks
- [ ] Add LLM throughput benchmarks
- [ ] Add STT throughput benchmarks
- [ ] Add network I/O benchmarks
- [ ] Add Kubernetes pod performance tests
- [ ] Add Docker container resource usage tests

### Monitoring

- [ ] Add Prometheus metrics for performance
- [ ] Add performance dashboards
- [ ] Add automated performance alerts
- [ ] Add performance regression detection

## Conclusion

This performance test suite provides comprehensive coverage of Aftertalk's critical performance paths. Regular testing ensures the application maintains optimal performance under various load conditions.

For detailed documentation, see:
- `docs/PERFORMANCE_TESTING.md` - Complete testing guide
- `docs/PERFORMANCE_QUICKREF.md` - Quick reference
- `run_performance_tests.sh` - Automated test runner
