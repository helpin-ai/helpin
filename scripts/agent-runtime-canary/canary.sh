#!/usr/bin/env bash
#
# agent-runtime shadow-validation canary
#
# Starts a Mira (marketer-preset) workspace-target agent run through the
# Helpin API, then polls the run until it reaches a terminal state and
# asserts that the run was delegated to agent-runtime and projected back
# correctly (lifecycle, transcript, real-model token usage, billing marker).
#
# Required env:
#   HELPIN_API_URL     Base API URL, e.g. https://stage.helpin.ai/api
#   HELPIN_API_TOKEN   User JWT (Bearer). Needs pm.edit in the workspace
#                      and the Automation module enabled.
#   WORKSPACE_ID       Workspace UUID (used as X-Workspace-ID AND as the
#                      run target_id — workspace runs target the workspace).
#   AGENT_ID           Agent UUID. MUST be a marketer-preset (Mira) agent;
#                      delegation predicate is preset-based, not name-based.
#
# Optional env:
#   CANARY_TIMEOUT_SECS       Total poll budget (default 900).
#   CANARY_POLL_INTERVAL_SECS Poll interval (default 5).
#   CANARY_PROMPT             additional_context sent with the run.
#   CANARY_CANCEL_ON_TIMEOUT  Cancel the run if the timeout is hit so the
#                             canary stays re-runnable (default true).
#
# Exit codes: 0 = all assertions passed, non-zero = failure (message on stderr).
#
# API shape (grounded in server/internal/router/router.go and
# server/internal/handler/agent.go):
#   POST /pm/agent-runs                      body: {target_type, target_id, agent_id, additional_context}
#   GET  /pm/agent-runs/{id}                 returns the agent_run row as JSON
#   GET  /pm/agent-runs/{id}/messages        returns [] of agent_run_messages
#   POST /pm/agent-runs/{id}/cancel
# Workspace scoping via X-Workspace-ID header (middleware.RequireWorkspaceID).

set -euo pipefail

# ---------------------------------------------------------------------------
# Config & helpers
# ---------------------------------------------------------------------------

TIMEOUT_SECS="${CANARY_TIMEOUT_SECS:-900}"
POLL_SECS="${CANARY_POLL_INTERVAL_SECS:-5}"
PROMPT="${CANARY_PROMPT:-Shadow-validation canary run: briefly introduce yourself and list the marketing channels you can help with. Keep it short.}"
CANCEL_ON_TIMEOUT="${CANARY_CANCEL_ON_TIMEOUT:-true}"

RUN_ID=""
FAILURES=0

log()  { printf '[canary] %s\n' "$*"; }
warn() { printf '[canary] WARN: %s\n' "$*" >&2; }
die()  { printf '[canary] FATAL: %s\n' "$*" >&2; exit 1; }

fail_assert() {
  printf '[canary] ASSERTION FAILED: %s\n' "$*" >&2
  FAILURES=$((FAILURES + 1))
}

for bin in curl jq; do
  command -v "$bin" >/dev/null 2>&1 || die "required tool '$bin' not found in PATH"
done

: "${HELPIN_API_URL:?HELPIN_API_URL is required (e.g. https://stage.helpin.ai/api)}"
: "${HELPIN_API_TOKEN:?HELPIN_API_TOKEN is required (user JWT)}"
: "${WORKSPACE_ID:?WORKSPACE_ID is required}"
: "${AGENT_ID:?AGENT_ID is required (marketer-preset / Mira agent)}"

API="${HELPIN_API_URL%/}"

# api METHOD PATH [JSON_BODY] -> prints "HTTPCODE\nBODY"
api() {
  local method="$1" path="$2" body="${3:-}"
  local args=(
    -sS -o /tmp/canary-resp.$$ -w '%{http_code}'
    -X "$method"
    -H "Authorization: Bearer ${HELPIN_API_TOKEN}"
    -H "X-Workspace-ID: ${WORKSPACE_ID}"
    -H "Content-Type: application/json"
  )
  if [[ -n "$body" ]]; then
    args+=(--data "$body")
  fi
  local code
  code=$(curl "${args[@]}" "${API}${path}") || die "curl ${method} ${path} failed (network error)"
  printf '%s\n' "$code"
  cat /tmp/canary-resp.$$
  rm -f /tmp/canary-resp.$$
}

cleanup_on_timeout() {
  if [[ "$CANCEL_ON_TIMEOUT" == "true" && -n "$RUN_ID" ]]; then
    warn "cancelling run ${RUN_ID} so the canary stays re-runnable"
    api POST "/pm/agent-runs/${RUN_ID}/cancel" >/dev/null 2>&1 || warn "cancel request failed (run may already be terminal)"
  fi
}

