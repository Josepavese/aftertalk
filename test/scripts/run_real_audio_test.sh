#!/bin/bash
# test/scripts/run_real_audio_test.sh
# Main orchestrator for real audio testing with nido VMs

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="$PROJECT_ROOT/test/results"

# Configuration
SERVER_VM="aftertalk-server"
CLIENT_VM="aftertalk-client"
PRO_VM="aftertalk-professional"
API_KEY="real-audio-test-api-key-2024"

# Test audio files
CLIENT_AUDIO="/tmp/test_audio/client_conversation.wav"
PRO_AUDIO="/tmp/test_audio/professional_conversation.wav"

# Create results directory
mkdir -p "$RESULTS_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if test should run in local mode (no VMs)
check_local_mode() {
    if [[ "$1" == "--local" ]]; then
        echo "true"
    else
        echo "false"
    fi
}

LOCAL_MODE=$(check_local_mode "$1")

echo "🎬 Aftertalk Real Audio Test"
echo "=============================="
echo ""

if [[ "$LOCAL_MODE" == "true" ]]; then
    log_info "Modalità locale (senza VMs)"
    SERVER_URL="http://localhost:8080"
    WS_URL="ws://localhost:8080/ws"
else
    log_info "Modalità VM con nido"
    SERVER_URL="http://aftertalk-server:8080"
    WS_URL="ws://aftertalk-server:8080/ws"
fi

# Step 1: Setup VMs (if not local)
setup_environment() {
    if [[ "$LOCAL_MODE" == "true" ]]; then
        log_info "Modalità locale - salto setup VMs"
        
        # Check if local server is running
        if ! curl -s http://localhost:8080/v1/health > /dev/null 2>&1; then
            log_error "Server locale non in esecuzione su localhost:8080"
            log_info "Avvia il server con: go run ./cmd/aftertalk"
            exit 1
        fi
        
        log_success "Server locale rilevato"
        return 0
    fi
    
    log_info "Step 1: Setup ambiente con VMs..."
    
    # Check if VMs already exist
    if nido ls | grep -q "$SERVER_VM"; then
        log_warn "VMs già esistenti, utilizzo VMs esistenti"
    else
        log_info "Creazione VMs..."
        "$SCRIPT_DIR/setup_vms.sh"
    fi
    
    log_success "Ambiente pronto"
}

# Step 2: Generate test audio
generate_test_audio() {
    log_info "Step 2: Generazione audio di test..."
    
    # Check if audio already exists
    if [[ -f "$PROJECT_ROOT/test/audio/samples/client_conversation.wav" ]]; then
        log_success "Audio di test già presente"
        return 0
    fi
    
    cd "$PROJECT_ROOT"
    go run test/scripts/generate_audio.go test/audio/samples
    
    log_success "Audio generato"
}

# Step 3: Create session
create_session() {
    log_info "Step 3: Creazione sessione..."
    
    if [[ "$LOCAL_MODE" == "true" ]]; then
        # Local mode
        SESSION_RESPONSE=$(curl -s -X POST "$SERVER_URL/v1/sessions" \
            -H "Content-Type: application/json" \
            -H "X-API-Key: $API_KEY" \
            -d '{
                "participant_count": 2,
                "participants": [
                    {"user_id": "client-real-test", "role": "client"},
                    {"user_id": "professional-real-test", "role": "professional"}
                ]
            }')
    else
        # VM mode
        SESSION_RESPONSE=$(nido ssh $SERVER_VM "curl -s -X POST \"$SERVER_URL/v1/sessions\" \
            -H 'Content-Type: application/json' \
            -H 'X-API-Key: $API_KEY' \
            -d '{\"participant_count\": 2, \"participants\": [{\"user_id\": \"client-real-test\", \"role\": \"client\"}, {\"user_id\": \"professional-real-test\", \"role\": \"professional\"}]}'")
    fi
    
    SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.session_id')
    CLIENT_TOKEN=$(echo "$SESSION_RESPONSE" | jq -r '.participants[0].token')
    PRO_TOKEN=$(echo "$SESSION_RESPONSE" | jq -r '.participants[1].token')
    
    if [[ -z "$SESSION_ID" || "$SESSION_ID" == "null" ]]; then
        log_error "Creazione sessione fallita"
        echo "$SESSION_RESPONSE"
        exit 1
    fi
    
    log_success "Sessione creata: $SESSION_ID"
    
    # Save session info
    cat > "$RESULTS_DIR/session_info.json" << EOF
{
    "session_id": "$SESSION_ID",
    "client_token": "$CLIENT_TOKEN",
    "professional_token": "$PRO_TOKEN",
    "created_at": "$(date -Iseconds)"
}
EOF
}

