# Rust Capture (Events Pipeline)

Usermaven's event ingestion service built in Rust.
Handles event capture from frontend/backend SDKs, authentication, enrichment, and reliable delivery to Kafka.

## Architecture

```
Client SDKs
    |
    v
[Capture API] ---> [KafkaSink] ---> Kafka (primary path)
    :3000              |
                       v (on Kafka failure)
                  [FallbackSink] ---> [DiskSink] ---> data/fallback/pending/
                                                           |
                                                   [Replay Worker] ---> Kafka
```

### Binaries

| Binary | Entry point | Description |
|--------|------------|-------------|
| `events-pipeline` | `src/main.rs` | Capture API server (default) |
| `consumer` | `src/consumers/simple_consumer.rs` | Kafka consumer with enrichment (production) |
| `worker` | `src/consumers/async_consumer.rs` | Async Kafka consumer (alternative) |
| `replay-worker` | `src/replay_worker.rs` | Replays disk-buffered events back to Kafka |

### Sinks

- **KafkaSink** (`src/sinks/kafka_event_sink.rs`) -- Primary sink. Produces events to Kafka with partitioning by project ID. Includes `ClientContext` stats callback for broker health detection.
- **DiskSink** (`src/sinks/disk_sink.rs`) -- Writes events to rotating JSONL segment files under `data/fallback/pending/`. Atomic writes via `.tmp` rename. Rotates at 10MB or 60s.
- **FallbackSink** (`src/sinks/fallback_sink.rs`) -- Wraps KafkaSink + DiskSink. On retryable Kafka errors, transparently falls back to disk. Non-retryable errors propagate.
- **PrintSink** (`src/sinks/print_sink.rs`) -- Logs events to stdout. Used for local development.

### Health checks

| Endpoint | Purpose | Used by |
|----------|---------|---------|
| `GET /health/liveness` | Returns 200 if process is alive | k8s livenessProbe |
| `GET /health/readiness` | Returns 200 normally, 503 during graceful shutdown | k8s readinessProbe |
| `GET /health/status` | JSON with component health (Kafka broker status) | Debugging/dashboards |

On SIGTERM: marks pod not-ready, waits 5s for k8s endpoint updates, then shuts down gracefully.
Readiness stays 200 even when Kafka is down because FallbackSink buffers events to disk.

### Reliability features

- **Typed error handling** -- `CaptureError` enum with retryable vs non-retryable distinction. SDKs can decide whether to retry based on HTTP status code.
- **Kafka health detection** -- `rdkafka::ClientContext` stats callback (every 10s) detects broker connectivity and updates `HealthRegistry`. Exposed via `/health/status` and `capture_kafka_health` gauge.
- **Request timeout** -- Configurable middleware (default 60s) prevents stuck connections from exhausting file descriptors. Returns 504 on timeout.
- **Body size limit** -- Configurable max request body (default 2MB). Returns 413 on oversized payloads.
- **Kafka send timeout** -- Configurable timeout per Kafka produce (default 20s). No more `Timeout::Never` blocking forever.
- **Batch error handling** -- `send_batch` checks every result and aborts on first failure (no silent event loss).
- **Consumer manual offset commits** -- `enable.auto.commit=false`. Offsets committed only after successful downstream send (at-least-once semantics).
- **No panics in capture path** -- All `.unwrap()` replaced with proper error returns.

### Metrics

Prometheus metrics exposed on `:3001/metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Request count by method/path/status |
| `http_requests_duration_seconds` | Histogram | Request latency |
| `capture_errors_total` | Counter | Capture errors by error type |
| `capture_fallback_failovers_total` | Counter | Times FallbackSink fell back to disk |
| `capture_request_timeout_total` | Counter | Requests that hit the timeout |
| `capture_kafka_health` | Gauge | 1.0 = healthy, 0.0 = unhealthy |
| `capture_kafka_brokers_up` | Gauge | Number of brokers in UP state |
| `capture_kafka_callback_queue_depth` | Gauge | rdkafka internal reply queue depth |
| `capture_kafka_producer_queue_depth` | Gauge | Messages queued in producer |
| `replay_files_processed_total` | Counter | Fallback files replayed |
| `replay_events_total` | Counter | Events replayed from disk |

## Getting started

### Prerequisites

- Rust toolchain (`rustup`)
- Docker + Docker Compose (for Kafka)

### Environment variables

Copy `.env.example` to `.env` and adjust:

```bash
cp .env.example .env
```

Key variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PRINT_SINK` | `false` | Use PrintSink instead of Kafka |
| `NETWORK_ENRICHMENT_ENABLED` | `true` | Load GeoIP/proxy databases; disable for credential-free local development |
| `KAFKA_BROKERS` | `localhost:9092` | Kafka bootstrap servers |
| `KAFKA_TOPIC` | -- | Topic for captured events |
| `KAFKA_AUTH` | `false` | Enable Kafka authentication |
| `KAFKA_SECURITY_PROTOCOL` | `SASL_SSL` | `SASL_SSL`, `SSL`, or `PLAINTEXT` |
| `HTTP_TOKENS_URL` | -- | URL to fetch valid API tokens |
| `LOG_LEVEL` | `INFO` | Tracing log level |
| `SENTRY_DSN` | -- | Sentry DSN for error reporting |
| `MAX_BODY_SIZE` | `2097152` | Max request body in bytes (2MB) |
| `KAFKA_SEND_TIMEOUT_SECS` | `20` | Timeout per Kafka produce |
| `REQUEST_TIMEOUT_SECS` | `60` | HTTP request timeout |
| `FALLBACK_DIR` | `data/fallback` | Directory for disk fallback files |
| `REPLAY_BATCH_SIZE` | `100` | Events per replay batch |
| `REPLAY_POLL_INTERVAL_SECS` | `5` | Replay worker poll interval |
| `REPLAY_CLEANUP_HOURS` | `24` | Hours before cleaning completed files |

### Development with PrintSink

```bash
PRINT_SINK=true cargo run
```

### Development with Kafka

```bash
cd events-pipeline
docker-compose up -d
cd rust-capture
cargo run
```

### Running the consumer

```bash
cargo run --bin consumer
```

### Running tests

```bash
cargo test
```

### Sending test events

```bash
# See bin-request-examples/ for sample curl scripts
./bin-request-examples/send-event.sh
```

## API routes

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/event` | Capture single/batch events |
| `POST` | `/api/v1/events` | Capture single/batch events |
| `POST` | `/api/v1/s2s/event` | Server-to-server event capture |
| `POST` | `/api/v1/s2s/events` | Server-to-server event capture |
| `POST` | `/api.:ignored` | Randomized endpoint for ad-blocker bypass |
| `GET` | `/health/liveness` | Liveness probe |
| `GET` | `/health/readiness` | Readiness probe |
| `GET` | `/health/status` | Detailed health status |

## K8s deployment

The capture API runs as a Deployment with a `replay-worker` sidecar:

- **main** container: runs `events-pipeline` binary on port 3000
- **replay-worker** sidecar: runs `replay-worker` binary, shares `fallback-volume` with main
- **fallback-volume**: `emptyDir` (10Gi limit) mounted at `/app/data/fallback`
- Metrics server on port 3001

Both containers must use the same image tag since `replay-worker` is built from the same Cargo workspace.
