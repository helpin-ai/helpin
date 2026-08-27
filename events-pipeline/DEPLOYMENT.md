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

Argo CD is expected to recurse through `k8s/stage`. It creates a dedicated
three-member Helpin Keeper ensemble, a one-shard/two-replica Helpin ClickHouse
installation, the three-node NATS StatefulSet, two capture/replay StatefulSet
pods, two writer pods, and the Postgres and ClickHouse migration hooks. The
Altinity operator, External Secrets Operator, Sealed Secrets, and cert-manager
remain cluster prerequisites; the repository owns the Helpin custom resources
and application configuration.

Do not merge the first rollout with unattended Argo CD auto-sync. The merge
commit initially contains new Jobs that reference the previous release images;
the image-tag commits arrive only after both build workflows finish. Pause
auto-sync before merging, wait for both tag commits, then perform one controlled
sync. Later code-only releases can use the normal automated path.

## Required external services and secrets

The pipeline uses External Secrets Operator with a namespaced Doppler
`SecretStore`; it does not use the Doppler Operator. Separate staging and
production `SealedSecret` manifests create
`doppler-token-helpin-eventpipeline` in `helpin` from service tokens encrypted
against each environment's controller. Plaintext tokens are never committed.
The checked-in `ExternalSecret` then projects explicit Doppler keys into
`helpin-eventpipeline-secrets`.

When rotating a Doppler service token, create a temporary Secret with the exact
name and namespace, seal it against that environment's controller, replace only
that environment's `doppler-token.sealed.yaml`, and immediately delete the
plaintext file. A SealedSecret is cluster-specific; never copy the staging
ciphertext to production or vice versa.

The Doppler config must contain:

| Key | Purpose |
| --- | --- |
| `NATS_JETSTREAM_ENCRYPTION_KEY` | stable JetStream-at-rest key, at least 32 random bytes |
| `NATS_CAPTURE_PASSWORD`, `NATS_WRITER_PASSWORD`, `NATS_OPS_PASSWORD` | independent NATS users |
| `CLICKHOUSE_MIGRATION_PASSWORD` | schema owner used by the migration Job |
| `CLICKHOUSE_API_PASSWORD` | API read/retention user |
| `CLICKHOUSE_WRITER_PASSWORD` | event writer user |
| `CLICKHOUSE_R2_ENDPOINT` | S3-compatible URL including the Helpin bucket/prefix and ending in `/` |
| `CLICKHOUSE_R2_ACCESS_KEY_ID`, `CLICKHOUSE_R2_SECRET_ACCESS_KEY` | Cloudflare R2 S3 credentials |

Use URL-safe random ClickHouse passwords (hex is recommended), because the
migration and API manifests expand them into native-protocol DSNs. NATS TLS
keys are not stored in Doppler: cert-manager creates the private CA and the
server, capture, writer, and operations certificates.

The `helpin.events` table retains events for 400 days on the R2-backed `events`
storage policy. Replication metadata, the R2 cache, and
`helpin.session_seed_events` remain on each ClickHouse host's retained local
volume. The six-hour storage figures in the capacity baseline apply only to the
transient JetStream outage buffer;
size and back up ClickHouse separately from representative compressed data.

The Doppler configuration synchronized to `helpin-secrets` must contain:

- `INTERNAL_API_SECRET`: a strong shared bearer secret. The API and capture
  receive the same value. The capture token URL is set in the manifest to the
  in-cluster API service.
- one working LLM route. The checked-in CRM defaults use
  `OPENROUTER_API_KEY`; provider/model override pairs remain optional.
- `MAXMIND_ACCOUNT_ID` and `MAXMIND_LICENSE_KEY` when network enrichment is
  required. `IP2PROXY_DOWNLOADER_URL` must return the licensed binary directly.
  Missing databases fail open and are visible in capture metrics.
- `CRM_ENCRYPTION_KEY`, `GMAIL_CLIENT_ID`, `GMAIL_CLIENT_SECRET`, and the
  environment-specific `GMAIL_OAUTH_REDIRECT_URL` only when Gmail sync is
  enabled. The encryption key is a stable 32-byte hex value and must not be
  rotated without re-encrypting stored OAuth tokens.

Use independent random NATS passwords and at least 32 random bytes for the
JetStream encryption key. Never rotate that key in place: existing JetStream
files cannot be decrypted with the replacement.