# Step 4: Stream audio from both participants
stream_audio() {
    log_info "Step 4: Streaming audio dai partecipanti..."
    
    if [[ "$LOCAL_MODE" == "true" ]]; then
        # Local mode - stream directly
        log_info "Streaming audio client..."
        (
            cd "$PROJECT_ROOT"
            go run test/scripts/streamer.go \
                -url "$WS_URL" \
                -token "$CLIENT_TOKEN" \
                -session "$SESSION_ID" \
                -audio "$PROJECT_ROOT/test/audio/samples/client_conversation.wav" \
                -role "client" \
                > "$RESULTS_DIR/client_stream.log" 2>&1
        ) &
        CLIENT_PID=$!
        
        log_info "Streaming audio professional..."
        (
            cd "$PROJECT_ROOT"
            go run test/scripts/streamer.go \
                -url "$WS_URL" \
                -token "$PRO_TOKEN" \
                -session "$SESSION_ID" \
                -audio "$PROJECT_ROOT/test/audio/samples/professional_conversation.wav" \
                -role "professional" \
                > "$RESULTS_DIR/pro_stream.log" 2>&1
        ) &
        PRO_PID=$!
        
        # Wait for completion
        log_info "Attesa completamento streaming..."
        wait $CLIENT_PID
        wait $PRO_PID
        
    else
        # VM mode - copy and execute on VMs
        log_info "Copia audio nelle VMs..."
        
        # Copy audio files
        tar czf - -C "$PROJECT_ROOT/test/audio/samples" . | \
            nido ssh $CLIENT_VM "tar xzf - -C /tmp/test_audio"
        
        tar czf - -C "$PROJECT_ROOT/test/audio/samples" . | \
            nido ssh $PRO_VM "tar xzf - -C /tmp/test_audio"
        
        # Build streamer on VMs
        log_info "Build audio streamer sulle VMs..."
        
        # Copy streamer code
        tar czf - -C "$PROJECT_ROOT/test/scripts" streamer.go | \
            nido ssh $CLIENT_VM "tar xzf - -C /tmp"
        
        tar czf - -C "$PROJECT_ROOT/test/scripts" streamer.go | \
            nido ssh $PRO_VM "tar xzf - -C /tmp"
        
        # Stream from client VM
        log_info "Avvio streaming dal client..."
        nido ssh $CLIENT_VM "
            export PATH=\$PATH:/usr/local/go/bin
            cd /tmp
            go mod init streamer 2>/dev/null || true
            go get github.com/gorilla/websocket
            go run streamer.go \
                -url '$WS_URL' \
                -token '$CLIENT_TOKEN' \
                -session '$SESSION_ID' \
                -audio '$CLIENT_AUDIO' \
                -role 'client' \
                > /tmp/client_stream.log 2>&1 &
            echo \$! > /tmp/client.pid
        "
        
        # Stream from professional VM
        log_info "Avvio streaming dal professional..."
        nido ssh $PRO_VM "
            export PATH=\$PATH:/usr/local/go/bin
            cd /tmp
            go mod init streamer 2>/dev/null || true
            go get github.com/gorilla/websocket
            go run streamer.go \
                -url '$WS_URL' \
                -token '$PRO_TOKEN' \
                -session '$SESSION_ID' \
                -audio '$PRO_AUDIO' \
                -role 'professional' \
                > /tmp/pro_stream.log 2>&1 &
            echo \$! > /tmp/pro.pid
        "
        
        # Wait for streaming to complete
        log_info "Attesa completamento streaming (~40s)..."
        sleep 40
    fi
    
    log_success "Streaming completato"
}

