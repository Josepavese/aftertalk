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
  export AFTERTALK_EXPECTED_TAG="${AFTERTALK_EXPECTED_TAG:-$RELEASE}"
  if [[ -z "${AFTERTALK_EXPECTED_COMMIT:-}" ]] && command -v git >/dev/null 2>&1; then
    COMMIT="$(git ls-remote "https://github.com/${REPO}.git" "refs/tags/${RELEASE}^{}" 2>/dev/null | awk 'NR==1 {print $1}' || true)"
    if [[ -z "$COMMIT" ]]; then
      COMMIT="$(git ls-remote "https://github.com/${REPO}.git" "refs/tags/${RELEASE}" 2>/dev/null | awk 'NR==1 {print $1}' || true)"
    fi
    if [[ -n "$COMMIT" ]]; then
      export AFTERTALK_EXPECTED_COMMIT="$COMMIT"
    fi
  fi
fi

verify_checksum() {
  local asset="$1"
  local file="$2"

  if [[ ! -f checksums.txt ]]; then
    echo "⚠ checksums.txt unavailable; skipping checksum for ${asset}"
    return 0
  fi

  local expected
  expected="$(awk -v asset="$asset" '$2 == asset {print $1}' checksums.txt)"
  if [[ -z "$expected" ]]; then
    echo "⚠ checksum missing for ${asset}; skipping"
    return 0
  fi

  local actual
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$file" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$file" | awk '{print $1}')"
  else
    echo "⚠ no SHA256 tool found; skipping checksum for ${asset}"
    return 0
  fi

  if [[ "$actual" != "$expected" ]]; then
    echo "Checksum mismatch for ${asset}" >&2
    echo "expected: ${expected}" >&2
    echo "actual:   ${actual}" >&2
    exit 1
  fi

  echo "✓ checksum verified: ${asset}"
}

# ── Download ───────────────────────────────────────────────────────────────────

mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

if ! curl -fsSL "${BASE_URL}/checksums.txt" -o checksums.txt 2>/dev/null; then
  echo "⚠ checksums.txt not available for ${RELEASE}; downloads will not be hash-verified"
  rm -f checksums.txt
fi

echo "→ Downloading aftertalk server binary..."
curl -fsSL "${BASE_URL}/${SERVER_BIN}" -o aftertalk
verify_checksum "$SERVER_BIN" aftertalk
chmod +x aftertalk

echo "→ Downloading aftertalk-installer..."
curl -fsSL "${BASE_URL}/${INSTALLER_BIN}" -o aftertalk-installer
verify_checksum "$INSTALLER_BIN" aftertalk-installer
chmod +x aftertalk-installer

echo "→ Downloaded to ${INSTALL_DIR}"
./aftertalk --version || true
./aftertalk-installer --version || true

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
