#!/usr/bin/env bash
set -euo pipefail

namespace="${1:-helpin}"
failures=0
warnings=0

fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

warn() {
  echo "WARN: $*" >&2
  warnings=$((warnings + 1))
}

pass() {
  echo "PASS: $*"
}

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required" >&2
  exit 2
fi

context="$(kubectl config current-context 2>/dev/null || true)"
if [[ -z "$context" ]]; then
  echo "kubectl has no current context" >&2
  exit 2
fi
echo "Context: $context"
echo "Namespace: $namespace"

if ! kubectl get --raw=/readyz >/dev/null 2>&1; then
  echo "Cannot authenticate to the Kubernetes API for context $context. Refresh cluster credentials before interpreting preflight results." >&2
  exit 2
fi

if ! kubectl get namespace "$namespace" >/dev/null 2>&1; then
  fail "namespace $namespace does not exist"
fi

check_secret() {
  local name="$1"
  shift
  local keys
  if ! keys="$(kubectl -n "$namespace" get secret "$name" -o go-template='{{range $key, $_ := .data}}{{$key}}{{"\n"}}{{end}}' 2>/dev/null)"; then
    fail "secret $name is missing"
    return
  fi
  local key
  for key in "$@"; do
    if ! grep -Fxq "$key" <<<"$keys"; then
      fail "secret $name is missing key $key"
    fi
  done
}

check_secret helpin-secrets INTERNAL_API_SECRET CLICKHOUSE_DSN OPENROUTER_API_KEY
check_secret helpin-eventpipeline-writer CLICKHOUSE_HTTP_URL CLICKHOUSE_USER CLICKHOUSE_PASSWORD
check_secret helpin-eventpipeline-nats-secrets jetstream-encryption-key capture-password writer-password ops-password
check_secret helpin-eventpipeline-nats-server-tls ca.crt tls.crt tls.key
check_secret helpin-eventpipeline-nats-capture-tls ca.crt tls.crt tls.key
check_secret helpin-eventpipeline-nats-writer-tls ca.crt tls.crt tls.key
check_secret helpin-eventpipeline-nats-ops-tls ca.crt tls.crt tls.key
check_secret ghcr-helpin-json-key .dockerconfigjson

ready_amd64_nodes="$(kubectl get nodes -l kubernetes.io/arch=amd64 \
  -o 'custom-columns=READY:.status.conditions[?(@.type=="Ready")].status,SCHEDULING:.spec.unschedulable' \
  --no-headers 2>/dev/null | awk '$1 == "True" && $2 != "true" {count++} END {print count+0}')"
if (( ready_amd64_nodes < 3 )); then
  fail "three schedulable Ready amd64 nodes are required; found $ready_amd64_nodes"
else
  pass "$ready_amd64_nodes schedulable Ready amd64 nodes"
fi

default_storage_classes="$(kubectl get storageclass \
  -o jsonpath='{range .items[?(@.metadata.annotations.storageclass\.kubernetes\.io/is-default-class=="true")]}{.metadata.name}{"\n"}{end}' \
  2>/dev/null || true)"
if [[ -z "$default_storage_classes" ]]; then
  fail "no default StorageClass; set storageClassName in the NATS volumeClaimTemplate"
else
  pass "default StorageClass: $(tr '\n' ' ' <<<"$default_storage_classes" | xargs)"
fi

if kubectl -n "$namespace" get secret helpin-secrets >/dev/null 2>&1; then
  helpin_keys="$(kubectl -n "$namespace" get secret helpin-secrets -o go-template='{{range $key, $_ := .data}}{{$key}}{{"\n"}}{{end}}')"
  for enrichment_key in MAXMIND_ACCOUNT_ID MAXMIND_LICENSE_KEY IP2PROXY_DOWNLOADER_URL; do
    if ! grep -Fxq "$enrichment_key" <<<"$helpin_keys"; then
      warn "helpin-secrets lacks $enrichment_key; that enrichment database will fail open"
    fi
  done
  gmail_count=0
  for gmail_key in CRM_ENCRYPTION_KEY GMAIL_CLIENT_ID GMAIL_CLIENT_SECRET GMAIL_OAUTH_REDIRECT_URL; do
    if grep -Fxq "$gmail_key" <<<"$helpin_keys"; then
      gmail_count=$((gmail_count + 1))
    fi
  done
  if (( gmail_count > 0 && gmail_count < 4 )); then
    fail "Gmail CRM sync is partially configured; set all four Gmail/encryption keys or none"
  elif (( gmail_count == 0 )); then
    warn "Gmail CRM sync is not configured (optional)"
  else
    pass "Gmail CRM sync keys are present"
  fi
fi

if (( failures > 0 )); then
  echo "Preflight failed: $failures failure(s), $warnings warning(s)" >&2
  exit 1
fi

echo "Preflight passed with $warnings warning(s). Argo CD recursion, ClickHouse connectivity/capacity, certificate SANs, and image tags still require operator verification."
