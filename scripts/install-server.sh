#!/usr/bin/env bash
# install-server.sh — one-liner bootstrap for the aftertalk installer.
#
# Usage (interactive):
#   curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install-server.sh | bash
#
# Usage (non-interactive with env file):
#   INSTALL_ENV=/path/to/install.env \
#     curl -fsSL .../install-server.sh | bash
#
# Usage (agent mode for deploy orchestrator):
#   AGENT_MODE=1 AGENT_PORT=9977 \
#     curl -fsSL .../install-server.sh | bash
#
# Environment variables:
#   AFTERTALK_RELEASE  — release tag to download (default: latest stable)
#   INSTALL_ENV        — path to env file for non-interactive install
#   AGENT_MODE         — set to 1 to run installer as HTTP SSE agent
#   AGENT_PORT         — agent port (default 9977)
#   INSTALL_DIR        — directory to unpack installer (default: /tmp/aftertalk-install)
set -euo pipefail

REPO="Josepavese/aftertalk"
RELEASE="${AFTERTALK_RELEASE:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/tmp/aftertalk-install}"
AGENT_PORT="${AGENT_PORT:-9977}"

# ── Detect platform ────────────────────────────────────────────────────────────

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)             echo "Unsupported arch: $ARCH" >&2; exit 1 ;;
esac

INSTALLER_BIN="aftertalk-installer-${OS}-${ARCH}"
SERVER_BIN="aftertalk-${OS}-${ARCH}"

# ── Resolve download URL ───────────────────────────────────────────────────────

if [[ "$RELEASE" == "latest" ]]; then
  BASE_URL="https://github.com/${REPO}/releases/latest/download"
else
  BASE_URL="https://github.com/${REPO}/releases/download/${RELEASE}"
fi

# ── Download ───────────────────────────────────────────────────────────────────

mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

echo "→ Downloading aftertalk server binary..."
curl -fsSL "${BASE_URL}/${SERVER_BIN}" -o aftertalk
chmod +x aftertalk

echo "→ Downloading aftertalk-installer..."
curl -fsSL "${BASE_URL}/${INSTALLER_BIN}" -o aftertalk-installer
chmod +x aftertalk-installer

echo "→ Downloaded to ${INSTALL_DIR}"

# ── Run installer ──────────────────────────────────────────────────────────────

INSTALLER_ARGS=()

if [[ -n "${INSTALL_ENV:-}" ]]; then
  INSTALLER_ARGS+=(--env "$INSTALL_ENV")
fi

if [[ "${AGENT_MODE:-}" == "1" ]]; then
  INSTALLER_ARGS+=(--agent --port "$AGENT_PORT")
fi

echo "→ Starting installer..."
exec "${INSTALL_DIR}/aftertalk-installer" "${INSTALLER_ARGS[@]}"