# ---------------------------------------------------------------------------
# 1. Launch the Mira workspace run
# ---------------------------------------------------------------------------

log "starting Mira workspace run (workspace=${WORKSPACE_ID}, agent=${AGENT_ID})"

launch_body=$(jq -n \
  --arg target_id "$WORKSPACE_ID" \
  --arg agent_id "$AGENT_ID" \
  --arg ctx "$PROMPT" \
  '{target_type: "workspace", target_id: $target_id, agent_id: $agent_id, additional_context: $ctx}')

resp=$(api POST "/pm/agent-runs" "$launch_body")
http_code=$(head -n1 <<<"$resp")
run_json=$(tail -n +2 <<<"$resp")

if [[ "$http_code" != "201" ]]; then
  die "launch failed: HTTP ${http_code}: ${run_json}"
fi

RUN_ID=$(jq -r '.id // empty' <<<"$run_json")
[[ -n "$RUN_ID" ]] || die "launch response missing run id: ${run_json}"
log "run created: ${RUN_ID}"

# --- Assertion (a): delegated to agent-runtime immediately after launch ----
ext_runtime=$(jq -r '.external_runtime // "null"' <<<"$run_json")
ext_runtime_id=$(jq -r '.external_runtime_id // "null"' <<<"$run_json")

if [[ "$ext_runtime" != "agent-runtime" ]]; then
  fail_assert "external_runtime is '${ext_runtime}' (want 'agent-runtime'). Run was NOT delegated — check AGENT_RUNTIME_LAUNCH_ENABLED and that agent ${AGENT_ID} uses the marketer preset with target_type=workspace."
fi
if [[ -z "$ext_runtime_id" || "$ext_runtime_id" == "null" ]]; then
  fail_assert "external_runtime_id is null/empty immediately after launch"
else
  log "delegation confirmed: external_runtime=${ext_runtime} external_runtime_id=${ext_runtime_id}"
fi

# --- Assertion (b): no local Temporal workflow at launch --------------------
wf=$(jq -r '.workflow_id // "null"' <<<"$run_json")
if [[ "$wf" != "null" ]]; then
  fail_assert "workflow_id is '${wf}' at launch — delegated runs must not create a local Helpin Temporal workflow"
fi

if (( FAILURES > 0 )); then
  cleanup_on_timeout
  die "${FAILURES} launch assertion(s) failed for run ${RUN_ID}; aborting before poll"
fi

# ---------------------------------------------------------------------------
# 2. Poll until terminal or timeout
# ---------------------------------------------------------------------------

deadline=$(( $(date +%s) + TIMEOUT_SECS ))
status=""
pause_reason=""
seen_active="false"   # saw running or paused at least once
terminal="false"

log "polling run every ${POLL_SECS}s (timeout ${TIMEOUT_SECS}s)"

while (( $(date +%s) < deadline )); do
  resp=$(api GET "/pm/agent-runs/${RUN_ID}")
  http_code=$(head -n1 <<<"$resp")
  run_json=$(tail -n +2 <<<"$resp")
  if [[ "$http_code" != "200" ]]; then
    warn "poll got HTTP ${http_code}, retrying"
    sleep "$POLL_SECS"
    continue
  fi

  new_status=$(jq -r '.status // "unknown"' <<<"$run_json")
  new_pause=$(jq -r '.pause_reason // "none"' <<<"$run_json")
  if [[ "$new_status" != "$status" || "$new_pause" != "$pause_reason" ]]; then
    log "status: ${new_status} (pause_reason=${new_pause}, execution_stage=$(jq -r '.execution_stage // "-"' <<<"$run_json"))"
  fi
  status="$new_status"
  pause_reason="$new_pause"

  case "$status" in
    running|paused) seen_active="true" ;;
    completed|failed|cancelled) terminal="true"; break ;;
  esac

  # Ongoing consistency check: workflow_id must STAY null.
  wf=$(jq -r '.workflow_id // "null"' <<<"$run_json")
  if [[ "$wf" != "null" ]]; then
    fail_assert "workflow_id became '${wf}' mid-run — delegated run leaked into the local Temporal executor"
    break
  fi

  sleep "$POLL_SECS"
done

# ---------------------------------------------------------------------------
# 3. Terminal assertions
# ---------------------------------------------------------------------------

