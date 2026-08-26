# Event pipeline deployment runbook

This runbook covers the first staging rollout of the NATS-to-ClickHouse event
pipeline and the CRM behavioral signals that consume its data. The manifests
are sized from [`CAPACITY_BASELINE.md`](CAPACITY_BASELINE.md).

## What a merge deploys

A merge to `develop` triggers separate staging workflows:

- `Staging Release` builds the API, Temporal worker, and migration image, then
  commits their immutable image tags under `k8s/stage`;
- `Events Pipeline - Staging` builds the Rust image, then commits its immutable
  tag to the capture, replay, bootstrap, and writer manifests;
- the SDK workflow publishes the browser SDK to the staging CDN; and
- the npm staging workflow publishes package RCs when the PR is merged.

Argo CD is expected to recurse through `k8s/stage`. It creates the three-node
NATS StatefulSet, two capture/replay pods, two writer pods, and runs the
Postgres and ClickHouse migration hooks. ClickHouse itself is external: this
repository creates its schema, but does not provision a ClickHouse server,
storage, backups, or credentials.

Do not merge the first rollout with unattended Argo CD auto-sync. The merge
commit initially contains new Jobs that reference the previous release images;
the image-tag commits arrive only after both build workflows finish. Pause
auto-sync before merging, wait for both tag commits, then perform one controlled
sync. Later code-only releases can use the normal automated path.

## Required external services and secrets

Provision ClickHouse before syncing. Both endpoints below must reach the same
cluster and `usermaven` database:

| Consumer | Configuration | Required access |
| --- | --- | --- |
| API and `helpin-clickhouse-migrate` | `CLICKHOUSE_DSN` in Doppler-backed `helpin-secrets` | native protocol; schema migration plus CRM reads |
| session writers | `CLICKHOUSE_HTTP_URL`, `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD`, and optionally `CLICKHOUSE_DATABASE` in `helpin-eventpipeline-writer` | HTTP insert and session-seed reads |

The ClickHouse table retains events for 400 days. The six-hour storage figures
in the capacity baseline apply only to the transient JetStream outage buffer;
size and back up ClickHouse separately from representative compressed data.

The Doppler configuration synchronized to `helpin-secrets` must contain:

- `INTERNAL_API_SECRET`: a strong shared bearer secret. The API and capture
  receive the same value. The capture token URL is set in the manifest to the
  in-cluster API service.
- `CLICKHOUSE_DSN`: required for ClickHouse migrations and behavioral rules.
- one working LLM route. The checked-in CRM defaults use
  `OPENROUTER_API_KEY`; provider/model override pairs remain optional.
- `MAXMIND_ACCOUNT_ID` and `MAXMIND_LICENSE_KEY` when network enrichment is
  required. `IP2PROXY_DOWNLOADER_URL` must return the licensed binary directly.
  Missing databases fail open and are visible in capture metrics.
- `CRM_ENCRYPTION_KEY`, `GMAIL_CLIENT_ID`, `GMAIL_CLIENT_SECRET`, and the
  environment-specific `GMAIL_OAUTH_REDIRECT_URL` only when Gmail sync is
  enabled. The encryption key is a stable 32-byte hex value and must not be
  rotated without re-encrypting stored OAuth tokens.

Create these Kubernetes secrets before Argo CD sync:

| Secret | Required keys |
| --- | --- |
| `helpin-eventpipeline-nats-secrets` | `jetstream-encryption-key`, `capture-password`, `writer-password`, `ops-password` |
| `helpin-eventpipeline-writer` | `CLICKHOUSE_HTTP_URL`, `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD`; `CLICKHOUSE_DATABASE` is optional |
| `helpin-eventpipeline-nats-server-tls` | `ca.crt`, `tls.crt`, `tls.key` |
| `helpin-eventpipeline-nats-capture-tls` | `ca.crt`, `tls.crt`, `tls.key` |
| `helpin-eventpipeline-nats-writer-tls` | `ca.crt`, `tls.crt`, `tls.key` |
| `helpin-eventpipeline-nats-ops-tls` | `ca.crt`, `tls.crt`, `tls.key` |

