#!/bin/bash

# Aftertalk Performance Testing Suite
# This script runs all performance tests and generates reports

set -e

echo "=== Aftertalk Performance Testing Suite ==="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Directories
AFTERTALK_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPORT_DIR="${AFTERTALK_ROOT}/performance-reports"

# Create report directory
mkdir -p "${REPORT_DIR}"
echo "Reports will be saved to: ${REPORT_DIR}"
echo ""

# Function to run benchmark and save result
run_benchmark() {
    local benchmark_name=$1
    local benchmark_cmd=$2
    local output_file="${REPORT_DIR}/${benchmark_name}.txt"

    echo -e "${YELLOW}Running: ${benchmark_name}${NC}"
    echo "Command: ${benchmark_cmd}"
    echo "Output: ${output_file}"
    echo ""

    ${benchmark_cmd} > "${output_file}" 2>&1 || {
        echo -e "${RED}❌ Benchmark failed: ${benchmark_name}${NC}"
        return 1
    }

    echo -e "${GREEN}✓ Completed: ${benchmark_name}${NC}"
    echo ""
}

# Function to run test and save result
run_test() {
    local test_name=$1
    local test_cmd=$2
    local output_file="${REPORT_DIR}/${test_name}.txt"

    echo -e "${YELLOW}Running: ${test_name}${NC}"
    echo "Command: ${test_cmd}"
    echo "Output: ${output_file}"
    echo ""

    ${test_cmd} > "${output_file}" 2>&1 || {
        echo -e "${RED}❌ Test failed: ${test_name}${NC}"
        return 1
    }

    echo -e "${GREEN}✓ Completed: ${test_name}${NC}"
    echo ""
}

# Generate timestamp
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
SUMMARY_FILE="${REPORT_DIR}/summary_${TIMESTAMP}.txt"

# Initialize summary
cat > "${SUMMARY_FILE}" << EOF
Aftertalk Performance Testing Summary
=======================================
Generated: ${TIMESTAMP}
Go Version: $(go version)
=======================================
EOF

echo "Running comprehensive performance tests..."
echo ""

# 1. Benchmark Tests
echo "=== Running Benchmark Tests ==="
echo ""

# Session creation benchmark (1000 iterations)
run_benchmark "session_creation_1000" \
    "go test -bench=BenchmarkSessionCreation1000 -benchmem ./internal/performance -benchtime=5s -count=3"

# Session retrieval benchmark
run_benchmark "session_retrieval" \
    "go test -bench=BenchmarkSessionRetrieval -benchmem ./internal/performance -benchtime=5s -count=3"

# Transcription processing benchmark
run_benchmark "transcription_processing_100" \
    "go test -bench=BenchmarkTranscriptionProcessing100 -benchmem ./internal/performance -benchtime=5s -count=3"

# Transcription retrieval benchmark
run_benchmark "transcription_retrieval" \
    "go test -bench=BenchmarkTranscriptionRetrieval -benchmem ./internal/performance -benchtime=5s -count=3"

# Minutes generation benchmark
run_benchmark "minutes_generation" \
    "go test -bench=BenchmarkMinutesGeneration -benchmem ./internal/performance -benchtime=5s -count=3"

# Cache operations benchmark
run_benchmark "cache_operations" \
    "go test -bench=BenchmarkCacheGetSetDelete -benchmem ./internal/performance -benchtime=5s -count=3"

# Database operations benchmarks
run_benchmark "database_insert" \
    "go test -bench=BenchmarkDatabaseInsert -benchmem ./internal/performance -benchtime=5s -count=3"

run_benchmark "database_select" \
    "go test -bench=BenchmarkDatabaseSelect -benchmem ./internal/performance -benchtime=5s -count=3"

run_benchmark "database_update" \
    "go test -bench=BenchmarkDatabaseUpdate -benchmem ./internal/performance -benchtime=5s -count=3"

run_benchmark "database_delete" \
    "go test -bench=BenchmarkDatabaseDelete -benchmem ./internal/performance -benchtime=5s -count=3"

# JWT generation benchmark
run_benchmark "jwt_generation" \
    "go test -bench=BenchmarkJWTGeneration -benchmem ./internal/performance -benchtime=5s -count=3"

