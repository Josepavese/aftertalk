#!/usr/bin/env bash
# ============================================================================
# Aftertalk Installer — Linux / macOS
# ============================================================================
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash
#   OR: ./scripts/install.sh
#
# Environment overrides:
#   AFTERTALK_HOME   install directory (default: ~/.aftertalk)
#   WHISPER_MODEL    faster-whisper model (default: base)
#   WHISPER_LANGUAGE transcription language, e.g. "it" (default: auto)
#   OLLAMA_MODEL     LLM model to pull (default: qwen3:4b)
#   SKIP_OLLAMA      set to 1 to skip Ollama install
#   SKIP_WHISPER     set to 1 to skip Whisper server setup
# ============================================================================
set -euo pipefail

REPO_RAW="https://raw.githubusercontent.com/Josepavese/aftertalk/master"
REPO_URL="https://github.com/Josepavese/aftertalk.git"
GO_MIN_VERSION="1.22"
WHISPER_MODEL="${WHISPER_MODEL:-base}"
WHISPER_LANGUAGE="${WHISPER_LANGUAGE:-}"
OLLAMA_MODEL="${OLLAMA_MODEL:-qwen3:4b}"

# ── Colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'
info()    { echo -e "${BLUE}▶${NC} $*"; }
success() { echo -e "${GREEN}✓${NC} $*"; }
warn()    { echo -e "${YELLOW}⚠${NC} $*"; }
error()   { echo -e "${RED}✗${NC} $*" >&2; }
die()     { error "$*"; exit 1; }
header()  { echo -e "\n${BOLD}${BLUE}═══ $* ═══${NC}"; }

# ── Source platform layer ─────────────────────────────────────────────────
# When run via curl the _platform.sh may not be present; fetch or inline.
_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || echo /tmp)"
if [[ -f "$_SCRIPT_DIR/_platform.sh" ]]; then
  # shellcheck source=_platform.sh
  source "$_SCRIPT_DIR/_platform.sh"
else
  # Minimal inline platform detection for curl-pipe installs
  case "$(uname -s)" in Linux*) AT_OS=linux ;; Darwin*) AT_OS=macos ;; *) AT_OS=unknown ;; esac
  case "$(uname -m 2>/dev/null)" in x86_64|amd64) AT_ARCH=amd64 ;; aarch64|arm64) AT_ARCH=arm64 ;; *) AT_ARCH=unknown ;; esac
  if command -v apt-get &>/dev/null; then AT_PKG=apt
  elif command -v brew &>/dev/null; then AT_PKG=brew
  elif command -v pacman &>/dev/null; then AT_PKG=pacman
  elif command -v dnf &>/dev/null; then AT_PKG=dnf
  elif command -v apk &>/dev/null; then AT_PKG=apk
  else AT_PKG=none; fi
  AT_HOME="${AFTERTALK_HOME:-$HOME/.aftertalk}"
  install_pkg() {
    local name="$1" apt_n="${2:-$1}" brew_n="${3:-$1}" pac_n="${4:-$1}" dnf_n="${5:-$1}" apk_n="${6:-$1}"
    info "Installing $name via $AT_PKG..."
    case "$AT_PKG" in
      apt)    sudo apt-get install -y "$apt_n" ;;
      brew)   brew install "$brew_n" ;;
      pacman) sudo pacman -S --noconfirm "$pac_n" ;;
      dnf)    sudo dnf install -y "$dnf_n" ;;
      apk)    sudo apk add "$apk_n" ;;
      none)   die "No package manager found. Install $name manually." ;;
    esac
  }
fi

AFTERTALK_HOME="$AT_HOME"
AFTERTALK_BIN="$AFTERTALK_HOME/bin"
AFTERTALK_DATA="$AFTERTALK_HOME/data"
AFTERTALK_LOGS="$AFTERTALK_HOME/logs"
AFTERTALK_CONFIG="$AFTERTALK_HOME/config"
AFTERTALK_MODELS="$AFTERTALK_HOME/models/whisper"
AFTERTALK_SRC="$AFTERTALK_HOME/src"
CLI_LINK="/usr/local/bin/aftertalk"

