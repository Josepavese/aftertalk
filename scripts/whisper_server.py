#!/usr/bin/env python3
"""
Aftertalk Whisper HTTP server.
Serves POST /inference and POST /v1/audio/transcriptions (OpenAI-compatible).

Environment variables:
  WHISPER_MODEL     faster-whisper model size (default: base)
  WHISPER_LANGUAGE  force language, e.g. "it" (default: auto-detect)
  WHISPER_DEVICE    compute device: cpu | cuda | auto (default: auto)
  WHISPER_COMPUTE   compute type: int8 | float16 | float32 (default: int8)
  WHISPER_MODELS_DIR  path to store downloaded models (default: ~/.aftertalk/models/whisper)
  PORT              HTTP port (default: 9001)
"""
import json, os, sys, tempfile
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path

# ── Configuration (Provider layer: reads environment) ──────────────────────
MODEL_SIZE   = os.environ.get("WHISPER_MODEL", "base")
LANGUAGE     = os.environ.get("WHISPER_LANGUAGE", "")
DEVICE       = os.environ.get("WHISPER_DEVICE", "auto")
COMPUTE_TYPE = os.environ.get("WHISPER_COMPUTE", "int8")
MODELS_DIR   = os.environ.get("WHISPER_MODELS_DIR",
               str(Path.home() / ".aftertalk" / "models" / "whisper"))
PORT         = int(os.environ.get("PORT", "9001"))

# ── Model bootstrap ────────────────────────────────────────────────────────
try:
    from faster_whisper import WhisperModel
except ImportError:
    print("ERROR: faster-whisper not installed. Run: pip install faster-whisper", file=sys.stderr)
    sys.exit(1)

if DEVICE == "auto":
    try:
        import torch
        DEVICE = "cuda" if torch.cuda.is_available() else "cpu"
    except ImportError:
        DEVICE = "cpu"

os.makedirs(MODELS_DIR, exist_ok=True)
print(f"[whisper-server] Loading model='{MODEL_SIZE}' device={DEVICE} compute={COMPUTE_TYPE}", flush=True)
print(f"[whisper-server] Models dir: {MODELS_DIR}", flush=True)

model = WhisperModel(MODEL_SIZE, device=DEVICE, compute_type=COMPUTE_TYPE,
                     download_root=MODELS_DIR)
print(f"[whisper-server] Ready on :{PORT}", flush=True)


# ── Transcription logic (Middleware layer: normalizes output) ──────────────
def transcribe(audio_path: str) -> dict:
    lang = LANGUAGE or None
    segments_gen, info = model.transcribe(audio_path, language=lang, word_timestamps=True)
    all_segs = []
    for i, seg in enumerate(segments_gen):
        words = [
            {"word": w.word, "start": round(w.start, 3),
             "end": round(w.end, 3), "probability": round(w.probability, 4)}
            for w in (seg.words or [])
        ]
        all_segs.append({
            "id": i, "text": seg.text, "start": round(seg.start, 3),
            "end": round(seg.end, 3), "tokens": [], "words": words,
        })
    text = " ".join(s["text"].strip() for s in all_segs if s["text"].strip())
    duration = all_segs[-1]["end"] if all_segs else 0.0
    return {
        "task": "transcribe",
        "language": info.language,
        "duration": duration,
        "text": text,
        "segments": all_segs,
    }


# ── HTTP handler (Provider layer: HTTP transport) ──────────────────────────
_ENDPOINTS = frozenset(["/inference", "/v1/audio/transcriptions"])

class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print(f"[whisper-server] {fmt % args}", flush=True)

    def _send_json(self, code: int, obj: dict):
        body = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        self._send_json(200, {"status": "ok", "model": MODEL_SIZE})

    def do_POST(self):
        if self.path not in _ENDPOINTS:
            self._send_json(404, {"error": f"not found: {self.path}"}); return

        ct = self.headers.get("Content-Type", "")
        if "multipart/form-data" not in ct:
            self._send_json(400, {"error": "expected multipart/form-data"}); return

        # Extract boundary
        boundary = None
        for token in ct.split(";"):
            token = token.strip()
            if token.startswith("boundary="):
                boundary = token[9:].strip('"').encode()
                break
        if not boundary:
            self._send_json(400, {"error": "missing boundary"}); return

        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)

        # Find file part in multipart body
        file_data = None
        for part in body.split(b"--" + boundary):
            if b'filename=' not in part:
                continue
            sep = part.find(b"\r\n\r\n")
            if sep == -1:
                continue
            chunk = part[sep + 4:]
            if chunk.endswith(b"\r\n"):
                chunk = chunk[:-2]
            file_data = chunk
            break

        if not file_data:
            self._send_json(400, {"error": "no audio file in request"}); return

        # Detect extension from Content-Disposition header or fall back to .wav
        suffix = ".wav"
        for part in body.split(b"--" + boundary):
            if b'filename=' not in part:
                continue
            hdr_end = part.find(b"\r\n\r\n")
            if hdr_end == -1:
                continue
            hdr = part[:hdr_end].decode(errors="replace")
            for tok in hdr.split(";"):
                tok = tok.strip()
                if tok.lower().startswith("filename="):
                    # Strip quotes, CR/LF, and any trailing header tokens
                    fname = tok[9:].strip().strip('"').strip("'").split("\r")[0].split("\n")[0]
                    if "." in fname:
                        ext = fname.rsplit(".", 1)[1].strip()
                        if ext and ext.isalnum():
                            suffix = "." + ext
            break

        with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as f:
            f.write(file_data)
            tmp = f.name

        try:
            result = transcribe(tmp)
        except Exception as exc:
            print(f"[whisper-server] ERROR: {exc}", flush=True)
            self._send_json(500, {"error": str(exc)})
            return
        finally:
            os.unlink(tmp)

        self._send_json(200, result)


if __name__ == "__main__":
    server = HTTPServer(("0.0.0.0", PORT), Handler)
    print(f"[whisper-server] Listening on 0.0.0.0:{PORT}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("[whisper-server] Stopped.", flush=True)