Use independent random NATS passwords and at least 32 random bytes for the
JetStream encryption key. Never rotate that key in place: existing JetStream
files cannot be decrypted with the replacement.

All NATS certificates must chain to one private CA. Client certificates need
`clientAuth`. The server certificate needs both `serverAuth` and `clientAuth`
because NATS nodes accept and initiate cluster routes. Include the client
service and StatefulSet identities in its SANs:

```text
helpin-eventpipeline-nats
helpin-eventpipeline-nats.helpin.svc
helpin-eventpipeline-nats.helpin.svc.cluster.local
helpin-eventpipeline-nats-{0,1,2}.helpin-eventpipeline-nats-headless.helpin.svc.cluster.local
```

The existing `ghcr-helpin-json-key` image-pull secret is also required.

## Cluster prerequisites

- At least three schedulable amd64 nodes are required. NATS uses hard
  hostname anti-affinity, so fewer nodes leave pods Pending.
- A default `ReadWriteOnce` StorageClass must exist, or set
  `storageClassName` explicitly before the first sync. Stage creates three 8
  GiB PVCs; production creates three 16 GiB PVCs. Use provisioned-IOPS storage
  before performance qualification or a major traffic increase.
- Confirm the Argo CD application includes nested directories under
  `k8s/stage`; otherwise the event-pipeline manifests are never applied.
- Confirm `client.stage.helpin.ai` resolves to the ingress and its wildcard TLS
  secret exists.

Run the read-only checks after selecting the intended kubectl context:

```bash
# Refresh the configured AWS SSO session first when the EKS token has expired.
aws sso login --profile <staging-profile>
events-pipeline/scripts/k8s-preflight.sh helpin
```

## Controlled first staging rollout

1. Pause Argo CD auto-sync and verify the prerequisites above.
2. Merge the PR to `develop`.
3. Wait for `Staging Release`, `Events Pipeline - Staging`, and the staging SDK
   deployment to succeed. Wait for their manifest-tag commits on `develop`.
4. Refresh Argo CD and inspect the rendered diff. The ClickHouse and Postgres
   migration images and every event-pipeline command must use the new tags.
5. Sync once. The NATS resources run at wave `-2`, bootstrap at wave `-1`, and
   capture/writers at wave `0`. Migration hooks must complete before workloads.
6. If an existing managed consumer topology is present, stop all writer pods
   before bootstrap. The checked-in guard only permits an exact `r003` to
   `r002` transition. If bootstrap reports another topology, stop and set
   `EVENTS_CONSUMER_REBALANCE_FROM` to that exact value; never bypass the guard
   while writers are live. A new cluster has no topology to rebalance.
7. Resume auto-sync only after the verification below passes.

## Verification

Verify Kubernetes health without printing secret values:

```bash
kubectl -n helpin rollout status statefulset/helpin-eventpipeline-nats --timeout=10m
kubectl -n helpin rollout status deployment/helpin-eventpipeline-web --timeout=10m
kubectl -n helpin rollout status statefulset/helpin-eventpipeline-writer --timeout=10m
kubectl -n helpin get pods,pvc
```

Then verify behavior:

1. `GET https://client.stage.helpin.ai/health/readiness` is healthy.
2. Capture logs show a non-zero token registry and both enrichment database
   availability metrics are `1` if licensed enrichment is required.
3. Send one browser event with a staging widget credential and one authenticated
   server event. Neither request may use a raw workspace UUID as authority.
4. Confirm exact event IDs in `usermaven.events`, and confirm both writer
   ordinals are ready with no fallback, poison spill, DLQ, or redelivery growth.
5. Confirm the API reports a successful ClickHouse connection and the CRM
   evaluator records a successful ten-minute run.
6. Confirm the event's `project_id` matches the workspace and its external user
   or company identity maps to a CRM contact/company. Open CRM Insights and
   verify the generated signal.

Seeded deterministic rules intentionally start in shadow mode. They store and
display evidence but cannot route notifications or create tasks. Collect
precision feedback first; then activate an eligible exact rule version and an
explicit routing-policy version through the `/api/crm/signals/...` endpoints.
That promotion is a product decision, not a deployment step.