# Step 5: Wait for transcription
wait_for_transcription() {
    log_info "Step 5: Attesa elaborazione trascrizione..."
    
    sleep 5  # Give time for processing
    
    log_info "Recupero trascrizioni..."
    
    if [[ "$LOCAL_MODE" == "true" ]]; then
        TRANSCRIPTIONS=$(curl -s "$SERVER_URL/v1/transcriptions?session_id=$SESSION_ID" \
            -H "X-API-Key: $API_KEY")
    else
        TRANSCRIPTIONS=$(nido ssh $SERVER_VM "curl -s \"$SERVER_URL/v1/transcriptions?session_id=$SESSION_ID\" \
            -H 'X-API-Key: $API_KEY'")
    fi
    
    # Save transcriptions
    echo "$TRANSCRIPTIONS" | jq . > "$RESULTS_DIR/transcriptions.json"
    
    NUM_SEGMENTS=$(echo "$TRANSCRIPTIONS" | jq '. | length')
    log_success "Trovati $NUM_SEGMENTS segmenti di trascrizione"
    
    if [[ $NUM_SEGMENTS -eq 0 ]]; then
        log_warn "Nessuna trascrizione trovata - il servizio STT potrebbe non essere configurato"
    fi
    
    # Display sample transcriptions
    echo ""
    echo "📝 Esempio trascrizioni:"
    echo "$TRANSCRIPTIONS" | jq -r '.[0:3].text' | while read line; do
        if [[ ! -z "$line" && "$line" != "null" ]]; then
            echo "   - $line"
        fi
    done
    echo ""
}

# Step 6: Generate and validate minutes
generate_minutes() {
    log_info "Step 6: Generazione minutes..."
    
    if [[ "$LOCAL_MODE" == "true" ]]; then
        MINUTES=$(curl -s -X PUT "$SERVER_URL/v1/minutes/$SESSION_ID" \
            -H "Content-Type: application/json" \
            -H "X-API-Key: $API_KEY")
    else
        MINUTES=$(nido ssh $SERVER_VM "curl -s -X PUT \"$SERVER_URL/v1/minutes/$SESSION_ID\" \
            -H 'Content-Type: application/json' \
            -H 'X-API-Key: $API_KEY'")
    fi
    
    # Save minutes
    echo "$MINUTES" | jq . > "$RESULTS_DIR/minutes.json"
    
    log_success "Minutes generati"
    
    # Display summary
    echo ""
    echo "📋 Riassunto Minutes:"
    echo "$MINUTES" | jq -r '{themes: .themes, next_steps: .next_steps}'
    echo ""
}

# Step 7: Collect metrics
collect_metrics() {
    log_info "Step 7: Raccolta metriche..."
    
    if [[ "$LOCAL_MODE" == "true" ]]; then
        METRICS=$(curl -s "$SERVER_URL/metrics")
    else
        METRICS=$(nido ssh $SERVER_VM "curl -s \"$SERVER_URL/metrics\"")
    fi
    
    echo "$METRICS" > "$RESULTS_DIR/metrics.txt"
    
    # Extract key metrics
    SESSIONS_CREATED=$(echo "$METRICS" | grep "aftertalk_sessions_created_total" | tail -1 | awk '{print $2}')
    TRANSCRIPTIONS_GEN=$(echo "$METRICS" | grep "aftertalk_transcriptions_generated_total" | tail -1 | awk '{print $2}')
    MINUTES_GEN=$(echo "$METRICS" | grep "aftertalk_minutes_generated_total" | tail -1 | awk '{print $2}')
    
    log_success "Metriche raccolte"
    
    echo ""
    echo "📊 Metriche chiave:"
    echo "   Sessioni create: ${SESSIONS_CREATED:-0}"
    echo "   Trascrizioni: ${TRANSCRIPTIONS_GEN:-0}"
    echo "   Minutes: ${MINUTES_GEN:-0}"
    echo ""
}

