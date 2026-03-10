---
name: local-test-reinstall
description: >
  Reinstall and restart the full Aftertalk stack locally to test code changes.
  Use this skill whenever you have modified Go source, Python whisper server,
  config defaults, or the CLI wrapper and need to verify the change end-to-end
  before committing or reporting results to the user.
---

# Local Test Reinstall

Apply this skill to rebuild and restart the entire local Aftertalk stack with
current source code, then verify it is functioning before declaring any task done.

## When to apply

- After modifying any Go file in `cmd/`, `internal/`, or `pkg/`.
- After modifying `scripts/whisper_server.py`.
- After changing default config values or the installer.
- Before reporting "it works" to the user on any audio/STT/LLM pipeline task.
- When the user says the demo is broken or empty.

## Stack overview

| Service           | Binary / Script                          | Port |
|-------------------|------------------------------------------|------|
| aftertalk-server  | `~/.aftertalk/bin/aftertalk-server`      | 8080 |
| whisper-server    | `~/.aftertalk/bin/whisper_server.py`     | 9001 |
| Ollama            | system service (`ollama serve`)          | 11434|

Config: `~/.aftertalk/config/config.yaml`
Logs:   `~/.aftertalk/logs/`

## Reinstall procedure

### 1 — Build updated binary

```bash
go build -o ~/.aftertalk/bin/aftertalk-server ./cmd/aftertalk/
```

### 2 — Update whisper server (if changed)

```bash
cp scripts/whisper_server.py ~/.aftertalk/bin/whisper_server.py
```

### 3 — Stop running services

```bash
pkill -f aftertalk-server 2>/dev/null || true
pkill -f whisper_server   2>/dev/null || true
```

### 4 — Start whisper server

```bash
WHISPER_MODEL=tiny \
WHISPER_LANGUAGE=it \
WHISPER_MODELS_DIR=~/.aftertalk/models/whisper \
PORT=9001 \
  python3 ~/.aftertalk/bin/whisper_server.py >> ~/.aftertalk/logs/whisper.log 2>&1 &

# Wait for model load (up to 30s)
for i in $(seq 1 30); do
  curl -sf http://localhost:9001/ >/dev/null 2>&1 && break
  sleep 1
done
```

### 5 — Start aftertalk server

```bash
~/.aftertalk/bin/aftertalk-server \
  --config ~/.aftertalk/config/config.yaml \
  >> ~/.aftertalk/logs/aftertalk.log 2>&1 &
sleep 2
```

### 6 — Verify health

```bash
# Get the API key from config
API_KEY=$(grep 'key:' ~/.aftertalk/config/config.yaml | awk '{print $2}')

curl -sf -H "Authorization: Bearer $API_KEY" http://localhost:8080/v1/health
```

Expected: `{"status":"ok"}`

### 7 — Smoke-test the pipeline (optional but recommended)

```bash
# Create test session
SID=$(curl -sf -X POST http://localhost:8080/test/start \
  -H "Content-Type: application/json" \
  -d '{"code":"SMOKE01","role":"host","name":"Tester"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['session_id'])")

# End it immediately
curl -sf -X POST "http://localhost:8080/v1/sessions/${SID}/end" \
  -H "Authorization: Bearer $API_KEY"

sleep 3

# Get minutes (should be ready with empty arrays — no audio, correct behaviour)
curl -sf "http://localhost:8080/v1/minutes?session_id=${SID}" \
  -H "Authorization: Bearer $API_KEY" \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print('status='+d['status']+' provider='+d['provider'])"
```

Expected: `status=ready provider=ollama`

### 8 — Test whisper transcription directly

```bash
ffmpeg -f lavfi -i "sine=f=440:d=2" -ar 16000 -ac 1 /tmp/smoke.wav -y -loglevel quiet
curl -X POST http://localhost:9001/inference -F "file=@/tmp/smoke.wav" \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print('whisper ok duration='+str(d['duration']))"
```

Expected: valid JSON with `duration` field.

## Shortcuts

If `aftertalk` CLI is in PATH:

```bash
aftertalk restart   # stop + rebuild + start
aftertalk status    # check if services are running
aftertalk logs      # tail aftertalk log
aftertalk logs whisper  # tail whisper log
```

## Debugging checklist

| Symptom                          | Check                                               |
|----------------------------------|-----------------------------------------------------|
| `connection refused :8080`       | aftertalk-server not started or wrong port          |
| `connection refused :9001`       | whisper-server not started / still loading model    |
| minutes all empty arrays         | check whisper log for transcription errors          |
| `Invalid API key`                | UI or curl not sending correct key from config.yaml |
| `opus: create decoder` error     | kazzmir/opus-go build issue — run `go build ./...`  |
| `whisper-local: server returned` | whisper crashed — tail `~/.aftertalk/logs/whisper.log` |

## Important notes

- Never test with `/tmp` paths — they disappear across sessions.
- The whisper model takes **10-20 seconds** to load on first start.
- The `aftertalk stop` command reads PID files from `~/.aftertalk/*.pid`.
- Config is generated once at install; to reset: `rm ~/.aftertalk/config/config.yaml && aftertalk update`.
- The demo UI is at `http://localhost:8080/` (root), not `/demo`.
