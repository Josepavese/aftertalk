#!/bin/bash
# test/scripts/setup_vms.sh
# Setup VMs for real audio testing with nido

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# VM Names
SERVER_VM="aftertalk-server"
CLIENT_VM="aftertalk-client"
PRO_VM="aftertalk-professional"

# Configuration
SERVER_IP="192.168.100.2"
CLIENT_IP="192.168.100.3"
PRO_IP="192.168.100.4"

echo "🚀 Aftertalk Real Audio Test - VM Setup"
echo "========================================"

# Function to check if nido is installed
check_nido() {
    if ! command -v nido &> /dev/null; then
        echo "❌ nido non trovato. Installazione..."
        curl -fsSL https://nido.dev/install.sh | bash
        export PATH="$HOME/.nido/bin:$PATH"
    fi
    echo "✅ nido installato"
}

# Function to cleanup existing VMs
cleanup_vms() {
    echo "🧹 Pulizia VM esistenti..."
    nido stop $SERVER_VM 2>/dev/null || true
    nido stop $CLIENT_VM 2>/dev/null || true
    nido stop $PRO_VM 2>/dev/null || true
    
    nido delete $SERVER_VM --force 2>/dev/null || true
    nido delete $CLIENT_VM --force 2>/dev/null || true
    nido delete $PRO_VM --force 2>/dev/null || true
    
    sleep 2
}

# Function to create server VM
setup_server() {
    echo ""
    echo "📦 Creazione Server VM..."
    
    cat > /tmp/server-user-data.yaml << 'EOF'
#cloud-config
package_update: true
packages:
  - golang-go
  - git
  - curl
  - jq
  - sqlite3

runcmd:
  - mkdir -p /opt/aftertalk
  - echo "Aftertalk Server Ready" > /var/log/aftertalk-setup.log
EOF

    nido spawn $SERVER_VM \
        --image ubuntu-22.04 \
        --memory 2048 \
        --cpus 2 \
        --user-data /tmp/server-user-data.yaml
    
    echo "✅ Server VM creata"
    
    # Wait for VM to be ready
    echo "⏳ Attesa avvio VM..."
    sleep 10
    
    # Install Go 1.25
    nido ssh $SERVER_VM "
        wget -q https://go.dev/dl/go1.25.0.linux-amd64.tar.gz -O /tmp/go.tar.gz
        tar -C /usr/local -xzf /tmp/go.tar.gz
        echo 'export PATH=\$PATH:/usr/local/go/bin' >> /etc/profile
        echo 'export PATH=\$PATH:/usr/local/go/bin' >> ~/.bashrc
        export PATH=\$PATH:/usr/local/go/bin
        go version
    "
    
    echo "✅ Go installato"
}

# Function to create client VMs
setup_client_vms() {
    echo ""
    echo "📦 Creazione Client VMs..."
    
    cat > /tmp/client-user-data.yaml << 'EOF'
#cloud-config
package_update: true
packages:
  - curl
  - jq
  - sox
  - libsox-fmt-all

runcmd:
  - mkdir -p /tmp/test_audio
  - echo "Client Ready" > /var/log/client-setup.log
EOF

    # Create client VM
    nido spawn $CLIENT_VM \
        --image ubuntu-22.04 \
        --memory 1024 \
        --cpus 1 \
        --user-data /tmp/client-user-data.yaml
    
    echo "✅ Client VM creata"
    
    # Create professional VM
    nido spawn $PRO_VM \
        --image ubuntu-22.04 \
        --memory 1024 \
        --cpus 1 \
        --user-data /tmp/client-user-data.yaml
    
    echo "✅ Professional VM creata"
    
    sleep 5
}

