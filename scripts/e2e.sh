#!/usr/bin/env bash
# Global end-to-end test: builds API and web, starts both on free ports with throwaway data,
# runs the Playwright suite against them, and always cleans up.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
free_port() { python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()'; }
API_PORT="$(free_port)"
WEB_PORT="$(free_port)"
TOKEN="e2e-$(date +%s)"
PIDS=()

cleanup() {
  for p in "${PIDS[@]:-}"; do [ -n "$p" ] && pkill -P "$p" 2>/dev/null; kill "$p" 2>/dev/null || true; done
  wait 2>/dev/null || true
  if [ "${KEEP:-0}" = 1 ]; then echo "logs kept in $TMP"; else rm -rf "$TMP"; fi
}
trap cleanup EXIT

wait_for() { # url name log
  for _ in $(seq 1 120); do curl -fsS -o /dev/null "$1" 2>/dev/null && return 0; sleep 0.5; done
  echo "$2 did not start; log:"; tail -40 "$3"; exit 1
}

echo "==> building api"
(cd "$ROOT/apps/api" && go build -o "$TMP/forge-api" ./cmd/api)
echo "==> building web"
(cd "$ROOT/apps/web" && npm run build >"$TMP/web-build.log" 2>&1) || { tail -40 "$TMP/web-build.log"; exit 1; }

echo "==> starting api on :$API_PORT"
(cd "$ROOT/apps/api" && exec env -u ANTHROPIC_API_KEY FORGE_ADDR="127.0.0.1:$API_PORT" FORGE_DATA_DIR="$TMP/data" \
  FORGE_WORKSPACES_DIR="$TMP/ws" FORGE_TEMPLATES_DIR="$ROOT/templates" FORGE_API_TOKEN="$TOKEN" \
  FORGE_LLM=fake FORGE_FAKE_DELAY=150ms "$TMP/forge-api" >"$TMP/api.log" 2>&1) &
PIDS+=($!)
wait_for "http://127.0.0.1:$API_PORT/healthz" api "$TMP/api.log"

echo "==> starting web on :$WEB_PORT"
(cd "$ROOT/apps/web" && FORGE_API_URL="http://127.0.0.1:$API_PORT" FORGE_API_TOKEN="$TOKEN" \
  exec npx next start -H 127.0.0.1 -p "$WEB_PORT" >"$TMP/web.log" 2>&1) &
PIDS+=($!)
wait_for "http://127.0.0.1:$WEB_PORT/" web "$TMP/web.log"

echo "==> running playwright"
cd "$ROOT/apps/web"
E2E_BASE_URL="http://127.0.0.1:$WEB_PORT" npx playwright test "$@" || { echo "--- api log tail"; tail -30 "$TMP/api.log"; exit 1; }