echo -e "${BOLD}${GREEN}"
echo "  ╔═══════════════════════════════════╗"
echo "  ║     Aftertalk Installer v1.0      ║"
echo "  ║  AI meeting minutes, local-first  ║"
echo "  ╚═══════════════════════════════════╝"
echo -e "${NC}"
info "OS: $AT_OS / $AT_ARCH  |  Package manager: $AT_PKG"
info "Install home: $AFTERTALK_HOME"

# ── 1. Check / install system prerequisites ───────────────────────────────
header "1. Prerequisites"

# git
if ! command -v git &>/dev/null; then
  install_pkg "git" git git git git git
fi
success "git: $(git --version | head -1)"

# python3
if ! command -v python3 &>/dev/null; then
  install_pkg "python3" python3 python3 python python3 python3
fi
PYTHON=$(command -v python3 || command -v python)
PY_VER=$("$PYTHON" --version 2>&1 | awk '{print $2}')
PY_MAJOR=$(echo "$PY_VER" | cut -d. -f1)
PY_MINOR=$(echo "$PY_VER" | cut -d. -f2)
if [[ "$PY_MAJOR" -lt 3 || ("$PY_MAJOR" -eq 3 && "$PY_MINOR" -lt 9) ]]; then
  die "Python 3.9+ required (found $PY_VER)"
fi
success "python: $PY_VER"

# pip
if ! "$PYTHON" -m pip --version &>/dev/null; then
  install_pkg "pip" python3-pip "python3 pip" python-pip python3-pip py3-pip
fi
success "pip: $("$PYTHON" -m pip --version | awk '{print $2}')"

# ffmpeg (optional but useful for audio debugging)
if ! command -v ffmpeg &>/dev/null; then
  warn "ffmpeg not found — installing (used for audio diagnostics)"
  install_pkg "ffmpeg" ffmpeg ffmpeg ffmpeg ffmpeg ffmpeg
fi
success "ffmpeg: $(ffmpeg -version 2>&1 | head -1 | awk '{print $3}')"

# Go
_need_go=1
if command -v go &>/dev/null; then
  GO_VER=$(go version | awk '{print $3}' | sed 's/go//')
  GO_MAJOR=$(echo "$GO_VER" | cut -d. -f1)
  GO_MINOR=$(echo "$GO_VER" | cut -d. -f2)
  if [[ "$GO_MAJOR" -gt 1 || ("$GO_MAJOR" -eq 1 && "$GO_MINOR" -ge 22) ]]; then
    _need_go=0
    success "go: $GO_VER"
  else
    warn "Go $GO_VER found but $GO_MIN_VERSION+ required — will install"
  fi
fi
if [[ "$_need_go" -eq 1 ]]; then
  GO_LATEST="1.23.4"
  GO_TARBALL="go${GO_LATEST}.${AT_OS}-${AT_ARCH}.tar.gz"
  GO_URL="https://go.dev/dl/${GO_TARBALL}"
  info "Downloading Go $GO_LATEST..."
  curl -fsSL "$GO_URL" -o "/tmp/${GO_TARBALL}"
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "/tmp/${GO_TARBALL}"
  rm "/tmp/${GO_TARBALL}"
  export PATH="/usr/local/go/bin:$PATH"
  # Persist in shell profile
  for f in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
    if [[ -f "$f" ]] && ! grep -q '/usr/local/go/bin' "$f"; then
      echo 'export PATH="/usr/local/go/bin:$PATH"' >> "$f"
    fi
  done
  success "go: $(go version | awk '{print $3}')"
fi

