#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
compose_file="$repo_root/events-pipeline/e2e/compose.yaml"
capture_dir="$repo_root/events-pipeline/rust-capture"
fixture_dir="$repo_root/events-pipeline/e2e/fixtures"
rate=${SUSTAINED_RATE:-300}
duration=${SUSTAINED_DURATION_SECONDS:-900}
batch_size=${SUSTAINED_BATCH_SIZE:-10}
workers=${SUSTAINED_WORKERS:-64}
visitors=${SUSTAINED_VISITORS:-10000}
load_driver=${SUSTAINED_LOAD_DRIVER:-python}
connections=${SUSTAINED_CONNECTIONS:-70}
source_label=${SUSTAINED_SOURCE_LABEL:-}
if [[ -z "$source_label" ]]; then
  if [[ "$load_driver" == "k6-connections" ]]; then
    source_label=k6-capture
  else
    source_label=sustained-e2e
  fi
fi
network_enrichment=${SUSTAINED_NETWORK_ENRICHMENT_ENABLED:-false}
capture_env_file=${SUSTAINED_CAPTURE_ENV_FILE:-}
ip2proxy_db_path=${SUSTAINED_IP2PROXY_DB_PATH:-}
require_ip2proxy=${SUSTAINED_REQUIRE_IP2PROXY:-false}
work_compression=${SUSTAINED_WORK_COMPRESSION:-s2}
work_max_bytes=${SUSTAINED_WORK_MAX_BYTES:-1073741824}
expected=$((rate * duration))
run_id="sustained-$(date -u +%Y%m%dT%H%M%SZ)-$$"
run_dir=${SUSTAINED_RESULTS_DIR:-"/tmp/helpin-$run_id"}
source "$repo_root/events-pipeline/scripts/e2e-nats-tls.sh"
export E2E_NATS_TLS_DIR="$run_dir/nats-tls"
compose=(docker compose --project-name helpin-event-sustained --file "$compose_file")
capture_pid=""
writer_pid=""
token_pid=""
monitor_pid=""
k6_status=0

mkdir -p "$run_dir"
prepare_e2e_nats_tls "$E2E_NATS_TLS_DIR"
if [[ -n "$ip2proxy_db_path" && ! -f "$ip2proxy_db_path" ]]; then
  echo "SUSTAINED_IP2PROXY_DB_PATH does not exist: $ip2proxy_db_path" >&2
  exit 1
fi
if [[ "$require_ip2proxy" == "true" && -z "$ip2proxy_db_path" ]]; then
  echo "SUSTAINED_REQUIRE_IP2PROXY=true requires SUSTAINED_IP2PROXY_DB_PATH" >&2
  exit 1
fi

