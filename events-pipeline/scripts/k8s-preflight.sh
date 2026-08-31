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

check_secret helpin-secrets INTERNAL_API_SECRET OPENROUTER_API_KEY
check_secret ghcr-helpin-json-key .dockerconfigjson

for crd in \
  clickhouseinstallations.clickhouse.altinity.com \
  clickhousekeeperinstallations.clickhouse-keeper.altinity.com \
  secretstores.external-secrets.io \
  externalsecrets.external-secrets.io \
  sealedsecrets.bitnami.com \
  certificates.cert-manager.io; do
  if kubectl get crd "$crd" >/dev/null 2>&1; then
    pass "CRD $crd"
  else
    fail "required CRD $crd is missing"
  fi
done

if kubectl -n "$namespace" get secret doppler-token-helpin-eventpipeline >/dev/null 2>&1; then
  check_secret doppler-token-helpin-eventpipeline dopplerToken
elif kubectl -n "$namespace" get sealedsecret doppler-token-helpin-eventpipeline >/dev/null 2>&1; then
  fail "SealedSecret doppler-token-helpin-eventpipeline exists but its Secret has not been unsealed"
else
  warn "the pipeline Doppler token is not deployed yet; the checked-in SealedSecret creates it at sync wave -7"
fi

ready_cloud_nodes="$(kubectl get nodes -l kubernetes.io/arch=amd64,node.hetzner.com/type=cloud \
  -o 'custom-columns=READY:.status.conditions[?(@.type=="Ready")].status,SCHEDULING:.spec.unschedulable' \
  --no-headers 2>/dev/null | awk '$1 == "True" && $2 != "true" {count++} END {print count+0}')"
if (( ready_cloud_nodes < 3 )); then
  fail "three schedulable Ready amd64 cloud nodes are required for NATS anti-affinity; found $ready_cloud_nodes"
else
  pass "$ready_cloud_nodes schedulable Ready amd64 cloud nodes"
fi

for role_and_count in clickhouse:2 pipeline:3; do
  role="${role_and_count%%:*}"
  required="${role_and_count##*:}"
  ready="$(kubectl get nodes -l "node-role.kubernetes.io/worker=$role" \
    -o 'custom-columns=READY:.status.conditions[?(@.type=="Ready")].status,SCHEDULING:.spec.unschedulable' \
    --no-headers 2>/dev/null | awk '$1 == "True" && $2 != "true" {count++} END {print count+0}')"
  if (( ready < required )); then
    fail "$required schedulable Ready $role nodes are required; found $ready"
  else
    pass "$ready schedulable Ready $role nodes"
  fi
done

for storage_class in hcloud-volumes-retain local-path; do
  if kubectl get storageclass "$storage_class" >/dev/null 2>&1; then
    pass "StorageClass $storage_class"
  else
    fail "required StorageClass $storage_class is missing"
  fi
done

if kubectl -n clickhouse get chi clickhouse >/dev/null 2>&1; then
  shared_anti_affinity="$(kubectl -n clickhouse get chi clickhouse \
    -o jsonpath='{range .spec.templates.podTemplates[*].spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[*].labelSelector.matchExpressions[*]}{.key}{"="}{.values[*]}{"\n"}{end}' \
    2>/dev/null || true)"
  if grep -Fqx 'clickhouse.altinity.com/app=chop' <<<"$shared_anti_affinity"; then
    fail "the legacy clickhouse/clickhouse CHI anti-affinity matches every operator pod; scope it to clickhouse.altinity.com/chi=clickhouse before creating the Helpin CHI"
  else
    pass "legacy ClickHouse anti-affinity does not block the Helpin CHI"
  fi
fi

if kubectl -n "$namespace" get secret helpin-eventpipeline-secrets >/dev/null 2>&1; then
  check_secret helpin-eventpipeline-secrets \
    NATS_JETSTREAM_ENCRYPTION_KEY NATS_CAPTURE_PASSWORD NATS_WRITER_PASSWORD NATS_OPS_PASSWORD \
    CLICKHOUSE_MIGRATION_PASSWORD CLICKHOUSE_API_PASSWORD CLICKHOUSE_WRITER_PASSWORD \
    CLICKHOUSE_R2_ENDPOINT CLICKHOUSE_R2_ACCESS_KEY_ID CLICKHOUSE_R2_SECRET_ACCESS_KEY
else
  warn "helpin-eventpipeline-secrets is not synchronized yet; verify the ExternalSecret after the first secrets sync wave"
fi

for tls_secret in \
  helpin-eventpipeline-nats-server-tls \
  helpin-eventpipeline-nats-capture-tls \
  helpin-eventpipeline-nats-writer-tls \
  helpin-eventpipeline-nats-ops-tls; do
  if kubectl -n "$namespace" get secret "$tls_secret" >/dev/null 2>&1; then
    check_secret "$tls_secret" ca.crt tls.crt tls.key
  else
    warn "$tls_secret is not issued yet; verify its Certificate after the cert-manager sync waves"
  fi
done

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

echo "Preflight passed with $warnings warning(s). Argo CD recursion, ClickHouse connectivity, the legacy CHI anti-affinity prerequisite, certificate readiness, and image tags still require operator verification."
