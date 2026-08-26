#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
compose_file="$repo_root/events-pipeline/e2e/compose.yaml"
capture_dir="$repo_root/events-pipeline/rust-capture"
fixture_dir="$repo_root/events-pipeline/e2e/fixtures"
run_dir=$(mktemp -d /tmp/helpin-event-e2e.XXXXXX)
source "$repo_root/events-pipeline/scripts/e2e-nats-tls.sh"
export E2E_NATS_TLS_DIR="$run_dir/nats-tls"
prepare_e2e_nats_tls "$E2E_NATS_TLS_DIR"
compose=(docker compose --project-name helpin-event-e2e --file "$compose_file")
capture_pid=""
writer_replicas=${E2E_WRITER_REPLICAS:-1}
writer_pids=()
token_pid=""

cleanup() {
  status=$?
  trap - EXIT
  if [[ $status -ne 0 ]]; then
    echo "E2E failed; recent service logs:" >&2
    for log in token capture writer; do
      if [[ -f "$run_dir/$log.log" ]]; then
        echo "== $log ==" >&2
        tail -n 80 "$run_dir/$log.log" >&2 || true
      fi
    done
    "${compose[@]}" logs --no-color --tail=80 >&2 || true
  fi
  for pid in "$capture_pid" "${writer_pids[@]}" "$token_pid"; do
    if [[ -n "$pid" ]]; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$run_dir"
  exit "$status"
}
trap cleanup EXIT

wait_http() {
  name=$1
  url=$2
  for _ in $(seq 1 120); do
    if curl --fail --silent "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "$name did not become ready: $url" >&2
  return 1
}

clickhouse_query() {
  curl --fail-with-body --silent --show-error \
    --user helpin:helpin \
    'http://127.0.0.1:18123/?database=usermaven&default_format=TSVRaw' \
    --data-binary "$1"
}

wait_for_row_count() {
  expected=$1
  visitor=$2
  for _ in $(seq 1 60); do
    count=$(clickhouse_query "SELECT count() FROM events FINAL WHERE user_anonymous_id = '$visitor'")
    if [[ "$count" == "$expected" ]]; then
      return 0
    fi
    sleep 1
  done
  echo "expected $expected ClickHouse rows for $visitor, found $count" >&2
  return 1
}

stream_messages() {
  stream_name=$1
  STREAM_NAME="$stream_name" python3 -c '
import json, os, urllib.request
for port in (18222, 18223, 18224):
    with urllib.request.urlopen(f"http://127.0.0.1:{port}/jsz?streams=true&consumers=false") as response:
        state = json.load(response)
    for account in state.get("account_details", []):
        for stream in account.get("stream_detail", []):
            if stream.get("name") == os.environ["STREAM_NAME"]:
                print(stream["state"]["messages"])
                raise SystemExit(0)
raise SystemExit("stream not found: " + os.environ["STREAM_NAME"])
'
}

wait_jetstream_cluster() {
  for _ in $(seq 1 60); do
    if python3 -c '
import json, urllib.request
with urllib.request.urlopen("http://127.0.0.1:18222/jsz") as response:
    state = json.load(response)
cluster = state.get("meta_cluster", {})
raise SystemExit(0 if cluster.get("cluster_size") == 3 and cluster.get("leader") else 1)
'; then
      return 0
    fi
    sleep 1
  done
  echo "three-node JetStream cluster did not elect a leader" >&2
  return 1
}

post_event() {
  payload=$1
  curl --fail-with-body --silent --show-error \
    --request POST \
    --header 'content-type: application/json' \
    --header 'x-auth-token: e2e-server-secret' \
    --header 'user-agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/124.0 Safari/537.36' \
    --header 'x-forwarded-for: 203.0.113.42' \
    --data "$payload" \
    http://127.0.0.1:3000/api/v1/event >/dev/null
}

