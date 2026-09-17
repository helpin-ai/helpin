# PRD: Events Pipeline Reliability & Resilience

> Historical document: this describes the retired Kafka implementation. The
> current NATS JetStream pipeline contract is documented in
> [`../../docs/plans/2026-08-25-nats-event-pipeline.md`](../../docs/plans/2026-08-25-nats-event-pipeline.md).

## Context

Our Rust events-pipeline (capture API + consumer/worker) is functional but lacks production-hardening. The reliability review identified critical gaps: silent event loss in Kafka batch sends, no health checks (k8s can't route traffic away from broken pods), no fallback when Kafka is down, and several panic-on-error patterns that crash the entire process. Events are being permanently lost in production today.

This PRD covers two phases: **Phase 1** (quick wins that prevent data loss and crashes) and **Phase 2** (disk fallback sink with a replay worker for full durability).

---

## Phase 1: Critical Quick Wins

**Goal**: Stop losing events, stop crashing on transient failures, give k8s visibility into pod health.

### 1.1 Typed CaptureError Enum

**Problem**: All errors return `(StatusCode, String)` or `anyhow::Result`. No distinction between retryable (Kafka down) vs non-retryable (bad event) errors. Client SDKs can't decide whether to retry.

**Solution**: Create a `CaptureError` enum in `src/api.rs`:

| Variant | HTTP Status | When |
|---------|------------|------|
| `RequestDecodingError(String)` | 400 | Malformed JSON, bad base64, missing fields |
| `EmptyBatch` | 400 | Empty events array |
| `NoTokenError` | 401 | Missing API key |
| `TokenValidationError` | 401 | Invalid/unknown API key |
| `RetryableSinkError` | 503 | Kafka down, timeout — client should retry |
| `NonRetryableSinkError` | 400 | Event serialization failure |
| `EventTooBig(String)` | 413 | Single event exceeds size limit |
| `PayloadTooLarge` | 413 | Request body exceeds limit |

Each variant implements `IntoResponse` for proper HTTP status codes and `to_metric_tag()` for Prometheus labels. The `EventSink` trait changes from `Result<()>` to `Result<(), CaptureError>`.

**Files**:
- `src/api.rs` — define enum, implement `IntoResponse`
- `src/sinks/mod.rs` — update `EventSink` trait return type
- `src/sinks/kafka_event_sink.rs` — return typed errors
- `src/capture.rs` — use `CaptureError` instead of `(StatusCode, String)`

---

### 1.2 Fix Silent Event Loss in KafkaSink::send_batch

**Problem**: `send_batch` spawns Kafka sends into a `JoinSet` then ignores all results, always returning `Ok(())`. Events silently vanish when Kafka produces fail.

**Current code** (`kafka_event_sink.rs:122-139`):
```rust
while let Some(res) = set.join_next().await {
    tracing::debug!("Joining kafka threads: {:?}", res);
}
Ok(())  // Always returns Ok, even if sends failed!
```

**Solution**: Check each result, abort on first failure, return `RetryableSinkError`:
```rust
while let Some(res) = set.join_next().await {
    match res {
        Ok(Ok(_)) => {}
        Ok(Err(err)) => {
            set.abort_all();
            return Err(CaptureError::RetryableSinkError);
        }
        Err(err) => {
            set.abort_all();
            return Err(CaptureError::RetryableSinkError);
        }
    }
}
```

**Files**: `src/sinks/kafka_event_sink.rs`

---

### 1.3 Fix Kafka Timeout::Never

**Problem**: `Timeout::Never` means a Kafka send blocks forever if the broker is unreachable. Stalls the entire Tokio task indefinitely.

**Solution**: Change to `Timeout::After(Duration::from_secs(20))`. Make configurable via `KAFKA_SEND_TIMEOUT_SECS` env var.

**Files**: `src/sinks/kafka_event_sink.rs` (line 65)

---

### 1.4 Health Check Endpoints

**Problem**: No `/_readiness` or `/_liveness` routes. K8s sends traffic to broken pods. When Kafka goes down, we accept requests and silently drop events.

**Solution**: Build a `HealthRegistry` module:

```
HealthRegistry
  ├── register("kafka", deadline: 30s) → HealthHandle
  │     └── KafkaSink calls handle.report_healthy() via rdkafka stats callback
  ├── get_status() → { overall: healthy/unhealthy, components: {...} }
  │
  Routes:
  ├── /health/liveness  → 200 if all components healthy, 503 otherwise
  └── /health/readiness → 503 during shutdown (checks /tmp/shutdown for k8s prestop)
```

**ComponentStatus state machine**:
```
Starting → HealthyUntil(deadline) → Stalled (if deadline expires without report)
                                  → Unhealthy (explicit failure signal)
```

Wire Kafka health via `rdkafka::ClientContext::stats()` callback — when the stats callback fires and at least one broker is UP, report healthy. Enable `statistics.interval.ms = 10000` on the producer.

**Files**:
- `src/health.rs` — new: `HealthRegistry`, `HealthHandle`, `ComponentStatus`, `ShutdownStatus`
- `src/sinks/kafka_event_sink.rs` — implement `ClientContext`, accept `HealthHandle`
- `src/router.rs` — add health routes
- `src/main.rs` — create registry, pass to sink, set shutdown status on SIGTERM
- K8s deployments — update probe paths

---

### 1.5 Request Body Size Limit

**Problem**: No limit on request body size. A single malicious POST can OOM the pod.

**Solution**: Add `DefaultBodyLimit::max(2 * 1024 * 1024)` (2MB hard limit) as router layer.

**Files**: `src/router.rs`

---

### 1.6 Fix Token Refresh Panic

**Problem**: `http_tokens.rs` calls `panic!()` on fetch failure (lines 33, 50), crashing the entire process. Also, `fetch_tokens()` silently returns `Ok(vec![])` on HTTP errors, which would clear all tokens and cause all requests to 401.

**Solution**:
- Remove all `panic!()` calls — log error and keep stale tokens
- On startup: retry with backoff (3 attempts, 2s/5s/10s) before failing
- On refresh: log warn, preserve existing tokens, try again next cycle
- Replace `std::sync::Mutex` with `tokio::sync::RwLock` for async safety
- Fix `fetch_tokens()` to return `Err` on HTTP failures instead of `Ok(vec![])`

**Files**: `src/auth/http_tokens.rs`

---

### 1.7 Fix Remaining Panics in Capture Path

**Problem**: Several `.unwrap()` calls in the HTTP capture path that panic on malformed input:

| Location | Panic trigger |
|----------|--------------|
| `event.rs:145` | `serde_json::from_str(&payload).unwrap()` — non-JSON body |
| `event.rs:103` | `obj.get("api_key").unwrap()` — missing api_key field |
| `capture.rs:52,54` | `.unwrap()` on base64 decode — malformed form data |
| `authorization.rs:119` | `token.to_str().unwrap()` — non-UTF-8 header |

**Solution**: Replace all `.unwrap()` with proper error returns using `CaptureError::RequestDecodingError`.

**Files**: `src/events/event.rs`, `src/capture.rs`, `src/auth/authorization.rs`

---

### 1.8 Remove Hardcoded Sentry DSN

**Problem**: Two different Sentry DSNs hardcoded in `main.rs` (lines 39 and 98). Sentry is initialized twice — second init replaces first. Credentials in source code.

**Solution**: Read `SENTRY_DSN` from env var (via Config struct). Single init in `main()`. Remove duplicate.

**Files**: `src/main.rs`

---

### 1.9 Remove /tokens Debug Endpoint

**Problem**: `GET /tokens` returns hardcoded test credentials and is publicly accessible with no auth.

**Solution**: Remove the route entirely, or gate behind `APP_ENV=dev`.

**Files**: `src/router.rs`

---

### 1.10 Typed Config Struct

**Problem**: Env vars read via scattered `env::var()` calls with inconsistent defaults and error handling.

**Solution**: Create `src/config.rs` with a typed struct using `envconfig`:

```rust
#[derive(Envconfig, Clone)]
pub struct Config {
    #[envconfig(default = "0.0.0.0:3000")]
    pub address: SocketAddr,
    pub kafka_brokers: String,
    pub kafka_topic: String,
    #[envconfig(default = "20000")]
    pub kafka_send_timeout_ms: u64,
    #[envconfig(default = "false")]
    pub print_sink: bool,
    #[envconfig(default = "fallback-events")]
    pub disk_sink_path: String,
    pub http_tokens_url: Option<String>,
    pub sentry_dsn: Option<String>,
    #[envconfig(default = "info")]
    pub log_level: String,
    #[envconfig(default = "2097152")]
    pub max_body_size: usize,
}
```

**Files**: `src/config.rs` (new), `Cargo.toml`, `src/main.rs`, `src/consumers/*.rs`

---

## Phase 2: Disk Fallback Sink + Replay Worker

**Goal**: When Kafka is unavailable, buffer events to local disk. A separate worker process replays them back to Kafka when it recovers. Zero event loss.

### 2.1 FallbackSink

**Problem**: When Kafka is down, events are lost. The `DiskSink` exists in code but is commented out and never wired in.

**Solution**: Create `src/sinks/fallback_sink.rs` that wraps a primary sink (Kafka) and fallback sink (Disk):

```
HTTP Request → CaptureHandler → FallbackSink
                                   │
                                   ├─ primary_healthy? ──→ KafkaSink ──→ Kafka
                                   │    (AtomicBool)         │
                                   │                         ├─ Ok → done
                                   │                         └─ RetryableSinkError → DiskSink
                                   │
                                   └─ primary_unhealthy? ──→ DiskSink ──→ Local Disk
```

**Key design**:
- `FallbackSink` holds `Arc<KafkaSink>` + `Arc<DiskSink>` + `Arc<AtomicBool>` for primary health
- Background task polls `HealthRegistry` every 10s. If Kafka component is unhealthy, sets `AtomicBool` to false — skips Kafka entirely (avoids wasted 20s timeout per event)
- On `RetryableSinkError` from Kafka (even when "healthy"), immediately falls over to disk
- On non-retryable errors (bad event), propagates error directly — don't write garbage to disk
- `Drop` impl sends shutdown signal to background task via `oneshot`
- Metrics: `capture_fallback_sink_failovers_total` counter, `capture_primary_sink_health` gauge

**Files**:
- `src/sinks/fallback_sink.rs` — new file
- `src/sinks/mod.rs` — export FallbackSink
- `src/main.rs` — wire: `FallbackSink::new(KafkaSink, DiskSink, health_registry, "kafka")`

---

### 2.2 Improved DiskSink with Segment Rotation

**Problem**: Current `DiskSink` appends JSON lines to a single file. Not suitable for production replay — no rotation, no atomic writes, no way to mark events as replayed.

**Solution**: Redesign `DiskSink` to write to rotating segment files:

**File naming**: `{unix_timestamp}_{uuid}.jsonl`

**Rotation policy**: New segment when current file reaches 10MB or is 60 seconds old.

**Atomic writes**: Write to `.tmp` suffix, then `rename()` to final name on rotation.

**Directory structure**:
```
data/fallback/
├── pending/        ← DiskSink writes here
│   ├── 1709400000_abc123.jsonl
│   └── 1709400060_def456.jsonl
├── processing/     ← ReplayWorker moves files here while replaying
└── completed/      ← Successfully replayed (cleaned up after 24h)
```

**Files**: `src/sinks/disk_sink.rs` — rewrite with segment rotation

---

### 2.3 Replay Worker Binary

**Problem**: Events written to disk need to get back into Kafka when it recovers. No mechanism exists.

**Solution**: New binary `src/replay_worker.rs`:

```
ReplayWorker (separate process)
  │
  ├─ Scan data/fallback/pending/ for .jsonl files
  ├─ Wait for file to be "stable" (no writes for 5s, not .tmp)
  ├─ Move file → processing/
  ├─ Read line by line, send to Kafka in batches of 100
  │   ├─ Kafka healthy → send batch, continue
  │   └─ Kafka unhealthy → sleep 10s, retry
  ├─ All events sent → move to completed/
  └─ Cleanup: delete completed/ files older than 24h
```

**Key design**:
- **Separate binary** — different resource profile, independent lifecycle, can be scaled independently
- Reads from `pending/`, moves to `processing/` atomically (prevents double-replay if multiple workers)
- Uses its own `KafkaSink` with same topic/config
- Backpressure: if Kafka is still down, just waits — files accumulate in `pending/`
- Configurable poll interval (default 5s) and batch size (default 100)
- Metrics: `replay_events_total`, `replay_files_processed`, `replay_lag_seconds` (age of oldest pending file)

**Files**:
- `src/replay_worker.rs` — new binary
- `Cargo.toml` — add `[[bin]] name = "replay-worker" path = "src/replay_worker.rs"`

---

### 2.4 K8s Deployment for Replay Worker

The replay worker must share the same volume as the capture pod. **Sidecar approach** (recommended):

```yaml
containers:
  - name: main
    image: ghcr.io/usermaven/events-pipeline:latest
    command: ["./target/release/events-pipeline"]
    volumeMounts:
      - name: fallback-volume
        mountPath: /app/data/fallback
  - name: replay-worker
    image: ghcr.io/usermaven/events-pipeline:latest
    command: ["./target/release/replay-worker"]
    resources:
      requests:
        cpu: "100m"
        memory: 256Mi
      limits:
        cpu: "500m"
        memory: 512Mi
    volumeMounts:
      - name: fallback-volume
        mountPath: /app/data/fallback
volumes:
  - name: fallback-volume
    emptyDir:
      sizeLimit: 10Gi
```

Same pattern applies to `eventpipeline-worker.yaml` (the consumer pods).

**Files**: K8s deployment manifests for web and worker

---

### 2.5 Consumer Offset Commit Fix

**Problem**: `enable.auto.commit = true` means consumer offsets advance on a timer regardless of whether downstream Kafka produce succeeded. Combined with the `send_batch` silent error swallowing, this causes permanent event loss in the consumer/worker path.

**Solution**:
- Set `enable.auto.commit = false` in consumer Kafka config
- After successful `send_batch`, manually commit: `consumer.commit_message(&msg, CommitMode::Async)`
- On `send_batch` failure: don't commit — message will be re-delivered on next poll
- This gives **at-least-once** delivery semantics (safe for our idempotent ClickHouse inserts)

**Files**:
- `src/utils/kafka_config.rs` — change `enable.auto.commit` to `false`
- `src/consumers/simple_consumer.rs` — add manual commit after successful processing
- `src/consumers/async_consumer.rs` — same

---

## Phase 3: Future Improvements (Not in scope)

Tracked for future work:

| # | Improvement | Description |
|---|-------------|-------------|
| 1 | **Overflow routing** | Per-token rate limiter. When exceeded, route to overflow Kafka topic |
| 2 | **Kafka producer trait** | Abstract `KafkaProducer` trait + `MockKafkaProducer` for testing |
| 3 | **OpenTelemetry tracing** | Distributed tracing with configurable sampling rate |
| 4 | **Gzip bomb protection** | Validate decompressed size during streaming decompression |
| 5 | **Quota/billing enforcement** | Per-account event quotas via Redis |
| 6 | **Graceful shutdown v2** | Custom hyper accept loop with TCP backlog draining |
| 7 | **Event restriction service** | Dynamic per-token controls via Redis (force overflow, drop, DLQ) |
| 8 | **Kafka metrics dashboard** | Per-broker RTT, queue depth, batch sizes → Grafana |
| 9 | **Dynamic partition discovery** | Query Kafka for partition count instead of hardcoded 18 |
| 10 | **Configurable Kafka producer** | Expose linger.ms, batch.size, compression, acks via env vars |

---

## Implementation Timeline

```
Phase 1 (1-2 weeks):
  Week 1:
    1.1  CaptureError enum
    1.2  Fix send_batch error handling
    1.3  Fix Kafka timeout
    1.5  Body size limit
    1.6  Fix token refresh panic
    1.7  Fix capture path panics
    1.8  Sentry DSN from env
    1.9  Remove /tokens endpoint
  Week 2:
    1.4  Health check endpoints + Kafka ClientContext
    1.10 Config struct

Phase 2 (2-3 weeks):
  Week 3:
    2.1  FallbackSink
    2.2  Improved DiskSink with rotation
  Week 4:
    2.3  Replay Worker binary
    2.5  Consumer offset commit fix
  Week 5:
    2.4  K8s sidecar deployment
    Testing & staging validation
```

## Verification

**Phase 1**:
- `cargo test` — all existing + new tests pass
- Deploy to staging, verify `/health/liveness` returns 200
- Kill Kafka broker on staging, verify `/health/readiness` returns 503 within 30s
- Send event with 3MB body, verify 413 response
- Send event with invalid token, verify 401 (not panic/crash)
- Check Prometheus `/metrics` for new `capture_*` metrics

**Phase 2**:
- Deploy to staging with disk fallback enabled
- Kill Kafka broker, send events — verify they write to `data/fallback/pending/`
- Restore Kafka — verify replay-worker picks up files and replays to Kafka
- Check `data/fallback/completed/` has the replayed files
- Verify events appear in the downstream Kafka topic
- Run for 24h, verify completed files are cleaned up
