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
  if [[ "$load_driver" == k6-* ]]; then
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
work_storage=${SUSTAINED_WORK_STORAGE:-file}
work_replicas=${SUSTAINED_WORK_REPLICAS:-3}
work_max_bytes=${SUSTAINED_WORK_MAX_BYTES:-1073741824}
work_max_in_flight=${SUSTAINED_WORK_MAX_IN_FLIGHT:-512}
print_sink=${SUSTAINED_PRINT_SINK:-false}
raw_archive_enabled=${SUSTAINED_RAW_ARCHIVE_ENABLED:-true}
consumers_enabled=${SUSTAINED_CONSUMERS_ENABLED:-true}
consumer_memory_storage=${SUSTAINED_CONSUMER_MEMORY_STORAGE:-false}
writer_replicas=${SUSTAINED_WRITER_REPLICAS:-1}
capture_replicas=${SUSTAINED_CAPTURE_REPLICAS:-1}
k6_p50_limit_ms=${SUSTAINED_K6_P50_LIMIT_MS:-10}
k6_p99_limit_ms=${SUSTAINED_K6_P99_LIMIT_MS:-25}
k6_data_profile=${SUSTAINED_K6_DATA_PROFILE:-simple}
k6_realistic_user_agents=${SUSTAINED_K6_REALISTIC_USER_AGENTS:-9999}
k6_realistic_ips=${SUSTAINED_K6_REALISTIC_IPS:-65536}
realistic_min_distinct=${SUSTAINED_REALISTIC_MIN_DISTINCT:-100}
resource_monitor_enabled=${SUSTAINED_RESOURCE_MONITOR_ENABLED:-true}
docker_resource_monitor_enabled=${SUSTAINED_DOCKER_RESOURCE_MONITOR_ENABLED:-true}
jetstream_disk_monitor_enabled=${SUSTAINED_JETSTREAM_DISK_MONITOR_ENABLED:-false}
resource_sample_interval=${SUSTAINED_RESOURCE_SAMPLE_INTERVAL_SECONDS:-5}
clickhouse_storage=${SUSTAINED_CLICKHOUSE_STORAGE:-local}
clickhouse_r2_env_file=${SUSTAINED_CLICKHOUSE_R2_ENV_FILE:-"$repo_root/events-pipeline/e2e/.env.r2"}
clickhouse_r2_keep_data=${SUSTAINED_CLICKHOUSE_R2_KEEP_DATA:-false}
expected=$((rate * duration))
run_id="sustained-$(date -u +%Y%m%dT%H%M%SZ)-$$"
run_dir=${SUSTAINED_RESULTS_DIR:-"/tmp/helpin-$run_id"}
source "$repo_root/events-pipeline/scripts/e2e-nats-tls.sh"
source "$repo_root/events-pipeline/scripts/e2e-host-resources.sh"
export E2E_NATS_TLS_DIR="$run_dir/nats-tls"
compose=(docker compose --project-name helpin-event-sustained --file "$compose_file")
capture_pids=()
capture_urls=()
replay_pids=()
writer_pids=()
token_pid=""
monitor_pid=""
k6_status=0

mkdir -p "$run_dir"
prepare_e2e_nats_tls "$E2E_NATS_TLS_DIR"
if [[ "$clickhouse_storage" != "local" && "$clickhouse_storage" != "r2" ]]; then
  echo "SUSTAINED_CLICKHOUSE_STORAGE must be local or r2" >&2
  exit 1