start_writers() {
  writer_pids=()
  for ordinal in $(seq 0 $((writer_replicas - 1))); do
    health_port=$((3010 + ordinal * 2))
    metrics_port=$((health_port + 1))
    (
      cd "$capture_dir"
      exec env \
        NATS_URL=tls://127.0.0.1:14222 \
        NATS_CA_FILE="$E2E_NATS_TLS_DIR/ca.crt" \
        NATS_CLIENT_CERT_FILE="$E2E_NATS_TLS_DIR/client.crt" \
        NATS_CLIENT_KEY_FILE="$E2E_NATS_TLS_DIR/client.key" \
        CLICKHOUSE_HTTP_URL=http://127.0.0.1:18123 \
        CLICKHOUSE_DATABASE=usermaven \
        CLICKHOUSE_USER=helpin \
        CLICKHOUSE_PASSWORD=helpin \
        WRITER_REPLICAS="$writer_replicas" \
        WRITER_ORDINAL="$ordinal" \
        WRITER_HEALTH_PORT="$health_port" \
        WRITER_METRICS_PORT="$metrics_port" \
        SESSION_CACHE_MAX_ENTRIES_PER_SHARD=1000 \
        WRITER_POISON_SPILL_DIR="$run_dir/poison-spill-$ordinal" \
        WRITER_POISON_SPILL_MAX_BYTES=16777216 \
        RUST_LOG=info \
        ./target/debug/session-writer
    ) >"$run_dir/writer-$ordinal.log" 2>&1 &
    writer_pids+=("$!")
  done
  for ordinal in $(seq 0 $((writer_replicas - 1))); do
    health_port=$((3010 + ordinal * 2))
    wait_http "session writer $ordinal" "http://127.0.0.1:$health_port/health/readiness"
  done
}

stop_writers() {
  for pid in "${writer_pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  for pid in "${writer_pids[@]}"; do
    wait "$pid" 2>/dev/null || true
  done
  writer_pids=()
}

echo "Starting disposable NATS and ClickHouse infrastructure"
"${compose[@]}" up --detach --wait
wait_http "NATS monitor" http://127.0.0.1:18222/healthz
wait_jetstream_cluster

echo "Applying ClickHouse migrations and building pipeline binaries"
(
  cd "$repo_root/server"
  env CLICKHOUSE_DSN=clickhouse://helpin:helpin@127.0.0.1:19000/usermaven \
    go run ./cmd/clickhouse-migrate up
)
(
  cd "$capture_dir"
  cargo build --bins --jobs "${CARGO_BUILD_JOBS:-2}"
  env \
    NATS_URL=tls://127.0.0.1:14222 \
    NATS_CA_FILE="$E2E_NATS_TLS_DIR/ca.crt" \
    NATS_CLIENT_CERT_FILE="$E2E_NATS_TLS_DIR/client.crt" \
    NATS_CLIENT_KEY_FILE="$E2E_NATS_TLS_DIR/client.key" \
    EVENTS_WORK_MAX_BYTES=67108864 \
    EVENTS_RAW_MAX_BYTES=33554432 \
    EVENTS_DLQ_MAX_BYTES=16777216 \
    WRITER_REPLICAS="$writer_replicas" \
    ./target/debug/nats-bootstrap
)

python3 -m http.server --bind 127.0.0.1 --directory "$fixture_dir" 18080 \
  >"$run_dir/token.log" 2>&1 &
token_pid=$!
wait_http "token fixture" http://127.0.0.1:18080/tokens.json

start_writers
(
  cd "$capture_dir"
  exec env \
    NATS_URL=tls://127.0.0.1:14222 \
    NATS_CA_FILE="$E2E_NATS_TLS_DIR/ca.crt" \
    NATS_CLIENT_CERT_FILE="$E2E_NATS_TLS_DIR/client.crt" \
    NATS_CLIENT_KEY_FILE="$E2E_NATS_TLS_DIR/client.key" \
    PRINT_SINK=false \
    NETWORK_ENRICHMENT_ENABLED=false \
    HTTP_TOKENS_URL=http://127.0.0.1:18080/tokens.json \
    FALLBACK_DIR="$run_dir/fallback" \
    ARCHIVE_SPILL_DIR="$run_dir/archive-spill" \
    ARCHIVE_SPILL_MAX_BYTES=16777216 \
    LOG_LEVEL=INFO \
    ./target/debug/events-pipeline
) >"$run_dir/capture.log" 2>&1 &
capture_pid=$!
wait_http "event capture" http://127.0.0.1:3000/health/readiness

