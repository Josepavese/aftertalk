#!/usr/bin/env bash
# Provider: faster-whisper Python package
# Requires: PYTHON (from _python.sh), SKIP_WHISPER env override

ensure_whisper() {
  [[ "${SKIP_WHISPER:-0}" == "1" ]] && return

  header "Whisper (faster-whisper)"
  if ! "$PYTHON" -c "import faster_whisper" 2>/dev/null; then
    info "Installing faster-whisper..."
    "$PYTHON" -m pip install --quiet faster-whisper
  fi
  local ver
  ver=$("$PYTHON" -c "import faster_whisper; print(faster_whisper.__version__)" 2>/dev/null || echo "unknown")
  success "faster-whisper: $ver"
}
