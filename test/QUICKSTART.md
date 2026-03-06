# Aftertalk Real Audio Test - Quick Start

## 🚀 Run in 3 Simple Steps

### Step 1: Start the Server
```bash
# Use test values to bypass validation
export JWT_SECRET="test-secret-real-audio-2025"
export API_KEY="test-api-key-2024"
go run ./cmd/aftertalk
```

### Step 2: Run the Test
```bash
# In a second terminal
./test/scripts/run_real_audio_test.sh --local
```

### Step 3: View Results
```bash
cat test/results/validation_report.md
```

## ✅ Test Components (All Ready)

- ✅ Audio generation (2 realistic audio files)
- ✅ WebSocket streaming client
- ✅ Test orchestrator
- ✅ VM setup script
- ✅ Validation and reporting

## 📁 Files Created

```
test/
├── audio/samples/
│   ├── client_conversation.wav (1.1MB)
│   ├── professional_conversation.wav (1.1MB)
│   └── *_transcription.txt
├── scripts/
│   ├── generate_audio.go
│   ├── streamer.go
│   ├── setup_vms.sh
│   └── run_real_audio_test.sh
├── results/              # Created after test run
├── README.md            # Full documentation
├── STATUS.md            # Status report
└── IMPLEMENTATION_COMPLETE.md
```

## 🎯 What's Tested

1. **Audio Streaming** - Real-time WebSocket streaming (20ms chunks)
2. **Session Management** - Create with 2 participants
3. **Authentication** - JWT token validation
4. **STT Processing** - Audio to text transcription
5. **AI Minutes** - Structured minutes generation
6. **Data Persistence** - SQLite database operations

## 📊 Expected Output

```
🎬 Aftertalk Real Audio Test
==============================

[SUCCESS] Sessione creata: sess-abc-123
[SUCCESS] Streaming completato
[SUCCESS] Trovati 12 segmenti di trascrizione
[SUCCESS] Minutes generati
[SUCCESS] Metriche raccolte

## ✅ TEST PASSED
```

## 🔍 Check Status

```bash
# List all test scripts
ls -la test/scripts/

# Check audio files
ls -lh test/audio/samples/

# View documentation
cat test/README.md
```

## 🆘 Need Help?

```bash
# Quick fix for server validation
./test/scripts/quick-fix.sh

# Run test (requires server running)
./test/scripts/run_real_audio_test.sh --local
```

---

**Status**: 100% Complete - Ready to run!
