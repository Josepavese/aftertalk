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

release_asset_os() {
  if [[ "$AT_OS" == "macos" ]]; then
    printf 'darwin'
  else
    printf '%s' "$AT_OS"
  fi
}

download_release_checksums() {
  local base_url="$1"
  local checksums_file="$2"

  curl -fsSL "${base_url}/checksums.txt" -o "$checksums_file" 2>/dev/null
}

verify_release_checksum() {
  local checksums_file="$1"
  local asset="$2"
  local file="$3"

  if [[ ! -f "$checksums_file" ]]; then
    warn "checksums.txt unavailable; skipping checksum for ${asset}"
    return 0
  fi

  local expected
  expected="$(awk -v asset="$asset" '$2 == asset {print $1}' "$checksums_file")"
  if [[ -z "$expected" ]]; then
    warn "checksum missing for ${asset}; skipping"
    return 0
  fi

  local actual
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$file" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$file" | awk '{print $1}')"
  else
    warn "no SHA256 tool found; skipping checksum for ${asset}"
    return 0
  fi

  if [[ "$actual" != "$expected" ]]; then
    die "Checksum mismatch for ${asset}
  expected: ${expected}
  actual:   ${actual}"
  fi

  success "Checksum verified: ${asset}"
}

step_binary() {
  header "4. Aftertalk binary"

  local release="${AFTERTALK_RELEASE:-latest}"
  local asset_os
  asset_os="$(release_asset_os)"
  local bin_name="aftertalk-${asset_os}-${AT_ARCH}"
  local dest="$AFTERTALK_BIN/aftertalk-server"
  local base_url url

  if [[ "$release" == "latest" ]]; then
    base_url="https://github.com/Josepavese/aftertalk/releases/latest/download"
  else
    base_url="https://github.com/Josepavese/aftertalk/releases/download/${release}"
  fi
  url="${base_url}/${bin_name}"

  mkdir -p "$AFTERTALK_BIN"
  local checksums_file
  checksums_file="$(mktemp)"
  if ! download_release_checksums "$base_url" "$checksums_file"; then
    rm -f "$checksums_file"
    checksums_file=""
    warn "checksums.txt not available for ${release}; downloads will not be hash-verified"
  fi

  info "Downloading aftertalk-server (${asset_os}/${AT_ARCH}, release: ${release})..."
  if curl -fL --progress-bar "$url" -o "$dest"; then
    verify_release_checksum "$checksums_file" "$bin_name" "$dest"
    chmod +x "$dest"
    success "Binary: $dest"
    "$dest" --version 2>/dev/null | sed 's/^/  /' || true
  else
    [[ -n "$checksums_file" ]] && rm -f "$checksums_file"
    die "Failed to download binary.
  URL: $url
  Make sure a release exists at https://github.com/Josepavese/aftertalk/releases
  or set AFTERTALK_RELEASE=edge to use the latest master build."
  fi

  # whisper_server.py is a Python script — download from release assets
  # (it was uploaded alongside the binaries by the release workflow).
  local whisper_url
  whisper_url="${base_url}/whisper_server.py"

  info "Downloading whisper_server.py..."
  if curl -fsSL "$whisper_url" -o "$AFTERTALK_BIN/whisper_server.py" 2>/dev/null; then
    verify_release_checksum "$checksums_file" "whisper_server.py" "$AFTERTALK_BIN/whisper_server.py"
  else
    # Fallback: fetch directly from source if release asset is missing
    warn "whisper_server.py release asset missing; falling back to raw source"
    curl -fsSL "${REPO_RAW}/scripts/whisper_server.py" -o "$AFTERTALK_BIN/whisper_server.py"
  fi
  [[ -n "$checksums_file" ]] && rm -f "$checksums_file"
  chmod +x "$AFTERTALK_BIN/whisper_server.py"
  success "Whisper server: $AFTERTALK_BIN/whisper_server.py"
}
