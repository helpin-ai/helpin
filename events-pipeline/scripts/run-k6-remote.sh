#!/usr/bin/env bash
set -euo pipefail

: "${TARGET_URL:?TARGET_URL is required}"
: "${TOKEN:?TOKEN is required}"

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
results_dir=${RESULTS_DIR:-"/tmp/helpin-k6-remote-$(date -u +%Y%m%dT%H%M%SZ)"}
mkdir -p "$results_dir"
chmod 0777 "$results_dir"

docker run --rm \
  --volume "$script_dir:/scripts:ro" \
  --volume "$results_dir:/results" \
  --env TARGET_URL \
  --env TOKEN \
  --env PROFILE="${PROFILE:-connections}" \
  --env EVENT_RATE="${EVENT_RATE:-15000}" \
  --env CONNECTIONS="${CONNECTIONS:-1000}" \
  --env DURATION="${DURATION:-10m}" \
  --env BATCH_SIZE="${BATCH_SIZE:-1}" \
  --env VISITORS="${VISITORS:-5000000}" \
  --env PRE_ALLOCATED_VUS="${PRE_ALLOCATED_VUS:-1000}" \
  --env MAX_VUS="${MAX_VUS:-2000}" \
  --env P50_LIMIT_MS="${P50_LIMIT_MS:-10}" \
  --env P99_LIMIT_MS="${P99_LIMIT_MS:-25}" \
  --env SUMMARY_PATH=/results/k6-summary.json \
  grafana/k6:1.8.1 run /scripts/k6-capture.js

echo "Remote k6 results: $results_dir"