# ── 2. faster-whisper Python package ─────────────────────────────────────
if [[ "${SKIP_WHISPER:-0}" != "1" ]]; then
  header "2. Whisper (faster-whisper)"
  if ! "$PYTHON" -c "import faster_whisper" 2>/dev/null; then
    info "Installing faster-whisper..."
    "$PYTHON" -m pip install --quiet faster-whisper
  fi
  FW_VER=$("$PYTHON" -c "import faster_whisper; print(faster_whisper.__version__)" 2>/dev/null || echo "unknown")
  success "faster-whisper: $FW_VER"
fi

# ── 3. Ollama ─────────────────────────────────────────────────────────────
if [[ "${SKIP_OLLAMA:-0}" != "1" ]]; then
  header "3. Ollama LLM"
  if ! command -v ollama &>/dev/null; then
    info "Installing Ollama..."
    if [[ "$AT_OS" == "macos" ]]; then
      if command -v brew &>/dev/null; then brew install ollama
      else
        warn "Install Ollama manually from https://ollama.com/download"
      fi
    else
      curl -fsSL https://ollama.com/install.sh | sh
    fi
  fi
  success "ollama: $(ollama --version 2>/dev/null | head -1 || echo 'installed')"

  # Start ollama and pull model if not present
  if ! pgrep -x ollama &>/dev/null; then
    info "Starting Ollama service..."
    nohup ollama serve > /dev/null 2>&1 &
    sleep 3
  fi
  if ! ollama list 2>/dev/null | grep -q "^${OLLAMA_MODEL}"; then
    info "Pulling $OLLAMA_MODEL (this may take a while)..."
    ollama pull "$OLLAMA_MODEL"
  fi
  success "model: $OLLAMA_MODEL ready"
fi

# ── 4. Clone / update source & build binary ──────────────────────────────
header "4. Aftertalk source & binary"

if [[ -d "$AFTERTALK_SRC/.git" ]]; then
  info "Updating source in $AFTERTALK_SRC..."
  git -C "$AFTERTALK_SRC" pull --ff-only
else
  info "Cloning into $AFTERTALK_SRC..."
  mkdir -p "$(dirname "$AFTERTALK_SRC")"
  git clone "$REPO_URL" "$AFTERTALK_SRC"
fi

info "Building aftertalk binary..."
(cd "$AFTERTALK_SRC" && go build -o "$AFTERTALK_HOME/bin/aftertalk-server" ./cmd/aftertalk/)
success "Binary: $AFTERTALK_HOME/bin/aftertalk-server"

# Copy whisper server script
cp "$AFTERTALK_SRC/scripts/whisper_server.py" "$AFTERTALK_BIN/whisper_server.py"
chmod +x "$AFTERTALK_BIN/whisper_server.py"
success "Whisper server: $AFTERTALK_BIN/whisper_server.py"

# ── 5. Create directory structure ─────────────────────────────────────────
header "5. Home directory: $AFTERTALK_HOME"
mkdir -p "$AFTERTALK_BIN" "$AFTERTALK_DATA" "$AFTERTALK_LOGS" \
         "$AFTERTALK_CONFIG" "$AFTERTALK_MODELS"
success "Directories created"

# ── 6. Write default config ───────────────────────────────────────────────
CONFIG_FILE="$AFTERTALK_CONFIG/config.yaml"
if [[ ! -f "$CONFIG_FILE" ]]; then
  cat > "$CONFIG_FILE" <<YAML
database:
  path: ${AFTERTALK_DATA}/aftertalk.db

http:
  host: 0.0.0.0
  port: 8080

logging:
  level: info
  format: json

api:
  key: $(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32)

jwt:
  secret: $(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 48)
  issuer: aftertalk
  expiration: 2h

stt:
  provider: whisper-local
  whisperlocal:
    url: http://localhost:9001
    model: ${WHISPER_MODEL}
    language: ${WHISPER_LANGUAGE}
    response_format: verbose_json
    endpoint: /inference

