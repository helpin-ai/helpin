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

## Local stack

From the Helpin repository root:

```bash
cp events-pipeline/.env.events.example events-pipeline/.env.events
just events-up
just events-smoke
just events-browser-smoke
```

- `events-up` starts the local transport, capture, processing, replay, and
  ClickHouse services defined by the checked-in stack scripts.
- `events-smoke` sends authenticated events and verifies their canonical
  project at each processing boundary and in ClickHouse.
- `events-browser-smoke` runs the browser signal lab and generates page, form,
  article, interaction, and product events.
- `just events-logs` follows the local services.
- `just events-down` stops the stack.

The stack reads the internal API secret from `server/.env`; event credentials
are not committed into Compose files.

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

The complete verification path is the pair of repository-root smoke commands.
They cover service health, authenticated tenancy propagation, ingestion into
ClickHouse, browser capture, and isolation between test projects.

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
