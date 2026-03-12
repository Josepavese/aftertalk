#!/usr/bin/env bash
# ============================================================================
# Aftertalk Installer — Linux / macOS  (Orchestrator)
# ============================================================================
# Usage:
#   ./scripts/install.sh [--mode=MODE] [options]
#   curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash
#
# Modes (--mode=):
#   local-ai  (default) Install Whisper + Ollama for fully local AI pipeline
#   cloud               Skip local AI; configure cloud STT/LLM providers post-install
#   offline             Skip Whisper + Ollama; use stub providers (no AI output)
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
export GO_MIN_VERSION="1.22"
export WHISPER_MODEL="${WHISPER_MODEL:-base}"
export WHISPER_LANGUAGE="${WHISPER_LANGUAGE:-}"
export OLLAMA_MODEL="${OLLAMA_MODEL:-qwen3:4b}"

# ── Parse --mode flag ─────────────────────────────────────────────────────────
INSTALL_MODE="local-ai"
for _arg in "$@"; do
  case "$_arg" in
    --mode=offline)   INSTALL_MODE=offline  ; export SKIP_WHISPER=1 ; export SKIP_OLLAMA=1 ;;
    --mode=cloud)     INSTALL_MODE=cloud    ; export SKIP_WHISPER=1 ; export SKIP_OLLAMA=1 ;;
    --mode=local-ai)  INSTALL_MODE=local-ai ;;
    --mode=*)         echo "Unknown mode: $_arg. Valid: local-ai | cloud | offline" ; exit 1 ;;
  esac
done
export INSTALL_MODE

# ── Colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'
info()    { echo -e "${BLUE}▶${NC} $*"; }
success() { echo -e "${GREEN}✓${NC} $*"; }
warn()    { echo -e "${YELLOW}⚠${NC} $*"; }
error()   { echo -e "${RED}✗${NC} $*" >&2; }
die()     { error "$*"; exit 1; }
header()  { echo -e "\n${BOLD}${BLUE}═══ $* ═══${NC}"; }
export -f info success warn error die header

# ── Source a module: prefer local file, fallback to GitHub for curl-pipe ─────
_source_module() {
  local rel_path="$1"  # e.g. "providers/_go.sh"
  local local_path="$_SCRIPT_DIR/${rel_path}"
  if [[ -f "$local_path" ]]; then
    # shellcheck disable=SC1090
    source "$local_path"
  else
    source <(curl -fsSL "${REPO_RAW}/scripts/${rel_path}")
  fi
}

# ── Source platform layer ─────────────────────────────────────────────────────
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

# ── Paths ─────────────────────────────────────────────────────────────────────
export AFTERTALK_HOME="$AT_HOME"
export AFTERTALK_BIN="$AFTERTALK_HOME/bin"
export AFTERTALK_DATA="$AFTERTALK_HOME/data"
export AFTERTALK_LOGS="$AFTERTALK_HOME/logs"
export AFTERTALK_CONFIG="$AFTERTALK_HOME/config"
export AFTERTALK_MODELS="$AFTERTALK_HOME/models/whisper"
export AFTERTALK_SRC="$AFTERTALK_HOME/src"
export CLI_LINK="/usr/local/bin/aftertalk"

# Read version from SSOT (internal/version/version.txt).
_VERSION_FILE="$_SCRIPT_DIR/../internal/version/version.txt"
AT_VERSION="$([[ -f "$_VERSION_FILE" ]] && tr -d '[:space:]' < "$_VERSION_FILE" || echo 'dev')"

echo -e "${BOLD}${GREEN}"
printf "  ╔═══════════════════════════════════╗\n"
printf "  ║   Aftertalk Installer %-11s ║\n" "v${AT_VERSION}"
printf "  ║  AI meeting minutes, local-first  ║\n"
printf "  ╚═══════════════════════════════════╝\n"
echo -e "${NC}"
info "OS: $AT_OS / $AT_ARCH  |  Package manager: $AT_PKG"
info "Install home: $AFTERTALK_HOME"
info "Mode: $INSTALL_MODE"

