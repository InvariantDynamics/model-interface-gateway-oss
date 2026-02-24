#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

BASE_URL="${MIG_DEMO_BASE_URL:-http://localhost:8080}"
TENANT="${MIG_DEMO_TENANT:-acme}"
TOPIC="${MIG_DEMO_TOPIC:-observatory.inference.completed}"
CAPABILITY="${MIG_DEMO_CAPABILITY:-observatory.models.infer}"
TOKEN="${MIG_DEMO_TOKEN:-}"

pretty_print() {
  if command -v jq >/dev/null 2>&1; then
    jq .
  else
    cat
  fi
}

usage() {
  cat <<'EOF'
MIG demo helper script.

Usage:
  ./scripts/demo.sh <command> [args]

Commands:
  doctor                 Check that migd is reachable
  hello                  Run HELLO
  discover               Run DISCOVER
  invoke [text]          Run INVOKE (default text: "demo run")
  publish [status]       Run PUBLISH (default status: "done")
  connections            Show active connections for tenant
  sse                    Start SSE subscription (Ctrl-C to stop)
  run                    Execute: doctor -> hello -> discover -> invoke -> publish -> connections
  talk-track             Print a 2-minute narration script
  help                   Show this help

Environment overrides:
  MIG_DEMO_BASE_URL      Default: http://localhost:8080
  MIG_DEMO_TENANT        Default: acme
  MIG_DEMO_TOPIC         Default: observatory.inference.completed
  MIG_DEMO_CAPABILITY    Default: observatory.models.infer
  MIG_DEMO_TOKEN         Optional bearer token (JWT mode)
EOF
}

json_post() {
  local path="$1"
  local body="$2"
  local -a args
  args=(
    -sS
    --fail
    -X POST "${BASE_URL}${path}"
    -H "Content-Type: application/json"
    -H "X-Tenant-ID: ${TENANT}"
    -d "${body}"
  )
  if [[ -n "$TOKEN" ]]; then
    args+=(-H "Authorization: Bearer ${TOKEN}")
  fi
  curl "${args[@]}"
}

json_get() {
  local path="$1"
  local -a args
  args=(
    -sS
    --fail
    "${BASE_URL}${path}"
    -H "X-Tenant-ID: ${TENANT}"
  )
  if [[ -n "$TOKEN" ]]; then
    args+=(-H "Authorization: Bearer ${TOKEN}")
  fi
  curl "${args[@]}"
}

run_doctor() {
  echo "Checking ${BASE_URL}/admin/v0.1/health/conformance ..."
  json_get "/admin/v0.1/health/conformance" | pretty_print
}

run_hello() {
  json_post "/mig/v0.1/hello" "$(cat <<EOF
{"header":{"tenant_id":"${TENANT}"},"supported_versions":["0.1"],"requested_bindings":["http"]}
EOF
)" | pretty_print
}

run_discover() {
  json_post "/mig/v0.1/discover" "$(cat <<EOF
{"header":{"tenant_id":"${TENANT}"}}
EOF
)" | pretty_print
}

run_invoke() {
  local text="${1:-demo run}"
  json_post "/mig/v0.1/invoke/${CAPABILITY}" "$(cat <<EOF
{"header":{"tenant_id":"${TENANT}"},"payload":{"input":"${text}"}}
EOF
)" | pretty_print
}

run_publish() {
  local status="${1:-done}"
  json_post "/mig/v0.1/publish/${TOPIC}" "$(cat <<EOF
{"header":{"tenant_id":"${TENANT}"},"payload":{"status":"${status}","job_id":"demo-$(date +%s)"}}
EOF
)" | pretty_print
}

run_connections() {
  json_get "/admin/v0.1/connections?tenant_id=${TENANT}" | pretty_print
}

run_sse() {
  local -a args
  args=(
    -N
    -sS
    "${BASE_URL}/mig/v0.1/subscribe/${TOPIC}"
    -H "X-Tenant-ID: ${TENANT}"
  )
  if [[ -n "$TOKEN" ]]; then
    args+=(-H "Authorization: Bearer ${TOKEN}")
  fi
  echo "Streaming SSE for topic '${TOPIC}' (Ctrl-C to stop)"
  curl "${args[@]}"
}

run_full_demo() {
  echo "Running MIG demo sequence against ${BASE_URL} (tenant=${TENANT})"
  run_doctor
  echo
  echo "HELLO"
  run_hello
  echo
  echo "DISCOVER"
  run_discover
  echo
  echo "INVOKE"
  run_invoke "recording sample"
  echo
  echo "PUBLISH"
  run_publish "done"
  echo
  echo "CONNECTIONS"
  run_connections
}

print_talk_track() {
  cat <<'EOF'
2-minute MIG demo talk track

00:00 - 00:20
- "This is MIG Core running as a standalone gateway called migd."
- "I will show protocol calls, live connections, and the OSS UI."

00:20 - 00:45
- "First, health and conformance are green."
- Run: ./scripts/demo.sh doctor

00:45 - 01:15
- "Now I run HELLO, DISCOVER, and INVOKE on a real capability."
- Run: ./scripts/demo.sh hello
- Run: ./scripts/demo.sh discover
- Run: ./scripts/demo.sh invoke "demo narration"

01:15 - 01:45
- "In a second terminal, I keep a live event subscription open."
- Run there: ./scripts/demo.sh sse
- Back in operator terminal: ./scripts/demo.sh publish done

01:45 - 02:00
- "Finally, active connections are visible by tenant in the admin API."
- Run: ./scripts/demo.sh connections
- "The same runtime view is available in the built-in /ui dashboard."
EOF
}

cmd="${1:-help}"
shift || true

case "$cmd" in
  doctor) run_doctor ;;
  hello) run_hello ;;
  discover) run_discover ;;
  invoke) run_invoke "${1:-demo run}" ;;
  publish) run_publish "${1:-done}" ;;
  connections) run_connections ;;
  sse) run_sse ;;
  run) run_full_demo ;;
  talk-track) print_talk_track ;;
  help|-h|--help) usage ;;
  *)
    echo "Unknown command: $cmd" >&2
    echo >&2
    usage
    exit 1
    ;;
esac
