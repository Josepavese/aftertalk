#!/bin/bash

# E2E Test Runner for Aftertalk
# This script starts the application and runs comprehensive E2E tests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DB_PATH="/tmp/test_aftertalk_e2e.db"
APP_DB_PATH="./aftertalk.db"

echo "🚀 Starting Aftertalk E2E Test Suite"
echo "======================================"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    rm -f "$TEST_DB_PATH"
    echo "✅ Cleanup complete"
}

trap cleanup EXIT

# Kill any existing process on the ports
echo "📡 Checking for existing processes..."
pkill -f "aftertalk" || true
sleep 2

# Start the application
echo ""
echo "📦 Starting Aftertalk application..."
export DATABASE_PATH="$TEST_DB_PATH"
export API_KEY="test-api-key-e2e"
export JWT_SECRET="e2e-test-secret-key"
export JWT_ISSUER="aftertalk-e2e"
export JWT_EXPIRATION="2h"

cd "$SCRIPT_DIR/.."

./bin/aftertalk > /tmp/aftertalk_e2e.log 2>&1 &
APP_PID=$!

echo "✅ Application started (PID: $APP_PID)"
echo "⏳ Waiting for server to be ready..."

# Wait for server to be ready
for i in {1..30}; do
    if curl -s "http://localhost:8080/health" > /dev/null 2>&1; then
        echo "✅ Server ready!"
        break
    fi
    echo "⏳ Waiting... ($i/30)"
    sleep 1
done

if ! curl -s "http://localhost:8080/health" > /dev/null 2>&1; then
    echo "❌ Server failed to start"
    cat /tmp/aftertalk_e2e.log
    exit 1
fi

echo ""
echo "🎯 Running E2E Tests..."
echo "======================================"
echo ""

# Run the E2E tests
go test -v ./e2e/ 2>&1 | tee /tmp/e2e_test_results.log

TEST_RESULT=$?

echo ""
echo "======================================"
echo "📊 Test Results Summary"
echo "======================================"

if [ $TEST_RESULT -eq 0 ]; then
    echo "✅ All E2E tests passed!"
    echo ""
    echo "📝 Test execution details:"
    grep -E "^(PASS|FAIL|===|---)" /tmp/e2e_test_results.log
else
    echo "❌ Some E2E tests failed"
    echo ""
    echo "📝 Failed tests:"
    grep -E "^(FAIL:)" /tmp/e2e_test_results.log || echo "No specific failures logged"
fi

echo ""
echo "🔧 Application PID: $APP_PID"
echo "📁 Log file: /tmp/aftertalk_e2e.log"
echo "📄 Test results: /tmp/e2e_test_results.log"

exit $TEST_RESULT