# Function to deploy Aftertalk to server VM
deploy_aftertalk() {
    echo ""
    echo "🚀 Deploy Aftertalk sul Server..."
    
    # Copy project files
    echo "  📤 Copia file progetto..."
    cd "$PROJECT_ROOT"
    tar czf - --exclude='bin' --exclude='aftertalk.db' --exclude='.git' . | \
        nido ssh $SERVER_VM "tar xzf - -C /opt/aftertalk"
    
    # Build application
    echo "  🔨 Build applicazione..."
    nido ssh $SERVER_VM "
        export PATH=\$PATH:/usr/local/go/bin
        cd /opt/aftertalk
        go mod download
        go build -o bin/aftertalk ./cmd/aftertalk
        echo 'Build completata'
    "
    
    # Create environment file
    echo "  ⚙️  Configurazione..."
    nido ssh $SERVER_VM "cat > /opt/aftertalk/.env << 'EOF'
DATABASE_PATH=/tmp/aftertalk_test.db
HTTP_HOST=0.0.0.0
HTTP_PORT=8080
WEBSOCKET_HOST=0.0.0.0
WEBSOCKET_PORT=8081
LOG_LEVEL=info
LOG_FORMAT=json
JWT_SECRET=test-secret-real-audio-test-do-not-use-in-production-12345
JWT_ISSUER=aftertalk-test
JWT_EXPIRATION=2h
API_KEY=real-audio-test-api-key-2024
STT_PROVIDER=google
STT_GOOGLE_CREDENTIALS_PATH=
LLM_PROVIDER=openai
LLM_OPENAI_API_KEY=
LLM_OPENAI_MODEL=gpt-4
LLM_OPENAI_TIMEOUT=30s
WEBHOOK_URL=
WEBHOOK_TIMEOUT=10s
PROCESSING_MAX_CONCURRENT_TRANSCRIPTIONS=5
PROCESSING_MAX_CONCURRENT_MINUTES_GENERATIONS=3
PROCESSING_TRANSCRIPTION_TIMEOUT=5m
PROCESSING_MINUTES_GENERATION_TIMEOUT=3m
SESSION_MAX_DURATION=1h
SESSION_MAX_PARTICIPANTS_PER_SESSION=10
RETENTION_TRANSCRIPTION_DAYS=30
RETENTION_MINUTES_DAYS=30
RETENTION_WEBHOOK_EVENTS_DAYS=7
PERFORMANCE_ENABLE_PROFILING=true
PERFORMANCE_PROFILING_PORT=6060
EOF"
    
    echo "✅ Aftertalk deployato"
}

# Function to start the application
start_application() {
    echo ""
    echo "▶️  Avvio applicazione..."
    
    # Start in background
    nido ssh $SERVER_VM "
        cd /opt/aftertalk
        export PATH=\$PATH:/usr/local/go/bin
        nohup ./bin/aftertalk > /var/log/aftertalk.log 2>&1 &
        echo \$! > /var/run/aftertalk.pid
        sleep 3
    "
    
    # Wait for health check
    echo "⏳ Attesa avvio applicazione..."
    for i in {1..30}; do
        if nido ssh $SERVER_VM "curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/v1/health" 2>/dev/null | grep -q "200"; then
            echo "✅ Applicazione avviata e pronta!"
            return 0
        fi
        echo -n "."
        sleep 2
    done
    
    echo ""
    echo "❌ Applicazione non avviata correttamente"
    nido ssh $SERVER_VM "tail -50 /var/log/aftertalk.log"
    exit 1
}

# Function to copy audio files to client VMs
copy_audio_files() {
    echo ""
    echo "📤 Copia file audio nelle VM client..."
    
    # Copy to client VM
    tar czf - -C "$PROJECT_ROOT/test/audio/samples" . | \
        nido ssh $CLIENT_VM "tar xzf - -C /tmp/test_audio"
    
    # Copy to professional VM
    tar czf - -C "$PROJECT_ROOT/test/audio/samples" . | \
        nido ssh $PRO_VM "tar xzf - -C /tmp/test_audio"
    
    echo "✅ File audio copiati"
}

# Main setup
main() {
    echo "Inizio setup VMs per test audio reale..."
    
    check_nido
    cleanup_vms
    setup_server
    setup_client_vms
    deploy_aftertalk
    start_application
    copy_audio_files
    
    echo ""
    echo "✅ Setup completato!"
    echo ""
    echo "📊 Stato VMs:"
    nido ls | grep -E "(aftertalk-server|aftertalk-client|aftertalk-professional)"
    echo ""
    echo "🎯 Prossimi passi:"
    echo "  ./test/scripts/run_real_audio_test.sh"
}

# Run if executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
