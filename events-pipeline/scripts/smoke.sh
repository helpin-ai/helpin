#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

set -a
source server/.env
source events-pipeline/.env.events
set +a

capture_url=${HELPIN_CAPTURE_URL:-http://127.0.0.1:3000}
nats_monitor_url=${HELPIN_NATS_MONITOR_URL:-http://127.0.0.1:8222}
clickhouse_http_url=${HELPIN_CLICKHOUSE_HTTP_URL:-http://127.0.0.1:8123}
token_url=${HELPIN_EVENT_TOKEN_URL:-http://127.0.0.1:8080/api/internal/widget-tokens}
test_workspace_id=${HELPIN_EVENT_TEST_WORKSPACE_ID:-${EVENT_TEST_PROJECT_ID:-}}
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
wait_for_http "NATS JetStream" "$nats_monitor_url/jsz"

clickhouse_query() {
  curl --fail-with-body --silent --show-error \
    --user helpin:helpin \
    "$clickhouse_http_url/?database=helpin&default_format=TSVRaw" \
    --data-binary "$1"
}

stream_messages() {
  local stream_name=$1
  NATS_MONITOR_URL="$nats_monitor_url" STREAM_NAME="$stream_name" python3 -c '
import json, os, urllib.request
with urllib.request.urlopen(os.environ["NATS_MONITOR_URL"].rstrip("/") + "/jsz?streams=true&consumers=false") as response:
    state = json.load(response)
for account in state.get("account_details", []):
    for stream in account.get("stream_detail", []):
        if stream.get("name") == os.environ["STREAM_NAME"]:
            print(stream["state"]["messages"])
            raise SystemExit(0)
raise SystemExit("stream not found: " + os.environ["STREAM_NAME"])
'
}

raw_messages_before=$(stream_messages EVENTS_RAW_V1)

token_response=$(curl -fsS -H "Authorization: Bearer $INTERNAL_API_SECRET" "$token_url")
mapfile -t token_context < <(TOKEN_RESPONSE="$token_response" HELPIN_EVENT_TEST_WORKSPACE_ID="$test_workspace_id" python3 -c '
import json, os
tokens = json.loads(os.environ["TOKEN_RESPONSE"])["tokens"]
project = os.environ.get("HELPIN_EVENT_TEST_WORKSPACE_ID", "").strip().lower()
token = next((item for item in tokens if item["workspace_id"].lower() == project), None) if project else (tokens[0] if len(tokens) == 1 else None)
if token is None:
    raise SystemExit(f"expected one active widget installation or an installation for workspace {project}")
print(token["server_secret"])
print(token["workspace_id"])
')
unset token_response
credentials=${token_context[0]}
expected_project_id=${token_context[1]}

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

clickhouse_row=""
for _ in $(seq 1 40); do
  clickhouse_row=$(clickhouse_query "SELECT project_id, event_type, session_id, identity_method, identity_trust, _nats_subject, _nats_stream_sequence, _nats_delivery_attempt, raw_event FROM events FINAL WHERE position(event_attributes, '$marker') > 0 ORDER BY _timestamp DESC LIMIT 1 FORMAT JSONEachRow" 2>/dev/null || true)
  if [[ -n "$clickhouse_row" ]]; then
    break
  fi
  sleep 1
done
if [[ -z "$clickhouse_row" ]]; then
  echo "event did not reach ClickHouse through JetStream" >&2
  exit 1
fi

CLICKHOUSE_ROW="$clickhouse_row" EXPECTED_PROJECT="$expected_project_id" MARKER="$marker" python3 -c '
import json, os
row = json.loads(os.environ["CLICKHOUSE_ROW"])
event = json.loads(row["raw_event"])
assert row["project_id"].lower() == os.environ["EXPECTED_PROJECT"].lower()
assert row["event_type"] == "pipeline_smoke"
assert row["session_id"]
assert row["identity_method"] == "server_event"
assert row["identity_trust"] == "verified"
assert row["_nats_subject"].startswith("events.enriched.v1.")
assert int(row["_nats_stream_sequence"]) > 0
assert int(row["_nats_delivery_attempt"]) >= 1
assert json.loads(event["event_attributes"])["smoke_marker"] == os.environ["MARKER"]
assert event["project_id"].lower() == os.environ["EXPECTED_PROJECT"].lower()
assert event["api_key"] != "payload-must-not-authorize"
'
unset clickhouse_row

raw_messages_after=$raw_messages_before
for _ in $(seq 1 20); do
  raw_messages_after=$(stream_messages EVENTS_RAW_V1)
  if (( raw_messages_after > raw_messages_before )); then
    break
  fi
  sleep 1
done
if (( raw_messages_after <= raw_messages_before )); then
  echo "raw JetStream archive did not receive the event" >&2
  exit 1
fi

invalid_count=$(clickhouse_query "SELECT count() FROM events FINAL WHERE position(event_attributes, '$invalid_marker') > 0")
if [[ "$invalid_count" != "0" ]]; then
  echo "invalid-token event reached ClickHouse" >&2
  exit 1
fi

echo "Pipeline smoke passed: authenticated project_id propagated through JetStream sessionization into ClickHouse."