# UUID generation benchmark
run_benchmark "uuid_generation" \
    "go test -bench=BenchmarkUUIDGeneration -benchmem ./internal/performance -benchtime=5s -count=3"

# 2. Load Tests
echo ""
echo "=== Running Load Tests ==="
echo ""

# Concurrent session creation
run_test "concurrent_session_creation" \
    "go test -v -run=TestConcurrentSessionCreation ./internal/performance -timeout=5m"

# Concurrent API requests
run_test "concurrent_api_requests" \
    "go test -v -run=TestConcurrentAPIRequests ./internal/performance -timeout=5m"

# Database connection pool stress
run_test "database_connection_pool_stress" \
    "go test -v -run=TestDatabaseConnectionPoolStress ./internal/performance -timeout=5m"

# Memory usage under load
run_test "memory_usage_under_load" \
    "go test -v -run=TestMemoryUsageUnderLoad ./internal/performance -timeout=5m"

# WebSocket connection handling
run_test "websocket_connection_handling" \
    "go test -v -run=TestWebSocketConnectionHandling ./internal/performance -timeout=5m"

# 3. Stress Tests
echo ""
echo "=== Running Stress Tests ==="
echo ""

# Long-running sessions (24 hours)
run_test "long_running_sessions_24h" \
    "go test -v -run=TestLongRunningSessions24Hours ./internal/performance -timeout=24h"

# High-frequency session creation
run_test "high_frequency_session_creation" \
    "go test -v -run=TestHighFrequencySessionCreation ./internal/performance -timeout=10m"

# Large transcription data processing
run_test "large_transcription_data_processing" \
    "go test -v -run=TestLargeTranscriptionDataProcessing ./internal/performance -timeout=10m"

# Database WAL stress
run_test "database_wal_stress" \
    "go test -v -run=TestDatabaseWALStress ./internal/performance -timeout=5m"

# 4. Pprof Benchmarks
echo ""
echo "=== Running Pprof Benchmarks ==="
echo ""

# CPU profiling benchmark
run_benchmark "cpu_profile" \
    "go test -bench=BenchmarkCPUProfile -benchmem ./internal/performance -benchtime=10s -count=3"

# Memory profiling benchmark
run_benchmark "memory_profile" \
    "go test -bench=BenchmarkMemoryProfile -benchmem ./internal/performance -benchtime=10s -count=3"

# Goroutine profiling benchmark
run_benchmark "goroutine_profile" \
    "go test -bench=BenchmarkGoroutineProfile -benchmem ./internal/performance -benchtime=10s -count=3"

# Latency profiling benchmark
run_benchmark "latency_profile" \
    "go test -bench=BenchmarkLatencyProfile -benchmem ./internal/performance -benchtime=10s -count=3"

# Create summary
echo ""
echo "=== Creating Performance Summary ==="
cat >> "${SUMMARY_FILE}" << EOF

Benchmark Tests Summary
=======================
All benchmark tests completed.
Please review individual reports for detailed metrics.

Load Tests Summary
==================
$(ls -1 ${REPORT_DIR}/concurrent_* 2>/dev/null | wc -l) load tests completed.

Stress Tests Summary
====================
$(ls -1 ${REPORT_DIR}/stress_* 2>/dev/null | wc -l) stress tests completed.

Pprof Benchmarks Summary
========================
All pprof benchmarks completed.

Next Steps
==========
1. Review individual test reports in ${REPORT_DIR}
2. Use pprof tools for detailed profiling:
   - CPU profile: go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
   - Memory profile: go tool pprof http://localhost:6060/debug/pprof/heap
   - Goroutine profile: go tool pprof http://localhost:6060/debug/pprof/goroutine
   - Profile browser: http://localhost:6060/debug/pprof/

Performance Metrics to Track
==============================
Throughput (operations/second)
Average Latency (ms)
Memory Usage (MB)
CPU Usage (%)
Goroutine Count
Database Connection Pool Performance
EOF

echo ""
echo -e "${GREEN}✓ All performance tests completed successfully!${NC}"
echo ""
echo "Summary file: ${SUMMARY_FILE}"
echo "Reports directory: ${REPORT_DIR}"
