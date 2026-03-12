#!/usr/bin/env bash
# Provider: Python 3.9+ and pip
# Sets global PYTHON variable; dies if version requirement not met.

ensure_python() {
  if ! command -v python3 &>/dev/null; then
    install_pkg "python3" python3 python3 python python3 python3
  fi
  PYTHON=$(command -v python3 || command -v python)
  local ver major minor
  ver=$("$PYTHON" --version 2>&1 | awk '{print $2}')
  major=$(echo "$ver" | cut -d. -f1)
  minor=$(echo "$ver" | cut -d. -f2)
  if [[ "$major" -lt 3 || ("$major" -eq 3 && "$minor" -lt 9) ]]; then
    die "Python 3.9+ required (found $ver)"
  fi
  success "python: $ver"

  if ! "$PYTHON" -m pip --version &>/dev/null; then
    install_pkg "pip" python3-pip "python3 pip" python-pip python3-pip py3-pip
  fi
  success "pip: $("$PYTHON" -m pip --version | awk '{print $2}')"
}
