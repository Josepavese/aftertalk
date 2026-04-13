#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[$(date '+%H:%M:%S')] $*"
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

log "Building TypeScript SDK"
(cd sdk/ts && npm run build)

log "Building test UI"
(cd cmd/test-ui && npm run build)

log "Building Go server"
go build -o bin/aftertalk ./cmd/aftertalk

log "Build completed"

if [[ "${AFTERTALK_DEV_AUTO_COMMIT:-0}" == "1" ]]; then
  log "AFTERTALK_DEV_AUTO_COMMIT=1 -> creating local checkpoint commit"
  git add -A
  git commit -m "chore(dev): local checkpoint" || true
fi