cleanup() {
  status=$?
  trap - EXIT
  for pid in "$monitor_pid" "$capture_pid" "$writer_pid" "$token_pid"; do
    if [[ -n "$pid" ]]; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  if [[ $status -ne 0 ]]; then
    echo "Sustained E2E failed; preserving results in $run_dir" >&2
    "${compose[@]}" logs --no-color --tail=100 >"$run_dir/containers.log" 2>&1 || true
    for log in capture writer load; do
      if [[ -f "$run_dir/$log.log" ]]; then
        echo "== $log ==" >&2
        tail -n 50 "$run_dir/$log.log" >&2 || true
      fi
    done
  fi
  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  echo "Sustained E2E results: $run_dir"
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

clickhouse_query() {
  curl --fail-with-body --silent --show-error \
    --user helpin:helpin \
    'http://127.0.0.1:18123/?database=usermaven&default_format=TSVRaw' \
    --data-binary "$1"
}

sample_resources() {
  echo 'unix_time,capture_cpu_percent,capture_rss_kib,writer_cpu_percent,writer_rss_kib,mem_available_kib,swap_free_kib' >"$run_dir/process-resources.csv"
  while true; do
    now=$(date +%s)
    capture_stats=$(ps -p "$capture_pid" -o %cpu=,rss= 2>/dev/null | xargs || true)
    writer_stats=$(ps -p "$writer_pid" -o %cpu=,rss= 2>/dev/null | xargs || true)
    capture_cpu=$(awk '{print $1}' <<<"$capture_stats")
    capture_rss=$(awk '{print $2}' <<<"$capture_stats")
    writer_cpu=$(awk '{print $1}' <<<"$writer_stats")
    writer_rss=$(awk '{print $2}' <<<"$writer_stats")
    mem_available=$(awk '/MemAvailable:/ {print $2}' /proc/meminfo)
    swap_free=$(awk '/SwapFree:/ {print $2}' /proc/meminfo)
    echo "$now,${capture_cpu:-0},${capture_rss:-0},${writer_cpu:-0},${writer_rss:-0},$mem_available,$swap_free" >>"$run_dir/process-resources.csv"
    docker stats --no-stream --format "$now,{{.Name}},{{.CPUPerc}},{{.MemUsage}},{{.PIDs}}" \
      | grep 'helpin-event-sustained-' >>"$run_dir/docker-resources.csv" || true
    sleep 5
  done
}

echo "Preparing sustained test driver=$load_driver rate=$rate events/s duration=${duration}s target_events=$expected batch_size=$batch_size workers=$workers connections=$connections visitors=$visitors"
echo "Results will be retained in $run_dir"
printf 'run_id=%s\nload_driver=%s\nrate=%s\nduration_seconds=%s\ntarget_events=%s\nbatch_size=%s\nworkers=%s\nconnections=%s\nvisitors=%s\nsource_label=%s\nnetwork_enrichment=%s\nrequire_ip2proxy=%s\nwork_compression=%s\nwork_max_bytes=%s\n' \
  "$run_id" "$load_driver" "$rate" "$duration" "$expected" "$batch_size" "$workers" "$connections" "$visitors" "$source_label" "$network_enrichment" "$require_ip2proxy" "$work_compression" "$work_max_bytes" >"$run_dir/test.env"

"${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
"${compose[@]}" up --detach --wait
wait_http "NATS monitor" http://127.0.0.1:18222/healthz
wait_jetstream_cluster

echo "Applying ClickHouse migrations and building binaries"
(
  cd "$repo_root/server"
  env CLICKHOUSE_DSN=clickhouse://helpin:helpin@127.0.0.1:19000/usermaven \
    go run ./cmd/clickhouse-migrate up
)
(
  cd "$capture_dir"
  cargo build --release --bins --jobs "${CARGO_BUILD_JOBS:-2}"
  env \
    NATS_URL=tls://127.0.0.1:14222 \
    NATS_CA_FILE="$E2E_NATS_TLS_DIR/ca.crt" \
    NATS_CLIENT_CERT_FILE="$E2E_NATS_TLS_DIR/client.crt" \
    NATS_CLIENT_KEY_FILE="$E2E_NATS_TLS_DIR/client.key" \
    EVENTS_WORK_COMPRESSION="$work_compression" \
    EVENTS_WORK_MAX_BYTES="$work_max_bytes" \
    EVENTS_RAW_MAX_BYTES=536870912 \
    EVENTS_DLQ_MAX_BYTES=67108864 \
    ./target/release/nats-bootstrap
)

python3 -m http.server --bind 127.0.0.1 --directory "$fixture_dir" 18080 \
  >"$run_dir/token.log" 2>&1 &
token_pid=$!
wait_http "token fixture" http://127.0.0.1:18080/tokens.json

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
    WRITER_REPLICAS=1 \
    WRITER_ORDINAL=0 \
    WRITER_HEALTH_PORT=3010 \
    WRITER_METRICS_PORT=3011 \
    SESSION_CACHE_MAX_ENTRIES_PER_SHARD=100000 \
    WRITER_POISON_SPILL_DIR="$run_dir/poison-spill" \
    WRITER_POISON_SPILL_MAX_BYTES=67108864 \
    RUST_LOG=warn \
    ./target/release/session-writer
) >"$run_dir/writer.log" 2>&1 &
writer_pid=$!
wait_http "session writer" http://127.0.0.1:3010/health/readiness

(
  cd "$capture_dir"
  if [[ -n "$capture_env_file" ]]; then
    set -a
    source "$capture_env_file"
    set +a
  fi
  if [[ -n "$ip2proxy_db_path" ]]; then
    export IP2PROXY_DB_PATH="$ip2proxy_db_path"
  fi
  exec env \
    NATS_URL=tls://127.0.0.1:14222 \
    NATS_CA_FILE="$E2E_NATS_TLS_DIR/ca.crt" \
    NATS_CLIENT_CERT_FILE="$E2E_NATS_TLS_DIR/client.crt" \
    NATS_CLIENT_KEY_FILE="$E2E_NATS_TLS_DIR/client.key" \
    PRINT_SINK=false \
    NETWORK_ENRICHMENT_ENABLED="$network_enrichment" \
    HTTP_TOKENS_URL=http://127.0.0.1:18080/tokens.json \
    FALLBACK_DIR="$run_dir/fallback" \
    ARCHIVE_SPILL_DIR="$run_dir/archive-spill" \
    ARCHIVE_SPILL_MAX_BYTES=67108864 \
    LOG_LEVEL=WARN \
    ./target/release/events-pipeline
) >"$run_dir/capture.log" 2>&1 &
capture_pid=$!
wait_http "event capture" http://127.0.0.1:3000/health/readiness

