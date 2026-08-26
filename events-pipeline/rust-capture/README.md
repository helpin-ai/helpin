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
All are file-backed, S2-compressed, and bounded to a 48-hour maximum age.

## Local development

```bash
cp .env.example .env
PRINT_SINK=true cargo run
cargo test --lib pipeline::tests
cargo test --lib writer::tests
```

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
combines one pending batch per owned shard into synchronous inserts, and sends
AckProgress until ClickHouse and any terminal DLQ publication have completed.
Production mounts the private NATS CA and role-specific client certificate
through the `NATS_*_FILE` variables shown in `.env.example`.

The API listens on `:3000`, metrics on `:3001`, and exposes
`/health/liveness`, `/health/readiness`, and `/health/status`. NATS health is
reported separately while readiness remains true during a broker outage when
the durable spill is writable.

Capture spills work immediately when the async-NATS client is pending or
disconnected. Connected work and DLQ publishes wait up to 1.5 seconds for the
JetStream acknowledgement; each non-critical background archive publish waits
250 ms. At most 4,096 archive operations may be in flight; overload drops only
the diagnostic copy and increments `capture_raw_archive_dropped_total`. Override
these with `NATS_WORK_PUBLISH_TIMEOUT_MS`,
`NATS_ARCHIVE_PUBLISH_TIMEOUT_MS`, and `NATS_DLQ_PUBLISH_TIMEOUT_MS`.
`capture_publish_latency_seconds{stream,result}` records every outcome.

The complete runtime, storage, sessionization, and acceptance contract is in
[`../../docs/plans/2026-08-25-nats-event-pipeline.md`](../../docs/plans/2026-08-25-nats-event-pipeline.md).
