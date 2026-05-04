#!/usr/bin/env bash
# Step: Download the pre-built aftertalk binary from GitHub Releases.
#
# The binary is cross-compiled by the release workflow (.github/workflows/release.yml)
# and uploaded as an asset to https://github.com/Josepavese/aftertalk/releases.
#
# Requires (from installer environment):
#   AT_OS            — "linux" or "macos"
#   AT_ARCH          — "amd64" or "arm64"
#   AFTERTALK_BIN    — destination directory
#   REPO_RAW         — raw.githubusercontent.com base URL for scripts
#
# Optional:
#   AFTERTALK_RELEASE — release tag to download (default: "latest")
#                       Use "edge" for the latest master build.

step_binary() {
  header "4. Aftertalk binary"

  local release="${AFTERTALK_RELEASE:-latest}"
  local bin_name="aftertalk-${AT_OS}-${AT_ARCH}"
  local dest="$AFTERTALK_BIN/aftertalk-server"
  local url

  if [[ "$release" == "latest" ]]; then
    url="https://github.com/Josepavese/aftertalk/releases/latest/download/${bin_name}"
  else
    url="https://github.com/Josepavese/aftertalk/releases/download/${release}/${bin_name}"
  fi

  mkdir -p "$AFTERTALK_BIN"

  info "Downloading aftertalk-server (${AT_OS}/${AT_ARCH}, release: ${release})..."
  if curl -fL --progress-bar "$url" -o "$dest"; then
    chmod +x "$dest"
    success "Binary: $dest"
    "$dest" --version 2>/dev/null | sed 's/^/  /' || true
  else
    die "Failed to download binary.
  URL: $url
  Make sure a release exists at https://github.com/Josepavese/aftertalk/releases
  or set AFTERTALK_RELEASE=edge to use the latest master build."
  fi

  # whisper_server.py is a Python script — download from release assets
  # (it was uploaded alongside the binaries by the release workflow).
  local whisper_url
  if [[ "$release" == "latest" ]]; then
    whisper_url="https://github.com/Josepavese/aftertalk/releases/latest/download/whisper_server.py"
  else
    whisper_url="https://github.com/Josepavese/aftertalk/releases/download/${release}/whisper_server.py"
  fi

  info "Downloading whisper_server.py..."
  if ! curl -fsSL "$whisper_url" -o "$AFTERTALK_BIN/whisper_server.py" 2>/dev/null; then
    # Fallback: fetch directly from source if release asset is missing
    curl -fsSL "${REPO_RAW}/scripts/whisper_server.py" -o "$AFTERTALK_BIN/whisper_server.py"
  fi
  chmod +x "$AFTERTALK_BIN/whisper_server.py"
  success "Whisper server: $AFTERTALK_BIN/whisper_server.py"
}
