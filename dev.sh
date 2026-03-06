#!/bin/bash
set -e

log() { echo "[$(date '+%H:%M:%S')] $*"; }

build_all() {
    log "Building SDK..."
    cd sdk && npm run build && cd ..
    log "Building Test Platform..."
    cd test-platform && npm run build && cd ..
    log "Building Go..."
    go build -o bin/aftertalk ./cmd/aftertalk
    log "ALL BUILDS OK"
}

git_push() {
    local changes=$(git status --porcelain)
    [ -z "$changes" ] && return
    git add -A
    git commit -m "Auto: $(date '+%Y-%m-%d %H:%M')" || true
    git push origin master
}

build_all
git_push
log "DONE"
