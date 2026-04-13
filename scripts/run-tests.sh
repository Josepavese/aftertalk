#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

UI_URL="${AFTERTALK_UI_URL:-http://localhost:8080/}"
HEALTH_URL="${AFTERTALK_HEALTH_URL:-http://localhost:8080/v1/health}"
API_KEY="${AFTERTALK_API_KEY:-}"

say() {
  printf '[%s] %s\n' "$(date '+%H:%M:%S')" "$*"
}

curl_health() {
  if [[ -n "$API_KEY" ]]; then
    curl -fsS -H "Authorization: Bearer $API_KEY" "$HEALTH_URL" >/dev/null
    return
  fi
  curl -fsS "$HEALTH_URL" >/dev/null
}

say "Checking server health at $HEALTH_URL"
if ! curl_health; then
  cat <<'EOF'
Server is not reachable.

Start it in another terminal, for example:
  go build -o bin/aftertalk ./cmd/aftertalk
  AFTERTALK_JWT_SECRET="dev-secret-please-change" \
  AFTERTALK_API_KEY="dev-api-key-please-change" \
  AFTERTALK_STT_PROVIDER=stub \
  AFTERTALK_LLM_PROVIDER=stub \
  ./bin/aftertalk
EOF
  exit 1
fi

say "Opening the test UI at $UI_URL"
if command -v xdg-open >/dev/null 2>&1; then
  xdg-open "$UI_URL" >/dev/null 2>&1 || true
elif command -v open >/dev/null 2>&1; then
  open "$UI_URL" || true
fi

cat <<EOF

Interactive checklist:
1. Open two browser tabs on $UI_URL
2. Join the same room with different roles
3. Verify WebRTC connection and audio flow
4. End the session and wait for minutes generation
5. Inspect the generated summary, phases, sections, and citations

EOF
