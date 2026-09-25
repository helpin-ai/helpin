# Rust event capture

The capture service authenticates SDK requests, validates timestamps, performs
stateless enrichment inline, and publishes versioned envelopes to JetStream.
Kafka and Kafka Streams are not part of this pipeline.

```text
SDK -> capture -> events.enriched.v1.<visitor-shard> -> session writer -> ClickHouse
               \-> events.raw.v1
```

The capture process waits only for the work-stream acknowledgement. It publishes
the privacy-normalized diagnostic archive in bounded background tasks, so an
archive outage cannot delay a valid response. A work publish failure is fsynced
to `FALLBACK_DIR`; the replay worker later runs the same enrichment and NATS
publish path. The deployment must put this directory on an encrypted persistent
volume.

## Binaries

| Binary | Purpose |
| --- | --- |
| `events-pipeline` | HTTP capture API and inline enrichment |
| `replay-worker` | Replays fsynced capture spill to NATS |
| `nats-bootstrap` | Reconciles the three JetStream stream definitions |
| `session-writer` | Sessionizes shard batches and inserts them into ClickHouse |

`nats-bootstrap` requires `NATS_URL`, `EVENTS_WORK_MAX_BYTES`,
`EVENTS_RAW_MAX_BYTES`, and `EVENTS_DLQ_MAX_BYTES`. It creates an R3
WorkQueue/DiscardNew work stream, an R1 diagnostic raw archive, and an R3 DLQ.
By default, all are file-backed, S2-compressed, and bounded to a six-hour
maximum age. Work storage/compression and replica counts can be overridden;
see [bootstrap configuration](src/nats_bootstrap.rs). Bootstrap reads the process
environment directly, so export its settings rather than relying on `.env` loading.

## Local development

Run from `events-pipeline/rust-capture` with the Rust toolchain installed:

```bash
cp .env.example .env
PRINT_SINK=true cargo run
cargo test --lib pipeline::tests
cargo test --lib writer::tests
```

`PRINT_SINK=true` logs acceptance metadata without inline enrichment, NATS, or
ClickHouse writes. It still authenticates requests. Set `HTTP_TOKENS_URL` to a
running backend token endpoint and use the same `INTERNAL_API_SECRET` as the
backend; the endpoint must return credentials for your test workspace. An empty
registry can leave the process running while requests fail authentication. See
the [API guide](../rust-capture-api-guide.md) for request credentials and the
[local end-to-end harness](../e2e/README.md) for an isolated test setup.

Set `PRINT_SINK=false` and `NATS_URL` to exercise JetStream. With network
enrichment enabled, capture and replay download missing MaxMind and IP2Proxy
databases at startup using `MAXMIND_ACCOUNT_ID`, `MAXMIND_LICENSE_KEY`, and
`IP2PROXY_DOWNLOADER_URL`. They refresh MaxMind every 12 hours and IP2Proxy
every 24 hours; `MAXMIND_DB_REFRESH_INTERVAL_SECS` and
`IP2PROXY_DB_REFRESH_INTERVAL_SECS` override those intervals. Download or load
failures fail open, and `NETWORK_ENRICHMENT_ENABLED=false` disables both for
credential-free development.

For a local writer process set `NATS_URL`, `CLICKHOUSE_HTTP_URL`,
`CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD`, `WRITER_REPLICAS`, and
`WRITER_ORDINAL`. It reconstructs active sessions from `session_seed_events`,
pulls bounded batches from one multi-subject durable consumer per writer, and
sends AckProgress until ClickHouse and any terminal DLQ publication have
completed. The 100 logical subjects are assigned in balanced contiguous ranges;
the current two-writer topology owns 50 subjects per writer.
When running capture and writer on the same host, set distinct
`WRITER_HEALTH_PORT` and `WRITER_METRICS_PORT` values: the writer defaults to
3000/3001, which collide with capture.
Production mounts the private NATS CA and role-specific client certificate
through the `NATS_*_FILE` variables shown in `.env.example`.

The API listens on `:3000`, metrics on `:3001`, and exposes
`/health/liveness`, `/health/readiness`, and `/health/status`. NATS health is
reported separately. Capture readiness remains true until shutdown, including
during a broker outage; it does not test spill writability or token availability.
A ready response therefore does not prove ingestion works. Check the token
registry, fallback errors and disk capacity, then send an authenticated event and
verify its destination. See [health implementation](src/health.rs) and
[startup wiring](src/main.rs).

Capture uses separate named NATS connections for durable work, the raw archive,
and capture DLQ traffic. The session writer likewise separates its work
consumer and DLQ publisher connections. Capture spills work immediately when
the work client is pending, disconnected, or has 512 publish acknowledgements
in flight; this keeps a large HTTP burst out of the async-NATS client queue.
Set `NATS_WORK_MAX_IN_FLIGHT` to tune that cap. Connected work and DLQ publishes
wait up to 1.5 seconds for the JetStream acknowledgement; each non-critical
background archive publish waits 250 ms.

At most 4,096 archive operations may be in flight; overload drops only the
diagnostic copy and increments `capture_raw_archive_dropped_total`.
`RAW_ARCHIVE_ENABLED=false` completely disables its connection, background
publishes, replay, and spill for an isolation run. Override publish timeouts
with `NATS_WORK_PUBLISH_TIMEOUT_MS`, `NATS_ARCHIVE_PUBLISH_TIMEOUT_MS`, and
`NATS_DLQ_PUBLISH_TIMEOUT_MS`. `capture_publish_latency_seconds{stream,result}`
records every outcome; `capture_work_publish_saturated_total` records work sent
directly to the durable fallback because the explicit limit was full.
Fallback segments become replayable after 10 MiB or 60 seconds even when no
later request arrives. `FALLBACK_MAX_SEGMENT_BYTES` and
`FALLBACK_MAX_SEGMENT_AGE_SECS` tune those boundaries.

`nats-bootstrap` accepts `EVENTS_CONSUMERS_ENABLED=false` for a disposable
capture-only isolation run and `EVENTS_CONSUMER_MEMORY_STORAGE=true` for a
consumer-state storage A/B test. Do not disable consumers on a persistent
environment without a deliberate backlog and recovery plan.

Changing `WRITER_REPLICAS` is a coordinated topology migration, not an online
Kafka-style consumer-group rebalance. Stop and drain every writer, set the
StatefulSet replica count and both writer/bootstrap `WRITER_REPLICAS` values,
then run `nats-bootstrap` with `EVENTS_CONSUMER_REBALANCE_FROM` equal to the
observed topology (`legacy`, `r003`, `r004`, and so on). Bootstrap refuses a
different or missing guard, snapshots each shard's safe seed floor into the new
consumer metadata, removes the old non-overlapping WorkQueue filters, and
creates the new consumers. Start writers only after bootstrap succeeds.

The historical runtime, storage, sessionization, and acceptance design is in
[`../../docs/plans/2026-08-25-nats-event-pipeline.md`](../../docs/plans/2026-08-25-nats-event-pipeline.md).

Use the current source and [deployment runbook](../DEPLOYMENT.md) to check
implementation and operating requirements; the original plan is not proof that
every acceptance criterion has been deployed or verified.
