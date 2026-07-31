#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <kubectl-context> --confirm" >&2
  exit 2
}

if [[ $# -ne 2 || "$2" != "--confirm" ]]; then
  usage
fi

context="$1"
namespace="${HELPIN_NAMESPACE:-helpin}"
deployments=(
  helpin-temporal-native-interactive
  helpin-temporal-native-autonomous
  helpin-temporal-opencode-autonomous
  helpin-temporal-codex-autonomous
  helpin-temporal-codex-interactive
)

if ! command -v kubectl >/dev/null 2>&1; then
  echo "missing required command: kubectl" >&2
  exit 1
fi

echo "deleting retired agent worker deployments from $context/$namespace"
kubectl --context "$context" -n "$namespace" delete deployment \
  "${deployments[@]}" \
  --ignore-not-found=true \
  --wait=true

echo "retired agent worker deployments deleted"