llm:
  provider: ollama
  ollama:
    url: http://localhost:11434
    model: ${OLLAMA_MODEL}

processing:
  chunkSizeMs: 15000
YAML
  success "Config: $CONFIG_FILE"
else
  info "Config exists, skipping (delete to regenerate)"
fi

# ── 7. Write env file ─────────────────────────────────────────────────────
cat > "$AFTERTALK_HOME/.env" <<ENV
AFTERTALK_HOME="${AFTERTALK_HOME}"
WHISPER_MODEL="${WHISPER_MODEL}"
WHISPER_LANGUAGE="${WHISPER_LANGUAGE}"
WHISPER_MODELS_DIR="${AFTERTALK_MODELS}"
OLLAMA_MODEL="${OLLAMA_MODEL}"
PORT=9001
ENV

# ── 8. Install CLI wrapper ─────────────────────────────────────────────────
header "6. CLI command: aftertalk"

CLI_WRAPPER="$AFTERTALK_BIN/aftertalk"
cat > "$CLI_WRAPPER" <<'WRAPPER'
#!/usr/bin/env bash
# aftertalk CLI — manages the Aftertalk server stack
AFTERTALK_HOME="${AFTERTALK_HOME:-$HOME/.aftertalk}"
BIN="$AFTERTALK_HOME/bin"
LOGS="$AFTERTALK_HOME/logs"
PIDFILE_AT="$AFTERTALK_HOME/aftertalk.pid"
PIDFILE_WH="$AFTERTALK_HOME/whisper.pid"
CONFIG="$AFTERTALK_HOME/config/config.yaml"
MODELS="$AFTERTALK_HOME/models/whisper"

_is_running() { [[ -f "$1" ]] && kill -0 "$(cat "$1")" 2>/dev/null; }

cmd_start() {
  echo "▶ Starting Aftertalk stack..."

  if _is_running "$PIDFILE_WH"; then
    echo "  whisper already running (PID $(cat "$PIDFILE_WH"))"
  else
    local wpid
    wpid=$(nohup bash -c "WHISPER_MODELS_DIR='$MODELS' WHISPER_MODEL='${WHISPER_MODEL:-base}' \
      WHISPER_LANGUAGE='${WHISPER_LANGUAGE:-}' PORT='9001' \
      python3 '$BIN/whisper_server.py' >> '$LOGS/whisper.log' 2>&1 & echo \$!" 2>/dev/null)
    echo "$wpid" > "$PIDFILE_WH"
    echo "  ✓ whisper starting (PID $wpid)"
  fi

  echo -n "  Waiting for whisper"
  local tries=0
  until curl -sf "http://localhost:9001/" >/dev/null 2>&1; do
    echo -n "."; sleep 1; tries=$(( tries + 1 ))
    if [[ $tries -ge 30 ]]; then echo " timeout!"; break; fi
  done
  echo ""

  if _is_running "$PIDFILE_AT"; then
    echo "  aftertalk already running (PID $(cat "$PIDFILE_AT"))"
  else
    local apid
    apid=$(nohup bash -c "'$BIN/aftertalk-server' --config '$CONFIG' >> '$LOGS/aftertalk.log' 2>&1 & echo \$!" 2>/dev/null)
    echo "$apid" > "$PIDFILE_AT"
    echo "  ✓ aftertalk starting (PID $apid)"
  fi

  sleep 2
  echo ""; echo "  UI  → http://localhost:8080"; echo "  Log → $LOGS/"
}

cmd_stop() {
  echo "⏹ Stopping..."
  for pf in "$PIDFILE_AT" "$PIDFILE_WH"; do
    [[ ! -f "$pf" ]] && continue
    local pid; pid=$(cat "$pf")
    if kill -0 "$pid" 2>/dev/null; then kill "$pid" && echo "  ✓ stopped (PID $pid)"; fi
    rm -f "$pf"
  done
  local p9001 p8080
  p9001=$(lsof -ti:9001 2>/dev/null || true); [[ -n "$p9001" ]] && kill "$p9001" 2>/dev/null && echo "  ✓ cleared port 9001"
  p8080=$(lsof -ti:8080 2>/dev/null || true); [[ -n "$p8080" ]] && kill "$p8080" 2>/dev/null && echo "  ✓ cleared port 8080"
}

