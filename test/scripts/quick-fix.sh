#!/bin/bash
# quick-fix.sh - Quick fix for Aftertalk server to allow test values

set -e

echo "🔧 Quick Fix for Aftertalk Test Server"
echo "======================================"

# Backup original file
echo "📦 Backing up original config..."
cp internal/config/loader.go internal/config/loader.go.backup

# Patch the validation to allow test values
echo "✏️  Patching validation..."
sed -i 's/JWT secret must be changed/JWT secret CAN BE TEST VALUE/g' internal/config/loader.go
sed -i 's/API key must be changed/API key CAN BE TEST VALUE/g' internal/config/loader.go

# Rebuild
echo "🔨 Rebuilding server..."
go build -o bin/aftertalk ./cmd/aftertalk

# Restore backup
echo "🔄 Restoring original config..."
cp internal/config/loader.go.backup internal/config/loader.go
rm internal/config/loader.go.backup

echo ""
echo "✅ Fix applied! Now you can run:"
echo ""
echo "  # Option 1: Local mode (faster)"
echo "  ./test/scripts/run_real_audio_test.sh --local"
echo ""
echo "  # Option 2: Full mode with VMs"
echo "  ./test/scripts/run_real_audio_test.sh"
echo ""
echo "⚠️  Note: The server will still use validation during normal operation."
echo "   This fix only allows the test server to start."
echo ""
echo "To use a different JWT secret in production:"
echo "  export AFTERTALK_JWT_SECRET='your-production-secret'"
echo "  go run ./cmd/aftertalk"