fi
if [[ "$clickhouse_storage" == "r2" ]]; then
  if [[ ! -f "$clickhouse_r2_env_file" ]]; then
    echo "R2 credential file is missing: $clickhouse_r2_env_file" >&2
    echo "Copy events-pipeline/e2e/.env.r2.example to .env.r2 and populate the S3 credential pair." >&2
    exit 1
  fi
  set -a
  source "$clickhouse_r2_env_file"
  set +a
  : "${CLICKHOUSE_R2_ACCESS_KEY_ID:?CLICKHOUSE_R2_ACCESS_KEY_ID must be set in $clickhouse_r2_env_file}"
  : "${CLICKHOUSE_R2_SECRET_ACCESS_KEY:?CLICKHOUSE_R2_SECRET_ACCESS_KEY must be set in $clickhouse_r2_env_file}"
  : "${CLICKHOUSE_R2_ENDPOINT:?CLICKHOUSE_R2_ENDPOINT must be set in $clickhouse_r2_env_file}"
  if [[ "$CLICKHOUSE_R2_ENDPOINT" != https://*.r2.cloudflarestorage.com/* ]]; then
    echo "CLICKHOUSE_R2_ENDPOINT must be an HTTPS R2 S3 endpoint containing the bucket name" >&2
    exit 1
  fi
  clickhouse_r2_base_endpoint=${CLICKHOUSE_R2_ENDPOINT%/}
  export CLICKHOUSE_R2_ENDPOINT="$clickhouse_r2_base_endpoint/helpin-sustained/$run_id/"
  export CLICKHOUSE_R2_CACHE_MAX_SIZE=${SUSTAINED_CLICKHOUSE_R2_CACHE_MAX_SIZE:-4Gi}
  compose+=(--file "$repo_root/events-pipeline/e2e/compose.r2.yaml")
  printf 'clickhouse_r2_endpoint=%s\n' "$CLICKHOUSE_R2_ENDPOINT" >"$run_dir/r2-prefix.txt"
fi
if [[ -n "$ip2proxy_db_path" && ! -f "$ip2proxy_db_path" ]]; then
  echo "SUSTAINED_IP2PROXY_DB_PATH does not exist: $ip2proxy_db_path" >&2
  exit 1
fi
if [[ "$require_ip2proxy" == "true" && -z "$ip2proxy_db_path" ]]; then
  echo "SUSTAINED_REQUIRE_IP2PROXY=true requires SUSTAINED_IP2PROXY_DB_PATH" >&2
  exit 1
fi
if ! [[ "$writer_replicas" =~ ^[1-9][0-9]*$ ]] || (( writer_replicas > 100 )); then
  echo "SUSTAINED_WRITER_REPLICAS must be between 1 and 100" >&2
  exit 1
fi
if ! [[ "$capture_replicas" =~ ^[1-4]$ ]]; then
  echo "SUSTAINED_CAPTURE_REPLICAS must be between 1 and 4" >&2
  exit 1
fi
if ! [[ "$resource_sample_interval" =~ ^[1-9][0-9]*$ ]]; then
  echo "SUSTAINED_RESOURCE_SAMPLE_INTERVAL_SECONDS must be a positive integer" >&2
  exit 1
fi

cleanup() {
  status=$?
  trap - EXIT
  for pid in "$monitor_pid" "$token_pid"; do
    if [[ -n "$pid" ]]; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  if ((${#capture_pids[@]})); then
    for pid in "${capture_pids[@]}"; do
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    done
  fi
  if ((${#replay_pids[@]})); then
    for pid in "${replay_pids[@]}"; do
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    done
  fi
  if ((${#writer_pids[@]})); then
    for pid in "${writer_pids[@]}"; do
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    done
  fi
  if [[ $status -ne 0 ]]; then
    echo "Sustained E2E failed; preserving results in $run_dir" >&2
    "${compose[@]}" logs --no-color --tail=100 >"$run_dir/containers.log" 2>&1 || true
    for log in load; do
      if [[ -f "$run_dir/$log.log" ]]; then
        echo "== $log ==" >&2
        tail -n 50 "$run_dir/$log.log" >&2 || true
      fi
    done
    for log in "$run_dir"/capture-*.log "$run_dir"/writer-*.log "$run_dir"/replay-*.log; do
      if [[ -f "$log" ]]; then
        echo "== $(basename "$log") ==" >&2
        tail -n 50 "$log" >&2 || true
      fi
    done
  fi
  if [[ "$clickhouse_storage" == "r2" && "$clickhouse_r2_keep_data" != "true" ]]; then
    curl --fail-with-body --silent --show-error \
      --user helpin:helpin \
      'http://127.0.0.1:18123/?database=default' \
      --data-binary 'DROP DATABASE IF EXISTS usermaven SYNC' \
      >"$run_dir/r2-cleanup.log" 2>&1 || \
      echo "R2 cleanup failed; remove the unique prefix recorded in $run_dir/r2-prefix.txt" >&2
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

wait_clickhouse_stable() {
  local container_id
  container_id=$("${compose[@]}" ps -q clickhouse)
  for _ in $(seq 1 120); do
    if docker exec "$container_id" sh -c \
      "tr '\000' ' ' </proc/1/cmdline | grep -q clickhouse-server" >/dev/null 2>&1 && \
      clickhouse_query "SELECT 1" >/dev/null 2>&1; then
      sleep 2
      if clickhouse_query "SELECT 1" >/dev/null 2>&1; then
        return 0
      fi
    fi
    sleep 1
  done
  echo "ClickHouse did not reach a stable post-entrypoint state" >&2
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

work_stream_state() {
  python3 - <<'PY'
import json
import urllib.request

with urllib.request.urlopen("http://127.0.0.1:18222/jsz?streams=true&consumers=true") as response:
    state = json.load(response)
for account in state.get("account_details", []):
    for stream in account.get("stream_detail", []):
        if stream.get("name") == "EVENTS_ENRICHED_V1":
            details = stream.get("state", {})
            print(details.get("messages", 0), details.get("consumer_count", 0))
            raise SystemExit
print(0, -1)
PY
}

work_stream_resources() {
  python3 - <<'PY'
import json
import urllib.request

with urllib.request.urlopen("http://127.0.0.1:18222/jsz?streams=true&consumers=true") as response:
    state = json.load(response)
for account in state.get("account_details", []):
    for stream in account.get("stream_detail", []):
        if stream.get("name") != "EVENTS_ENRICHED_V1":
            continue
        details = stream.get("state", {})
        consumers = stream.get("consumer_detail", [])
        print(
            details.get("messages", 0),
            details.get("bytes", 0),
            sum(consumer.get("num_ack_pending", 0) for consumer in consumers),
            sum(consumer.get("num_pending", 0) for consumer in consumers),
            sum(consumer.get("num_redelivered", 0) for consumer in consumers),
        )
        raise SystemExit
print(0, 0, 0, 0, 0)
PY
}

sample_jetstream_disk() {
  local now=$1
  local ordinal container_id total_kib work_kib raw_kib dlq_kib
  for ordinal in 0 1 2; do
    container_id=$("${compose[@]}" ps -q "nats-$ordinal" 2>/dev/null || true)
    if [[ -z "$container_id" ]]; then
      continue
    fi
    total_kib=$(docker exec "$container_id" du -sk /data 2>/dev/null \
      | awk 'NR == 1 { print $1 }' || true)
    work_kib=$(docker exec "$container_id" du -sk '/data/jetstream/$G/streams/EVENTS_ENRICHED_V1' 2>/dev/null \
      | awk 'NR == 1 { print $1 }' || true)
    raw_kib=$(docker exec "$container_id" du -sk '/data/jetstream/$G/streams/EVENTS_RAW_V1' 2>/dev/null \
      | awk 'NR == 1 { print $1 }' || true)
    dlq_kib=$(docker exec "$container_id" du -sk '/data/jetstream/$G/streams/EVENTS_DLQ_V1' 2>/dev/null \
      | awk 'NR == 1 { print $1 }' || true)
    echo "$now,$ordinal,$container_id,${total_kib:-0},${work_kib:-0},${raw_kib:-0},${dlq_kib:-0}" \
      >>"$run_dir/jetstream-disk.csv"
  done
}

clickhouse_query() {
  curl --fail-with-body --silent --show-error \
    --user helpin:helpin \
    'http://127.0.0.1:18123/?database=usermaven&default_format=TSVRaw' \
    --data-binary "$1"
}

save_capture_metrics() {
  for ordinal in $(seq 0 $((capture_replicas - 1))); do
    metrics_port=$((3001 + ordinal * 2))
    curl --fail --silent "http://127.0.0.1:$metrics_port/metrics" \
      >"$run_dir/capture-metrics-$ordinal.prom"
  done
  cp "$run_dir/capture-metrics-0.prom" "$run_dir/capture-metrics.prom"
}

sample_resources() {
  while true; do
    now=$(date +%s)
    capture_pid_list=$(IFS=,; echo "${capture_pids[*]}")
    capture_stats=$(ps -p "$capture_pid_list" -o %cpu=,rss= 2>/dev/null \
      | awk '{ cpu += $1; rss += $2 } END { print cpu + 0, rss + 0 }')
    if ((${#writer_pids[@]})); then
      writer_pid_list=$(IFS=,; echo "${writer_pids[*]}")
      writer_stats=$(ps -p "$writer_pid_list" -o %cpu=,rss= 2>/dev/null \
        | awk '{ cpu += $1; rss += $2 } END { print cpu + 0, rss + 0 }')
    else
      writer_stats="0 0"
    fi
    capture_cpu=$(awk '{print $1}' <<<"$capture_stats")
    capture_rss=$(awk '{print $2}' <<<"$capture_stats")
    writer_cpu=$(awk '{print $1}' <<<"$writer_stats")
    writer_rss=$(awk '{print $2}' <<<"$writer_stats")
    read -r mem_available swap_free <<<"$(sample_host_memory_kib)"
    echo "$now,${capture_cpu:-0},${capture_rss:-0},${writer_cpu:-0},${writer_rss:-0},$mem_available,$swap_free" >>"$run_dir/process-resources.csv"
    read -r work_messages work_bytes ack_pending consumer_pending redelivered <<<"$(work_stream_resources)"
    echo "$now,$work_messages,$work_bytes,$ack_pending,$consumer_pending,$redelivered" >>"$run_dir/jetstream-resources.csv"
    if [[ "$jetstream_disk_monitor_enabled" == "true" ]]; then
      sample_jetstream_disk "$now"
    fi
    for ordinal in "${!capture_pids[@]}"; do
      read -r cpu rss <<<"$(ps -p "${capture_pids[$ordinal]}" -o %cpu=,rss= 2>/dev/null | xargs || true)"
      echo "$now,capture,$ordinal,${capture_pids[$ordinal]},${cpu:-0},${rss:-0}" >>"$run_dir/process-resources-detailed.csv"
    done
    for ordinal in "${!writer_pids[@]}"; do
      read -r cpu rss <<<"$(ps -p "${writer_pids[$ordinal]}" -o %cpu=,rss= 2>/dev/null | xargs || true)"
      echo "$now,writer,$ordinal,${writer_pids[$ordinal]},${cpu:-0},${rss:-0}" >>"$run_dir/process-resources-detailed.csv"
    done
    for ordinal in "${!replay_pids[@]}"; do
      read -r cpu rss <<<"$(ps -p "${replay_pids[$ordinal]}" -o %cpu=,rss= 2>/dev/null | xargs || true)"
      echo "$now,replay,$ordinal,${replay_pids[$ordinal]},${cpu:-0},${rss:-0}" >>"$run_dir/process-resources-detailed.csv"
    done
    if [[ "$docker_resource_monitor_enabled" == "true" ]]; then
      docker stats --no-stream --format "$now,{{.Name}},{{.CPUPerc}},{{.MemUsage}},{{.PIDs}}" \
        | grep 'helpin-event-sustained-' >>"$run_dir/docker-resources.csv" || true
    fi
    sleep "$resource_sample_interval"
  done
}

assert_no_spill_files() {
  local spill_file
  spill_file=$(find "$run_dir" -maxdepth 2 -type f \( \
    -path "$run_dir/fallback/*" -o \
    -path "$run_dir/fallback-*/*" -o \
    -path "$run_dir/archive-spill/*" -o \
    -path "$run_dir/archive-spill-*/*" -o \
    -path "$run_dir/poison-spill-*/*" \
  \) -print -quit 2>/dev/null)
  if [[ -n "$spill_file" ]]; then
    echo "unexpected spill file: $spill_file" >&2
    return 1
  fi
}

echo "Preparing sustained test driver=$load_driver rate=$rate events/s duration=${duration}s target_events=$expected batch_size=$batch_size data_profile=$k6_data_profile workers=$workers connections=$connections visitors=$visitors print_sink=$print_sink archive=$raw_archive_enabled consumers=$consumers_enabled consumer_memory=$consumer_memory_storage writer_replicas=$writer_replicas capture_replicas=$capture_replicas work_storage=$work_storage work_replicas=$work_replicas work_in_flight=$work_max_in_flight clickhouse_storage=$clickhouse_storage resource_monitor=$resource_monitor_enabled docker_resource_monitor=$docker_resource_monitor_enabled jetstream_disk_monitor=$jetstream_disk_monitor_enabled resource_sample_interval=$resource_sample_interval"
echo "Results will be retained in $run_dir"
printf 'run_id=%s\nload_driver=%s\nrate=%s\nduration_seconds=%s\ntarget_events=%s\nbatch_size=%s\nk6_data_profile=%s\nk6_realistic_user_agents=%s\nk6_realistic_ips=%s\nworkers=%s\nconnections=%s\nvisitors=%s\nsource_label=%s\nnetwork_enrichment=%s\nrequire_ip2proxy=%s\nwork_compression=%s\nwork_storage=%s\nwork_replicas=%s\nwork_max_bytes=%s\nwork_max_in_flight=%s\nraw_archive_enabled=%s\nconsumers_enabled=%s\nconsumer_memory_storage=%s\nwriter_replicas=%s\ncapture_replicas=%s\nclickhouse_storage=%s\nresource_monitor_enabled=%s\ndocker_resource_monitor_enabled=%s\njetstream_disk_monitor_enabled=%s\nresource_sample_interval_seconds=%s\n' \
  "$run_id" "$load_driver" "$rate" "$duration" "$expected" "$batch_size" "$k6_data_profile" "$k6_realistic_user_agents" "$k6_realistic_ips" "$workers" "$connections" "$visitors" "$source_label" "$network_enrichment" "$require_ip2proxy" "$work_compression" "$work_storage" "$work_replicas" "$work_max_bytes" "$work_max_in_flight" "$raw_archive_enabled" "$consumers_enabled" "$consumer_memory_storage" "$writer_replicas" "$capture_replicas" "$clickhouse_storage" "$resource_monitor_enabled" "$docker_resource_monitor_enabled" "$jetstream_disk_monitor_enabled" "$resource_sample_interval" >"$run_dir/test.env"

"${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
"${compose[@]}" up --detach --wait
wait_clickhouse_stable
wait_http "NATS monitor" http://127.0.0.1:18222/healthz
wait_jetstream_cluster

echo "Applying ClickHouse migrations and building binaries"
(
  cd "$repo_root/server"
  env CLICKHOUSE_DSN=clickhouse://helpin:helpin@127.0.0.1:19000/usermaven \
    go run ./cmd/clickhouse-migrate up
)
if [[ "$clickhouse_storage" == "r2" ]]; then
  clickhouse_query "ALTER TABLE events MODIFY SETTING storage_policy = 'r2', min_bytes_for_wide_part = 1073741824, min_rows_for_wide_part = 10000000"
  clickhouse_query "ALTER TABLE session_seed_events MODIFY SETTING storage_policy = 'r2', min_bytes_for_wide_part = 1073741824, min_rows_for_wide_part = 10000000"
  clickhouse_query "
SELECT database, name, engine, storage_policy
FROM system.tables
WHERE database = 'usermaven'
ORDER BY name
" >"$run_dir/clickhouse-storage-policy.tsv"
  events_policy=$(clickhouse_query "SELECT storage_policy FROM system.tables WHERE database = 'usermaven' AND name = 'events'")
  seed_policy=$(clickhouse_query "SELECT storage_policy FROM system.tables WHERE database = 'usermaven' AND name = 'session_seed_events'")
  if [[ "$events_policy" != "r2" || "$seed_policy" != "r2" ]]; then
    echo "ClickHouse migrations did not create the event tables on the R2 storage policy" >&2
    cat "$run_dir/clickhouse-storage-policy.tsv" >&2
    exit 1
  fi
  clickhouse_query "SELECT name, type, path FROM system.disks WHERE name IN ('r2', 'r2_cache') ORDER BY name" \
    >"$run_dir/clickhouse-disks.tsv"
  clickhouse_query "SHOW CREATE TABLE events" >"$run_dir/clickhouse-events-create.sql"
fi
(
  cd "$capture_dir"
  cargo build --release --bins --jobs "${CARGO_BUILD_JOBS:-2}"
  env \
    NATS_URL=tls://127.0.0.1:14222 \
    NATS_CA_FILE="$E2E_NATS_TLS_DIR/ca.crt" \
    NATS_CLIENT_CERT_FILE="$E2E_NATS_TLS_DIR/client.crt" \
    NATS_CLIENT_KEY_FILE="$E2E_NATS_TLS_DIR/client.key" \
    EVENTS_WORK_COMPRESSION="$work_compression" \
    EVENTS_WORK_STORAGE="$work_storage" \
    EVENTS_WORK_REPLICAS="$work_replicas" \
    EVENTS_WORK_MAX_BYTES="$work_max_bytes" \
    EVENTS_RAW_MAX_BYTES=536870912 \
    EVENTS_DLQ_MAX_BYTES=67108864 \
    EVENTS_CONSUMERS_ENABLED="$consumers_enabled" \
    EVENTS_CONSUMER_MEMORY_STORAGE="$consumer_memory_storage" \
    WRITER_REPLICAS="$writer_replicas" \
    ./target/release/nats-bootstrap
)

python3 -m http.server --bind 127.0.0.1 --directory "$fixture_dir" 18080 \
  >"$run_dir/token.log" 2>&1 &
token_pid=$!
wait_http "token fixture" http://127.0.0.1:18080/tokens.json

if [[ "$consumers_enabled" == "true" ]]; then
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
        SESSION_CACHE_MAX_ENTRIES_PER_SHARD=100000 \
        WRITER_POISON_SPILL_DIR="$run_dir/poison-spill-$ordinal" \
        WRITER_POISON_SPILL_MAX_BYTES=67108864 \
        RUST_LOG=warn \
        ./target/release/session-writer
    ) >"$run_dir/writer-$ordinal.log" 2>&1 &
    writer_pids+=("$!")
  done
  for ordinal in $(seq 0 $((writer_replicas - 1))); do
    health_port=$((3010 + ordinal * 2))
    wait_http "session writer $ordinal" "http://127.0.0.1:$health_port/health/readiness"
  done
else
  echo "Writer and all durable consumers are disabled for capture/stream isolation"
fi

for ordinal in $(seq 0 $((capture_replicas - 1))); do
  http_port=$((3000 + ordinal * 2))
  metrics_port=$((http_port + 1))
  nats_port=$((14222 + ordinal % 3))
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
      NATS_URL="tls://127.0.0.1:$nats_port" \
      NATS_CA_FILE="$E2E_NATS_TLS_DIR/ca.crt" \
      NATS_CLIENT_CERT_FILE="$E2E_NATS_TLS_DIR/client.crt" \
      NATS_CLIENT_KEY_FILE="$E2E_NATS_TLS_DIR/client.key" \
      PRINT_SINK="$print_sink" \
      NETWORK_ENRICHMENT_ENABLED="$network_enrichment" \
      HTTP_TOKENS_URL=http://127.0.0.1:18080/tokens.json \
      FALLBACK_DIR="$run_dir/fallback-$ordinal" \
      FALLBACK_MAX_SEGMENT_AGE_SECS=5 \
      ARCHIVE_SPILL_DIR="$run_dir/archive-spill-$ordinal" \
      ARCHIVE_SPILL_MAX_BYTES=67108864 \
      NATS_WORK_MAX_IN_FLIGHT="$work_max_in_flight" \
      RAW_ARCHIVE_ENABLED="$raw_archive_enabled" \
      CAPTURE_HTTP_PORT="$http_port" \
      CAPTURE_METRICS_PORT="$metrics_port" \
      LOG_LEVEL=WARN \
      ./target/release/events-pipeline
  ) >"$run_dir/capture-$ordinal.log" 2>&1 &
  capture_pids+=("$!")
  capture_urls+=("http://127.0.0.1:$http_port/api/v1/event")
done
for ordinal in $(seq 0 $((capture_replicas - 1))); do
  http_port=$((3000 + ordinal * 2))
  wait_http "event capture $ordinal" "http://127.0.0.1:$http_port/health/readiness"
done

for ordinal in $(seq 0 $((capture_replicas - 1))); do
  nats_port=$((14222 + ordinal % 3))
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
      NATS_URL="tls://127.0.0.1:$nats_port" \
      NATS_CA_FILE="$E2E_NATS_TLS_DIR/ca.crt" \
      NATS_CLIENT_CERT_FILE="$E2E_NATS_TLS_DIR/client.crt" \
      NATS_CLIENT_KEY_FILE="$E2E_NATS_TLS_DIR/client.key" \
      NETWORK_ENRICHMENT_ENABLED="$network_enrichment" \
      FALLBACK_DIR="$run_dir/fallback-$ordinal" \
      ARCHIVE_SPILL_DIR="$run_dir/archive-spill-replay-$ordinal" \
      ARCHIVE_SPILL_MAX_BYTES=67108864 \
      NATS_WORK_MAX_IN_FLIGHT="$work_max_in_flight" \
      RAW_ARCHIVE_ENABLED="$raw_archive_enabled" \
      REPLAY_POLL_INTERVAL_SECS=1 \
      REPLAY_CLEANUP_HOURS=0 \
      LOG_LEVEL=WARN \
      ./target/release/replay-worker
  ) >"$run_dir/replay-$ordinal.log" 2>&1 &
  replay_pids+=("$!")
done

echo 'unix_time,capture_cpu_percent,capture_rss_kib,writer_cpu_percent,writer_rss_kib,mem_available_kib,swap_free_kib' >"$run_dir/process-resources.csv"
echo 'unix_time,component,ordinal,pid,cpu_percent,rss_kib' >"$run_dir/process-resources-detailed.csv"
echo 'unix_time,messages,bytes,ack_pending,consumer_pending,redelivered' >"$run_dir/jetstream-resources.csv"
echo 'unix_time,node_ordinal,container_id,total_physical_kib,work_physical_kib,raw_physical_kib,dlq_physical_kib' >"$run_dir/jetstream-disk.csv"
if [[ "$resource_monitor_enabled" == "true" ]]; then
  sample_resources &
  monitor_pid=$!
fi

echo "Driving the capture endpoint"
if [[ "$load_driver" == "k6-connections" || "$load_driver" == "k6-arrival" ]]; then
  if [[ "$load_driver" == "k6-arrival" ]]; then
    k6_profile=arrival
  else
    k6_profile=connections
  fi
  target_urls=$(IFS=,; echo "${capture_urls[*]}")
  chmod 0777 "$run_dir"
  set +e
  docker run --rm --network host \
    --name helpin-event-sustained-k6 \
    --volume "$repo_root/events-pipeline/scripts:/scripts:ro" \
    --volume "$capture_dir/data/user_agents_seed.txt:/fixtures/user-agents.txt:ro" \
    --volume "$run_dir:/results" \
    --env TARGET_URL="${capture_urls[0]}" \
    --env TARGET_URLS="$target_urls" \
    --env TOKEN=e2e-server-secret \
    --env PROFILE="$k6_profile" \
    --env EVENT_RATE="$rate" \
    --env CONNECTIONS="$connections" \
    --env PRE_ALLOCATED_VUS="${SUSTAINED_K6_PRE_ALLOCATED_VUS:-512}" \
    --env MAX_VUS="${SUSTAINED_K6_MAX_VUS:-2048}" \
    --env P50_LIMIT_MS="$k6_p50_limit_ms" \
    --env P99_LIMIT_MS="$k6_p99_limit_ms" \
    --env DURATION="${duration}s" \
    --env BATCH_SIZE="$batch_size" \
    --env VISITORS="$visitors" \
    --env DATA_PROFILE="$k6_data_profile" \
    --env REALISTIC_USER_AGENTS="$k6_realistic_user_agents" \
    --env REALISTIC_IPS="$k6_realistic_ips" \
    --env USER_AGENT_FILE=/fixtures/user-agents.txt \
    --env CLIENT_IP=8.8.8.8 \
    --env 'USER_AGENT=Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36' \
    --env SUMMARY_PATH=/results/k6-summary.json \
    grafana/k6:1.8.1 run /scripts/k6-capture.js \
    >"$run_dir/load.log" 2>&1
  k6_status=$?
  set -e
  if [[ ! -s "$run_dir/k6-summary.json" ]]; then
    echo "k6 did not produce a summary; inspect $run_dir/load.log" >&2
    if (( k6_status == 0 )); then
      exit 1
    fi
    exit "$k6_status"
  fi
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
  while true; do
    if [[ "$consumers_enabled" == "true" ]]; then
      observed=$(clickhouse_query "SELECT count() FROM events FINAL WHERE src = '$source_label'")
    else
      read -r observed _ < <(work_stream_state)
    fi
    if [[ "$observed" != "0" ]]; then
      break
    fi
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

if [[ "$print_sink" == "true" ]]; then
  save_capture_metrics
  if [[ "$k6_status" -ne 0 ]]; then
    echo "k6 HTTP-only diagnostic failed with status $k6_status" >&2
    exit "$k6_status"
  fi
  echo "Sustained HTTP-only diagnostic passed"
  exit 0
fi

if [[ "$consumers_enabled" != "true" ]]; then
  echo "Waiting for the unconsumed work stream to reach $expected messages"
  work_messages=0
  consumer_count=-1
  for _ in $(seq 1 180); do
    read -r work_messages consumer_count < <(work_stream_state)
    if (( work_messages >= expected )); then
      break
    fi
    sleep 1
  done

  final_disk_sample_time=$(date +%s)
  sample_jetstream_disk "$final_disk_sample_time"
  read -r work_messages work_bytes _ _ _ <<<"$(work_stream_resources)"

  save_capture_metrics
  for port in 18222 18223 18224; do
    curl --fail --silent "http://127.0.0.1:$port/jsz?streams=true&consumers=true" >"$run_dir/jetstream-$port.json"
  done
  "${compose[@]}" ps >"$run_dir/compose-ps.txt"
  "${compose[@]}" logs --no-color >"$run_dir/containers.log" 2>&1

  RUN_DIR="$run_dir" WORK_MESSAGES="$work_messages" WORK_BYTES="$work_bytes" \
    WORK_REPLICAS="$work_replicas" WORK_COMPRESSION="$work_compression" \
    WORK_STORAGE="$work_storage" CONSUMER_COUNT="$consumer_count" python3 - <<'PY'
import csv
import json
import os
from pathlib import Path

root = Path(os.environ["RUN_DIR"])
with (root / "jetstream-disk.csv").open() as source:
    disk_rows = list(csv.DictReader(source))
latest_disk = {}
latest_total_disk = {}
for row in disk_rows:
    latest_disk[row["node_ordinal"]] = int(row["work_physical_kib"]) * 1024
    latest_total_disk[row["node_ordinal"]] = int(row["total_physical_kib"]) * 1024
logical_bytes = int(os.environ["WORK_BYTES"])
replicas = int(os.environ["WORK_REPLICAS"])
physical_bytes = sum(latest_disk.values())
replicated_logical_bytes = logical_bytes * replicas
summary = {
    "load": json.loads((root / "load-result.json").read_text()),
    "jetstream": {
        "work_messages": int(os.environ["WORK_MESSAGES"]),
        "work_logical_bytes": logical_bytes,
        "work_logical_bytes_per_message": round(
            logical_bytes / int(os.environ["WORK_MESSAGES"]), 3
        ),
        "consumer_count": int(os.environ["CONSUMER_COUNT"]),
        "storage": os.environ["WORK_STORAGE"],
        "compression": os.environ["WORK_COMPRESSION"],
        "replicas": replicas,
        "physical_bytes_by_node": latest_disk,
        "physical_bytes_total": physical_bytes,
        "total_store_bytes_by_node": latest_total_disk,
        "total_store_bytes": sum(latest_total_disk.values()),
        "replicated_logical_bytes": replicated_logical_bytes,
        "physical_to_logical_ratio": round(
            physical_bytes / replicated_logical_bytes, 6
        ) if replicated_logical_bytes else 0,
    },
    "mode": "capture_without_consumers",
}
(root / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")
print(json.dumps(summary, indent=2, sort_keys=True))
PY

  if [[ "$k6_status" -ne 0 ]]; then
    echo "k6 capture-only load failed with status $k6_status" >&2
    exit "$k6_status"
  fi
  if [[ "$load_driver" == "external" ]]; then
    (( work_messages >= expected ))
  else
    [[ "$work_messages" == "$expected" ]]
  fi
  [[ "$consumer_count" == "0" ]]
  assert_no_spill_files
  echo "Sustained capture-only E2E passed"
  exit 0
fi

echo "Waiting for the writer consumers to drain"
drain_started=$(date +%s)
work_messages=-1
for _ in $(seq 1 180); do
  read -r work_messages _ < <(work_stream_state)
  if (( work_messages == 0 )); then
    break
  fi
  sleep 1
done
drain_seconds=$(($(date +%s) - drain_started))
if (( work_messages != 0 )); then
  echo "writer consumers did not drain within three minutes; $work_messages work messages remain" >&2
  exit 1
fi
echo "Verifying ClickHouse reached $expected logical rows"
logical_rows=$(clickhouse_query "SELECT count() FROM events FINAL WHERE src = '$source_label'")

save_capture_metrics
for ordinal in $(seq 0 $((writer_replicas - 1))); do
  metrics_port=$((3011 + ordinal * 2))
  curl --fail --silent "http://127.0.0.1:$metrics_port/metrics" >"$run_dir/writer-metrics-$ordinal.prom"
done
cp "$run_dir/writer-metrics-0.prom" "$run_dir/writer-metrics.prom"
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

# Keep the exact session invariant check bounded on long runs. Grouping every
# visitor in one query exceeded the small E2E ClickHouse container's memory at
# six million rows even though ingestion had fully drained. Ten shard ranges
# preserve the exact result while bounding each aggregation to roughly one
# tenth of the visitor set.
wrong_session_visitors=0
for shard_start in $(seq 0 10 90); do
  shard_end=$((shard_start + 9))
  wrong_in_range=$(clickhouse_query "
SELECT count()
FROM
(
  SELECT user_anonymous_id
  FROM events FINAL
  WHERE src = '$source_label'
    AND visitor_shard BETWEEN $shard_start AND $shard_end
  GROUP BY user_anonymous_id
  HAVING min(session_id) != max(session_id)
)
")
  wrong_session_visitors=$((wrong_session_visitors + wrong_in_range))
done
printf '%s\n' "$wrong_session_visitors" >"$run_dir/visitors-with-wrong-session-count.txt"

clickhouse_query "$(<"$repo_root/events-pipeline/scripts/clickhouse-parts-health.sql")" \
  >"$run_dir/clickhouse-parts-health.tsv"

if [[ "$k6_data_profile" == "realistic" ]]; then
  read -r distinct_ips distinct_user_agents geo_rows parsed_ua_rows utm_rows ids_rows click_rows company_rows autocapture_rows distinct_paths ai_bot_rows ai_bot_providers ai_bot_names < <(
    clickhouse_query "
SELECT
  uniqExact(source_ip),
  uniqExact(user_agent),
  countIf(location_country != ''),
  countIf(parsed_ua_ua_family != ''),
  countIf(utm_source != ''),
  countIf(ids_ga != ''),
  countIf(click_id_gclid != ''),
  countIf(company_id != ''),
  countIf(autocapture_attributes != '{}'),
  uniqExact(doc_path),
  countIf(startsWith(parsed_ua_bot_category, 'ai_')),
  uniqExactIf(parsed_ua_bot_provider, startsWith(parsed_ua_bot_category, 'ai_')),
  uniqExactIf(parsed_ua_bot_name, startsWith(parsed_ua_bot_category, 'ai_'))
FROM events FINAL
WHERE src = '$source_label'
FORMAT TSVRaw
" | tee "$run_dir/clickhouse-realistic-coverage.tsv"
  )
fi

if [[ "$clickhouse_storage" == "r2" ]]; then
  clickhouse_query "
SELECT table, disk_name, count() AS active_parts, sum(rows) AS rows, sum(bytes_on_disk) AS bytes_on_disk
FROM system.parts
WHERE active AND database = 'usermaven'
GROUP BY table, disk_name
ORDER BY table, disk_name
" >"$run_dir/clickhouse-r2-parts.tsv"
  clickhouse_query "
SELECT event, value
FROM system.events
WHERE positionCaseInsensitive(event, 'S3') > 0
   OR positionCaseInsensitive(event, 'ObjectStorage') > 0
ORDER BY event
" >"$run_dir/clickhouse-r2-events.tsv"
fi

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

with (root / "process-resources-detailed.csv").open() as source:
    detailed_rows = list(csv.DictReader(source))

with (root / "jetstream-resources.csv").open() as source:
    jetstream_rows = list(csv.DictReader(source))

with (root / "jetstream-disk.csv").open() as source:
    jetstream_disk_rows = list(csv.DictReader(source))

def stats(name):
    values = [float(row[name]) for row in rows]
    if not values:
        return {"average": 0, "peak": 0}
    return {"average": round(sum(values) / len(values), 3), "peak": round(max(values), 3)}

detailed = defaultdict(lambda: {"cpu_percent": [], "rss_mib": []})
for row in detailed_rows:
    name = f'{row["component"]}-{row["ordinal"]}'
    detailed[name]["cpu_percent"].append(float(row["cpu_percent"]))
    detailed[name]["rss_mib"].append(float(row["rss_kib"]) / 1024)

detailed_summary = {}
for name, samples in detailed.items():
    detailed_summary[name] = {
        metric: {"average": round(sum(values) / len(values), 3), "peak": round(max(values), 3)}
        for metric, values in samples.items()
        if values
    }

jetstream_summary = {}
for metric in ["messages", "bytes", "ack_pending", "consumer_pending", "redelivered"]:
    values = [int(row[metric]) for row in jetstream_rows]
    jetstream_summary[metric] = {
        "average": round(sum(values) / len(values), 3) if values else 0,
        "peak": max(values) if values else 0,
    }

jetstream_disk = defaultdict(lambda: defaultdict(list))
for row in jetstream_disk_rows:
    for stream in ["total", "work", "raw", "dlq"]:
        jetstream_disk[row["node_ordinal"]][stream].append(
            int(row[f"{stream}_physical_kib"]) / 1024
        )
jetstream_disk_summary = {
    f"nats-{ordinal}": {
        stream: {
            "average_mib": round(sum(values) / len(values), 3),
            "peak_mib": round(max(values), 3),
            "final_mib": round(values[-1], 3),
        }
        for stream, values in streams.items()
        if values
    }
    for ordinal, streams in jetstream_disk.items()
}

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
realistic_coverage_path = root / "clickhouse-realistic-coverage.tsv"
realistic_coverage = None
if realistic_coverage_path.exists():
    values = [int(value) for value in realistic_coverage_path.read_text().split()]
    realistic_coverage = dict(zip(
        ["distinct_ips", "distinct_user_agents", "geo_rows", "parsed_ua_rows", "utm_rows", "ids_rows", "click_rows", "company_rows", "autocapture_rows", "distinct_paths", "ai_bot_rows", "ai_bot_providers", "ai_bot_names"],
        values,
    ))
summary = {
    "load": load,
    "drain_seconds": int(os.environ["DRAIN_SECONDS"]),
    "clickhouse": {
        "physical": dict(zip(["rows", "unique_event_ids", "visitors", "sessions", "rows_without_session"], physical)),
        "final": dict(zip(["rows", "unique_event_ids", "visitors", "sessions", "rows_without_session"], logical)),
        "visitors_with_wrong_session_count": int((root / "visitors-with-wrong-session-count.txt").read_text()),
        "realistic_coverage": realistic_coverage,
    },
    "host_processes": {
        "capture_cpu_percent": stats("capture_cpu_percent"),
        "capture_rss_mib": {key: round(value / 1024, 3) for key, value in stats("capture_rss_kib").items()},
        "writer_cpu_percent": stats("writer_cpu_percent"),
        "writer_rss_mib": {key: round(value / 1024, 3) for key, value in stats("writer_rss_kib").items()},
        "host_mem_available_mib": {key: round(value / 1024, 3) for key, value in stats("mem_available_kib").items()},
        "host_swap_free_mib": {key: round(value / 1024, 3) for key, value in stats("swap_free_kib").items()},
    },
    "host_process_details": detailed_summary,
    "jetstream_backlog": jetstream_summary,
    "jetstream_disk": jetstream_disk_summary,
    "docker": docker_summary,
}
(root / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")
print(json.dumps(summary, indent=2, sort_keys=True))
PY

if [[ "$k6_status" -ne 0 ]]; then
  echo "k6 sustained load failed with status $k6_status" >&2
  exit "$k6_status"
fi
if [[ "$load_driver" == "external" ]]; then
  (( logical_rows >= expected ))
else
  [[ "$logical_rows" == "$expected" ]]
  [[ "$(awk '{print $1}' "$run_dir/clickhouse-physical.tsv")" == "$expected" ]]
  [[ "$(awk '{print $2}' "$run_dir/clickhouse-physical.tsv")" == "$expected" ]]
  [[ "$(awk '{print $1}' "$run_dir/clickhouse-final.tsv")" == "$expected" ]]
  [[ "$(awk '{print $2}' "$run_dir/clickhouse-final.tsv")" == "$expected" ]]
fi
[[ "$(awk '{print $5}' "$run_dir/clickhouse-final.tsv")" == "0" ]]
[[ "$(cat "$run_dir/visitors-with-wrong-session-count.txt")" == "0" ]]
if [[ "$k6_data_profile" == "realistic" ]]; then
  (( distinct_ips >= realistic_min_distinct ))
  (( distinct_user_agents >= realistic_min_distinct ))
  (( geo_rows > 0 ))
  (( parsed_ua_rows > 0 ))
  (( utm_rows > 0 ))
  (( ids_rows > 0 ))
  (( click_rows > 0 ))
  (( company_rows > 0 ))
  (( autocapture_rows > 0 ))
  (( distinct_paths >= 5 ))
  (( ai_bot_rows > 0 ))
  (( ai_bot_providers >= 3 ))
  (( ai_bot_names >= 5 ))
fi
assert_no_spill_files

echo "Sustained E2E passed"