# ── Load providers ────────────────────────────────────────────────────────────
_source_module "providers/_python.sh"
_source_module "providers/_go.sh"
_source_module "providers/_whisper.sh"
_source_module "providers/_ollama.sh"

# ── Load steps ────────────────────────────────────────────────────────────────
_source_module "steps/_binary.sh"
_source_module "steps/_config.sh"
_source_module "steps/_cli.sh"

# ── 1. Prerequisites ──────────────────────────────────────────────────────────
header "1. Prerequisites"

# git (used by step_binary; minimal dep, always needed)
if ! command -v git &>/dev/null; then
  install_pkg "git" git git git git git
fi
success "git: $(git --version | head -1)"

# ffmpeg (optional but useful for audio diagnostics)
if ! command -v ffmpeg &>/dev/null; then
  warn "ffmpeg not found — installing (used for audio diagnostics)"
  install_pkg "ffmpeg" ffmpeg ffmpeg ffmpeg ffmpeg ffmpeg
fi
success "ffmpeg: $(ffmpeg -version 2>&1 | head -1 | awk '{print $3}')"

ensure_python   # providers/_python.sh  → sets $PYTHON
ensure_go       # providers/_go.sh

# ── 2. Whisper ────────────────────────────────────────────────────────────────
ensure_whisper  # providers/_whisper.sh

# ── 3. Ollama ─────────────────────────────────────────────────────────────────
ensure_ollama   # providers/_ollama.sh

# ── 4. Source & binary ────────────────────────────────────────────────────────
step_binary     # steps/_binary.sh

# ── 5. Directory structure ────────────────────────────────────────────────────
header "5. Home directory: $AFTERTALK_HOME"
mkdir -p "$AFTERTALK_BIN" "$AFTERTALK_DATA" "$AFTERTALK_LOGS" \
         "$AFTERTALK_CONFIG" "$AFTERTALK_MODELS"
success "Directories created"

# ── 6. Config ─────────────────────────────────────────────────────────────────
step_config     # steps/_config.sh

# ── 7. Env file ───────────────────────────────────────────────────────────────
cat > "$AFTERTALK_HOME/.env" <<ENV
AFTERTALK_HOME="${AFTERTALK_HOME}"
WHISPER_MODEL="${WHISPER_MODEL}"
WHISPER_LANGUAGE="${WHISPER_LANGUAGE}"
WHISPER_MODELS_DIR="${AFTERTALK_MODELS}"
OLLAMA_MODEL="${OLLAMA_MODEL}"
PORT=9001
ENV

# ── 8. CLI wrapper ────────────────────────────────────────────────────────────
step_cli        # steps/_cli.sh

# ── 9. Shell PATH persistence ─────────────────────────────────────────────────
for RC in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
  if [[ -f "$RC" ]] && ! grep -q 'AFTERTALK_HOME' "$RC"; then
    { echo ""; echo "# Aftertalk"; echo "export AFTERTALK_HOME=\"$AFTERTALK_HOME\""; } >> "$RC"
  fi
done

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}${GREEN}╔══════════════════════════════════════╗${NC}"
echo -e "${BOLD}${GREEN}║  ✓ Aftertalk installed successfully  ║${NC}"
echo -e "${BOLD}${GREEN}╚══════════════════════════════════════╝${NC}"
echo ""
echo "  Mode:   $INSTALL_MODE"
echo "  Start:  aftertalk start"
echo "  Stop:   aftertalk stop"
echo "  Status: aftertalk status"
echo "  Logs:   aftertalk logs"
echo "  Update: aftertalk update"
echo ""
echo "  Config: $AFTERTALK_CONFIG/config.yaml"
echo "  Home:   $AFTERTALK_HOME"
echo ""
if [[ "$INSTALL_MODE" == "cloud" ]]; then
  warn "Mode 'cloud': edit $AFTERTALK_CONFIG/config.yaml to add your STT/LLM API keys."
elif [[ "$INSTALL_MODE" == "offline" ]]; then
  warn "Mode 'offline': stub providers active — no real transcription or minutes generation."
fi
warn "If 'aftertalk' is not found, open a new terminal or run: source ~/.bashrc"