cmd_status() {
  for svc in aftertalk whisper; do
    pf="$AFTERTALK_HOME/${svc}.pid"
    if _is_running "$pf"; then echo "  ✓ $svc   running (PID $(cat "$pf"))"
    else echo "  ✗ $svc   stopped"; fi
  done
  local API_KEY
  API_KEY=$(grep -E '^\s*key:' "$CONFIG" | awk '{print $2}' | head -1)
  curl -sf -H "Authorization: Bearer $API_KEY" http://localhost:8080/v1/health >/dev/null 2>&1 \
    && echo "  API: reachable ✓" || echo "  API: unreachable"
}

cmd_restart() { cmd_stop; sleep 1; cmd_start; }

cmd_update() {
  cmd_stop
  git -C "$AFTERTALK_HOME/src" pull --ff-only
  (cd "$AFTERTALK_HOME/src" && go build -o "$BIN/aftertalk-server" ./cmd/aftertalk/)
  cp "$AFTERTALK_HOME/src/scripts/whisper_server.py" "$BIN/whisper_server.py"
  echo "✓ Updated. Run: aftertalk start"
}

cmd_logs() { exec tail -n 100 -f "$LOGS/${1:-aftertalk}.log"; }

case "${1:-start}" in
  start)   cmd_start ;;
  stop)    cmd_stop ;;
  restart) cmd_restart ;;
  status)  cmd_status ;;
  update)  cmd_update ;;
  logs)    cmd_logs "${2:-aftertalk}" ;;
  *) echo "Usage: aftertalk {start|stop|restart|status|update|logs [aftertalk|whisper]}" ;;
esac
WRAPPER
chmod +x "$CLI_WRAPPER"

# Symlink to /usr/local/bin (or ~/.local/bin as fallback)
if sudo ln -sf "$CLI_WRAPPER" "$CLI_LINK" 2>/dev/null; then
  success "CLI symlink: $CLI_LINK -> $CLI_WRAPPER"
else
  LOCAL_BIN="$HOME/.local/bin"
  mkdir -p "$LOCAL_BIN"
  ln -sf "$CLI_WRAPPER" "$LOCAL_BIN/aftertalk"
  warn "Could not write to /usr/local/bin (no sudo). Installed to $LOCAL_BIN/aftertalk"
  warn "Make sure $LOCAL_BIN is in your PATH:"
  warn "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
fi

# ── 9. Shell PATH persistence ──────────────────────────────────────────────
for RC in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
  if [[ -f "$RC" ]] && ! grep -q 'AFTERTALK_HOME' "$RC"; then
    echo "" >> "$RC"
    echo "# Aftertalk" >> "$RC"
    echo "export AFTERTALK_HOME=\"$AFTERTALK_HOME\"" >> "$RC"
  fi
done

# ── Done ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}${GREEN}╔══════════════════════════════════════╗${NC}"
echo -e "${BOLD}${GREEN}║  ✓ Aftertalk installed successfully  ║${NC}"
echo -e "${BOLD}${GREEN}╚══════════════════════════════════════╝${NC}"
echo ""
echo "  Start:  aftertalk start"
echo "  Stop:   aftertalk stop"
echo "  Status: aftertalk status"
echo "  Logs:   aftertalk logs"
echo "  Update: aftertalk update"
echo ""
echo "  Config: $CONFIG_FILE"
echo "  Home:   $AFTERTALK_HOME"
echo ""
warn "If 'aftertalk' is not found, open a new terminal or run: source ~/.bashrc"
