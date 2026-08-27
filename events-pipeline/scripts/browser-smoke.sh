#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

set -a
source server/.env
source events-pipeline/.env.events
set +a

token_url=${HELPIN_EVENT_TOKEN_URL:-http://127.0.0.1:8080/api/internal/widget-tokens}
test_workspace_id=${HELPIN_EVENT_TEST_WORKSPACE_ID:-${EVENT_TEST_PROJECT_ID:-}}
token_response=$(curl -fsS -H "Authorization: Bearer $INTERNAL_API_SECRET" "$token_url")
mapfile -t token_context < <(TOKEN_RESPONSE="$token_response" HELPIN_EVENT_TEST_WORKSPACE_ID="$test_workspace_id" python3 -c '
import json, os
tokens = json.loads(os.environ["TOKEN_RESPONSE"])["tokens"]
project = os.environ.get("HELPIN_EVENT_TEST_WORKSPACE_ID", "").strip().lower()
token = next((item for item in tokens if item["workspace_id"].lower() == project), None) if project else (tokens[0] if len(tokens) == 1 else None)
if token is None:
    raise SystemExit(f"expected one active widget installation or an installation for workspace {project}")
print(token["client_secret"])
print(token["workspace_id"])
')
unset token_response
HELPIN_EVENT_TEST_WIDGET_KEY=${token_context[0]}
EVENT_TEST_PROJECT_ID=${token_context[1]}
export HELPIN_EVENT_TEST_WIDGET_KEY
export EVENT_TEST_PROJECT_ID

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
