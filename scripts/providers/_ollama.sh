#!/usr/bin/env bash
# Provider: Ollama LLM runtime
# Requires: AT_OS, OLLAMA_MODEL, SKIP_OLLAMA env override

ensure_ollama() {
  [[ "${SKIP_OLLAMA:-0}" == "1" ]] && return

  header "Ollama LLM"
  if ! command -v ollama &>/dev/null; then
    info "Installing Ollama..."
    if [[ "$AT_OS" == "macos" ]]; then
      if command -v brew &>/dev/null; then brew install ollama
      else warn "Install Ollama manually from https://ollama.com/download"; fi
    else
      curl -fsSL https://ollama.com/install.sh | sh
    fi
  fi
  success "ollama: $(ollama --version 2>/dev/null | head -1 || echo 'installed')"

  if ! pgrep -x ollama &>/dev/null; then
    info "Starting Ollama service..."
    nohup ollama serve > /dev/null 2>&1 &
    sleep 3
  fi
  if ! ollama list 2>/dev/null | grep -q "^${OLLAMA_MODEL}"; then
    info "Pulling ${OLLAMA_MODEL} (this may take a while)..."
    ollama pull "$OLLAMA_MODEL"
  fi
  success "model: ${OLLAMA_MODEL} ready"
}