# Step 8: Validate results
validate_results() {
    log_info "Step 8: Validazione risultati..."
    
    VALIDATION_REPORT="$RESULTS_DIR/validation_report.md"
    
    cat > "$VALIDATION_REPORT" << EOF
# Aftertalk Real Audio Test - Validation Report

**Data:** $(date)
**Session ID:** $SESSION_ID
**Modalità:** $([[ "$LOCAL_MODE" == "true" ]] && echo "Locale" || echo "VM con nido")

## Test Summary

### Audio Streams
- Client audio: test/audio/samples/client_conversation.wav
- Professional audio: test/audio/samples/professional_conversation.wav

### Results

EOF
    
    # Check transcriptions
    NUM_SEGMENTS=$(cat "$RESULTS_DIR/transcriptions.json" | jq '. | length')
    
    if [[ $NUM_SEGMENTS -gt 0 ]]; then
        echo "✅ **Trascrizione:** $NUM_SEGMENTS segmenti generati" >> "$VALIDATION_REPORT"
    else
        echo "❌ **Trascrizione:** Nessun segmento generato" >> "$VALIDATION_REPORT"
    fi
    
    # Check minutes
    if [[ -f "$RESULTS_DIR/minutes.json" ]]; then
        THEMES=$(cat "$RESULTS_DIR/minutes.json" | jq '.themes | length')
        echo "✅ **Minutes:** $THEMES temi identificati" >> "$VALIDATION_REPORT"
    else
        echo "❌ **Minutes:** Non generati" >> "$VALIDATION_REPORT"
    fi
    
    # Overall status
    if [[ $NUM_SEGMENTS -gt 0 && -f "$RESULTS_DIR/minutes.json" ]]; then
        echo "" >> "$VALIDATION_REPORT"
        echo "## ✅ TEST PASSED" >> "$VALIDATION_REPORT"
        log_success "Test completato con successo!"
    else
        echo "" >> "$VALIDATION_REPORT"
        echo "## ❌ TEST FAILED" >> "$VALIDATION_REPORT"
        log_warn "Test completato con avvisi - verifica configurazione STT/LLM"
    fi
    
    echo "" >> "$VALIDATION_REPORT"
    echo "## Files Generati" >> "$VALIDATION_REPORT"
    echo "- session_info.json" >> "$VALIDATION_REPORT"
    echo "- transcriptions.json" >> "$VALIDATION_REPORT"
    echo "- minutes.json" >> "$VALIDATION_REPORT"
    echo "- metrics.txt" >> "$VALIDATION_REPORT"
    
    cat "$VALIDATION_REPORT"
}

# Step 9: Cleanup
cleanup() {
    if [[ "$LOCAL_MODE" == "true" ]]; then
        log_info "Modalità locale - nessuna pulizia necessaria"
        return 0
    fi
    
    log_info "Step 9: Pulizia..."
    
    read -p "Eliminare le VMs? (y/N) " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Eliminazione VMs..."
        nido delete $SERVER_VM --force 2>/dev/null || true
        nido delete $CLIENT_VM --force 2>/dev/null || true
        nido delete $PRO_VM --force 2>/dev/null || true
        log_success "VMs eliminate"
    else
        log_info "VMs mantenute per debugging"
        echo "   Per eliminare manualmente: nido delete aftertalk-server --force"
    fi
}

# Main execution
main() {
    log_info "Avvio test audio reale..."
    
    setup_environment
    generate_test_audio
    create_session
    stream_audio
    wait_for_transcription
    generate_minutes
    collect_metrics
    validate_results
    cleanup
    
    echo ""
    echo "🎉 Test completato!"
    echo ""
    echo "📁 Risultati salvati in: $RESULTS_DIR"
    echo ""
    echo "Per visualizzare i risultati:"
    echo "   cat $RESULTS_DIR/validation_report.md"
    echo "   cat $RESULTS_DIR/transcriptions.json | jq ."
    echo "   cat $RESULTS_DIR/minutes.json | jq ."
}

# Show help
show_help() {
    cat << EOF
Aftertalk Real Audio Test

Usage: $0 [OPTIONS]

Options:
  --local     Esegui test in modalità locale (senza VMs)
  --help      Mostra questo messaggio

Esempi:
  # Test con VMs nido
  $0

  # Test in locale (server già in esecuzione)
  $0 --local

Risultati:
  I risultati sono salvati in: test/results/

EOF
}

# Parse arguments
case "$1" in
    --help|-h)
        show_help
        exit 0
        ;;
    --local)
        LOCAL_MODE="true"
        ;;
    *)
        LOCAL_MODE="false"
        ;;
esac

# Run main
main
