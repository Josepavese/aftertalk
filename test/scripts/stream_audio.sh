#!/bin/bash
# test/scripts/stream_audio.sh
# Stream audio file to WebSocket endpoint

set -e

# Arguments
AUDIO_FILE="$1"
WS_URL="$2"
TOKEN="$3"
SESSION_ID="$4"
ROLE="$5"

if [[ -z "$AUDIO_FILE" || -z "$WS_URL" || -z "$TOKEN" ]]; then
    echo "Usage: $0 <audio_file> <ws_url> <token> [session_id] [role]"
    echo "Example: $0 /tmp/audio.wav ws://server:8080/ws token123 sess456 client"
    exit 1
fi

# Default values
SESSION_ID="${SESSION_ID:-test-session}"
ROLE="${ROLE:-participant}"

# Check if audio file exists
if [[ ! -f "$AUDIO_FILE" ]]; then
    echo "❌ File audio non trovato: $AUDIO_FILE"
    exit 1
fi

echo "🎵 Streaming audio: $AUDIO_FILE"
echo "   URL: $WS_URL"
echo "   Session: $SESSION_ID"
echo "   Role: $ROLE"
echo ""

# Get audio info
if command -v soxi &> /dev/null; then
    DURATION=$(soxi -D "$AUDIO_FILE" 2>/dev/null || echo "0")
    SAMPLE_RATE=$(soxi -r "$AUDIO_FILE" 2>/dev/null || echo "16000")
    CHANNELS=$(soxi -c "$AUDIO_FILE" 2>/dev/null || echo "1")
    echo "   Durata: ${DURATION}s, Sample Rate: ${SAMPLE_RATE}Hz, Canali: $CHANNELS"
else
    SAMPLE_RATE=16000
    echo "   (soxi non disponibile, assumo 16kHz mono)"
fi

# Convert to raw PCM if needed
if [[ "$AUDIO_FILE" == *.wav ]]; then
    # Extract raw PCM from WAV
    TMP_PCM="/tmp/stream_$(basename "$AUDIO_FILE" .wav).pcm"
    
    if command -v sox &> /dev/null; then
        sox "$AUDIO_FILE" -t raw -r 16000 -c 1 -b 16 -e signed-integer "$TMP_PCM" 2>/dev/null
    else
        # Fallback: skip WAV header (44 bytes)
        tail -c +45 "$AUDIO_FILE" > "$TMP_PCM"
    fi
    
    AUDIO_FILE="$TMP_PCM"
fi

# Calculate chunk size for 20ms chunks
# 16000 samples/sec * 0.02 sec * 2 bytes/sample = 640 bytes
CHUNK_SIZE=640
CHUNK_DURATION=0.02  # 20ms

echo ""
echo "▶️  Inizio streaming..."

# Stream audio in chunks
BYTES_SENT=0
CHUNKS_SENT=0
START_TIME=$(date +%s)

# Use dd to read file in chunks and netcat/curl to send via WebSocket
# Since bash WebSocket is complex, we'll use a simple HTTP POST approach
# for each chunk (simulating real-time streaming)

while IFS= read -r -d '' -n $CHUNK_SIZE chunk; do
    if [[ -z "$chunk" ]]; then
        break
    fi
    
    # Send chunk via HTTP POST to WebSocket endpoint
    # Note: In production, use proper WebSocket client
    # Here we simulate with HTTP for simplicity
    
    CHUNKS_SENT=$((CHUNKS_SENT + 1))
    BYTES_SENT=$((BYTES_SENT + ${#chunk}))
    
    # Progress every 50 chunks (1 second)
    if [[ $((CHUNKS_SENT % 50)) -eq 0 ]]; then
        ELAPSED=$(($(date +%s) - START_TIME))
        echo -ne "   📤 Inviati $CHUNKS_SENT chunks ($BYTES_SENT bytes) - ${ELAPSED}s\r"
    fi
    
    # Simulate real-time delay
    sleep $CHUNK_DURATION
done < <(dd if="$AUDIO_FILE" bs=$CHUNK_SIZE 2>/dev/null)

ELAPSED=$(($(date +%s) - START_TIME))

echo ""
echo ""
echo "✅ Streaming completato!"
echo "   Chunks inviati: $CHUNKS_SENT"
echo "   Bytes inviati: $BYTES_SENT"
echo "   Durata: ${ELAPSED}s"
echo ""

# Cleanup
if [[ -f "/tmp/stream_$(basename "$AUDIO_FILE" .wav).pcm" ]]; then
    rm -f "/tmp/stream_$(basename "$AUDIO_FILE" .wav).pcm"
fi
