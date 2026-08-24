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
token_response=$(curl -fsS \
  -H "Authorization: Bearer $INTERNAL_API_SECRET" \
  "$backend_url/api/internal/widget-tokens")

HELPIN_EVENT_TEST_WIDGET_KEY=$(TOKEN_RESPONSE="$token_response" EVENT_TEST_PROJECT_ID="$EVENT_TEST_PROJECT_ID" python3 -c '
import json, os
tokens = json.loads(os.environ["TOKEN_RESPONSE"])["tokens"]
project = os.environ["EVENT_TEST_PROJECT_ID"]
token = next((item for item in tokens if item["workspace_id"].lower() == project.lower()), None)
if token is None:
    raise SystemExit(f"no active widget token for project {project}")
print(token["client_secret"])
')
unset token_response
export HELPIN_EVENT_TEST_WIDGET_KEY

if [[ -z "${PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH:-}" ]]; then
  for browser in google-chrome chromium chromium-browser; do
    if command -v "$browser" >/dev/null 2>&1; then
      PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=$(command -v "$browser")
      export PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH
      break
    fi
  done
fi

pnpm --dir frontend exec playwright test \
  --config=playwright.event-pipeline.config.ts \
  --project=chromium
