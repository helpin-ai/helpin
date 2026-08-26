# Helpin Events Pipeline

This directory contains Helpin's authenticated event-ingestion pipeline. It was
originally derived from Usermaven, but Helpin's tenancy, identity, deployment,
and ClickHouse contracts are authoritative here. The old documentation site
under `events-pipeline/docs` is retained only as upstream historical reference.

Buyer-signal behavior built on these events is documented in
[`../docs/crm-buyer-signals.md`](../docs/crm-buyer-signals.md).

## Responsibilities

The pipeline:

- accepts browser and authenticated server events;
- resolves the workspace from the matched credential;
- writes the canonical workspace UUID to `project_id`;
- records credential kind and identity provenance;
- enriches events with URL, device, bot, proxy, geo, and privacy context;
- assigns stable anonymous and session identifiers;
- buffers and replays retryable deliveries;
- writes the Usermaven-compatible typed event contract to
  `usermaven.events` in ClickHouse.

The browser never supplies or learns `project_id`. A payload field that looks
like workspace or project tenancy is not authoritative. Browser identity claims
also remain untrusted unless they are verified by Helpin's signed-identity
contract; authenticated server evidence can carry verified identity.

## Local pipeline verification

From the Helpin repository root:

```bash
just events-e2e
```

This starts an isolated three-node JetStream cluster and ClickHouse instance,
runs the real capture and writer binaries, executes the transport and recovery
checks below, and removes the disposable infrastructure. Its test credential is
a local fixture and is not shared with application development or deployment.

## ClickHouse schema

ClickHouse migrations are versioned under `server/internal/chmigrate/sql` and
carry checksums. Local stack startup runs the same Go migration implementation
used by deployment hooks.

```bash
cd server
go run ./cmd/clickhouse-migrate status
go run ./cmd/clickhouse-migrate validate
go run ./cmd/clickhouse-migrate up
```

Stage and production apply migrations before event consumers are updated. The
backend and migration job require `CLICKHOUSE_DSN`; transport-specific settings
are defined in the deployment and local stack configuration rather than this
document.

The final `usermaven.events` table retains the raw normalized event for replay
and debugging and exposes typed materialized columns for tenant-safe queries.
Signal evaluators query bounded project sets and time windows; CRM pages never
query this table synchronously.

## Reliability and security invariants

- The token registry is the only credential-to-workspace authority.
- Browser and server credentials for one installation resolve to the same
  canonical `project_id`.
- Credential rotation does not change the canonical project.
- Raw credentials must not be stored as event `api_key` values or logged.
- Failed, buffered, replayed, and successful paths preserve the authorized
  project and identity provenance.
- Delivery is retry-safe and downstream storage is idempotent.
- A stale token-registry refresh retains the last valid set rather than
  accepting unknown credentials.
- Shutdown drains in-flight work and exposes health/readiness state.

## Development and tests

Isolated end-to-end verification (three-node JetStream plus a disposable
ClickHouse instance, automatically removed when the test finishes):

```bash
just events-e2e
```

This verifies inline enrichment, archive/DLQ routing, explicit WorkQueue
acknowledgements, session-window behavior, ClickHouse insertion, and session
cache seeding after a writer restart. It uses ports 14222, 18222, 18123, and
19000 for disposable infrastructure while capture and writer health use
3000/3001 and 3010/3011 respectively.

Rust component tests:

```bash
cd events-pipeline/rust-capture
cargo test
```

ClickHouse migration tests:

```bash
cd server
go test ./internal/chmigrate
```

`events-e2e` is the canonical transport and storage verification. The separate
browser smoke suite covers browser instrumentation and application-level signal
generation. The complete local sustained and split-host capacity-test runbook,
including enrichment inputs, result files, interpretation, and cleanup, is in
[`e2e/README.md`](e2e/README.md).

The verified 15,000 events/second baseline, per-layer CPU and memory results,
stage/production topology, and 48-hour JetStream capacity calculation are in
[`CAPACITY_BASELINE.md`](CAPACITY_BASELINE.md). That document is the canonical
deployment-sizing reference; the benchmark's durability caveats apply.

### k6 capture load test

The k6 profile uses an open arrival model, so slow responses do not silently
reduce the requested arrival rate. Run k6 from a separate machine when
measuring production capacity. Preallocate all expected VUs; a non-zero
`dropped_iterations` value means the generator ran out of available VUs.

Single-event requests at 300 events and requests per second:

```bash
TARGET_URL=https://events.example.com/api/v1/event \
TOKEN=replace-with-server-token \
EVENT_RATE=300 \
BATCH_SIZE=1 \
DURATION=15m \
PRE_ALLOCATED_VUS=512 \
MAX_VUS=512 \
SUMMARY_PATH=k6-single-event.json \
k6 run events-pipeline/scripts/k6-capture.js
```

For a fixed simultaneous-connection saturation test, use the closed model:

```bash
PROFILE=connections \
CONNECTIONS=256 \
DURATION=60s \
TARGET_URL=https://events.example.com/api/v1/event \
TOKEN=replace-with-server-token \
k6 run events-pipeline/scripts/k6-capture.js
```

Connections are reused by default, matching SDK keep-alive behavior. Set
`NO_VU_CONNECTION_REUSE=true` to close a VU's connection between iterations,
or `NO_CONNECTION_REUSE=true` to disable HTTP keep-alive entirely. The test
fails on request errors, dropped iterations, p50 above 10 ms, or p99 above
25 ms. Override `P50_LIMIT_MS` and `P99_LIMIT_MS` only when intentionally
testing a different SLO.

## Directory map

| Area | Location |
|---|---|
| capture and processing | `rust-capture/` |
| local orchestration | `scripts/`, `../docker-compose.yaml` |
| browser signal lab | `../frontend/public/event-test/`, `../frontend/e2e/event-pipeline/` |
| ClickHouse migrations | `../server/internal/chmigrate/sql/` |
| ClickHouse migration CLI | `../server/cmd/clickhouse-migrate/` |
| CRM behavioral rules | `../server/internal/repository/event_clickhouse_rules.go` |

Component READMEs may describe low-level implementation details, but this file
is the canonical Helpin entry point for pipeline operations and invariants.