marker="event-e2e-$(date +%s)-$$"
base_payload=$(MARKER="$marker" python3 -c '
import json, os
marker = os.environ["MARKER"]
print(json.dumps({
    "api_key": "e2e-server-secret",
    "event_type": "page_view",
    "url": "https://example.test/e2e",
    "user": {"anonymous_id": marker, "id": f"user-{marker}"},
    "event_attributes": {"marker": marker},
    "src": "pipeline-e2e"
}))
')
post_event "$base_payload"

second_payload=$(MARKER="$marker" python3 -c '
import json, os
marker = os.environ["MARKER"]
print(json.dumps({
    "api_key": "e2e-server-secret",
    "event_type": "cta_clicked",
    "url": "https://example.test/e2e",
    "user": {"anonymous_id": marker, "id": f"user-{marker}"},
    "src": "pipeline-e2e"
}))
')
post_event "$second_payload"

late_payload=$(MARKER="$marker" python3 -c '
import json, os, time
marker = os.environ["MARKER"]
print(json.dumps({
    "api_key": "e2e-server-secret",
    "event_type": "late_event",
    "user": {"anonymous_id": marker, "id": f"user-{marker}"},
    "timestamp": int(time.time()) - 31 * 60,
    "src": "pipeline-e2e"
}))
')
post_event "$late_payload"

followup_payload=$(MARKER="$marker" python3 -c '
import json, os
marker = os.environ["MARKER"]
print(json.dumps({
    "api_key": "e2e-server-secret",
    "event_type": "followup_event",
    "user": {"anonymous_id": marker, "id": f"user-{marker}"},
    "src": "pipeline-e2e"
}))
')
post_event "$followup_payload"
wait_for_row_count 4 "$marker"

rows=$(clickhouse_query "SELECT event_type, session_id, parsed_ua_ua_family, identity_method, identity_trust FROM events FINAL WHERE user_anonymous_id = '$marker' ORDER BY _written_at")
ROWS="$rows" python3 -c '
import os
rows = [line.split("\t") for line in os.environ["ROWS"].splitlines()]
by_type = {row[0]: row for row in rows}
assert len(rows) == 4, rows
active = by_type["page_view"][1]
assert active
assert by_type["cta_clicked"][1] == active
assert by_type["late_event"][1] != active
assert by_type["followup_event"][1] == active
assert by_type["page_view"][2] == "Chrome"
assert by_type["page_view"][3:] == ["server_event", "verified"]
'

stop_writers
queued_payload=$(MARKER="$marker" python3 -c '
import json, os
marker = os.environ["MARKER"]
print(json.dumps({
    "api_key": "e2e-server-secret",
    "event_type": "queued_while_writer_down",
    "user": {"anonymous_id": marker, "id": f"user-{marker}"},
    "src": "pipeline-e2e"
}))
')
post_event "$queued_payload"
[[ "$(stream_messages EVENTS_ENRICHED_V1)" == "1" ]]
start_writers
wait_for_row_count 5 "$marker"

rows=$(clickhouse_query "SELECT event_type, session_id FROM events FINAL WHERE user_anonymous_id = '$marker' ORDER BY _written_at")
ROWS="$rows" python3 -c '
import os
rows = dict(line.split("\t") for line in os.environ["ROWS"].splitlines())
assert rows["queued_while_writer_down"] == rows["page_view"], rows
'

invalid_payload=$(MARKER="$marker" python3 -c '
import json, os, time
marker = os.environ["MARKER"]
print(json.dumps({
    "api_key": "e2e-server-secret",
    "event_type": "too_old_event",
    "user": {"anonymous_id": f"{marker}-invalid"},
    "timestamp": int(time.time()) - 8 * 24 * 60 * 60,
    "src": "pipeline-e2e"
}))
')
post_event "$invalid_payload"

for _ in $(seq 1 30); do
  if [[ "$(stream_messages EVENTS_ENRICHED_V1)" == "0" \
    && "$(stream_messages EVENTS_RAW_V1)" == "5" \
    && "$(stream_messages EVENTS_DLQ_V1)" == "1" ]]; then
    break
  fi
  sleep 1
done
[[ "$(stream_messages EVENTS_ENRICHED_V1)" == "0" ]]
[[ "$(stream_messages EVENTS_RAW_V1)" == "5" ]]
[[ "$(stream_messages EVENTS_DLQ_V1)" == "1" ]]
[[ "$(clickhouse_query "SELECT count() FROM events FINAL WHERE user_anonymous_id = '$marker-invalid'")" == "0" ]]

echo "Pipeline E2E passed with $writer_replicas writer(s): enrichment, R3 work/DLQ and R1 archive streams, session windows, ClickHouse writes, and restart seeding."