The generated NATS certificates chain to one private CA. Client certificates
need `clientAuth`. The server certificate needs both `serverAuth` and
`clientAuth` because NATS nodes accept and initiate cluster routes. Include the
client service and StatefulSet identities in its SANs:

```text
helpin-eventpipeline-nats
helpin-eventpipeline-nats.helpin.svc
helpin-eventpipeline-nats.helpin.svc.cluster.local
helpin-eventpipeline-nats-{0,1,2}.helpin-eventpipeline-nats-headless.helpin.svc.cluster.local
```

The existing `ghcr-helpin-json-key` image-pull secret is also required.

## Cluster prerequisites

- External Secrets Operator, Sealed Secrets, cert-manager, and Altinity
  ClickHouse Operator (including the CHI and CHKI CRDs) must already be
  installed.
- At least three schedulable amd64 cloud nodes labeled
  `node.hetzner.com/type=cloud` are required. NATS uses hard hostname
  anti-affinity; capture and writer use soft hostname anti-affinity.
- Two dedicated ClickHouse nodes must carry
  `node-role.kubernetes.io/worker=clickhouse`, and three dedicated Keeper nodes
  must carry `node-role.kubernetes.io/worker=pipeline`. Production additionally
  selects `topology.kubernetes.io/region=fsn1`, which pins the two Helpin
  replicas to the existing AX162-S pair instead of the third AX101 ClickHouse
  host. Stage has exactly two ClickHouse-role hosts, so it needs no additional
  selector.
- `hcloud-volumes-retain` and `local-path` StorageClasses must exist. NATS,
  capture fallback, and writer poison spill use retained HCloud volumes. Stage
  creates 10 GiB claims; production NATS uses 20 GiB while capture/writer claims
  remain 10 GiB. ClickHouse and Keeper use retained host-local storage; event
  table parts reside on R2 behind a bounded local cache.
- Before creating the Helpin CHI, change the legacy `clickhouse/clickhouse`
  installation's required pod anti-affinity selector from the broad
  `clickhouse.altinity.com/app=chop` selector to
  `clickhouse.altinity.com/chi=clickhouse` in every pod template. Otherwise it
  blocks the new Helpin replicas from the two shared dedicated hosts.
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
5. If a capture Deployment from an earlier preview exists, scale it to zero;
   this release replaces it with a StatefulSet so its fallback spool survives
   pod replacement.
6. Sync once. The Doppler token SealedSecret runs at wave `-7`; secret stores,
   external secrets, and certificate issuers run at waves `-6` through `-3`;
   Keeper runs at `-4`, ClickHouse at `-3`, NATS at `-2`, migration/bootstrap
   Jobs at `-1`, and capture/writers at wave `0`.
7. If an existing managed consumer topology is present, stop all writer pods
   before bootstrap and set `EVENTS_CONSUMER_REBALANCE_FROM` to the exact active
   topology reported by the guard. A new cluster has no topology to rebalance.
8. Resume auto-sync only after the verification below passes.

## Verification

Verify Kubernetes health without printing secret values:

```bash
kubectl -n helpin rollout status statefulset/helpin-eventpipeline-nats --timeout=10m
kubectl -n helpin rollout status statefulset/helpin-eventpipeline-web --timeout=10m
kubectl -n helpin rollout status statefulset/helpin-eventpipeline-writer --timeout=10m
kubectl -n helpin get chi,chk
kubectl -n helpin get pods,pvc
```

Then verify behavior:

1. `GET https://client.stage.helpin.ai/health/readiness` is healthy.
2. Capture logs show a non-zero token registry and both enrichment database
   availability metrics are `1` if licensed enrichment is required.
3. Send one browser event with a staging widget credential and one authenticated
   server event. Neither request may use a raw workspace UUID as authority.
4. Confirm exact event IDs in `helpin.events`, and confirm both writer
   ordinals are ready with no fallback, poison spill, DLQ, or redelivery growth.
5. Confirm the API reports a successful ClickHouse connection and the CRM
   evaluator records a successful ten-minute run.
6. Confirm the event's `project_id` matches the workspace and its external user
   or company identity maps to a CRM contact/company. Open CRM Signals and
   verify the generated signal.

Seeded deterministic rules intentionally start in shadow mode. They store and
display evidence but cannot route notifications or create tasks. Collect
precision feedback first; then activate an eligible exact rule version and an
explicit routing-policy version through the `/api/crm/signals/...` endpoints.
That promotion is a product decision, not a deployment step.
