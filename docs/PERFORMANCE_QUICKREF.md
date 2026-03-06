# Performance Testing Quick Reference

## Run All Tests

```bash
./run_performance_tests.sh
```

## Individual Tests

### Benchmarks
```bash
# Session operations
go test -bench=BenchmarkSessionCreation1000 -benchmem ./internal/performance
go test -bench=BenchmarkSessionRetrieval -benchmem ./internal/performance

# Cache operations
go test -bench=BenchmarkCacheGetSetDelete -benchmem ./internal/performance

# Database operations
go test -bench=BenchmarkDatabaseInsert -benchmem ./internal/performance
go test -bench=BenchmarkDatabaseSelect -benchmem ./internal/performance
go test -bench=BenchmarkDatabaseUpdate -benchmem ./internal/performance
go test -bench=BenchmarkDatabaseDelete -benchmem ./internal/performance
```

### Load Tests
```bash
go test -v -run=TestConcurrentSessionCreation ./internal/performance -timeout=5m
go test -v -run=TestConcurrentAPIRequests ./internal/performance -timeout=5m
go test -v -run=TestDatabaseConnectionPoolStress ./internal/performance -timeout=5m
go test -v -run=TestMemoryUsageUnderLoad ./internal/performance -timeout=5m
```

### Stress Tests
```bash
go test -v -run=TestLongRunningSessions24Hours ./internal/performance -timeout=24h
go test -v -run=TestHighFrequencySessionCreation ./internal/performance -timeout=10m
go test -v -run=TestLargeTranscriptionDataProcessing ./internal/performance -timeout=10m
go test -v -run=TestDatabaseWALStress ./internal/performance -timeout=5m
```

## Pprof Profiling

### Start Pprof Server
```bash
go test -bench=BenchmarkCPUProfile -benchmem ./internal/performance -run=TestMain
```

Access at: http://localhost:6060/debug/pprof/

### CPU Profile
```bash
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

### Memory Profile
```bash
go tool pprof http://localhost:6060/debug/pprof/heap
```

### Goroutine Profile
```bash
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### Save to File
```bash
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

## Quick Metrics

**Expected Throughput:**
- Session Creation: 5000 ops/sec
- Session Retrieval: 10000 ops/sec
- Cache Operations: 10000 ops/sec
- Database Insert: 5000 ops/sec
- Database Select: 10000 ops/sec

**Expected Latency:**
- Session Creation: < 1ms
- Cache Operations: < 100ns
- Database Operations: < 1ms

## Troubleshooting

**Database errors:**
```bash
rm -f /tmp/perf_*.db /tmp/load_*.db /tmp/stress_*.db
```

**Clean test cache:**
```bash
go clean -testcache
```

**Short test mode:**
```bash
go test -short ./internal/performance
```

**Verbose output:**
```bash
go test -v ./internal/performance
```
