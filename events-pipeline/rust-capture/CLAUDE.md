# Rust capture development guide

## Commands

- Build/check: `cargo check --all-targets`
- Run capture locally: `PRINT_SINK=true cargo run`
- Run spill replay: `cargo run --bin replay-worker`
- Reconcile JetStream: `cargo run --bin nats-bootstrap`
- Core tests: `cargo test --lib pipeline::tests` and `cargo test --lib writer::tests`

## Runtime rules

- Production capture uses inline stateless enrichment followed by NATS; do not
  reintroduce a separate enrichment consumer.
- `events.enriched.v1.<shard>` and its envelope are immutable. Breaking changes
  get new versioned subjects.
- Visitor sharding always hashes
  `lower(project_id) + ":" + user_anonymous_id` into 100 shards.
- Server-credential custom timestamps are rejected outside `now - 7d` through
  `now + 1h`; never clamp them because ClickHouse replacement depends on partition
  stability. Browser-credential events use receive time instead of custom timestamps;
  credential kind comes from the registry, not token punctuation.
- Missing MaxMind/IP2Proxy databases fail open with metrics and logs.
- A retryable NATS work or DLQ failure must reach the fsynced disk fallback
  before capture returns success. The deployment owns encryption of that
  persistent volume.
- The replay worker uses the same enrichment and NATS code as capture.

The complete pipeline contract is
[`../../docs/plans/2026-08-25-nats-event-pipeline.md`](../../docs/plans/2026-08-25-nats-event-pipeline.md).
