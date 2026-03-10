#!/usr/bin/env bash
# _platform.sh — PAL: Provider layer for OS/package-manager detection.
# Source this file; do not execute directly.
#
# Exports after sourcing:
#   AT_OS          linux | macos | windows
#   AT_ARCH        amd64 | arm64 | arm
#   AT_PKG         apt | brew | pacman | dnf | apk | none
#   AT_HOME        default AFTERTALK_HOME if not set by caller
#   install_pkg    function: install_pkg <name> [<apt-pkg> <brew-pkg> ...]

# ── OS detection ──────────────────────────────────────────────────────────
case "$(uname -s)" in
  Linux*)   AT_OS=linux ;;
  Darwin*)  AT_OS=macos ;;
  MINGW*|MSYS*|CYGWIN*) AT_OS=windows ;;
  *)        AT_OS=unknown ;;
esac

# ── Architecture normalisation ─────────────────────────────────────────────
case "$(uname -m 2>/dev/null || echo unknown)" in
  x86_64|amd64)   AT_ARCH=amd64 ;;
  aarch64|arm64)  AT_ARCH=arm64 ;;
  armv7l|armv6l)  AT_ARCH=arm ;;
  *)               AT_ARCH=unknown ;;
esac

# ── Package-manager detection ──────────────────────────────────────────────
if   command -v apt-get  &>/dev/null; then AT_PKG=apt
elif command -v brew     &>/dev/null; then AT_PKG=brew
elif command -v pacman   &>/dev/null; then AT_PKG=pacman
elif command -v dnf      &>/dev/null; then AT_PKG=dnf
elif command -v apk      &>/dev/null; then AT_PKG=apk
else                                       AT_PKG=none
fi

# ── Default home ──────────────────────────────────────────────────────────
AT_HOME="${AFTERTALK_HOME:-$HOME/.aftertalk}"

# ── Generic install helper ────────────────────────────────────────────────
# Usage: install_pkg <display-name> [apt-name] [brew-name] [pacman-name] [dnf-name] [apk-name]
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
