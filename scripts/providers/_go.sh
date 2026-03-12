#!/usr/bin/env bash
# Provider: Go installation
# Requires: AT_OS, AT_ARCH, GO_MIN_VERSION (from installer environment)

ensure_go() {
  local need_go=1
  if command -v go &>/dev/null; then
    local ver major minor
    ver=$(go version | awk '{print $3}' | sed 's/go//')
    major=$(echo "$ver" | cut -d. -f1)
    minor=$(echo "$ver" | cut -d. -f2)
    if [[ "$major" -gt 1 || ("$major" -eq 1 && "$minor" -ge 22) ]]; then
      need_go=0
      success "go: $ver"
    else
      warn "Go $ver found but ${GO_MIN_VERSION}+ required — will install"
    fi
  fi

  if [[ "$need_go" -eq 1 ]]; then
    local latest="1.23.4"
    local tarball="go${latest}.${AT_OS}-${AT_ARCH}.tar.gz"
    local url="https://go.dev/dl/${tarball}"
    info "Downloading Go ${latest}..."
    curl -fsSL "$url" -o "/tmp/${tarball}"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "/tmp/${tarball}"
    rm "/tmp/${tarball}"
    export PATH="/usr/local/go/bin:$PATH"
    for f in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
      if [[ -f "$f" ]] && ! grep -q '/usr/local/go/bin' "$f"; then
        echo 'export PATH="/usr/local/go/bin:$PATH"' >> "$f"
      fi
    done
    success "go: $(go version | awk '{print $3}')"
  fi
}