sample_resources &
monitor_pid=$!

echo "Driving the capture endpoint"
if [[ "$load_driver" == "k6-connections" ]]; then
  chmod 0777 "$run_dir"
  set +e
  docker run --rm --network host \
    --volume "$repo_root/events-pipeline/scripts:/scripts:ro" \
    --volume "$run_dir:/results" \
    --env TARGET_URL=http://127.0.0.1:3000/api/v1/event \
    --env TOKEN=e2e-server-secret \
    --env PROFILE=connections \
    --env EVENT_RATE="$rate" \
    --env CONNECTIONS="$connections" \
    --env DURATION="${duration}s" \
    --env BATCH_SIZE="$batch_size" \
    --env VISITORS="$visitors" \
    --env CLIENT_IP=8.8.8.8 \
    --env 'USER_AGENT=Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36' \
    --env SUMMARY_PATH=/results/k6-summary.json \
    grafana/k6:1.8.1 run /scripts/k6-capture.js \
    >"$run_dir/load.log" 2>&1
  k6_status=$?
  set -e
  expected=$(python3 -c 'import json,sys; print(int(json.load(open(sys.argv[1]))["results"]["events_accepted"]))' "$run_dir/k6-summary.json")
  cp "$run_dir/k6-summary.json" "$run_dir/load-result.json"
elif [[ "$load_driver" == "external" ]]; then
  cat >"$run_dir/load-result.json" <<EOF
{
  "driver": "external",
  "expected_events": $expected,
  "rate": $rate,
  "duration_seconds": $duration
}
EOF
  echo "External generator target is ready on 0.0.0.0:3000; waiting for the first $source_label event"
  first_event_deadline=$(($(date +%s) + 600))
  while [[ "$(clickhouse_query "SELECT count() FROM events FINAL WHERE src = '$source_label'")" == "0" ]]; do
    if (( $(date +%s) >= first_event_deadline )); then
      echo "external generator did not send a $source_label event within 10 minutes" >&2
      exit 1
    fi
    sleep 1
  done
  echo "External traffic detected; monitoring for ${duration}s"
  sleep "$duration"
else
  python3 "$repo_root/events-pipeline/scripts/sustained_load.py" \
    --rate "$rate" \
    --duration "$duration" \
    --batch-size "$batch_size" \
    --workers "$workers" \
    --visitors "$visitors" \
    --run-id "$run_id" \
    --output "$run_dir/load-result.json" \
    >"$run_dir/load.log" 2>&1
fi
printf 'accepted_events=%s\n' "$expected" >>"$run_dir/test.env"
printf 'k6_exit_status=%s\n' "$k6_status" >>"$run_dir/test.env"

echo "Waiting for ClickHouse to reach $expected logical rows"
drain_started=$(date +%s)
logical_rows=0
for _ in $(seq 1 180); do
  logical_rows=$(clickhouse_query "SELECT count() FROM events FINAL WHERE src = '$source_label'")
  if [[ "$logical_rows" == "$expected" ]]; then
    break
  fi
  sleep 1
done
drain_seconds=$(($(date +%s) - drain_started))

curl --fail --silent http://127.0.0.1:3001/metrics >"$run_dir/capture-metrics.prom"
curl --fail --silent http://127.0.0.1:3011/metrics >"$run_dir/writer-metrics.prom"
for port in 18222 18223 18224; do
  curl --fail --silent "http://127.0.0.1:$port/jsz?streams=true&consumers=true" >"$run_dir/jetstream-$port.json"
done
"${compose[@]}" ps >"$run_dir/compose-ps.txt"
"${compose[@]}" logs --no-color >"$run_dir/containers.log" 2>&1

clickhouse_query "
SELECT
  count() AS physical_rows,
  uniqExact(event_id) AS unique_event_ids,
  uniqExact(user_anonymous_id) AS visitors,
  uniqExact(session_id) AS sessions,
  countIf(empty(session_id)) AS rows_without_session
FROM events
WHERE src = '$source_label'
" >"$run_dir/clickhouse-physical.tsv"

clickhouse_query "
SELECT
  count() AS logical_rows,
  uniqExact(event_id) AS unique_event_ids,
  uniqExact(user_anonymous_id) AS visitors,
  uniqExact(session_id) AS sessions,
  countIf(empty(session_id)) AS rows_without_session
