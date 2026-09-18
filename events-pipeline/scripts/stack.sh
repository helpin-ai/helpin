#!/usr/bin/env bash
set -euo pipefail

# Public DNS names for the local event lab; override to use your own domain and Caddy TLS.
export HELPIN_EVENT_API_HOST="${HELPIN_EVENT_API_HOST:-helpin-dev.localhost}"
export HELPIN_EVENT_LAB_HOST="${HELPIN_EVENT_LAB_HOST:-helpin-dev-fe.localhost}"

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

set -a
if [[ -f "$repo_root/server/.env" ]]; then
  source "$repo_root/server/.env"
fi
if [[ -f "$repo_root/events-pipeline/.env.events" ]]; then
  source "$repo_root/events-pipeline/.env.events"
fi
set +a

runtime_dir=${HELPIN_EVENTS_RUNTIME_DIR:-/tmp/helpin-events-local}
log_dir="$runtime_dir/logs"
pid_dir="$runtime_dir/pids"
compose_file="$repo_root/events-pipeline/local/compose.yaml"
compose=(docker compose --project-name helpin-events-local --file "$compose_file")
capture_dir="$repo_root/events-pipeline/rust-capture"
token_url=${HELPIN_EVENT_TOKEN_URL:-http://127.0.0.1:8080/api/internal/widget-tokens}
caddy_bin=$(command -v caddy || true)
if [[ -x /snap/caddy/current/usr/bin/caddy ]]; then
  caddy_bin=/snap/caddy/current/usr/bin/caddy
fi

mkdir -p "$log_dir" "$pid_dir" "$runtime_dir/fallback" \
  "$runtime_dir/archive-spill" "$runtime_dir/poison-spill"

wait_http() {
  local name=$1
  local url=$2
  local attempts=${3:-60}

  for _ in $(seq 1 "$attempts"); do
    if curl --fail --silent "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  echo "$name did not become ready: $url" >&2
  return 1
}

fetch_token_registry() {
  local args=(--fail --silent --show-error)
  if [[ -n "${INTERNAL_API_SECRET:-}" ]]; then
    args+=(--header "Authorization: Bearer $INTERNAL_API_SECRET")
  fi
  curl "${args[@]}" "$token_url"
}

wait_token_registry() {
  local attempts=${1:-60}
  for _ in $(seq 1 "$attempts"); do
    if fetch_token_registry >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "Helpin widget token registry did not become ready: $token_url" >&2
  return 1
}

pid_is_running() {
  local name=$1
  local pid_file="$pid_dir/$name.pid"
  [[ -f "$pid_file" ]] && kill -0 "$(<"$pid_file")" 2>/dev/null
}

start_process() {
  local name=$1
  shift

  if pid_is_running "$name"; then
    echo "$name is already running (pid $(<"$pid_dir/$name.pid"))"
    return 0
  fi

  nohup "$@" >"$log_dir/$name.log" 2>&1 </dev/null &
  echo "$!" >"$pid_dir/$name.pid"
}

start_process_in_dir() {
  local name=$1
  local directory=$2
  shift 2

  if pid_is_running "$name"; then
    echo "$name is already running (pid $(<"$pid_dir/$name.pid"))"
    return 0
  fi

  (
    cd "$directory"
    nohup "$@" >"$log_dir/$name.log" 2>&1 </dev/null &
    echo "$!" >"$pid_dir/$name.pid"
  )
}

start_process_with_stdin() {
  local name=$1
  local input_file=$2
  shift 2

  if pid_is_running "$name"; then
    echo "$name is already running (pid $(<"$pid_dir/$name.pid"))"
    return 0
  fi

  nohup "$@" <"$input_file" >"$log_dir/$name.log" 2>&1 &
  echo "$!" >"$pid_dir/$name.pid"
}

stop_process() {
  local name=$1
  local pid_file="$pid_dir/$name.pid"
  [[ -f "$pid_file" ]] || return 0

  local pid
  pid=$(<"$pid_file")
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    for _ in $(seq 1 20); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.25
    done
    if kill -0 "$pid" 2>/dev/null; then
      kill -KILL "$pid" 2>/dev/null || true
    fi
  fi
  rm -f "$pid_file"
}

stop_local_processes() {
  for name in dev-caddy event-frontend event-proxy event-replay event-capture session-writer token-server; do
    stop_process "$name"
  done
}

start_stack() {
  wait_token_registry

  local token_response test_workspace_id
  token_response=$(fetch_token_registry)
  test_workspace_id=${HELPIN_EVENT_TEST_WORKSPACE_ID:-${EVENT_TEST_PROJECT_ID:-}}
  mapfile -t event_test_context < <(TOKEN_RESPONSE="$token_response" HELPIN_EVENT_TEST_WORKSPACE_ID="$test_workspace_id" python3 -c '
import json, os
tokens = json.loads(os.environ["TOKEN_RESPONSE"])["tokens"]
workspace = os.environ.get("HELPIN_EVENT_TEST_WORKSPACE_ID", "").strip().lower()
token = next((item for item in tokens if item["workspace_id"].lower() == workspace), None) if workspace else (tokens[0] if len(tokens) == 1 else None)
if token is None:
    raise SystemExit(f"expected one active widget installation or one for workspace {workspace}")
print(token["client_secret"])
print(token["workspace_id"])
')
  unset token_response
  if [[ ${#event_test_context[@]} -ne 2 ]]; then
    echo "Could not select an active widget installation from $token_url" >&2
    return 1
  fi
  local event_test_widget_key=${event_test_context[0]}
  local event_test_project_id=${event_test_context[1]}

  if curl --fail --silent http://127.0.0.1:8222/jsz >/dev/null 2>&1; then
    echo "Reusing the one-node NATS/JetStream server already listening on 4222"
    "${compose[@]}" rm --stop --force nats >/dev/null 2>&1 || true
    "${compose[@]}" up --detach --wait clickhouse
  else
    echo "Starting one-node NATS/JetStream and ClickHouse"
    "${compose[@]}" up --detach --wait nats clickhouse
  fi

  echo "Applying ClickHouse migrations"
  (
    cd "$repo_root/server"
    env CLICKHOUSE_DSN=clickhouse://helpin:helpin@127.0.0.1:9000/default \
      go run ./cmd/clickhouse-migrate up
  )

  echo "Building the pipeline and browser SDK"
  (
    cd "$capture_dir"
    cargo build --bins --jobs "${CARGO_BUILD_JOBS:-2}"
    env \
      NATS_URL=nats://127.0.0.1:4222 \
      EVENTS_WORK_MAX_BYTES=134217728 \
      EVENTS_WORK_REPLICAS=1 \
      EVENTS_RAW_MAX_BYTES=33554432 \
      EVENTS_RAW_REPLICAS=1 \
      EVENTS_DLQ_MAX_BYTES=16777216 \
      EVENTS_DLQ_REPLICAS=1 \
      WRITER_REPLICAS=1 \
      ./target/debug/nats-bootstrap
  )
  pnpm --dir "$repo_root/packages/sdk-js" build

  start_process_in_dir session-writer "$capture_dir" \
    env \
      NATS_URL=nats://127.0.0.1:4222 \
      CLICKHOUSE_HTTP_URL=http://127.0.0.1:8123 \
      CLICKHOUSE_DATABASE=helpin \
      CLICKHOUSE_USER=helpin \
      CLICKHOUSE_PASSWORD=helpin \
      WRITER_REPLICAS=1 \
      WRITER_ORDINAL=0 \
      WRITER_HEALTH_PORT=3010 \
      WRITER_METRICS_PORT=3011 \
      WRITER_POISON_SPILL_DIR="$runtime_dir/poison-spill" \
      RUST_LOG=info \
      "$capture_dir/target/debug/session-writer"
  wait_http "session writer" http://127.0.0.1:3010/health/readiness

  start_process_in_dir event-capture "$capture_dir" \
    env \
      NATS_URL=nats://127.0.0.1:4222 \
      PRINT_SINK=false \
      NETWORK_ENRICHMENT_ENABLED=false \
      HTTP_TOKENS_URL="$token_url" \
      INTERNAL_API_SECRET="${INTERNAL_API_SECRET:-}" \
      FALLBACK_DIR="$runtime_dir/fallback" \
      ARCHIVE_SPILL_DIR="$runtime_dir/archive-spill" \
      LOG_LEVEL=INFO \
      "$capture_dir/target/debug/events-pipeline"
  wait_http "event capture" http://127.0.0.1:3000/health/readiness 120

  start_process_in_dir event-replay "$capture_dir" \
    env \
      NATS_URL=nats://127.0.0.1:4222 \
      NETWORK_ENRICHMENT_ENABLED=false \
      FALLBACK_DIR="$runtime_dir/fallback" \
      REPLAY_POLL_INTERVAL_SECS=5 \
      LOG_LEVEL=INFO \
      "$capture_dir/target/debug/replay-worker"

  start_process event-proxy \
    env HELPIN_SDK_DIST="$repo_root/packages/sdk-js/dist" \
      python3 "$repo_root/events-pipeline/local/proxy.py"
  wait_http "event proxy" http://127.0.0.1:8095/sdk/lib.js

  start_process event-frontend \
    pnpm --dir "$repo_root/frontend" dev -- --host 0.0.0.0 --port 5173
  wait_http "event frontend" http://127.0.0.1:5173/event-test/

  if curl --fail --silent https://$HELPIN_EVENT_API_HOST/sdk/lib.js >/dev/null 2>&1; then
    echo "Reloading the Caddy process already serving the event lab DNS names"
    "$caddy_bin" reload --config - --adapter caddyfile <"$repo_root/Caddyfile.dev"
  else
    start_process_with_stdin dev-caddy "$repo_root/Caddyfile.dev" \
      "$caddy_bin" run --config - --adapter caddyfile
  fi
  wait_http "Caddy event route" https://$HELPIN_EVENT_API_HOST/sdk/lib.js 120

  echo
  echo "Local NATS event pipeline is ready."
  echo "Event lab: https://$HELPIN_EVENT_LAB_HOST/event-test/?key=$event_test_widget_key&host=https%3A%2F%2F$HELPIN_EVENT_API_HOST"
  echo "Workspace: $event_test_project_id"
  echo "Event API: https://$HELPIN_EVENT_API_HOST"

  if [[ "${HELPIN_EVENTS_FOREGROUND:-false}" == "true" ]]; then
    echo "Keeping local event processes attached; press Ctrl-C to stop them."
    trap stop_local_processes EXIT INT TERM
    wait
  fi
}

stop_stack() {
  stop_local_processes
  "${compose[@]}" down --remove-orphans
  echo "Local NATS event pipeline stopped (data volumes retained)."
}

show_status() {
  local failed=0
  if curl --fail --silent http://127.0.0.1:8222/jsz >/dev/null 2>&1; then
    printf '%-18s ready (single JetStream on 4222)\n' nats
  else
    printf '%-18s not ready\n' nats
    failed=1
  fi
  if fetch_token_registry >/dev/null 2>&1; then
    printf '%-18s ready (%s)\n' token-registry "$token_url"
  else
    printf '%-18s not ready (%s)\n' token-registry "$token_url"
    failed=1
  fi
  for entry in \
    "session-writer|http://127.0.0.1:3010/health/readiness" \
    "event-capture|http://127.0.0.1:3000/health/readiness" \
    "event-proxy|http://127.0.0.1:8095/sdk/lib.js" \
    "event-frontend|http://127.0.0.1:5173/event-test/"; do
    name=${entry%%|*}
    url=${entry#*|}
    if pid_is_running "$name" && curl --fail --silent "$url" >/dev/null 2>&1; then
      printf '%-18s ready (pid %s)\n' "$name" "$(<"$pid_dir/$name.pid")"
    else
      printf '%-18s not ready\n' "$name"
      failed=1
    fi
  done
  if pid_is_running event-replay; then
    printf '%-18s running (pid %s)\n' event-replay "$(<"$pid_dir/event-replay.pid")"
  else
    printf '%-18s not running\n' event-replay
    failed=1
  fi
  if curl --fail --silent https://$HELPIN_EVENT_API_HOST/sdk/lib.js >/dev/null 2>&1; then
    printf '%-18s ready (HTTPS/DNS)\n' dev-caddy
  else
    printf '%-18s not ready\n' dev-caddy
    failed=1
  fi
  "${compose[@]}" ps
  return "$failed"
}

show_logs() {
  "${compose[@]}" logs --no-color --tail=50
  touch "$log_dir/stack.log"
  tail -n 100 -F "$log_dir"/*.log
}

case "${1:-}" in
  up) start_stack ;;
  down) stop_stack ;;
  status) show_status ;;
  logs) show_logs ;;
  *)
    echo "usage: $0 {up|down|status|logs}" >&2
    exit 2
    ;;
esac
