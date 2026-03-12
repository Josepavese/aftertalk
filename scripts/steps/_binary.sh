#!/usr/bin/env bash
# Step: Clone/update source repository and build the aftertalk binary.
# Requires: AFTERTALK_SRC, AFTERTALK_BIN, REPO_URL (from installer environment)

step_binary() {
  header "Aftertalk source & binary"

  if [[ -d "$AFTERTALK_SRC/.git" ]]; then
    info "Updating source in $AFTERTALK_SRC..."
    git -C "$AFTERTALK_SRC" pull --ff-only
  else
    info "Cloning into $AFTERTALK_SRC..."
    mkdir -p "$(dirname "$AFTERTALK_SRC")"
    git clone "$REPO_URL" "$AFTERTALK_SRC"
  fi

  info "Building aftertalk binary..."
  (cd "$AFTERTALK_SRC" && go build -o "$AFTERTALK_BIN/aftertalk-server" ./cmd/aftertalk/)
  success "Binary: $AFTERTALK_BIN/aftertalk-server"

  cp "$AFTERTALK_SRC/scripts/whisper_server.py" "$AFTERTALK_BIN/whisper_server.py"
  chmod +x "$AFTERTALK_BIN/whisper_server.py"
  success "Whisper server: $AFTERTALK_BIN/whisper_server.py"
}