FROM events FINAL
WHERE src = '$source_label'
" >"$run_dir/clickhouse-final.tsv"

clickhouse_query "
SELECT count()
FROM
(
  SELECT user_anonymous_id
  FROM events FINAL
  WHERE src = '$source_label'
  GROUP BY user_anonymous_id
  HAVING uniqExact(session_id) != 1
)
" >"$run_dir/visitors-with-wrong-session-count.txt"

clickhouse_query "$(<"$repo_root/events-pipeline/scripts/clickhouse-parts-health.sql")" \
  >"$run_dir/clickhouse-parts-health.tsv"

RUN_DIR="$run_dir" DRAIN_SECONDS="$drain_seconds" python3 - <<'PY'
import csv
import json
import os
import re
from collections import defaultdict
from pathlib import Path

root = Path(os.environ["RUN_DIR"])
load = json.loads((root / "load-result.json").read_text())

with (root / "process-resources.csv").open() as source:
    rows = list(csv.DictReader(source))

def stats(name):
    values = [float(row[name]) for row in rows]
    return {"average": round(sum(values) / len(values), 3), "peak": round(max(values), 3)}

docker = defaultdict(lambda: {"cpu": [], "memory_mib": []})
memory_pattern = re.compile(r"([0-9.]+)([KMG]iB)")
docker_path = root / "docker-resources.csv"
if docker_path.exists():
    for line in docker_path.read_text().splitlines():
        parts = line.split(",", 4)
        if len(parts) != 5:
            continue
        _, name, cpu, memory, _ = parts
        docker[name]["cpu"].append(float(cpu.rstrip("%")))
        used = memory.split(" / ", 1)[0]
        match = memory_pattern.fullmatch(used)
        if match:
            value = float(match.group(1))
            unit = match.group(2)
            docker[name]["memory_mib"].append(value * {"KiB": 1 / 1024, "MiB": 1, "GiB": 1024}[unit])

docker_summary = {}
for name, samples in docker.items():
    docker_summary[name] = {}
    for metric, values in samples.items():
        if values:
            docker_summary[name][metric] = {
                "average": round(sum(values) / len(values), 3),
                "peak": round(max(values), 3),
            }

physical = [int(value) for value in (root / "clickhouse-physical.tsv").read_text().split()]
logical = [int(value) for value in (root / "clickhouse-final.tsv").read_text().split()]
summary = {
    "load": load,
    "drain_seconds": int(os.environ["DRAIN_SECONDS"]),
    "clickhouse": {
        "physical": dict(zip(["rows", "unique_event_ids", "visitors", "sessions", "rows_without_session"], physical)),
        "final": dict(zip(["rows", "unique_event_ids", "visitors", "sessions", "rows_without_session"], logical)),
        "visitors_with_wrong_session_count": int((root / "visitors-with-wrong-session-count.txt").read_text()),
    },
    "host_processes": {
        "capture_cpu_percent": stats("capture_cpu_percent"),
        "capture_rss_mib": {key: round(value / 1024, 3) for key, value in stats("capture_rss_kib").items()},
        "writer_cpu_percent": stats("writer_cpu_percent"),
        "writer_rss_mib": {key: round(value / 1024, 3) for key, value in stats("writer_rss_kib").items()},
        "host_mem_available_mib": {key: round(value / 1024, 3) for key, value in stats("mem_available_kib").items()},
        "host_swap_free_mib": {key: round(value / 1024, 3) for key, value in stats("swap_free_kib").items()},
    },
    "docker": docker_summary,
}
(root / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")
print(json.dumps(summary, indent=2, sort_keys=True))
PY

[[ "$logical_rows" == "$expected" ]]
[[ "$k6_status" == "0" ]]
[[ "$(awk '{print $5}' "$run_dir/clickhouse-final.tsv")" == "0" ]]
[[ "$(cat "$run_dir/visitors-with-wrong-session-count.txt")" == "0" ]]
[[ ! -s "$run_dir/fallback" || -z "$(find "$run_dir/fallback" -type f -print -quit 2>/dev/null)" ]]
[[ ! -s "$run_dir/archive-spill" || -z "$(find "$run_dir/archive-spill" -type f -print -quit 2>/dev/null)" ]]
[[ ! -s "$run_dir/poison-spill" || -z "$(find "$run_dir/poison-spill" -type f -print -quit 2>/dev/null)" ]]

echo "Sustained E2E passed"
