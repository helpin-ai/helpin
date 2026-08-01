#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <kubectl-context>" >&2
  exit 2
}

if [[ $# -ne 1 ]]; then
  usage
fi

context="$1"
namespace="${HELPIN_NAMESPACE:-helpin}"
deployment="${HELPIN_SERVER_DEPLOYMENT:-helpin-server}"
secret="${HELPIN_SECRET_NAME:-helpin-secrets}"

for command_name in kubectl psql base64; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "missing required command: $command_name" >&2
    exit 1
  fi
done

echo "checking effective launch flag in $context/$namespace/$deployment"
kubectl --context "$context" -n "$namespace" rollout status \
  "deployment/$deployment" --timeout=30s >/dev/null

mapfile -t server_pods < <(
  kubectl --context "$context" -n "$namespace" get pods \
    -l "app=$deployment" \
    --field-selector=status.phase=Running \
    -o name
)
if [[ ${#server_pods[@]} -eq 0 ]]; then
  echo "no running pods found for deployment $deployment" >&2
  exit 1
fi

for pod in "${server_pods[@]}"; do
  if ! kubectl --context "$context" -n "$namespace" exec \
    "$pod" -c "$deployment" -- \
    /bin/sh -c 'test "${AGENT_RUNTIME_LAUNCH_ENABLED:-}" = "true"'; then
    echo "AGENT_RUNTIME_LAUNCH_ENABLED is not true in $pod" >&2
    exit 1
  fi
done

database_url="$({
  kubectl --context "$context" -n "$namespace" get secret "$secret" \
    -o 'jsonpath={.data.DATABASE_URL}'
} | base64 --decode)"
if [[ -z "$database_url" ]]; then
  echo "DATABASE_URL is missing from secret $secret" >&2
  exit 1
fi

active_unmapped_runs="$({
  psql "$database_url" -v ON_ERROR_STOP=1 -Atqc \
    "SELECT count(*) FROM agent_runs WHERE status NOT IN ('completed', 'failed', 'cancelled') AND external_runtime_id IS NULL;"
})"
unset database_url

if [[ "$active_unmapped_runs" != "0" ]]; then
  echo "found $active_unmapped_runs non-terminal agent runs without external_runtime_id" >&2
  exit 1
fi

echo "preflight passed: launch flag is true and active unmapped run count is zero"