if [[ "$terminal" != "true" ]]; then
  if [[ "$status" == "paused" && "$pause_reason" == "authentication" ]]; then
    fail_assert "run is stuck paused on pause_reason=authentication — the runtime's Codex/model auth is not pre-provisioned (shadow validation requires a pre-authenticated runtime)"
  else
    fail_assert "run did not reach a terminal state within ${TIMEOUT_SECS}s (last status=${status:-none}, pause_reason=${pause_reason:-none})"
  fi
  cleanup_on_timeout
  die "canary FAILED for run ${RUN_ID} (${FAILURES} assertion(s) failed)"
fi

# Assertion (c): transitioned through running/paused to terminal.
if [[ "$seen_active" != "true" ]]; then
  if [[ "$status" == "completed" ]]; then
    warn "never observed running/paused between polls (fast run or coarse poll interval) — terminal 'completed' implies execution; not failing on this alone"
  else
    fail_assert "run reached terminal '${status}' without ever being observed running/paused — likely failed at or immediately after launch"
  fi
fi

log "terminal status: ${status}"

if [[ "$status" != "completed" ]]; then
  err_msg=$(jq -r '.error_message // "-"' <<<"$run_json")
  fail_assert "terminal status is '${status}' (want 'completed'); error_message=${err_msg}"
fi

# Assertion (b) final: workflow_id stayed null end-to-end.
wf=$(jq -r '.workflow_id // "null"' <<<"$run_json")
if [[ "$wf" != "null" ]]; then
  fail_assert "workflow_id is '${wf}' at terminal state — must stay null for delegated runs"
fi

# Re-check delegation fields survived projection updates.
ext_runtime=$(jq -r '.external_runtime // "null"' <<<"$run_json")
ext_runtime_id=$(jq -r '.external_runtime_id // "null"' <<<"$run_json")
if [[ "$ext_runtime" != "agent-runtime" || "$ext_runtime_id" == "null" || -z "$ext_runtime_id" ]]; then
  fail_assert "external_runtime pair was lost during projection (external_runtime=${ext_runtime}, external_runtime_id=${ext_runtime_id})"
fi

# --- Assertion (e): REAL-MODEL CANARY — tokens_used must be > 0 -------------
tokens_used=$(jq -r '.tokens_used // 0' <<<"$run_json")
input_tokens=$(jq -r '.input_tokens // 0' <<<"$run_json")
output_tokens=$(jq -r '.output_tokens // 0' <<<"$run_json")
log "usage: tokens_used=${tokens_used} input_tokens=${input_tokens} output_tokens=${output_tokens} cached_input_tokens=$(jq -r '.cached_input_tokens // 0' <<<"$run_json")"

if (( tokens_used <= 0 )); then
  fail_assert "tokens_used=${tokens_used} — zero token usage means the runtime ran a deterministic/stub fallback or usage projection is broken. Shadow validation requires real model execution."
fi

# --- Assertion (f): billing consumption marker on completed runs ------------
if [[ "$status" == "completed" ]]; then
  consumed=$(jq -r '.output_summary.agent_runtime_usage_consumed // false' <<<"$run_json")
  if [[ "$consumed" != "true" ]]; then
    fail_assert "output_summary.agent_runtime_usage_consumed is '${consumed}' (want true) — terminal billing consumption was not marked by the projection worker"
  else
    log "billing consumption marked at $(jq -r '.output_summary.agent_runtime_usage_consumed_at // "-"' <<<"$run_json")"
  fi
fi

# --- Assertion (d): at least one assistant message projected ----------------
resp=$(api GET "/pm/agent-runs/${RUN_ID}/messages")
http_code=$(head -n1 <<<"$resp")
msgs_json=$(tail -n +2 <<<"$resp")
if [[ "$http_code" != "200" ]]; then
  fail_assert "GET /pm/agent-runs/${RUN_ID}/messages returned HTTP ${http_code}"
else
  assistant_count=$(jq '[.[] | select(.role == "assistant")] | length' <<<"$msgs_json")
  total_count=$(jq 'length' <<<"$msgs_json")
  log "messages: total=${total_count} assistant=${assistant_count}"
  if (( assistant_count < 1 )); then
    fail_assert "no assistant messages projected into agent_run_messages — transcript projection from AGENT_RUNTIME_EVENTS is broken"
  fi
fi

# ---------------------------------------------------------------------------
# Verdict
# ---------------------------------------------------------------------------

if (( FAILURES > 0 )); then
  printf '[canary] FAILED: %d assertion(s) failed for run %s\n' "$FAILURES" "$RUN_ID" >&2
  exit 1
fi

log "PASSED: run ${RUN_ID} delegated, executed on a real model, and projected back cleanly"
exit 0
