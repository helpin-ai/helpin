#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

set -a
source server/.env
source events-pipeline/.env.events
set +a

: "${INTERNAL_API_SECRET:?INTERNAL_API_SECRET is required in server/.env}"
: "${EVENT_TEST_PROJECT_ID:?EVENT_TEST_PROJECT_ID is required in events-pipeline/.env.events}"

backend_url=${HELPIN_BACKEND_URL:-http://127.0.0.1:8080}
capture_url=${HELPIN_CAPTURE_URL:-http://127.0.0.1:3000}
marker="helpin-smoke-$(date +%s)-$$"
invalid_marker="$marker-invalid"

wait_for_http() {
  local name=$1 url=$2
  for _ in $(seq 1 60); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "$name did not become ready: $url" >&2
  return 1
}

wait_for_http "event capture" "$capture_url/health/liveness"

token_response=$(curl -fsS \
  -H "Authorization: Bearer $INTERNAL_API_SECRET" \
  "$backend_url/api/internal/widget-tokens")

credentials=$(TOKEN_RESPONSE="$token_response" EVENT_TEST_PROJECT_ID="$EVENT_TEST_PROJECT_ID" python3 -c '
import json, os
tokens = json.loads(os.environ["TOKEN_RESPONSE"])["tokens"]
project = os.environ["EVENT_TEST_PROJECT_ID"]
token = next((item for item in tokens if item["workspace_id"].lower() == project.lower()), None)
if token is None:
    raise SystemExit(f"no active widget token for project {project}")
print(token["server_secret"])
')
unset token_response

payload=$(MARKER="$marker" python3 -c '
import json, os
marker = os.environ["MARKER"]
print(json.dumps({
    "api_key": "payload-must-not-authorize",
    "project_id": "00000000-0000-0000-0000-000000000000",
    "event_type": "pipeline_smoke",
    "url": f"https://helpin-dev-fe.tryunhide.com/event-test/smoke?run={marker}",
    "doc_path": "/event-test/smoke",
    "user": {
        "anonymous_id": marker,
        "id": f"user-{marker}",
        "email": "pipeline-smoke@example.test"
    },
    "company": {"id": f"company-{marker}", "name": "Pipeline Smoke"},
    "event_attributes": {"smoke_marker": marker},
    "utm": {"source": "pipeline-smoke"},
    "ids": {"gclid": f"gclid-{marker}"},
    "src": "helpin-local-smoke"
}))
')

status=""
for _ in $(seq 1 30); do
  status=$(curl -sS -o /dev/null -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -H "x-auth-token: $credentials" \
    --data "$payload" \
    "$capture_url/api/v1/s2s/event")
  if [[ "$status" == "200" ]]; then
    break
  fi
  sleep 2
done
if [[ "$status" != "200" ]]; then
  echo "valid event returned HTTP $status" >&2
  exit 1
fi
unset credentials

invalid_payload=${payload//$marker/$invalid_marker}
invalid_status=$(curl -sS -o /dev/null -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -H 'x-auth-token: definitely-invalid-local-token' \
  --data "$invalid_payload" \
  "$capture_url/api/v1/s2s/event")
if [[ "$invalid_status" != "401" ]]; then
  echo "invalid token returned HTTP $invalid_status, expected 401" >&2
  exit 1
fi

find_kafka_record() {
  local topic=$1 record="" group
  group="helpin-smoke-${topic//./-}-$marker"
  for _ in $(seq 1 40); do
    record=$(docker exec helpin-kafka kafka-console-consumer \
      --bootstrap-server kafka:29092 \
      --topic "$topic" \
      --group "$group" \
      --from-beginning \
      --timeout-ms 2500 2>/dev/null \
      | MARKER="$marker" python3 -c '
import json, os, sys
marker = os.environ["MARKER"]
for line in sys.stdin:
    try:
        item = json.loads(line)
    except json.JSONDecodeError:
        continue
    if marker in json.dumps(item, separators=(",", ":")):
        print(json.dumps(item, separators=(",", ":")))
        break
' || true)
    if [[ -n "$record" ]]; then
      printf '%s' "$record"
      return 0
    fi
    sleep 1
  done
  echo "marker did not reach Kafka topic $topic" >&2
  return 1
}

raw_record=$(find_kafka_record helpin.events.raw)
RAW_RECORD="$raw_record" EXPECTED_PROJECT="$EVENT_TEST_PROJECT_ID" MARKER="$marker" python3 -c '
import json, os
item = json.loads(os.environ["RAW_RECORD"])
assert item["authorization"]["workspace_id"].lower() == os.environ["EXPECTED_PROJECT"].lower()
assert item["event"]["event_attributes"]["smoke_marker"] == os.environ["MARKER"]
assert "project_id" not in item["event"]
assert item["event"]["api_key"] != "payload-must-not-authorize"
'
unset raw_record

for topic in helpin.events.enriched helpin.events.sessionized; do
  record=$(find_kafka_record "$topic")
  TOPIC="$topic" RECORD="$record" EXPECTED_PROJECT="$EVENT_TEST_PROJECT_ID" MARKER="$marker" python3 -c '
import json, os
item = json.loads(os.environ["RECORD"])
assert item["project_id"].lower() == os.environ["EXPECTED_PROJECT"].lower()
assert json.loads(item["event_attributes"])["smoke_marker"] == os.environ["MARKER"]
if os.environ["TOPIC"].endswith("sessionized"):
    assert item["session_id"]
'
done

clickhouse_row=""
for _ in $(seq 1 40); do
  clickhouse_row=$(docker exec helpin-clickhouse clickhouse-client \
    --user helpin --password helpin --format TSVRaw \
    --query "SELECT project_id, event_type, session_id, identity_method, identity_trust FROM usermaven.events WHERE position(event_attributes, '$marker') > 0 ORDER BY _timestamp DESC LIMIT 1" 2>/dev/null || true)
  if [[ -n "$clickhouse_row" ]]; then
    break
  fi
  sleep 1
done

IFS=$'\t' read -r stored_project stored_event stored_session stored_method stored_trust <<<"$clickhouse_row"
if [[ "${stored_project,,}" != "${EVENT_TEST_PROJECT_ID,,}" || "$stored_event" != "pipeline_smoke" || -z "$stored_session" || "$stored_method" != "server_event" || "$stored_trust" != "verified" ]]; then
  echo "ClickHouse row did not preserve the authenticated project/session/identity contract" >&2
  exit 1
fi

invalid_count=$(docker exec helpin-clickhouse clickhouse-client \
  --user helpin --password helpin --format TSVRaw \
  --query "SELECT count() FROM usermaven.events WHERE position(event_attributes, '$invalid_marker') > 0")
if [[ "$invalid_count" != "0" ]]; then
  echo "invalid-token event reached ClickHouse" >&2
  exit 1
fi

echo "Pipeline smoke passed: authenticated project_id propagated through raw, enriched, sessionized, and ClickHouse stages."
