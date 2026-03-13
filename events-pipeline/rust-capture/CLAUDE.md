# Rust Capture - Development Guide

## Commands

- Build: `cargo build`
- Run capture API: `cargo run`
- Run consumer: `cargo run --bin consumer`
- Run replay worker: `cargo run --bin replay-worker`
- Run all tests: `cargo test`
- Run single test: `cargo test test_name`
- Run tests in a module: `cargo test module::tests`
- Check without building: `cargo check`
- Dev mode with PrintSink: `PRINT_SINK=true cargo run`

## Project structure

```
src/
  main.rs                    # Capture API server entry point
  lib.rs                     # Library crate exports
  api.rs                     # CaptureError enum, CaptureResponse types
  capture.rs                 # Event capture handler (POST /api/v1/event)
  router.rs                  # Axum router, State struct, middleware wiring
  health.rs                  # HealthRegistry, liveness/readiness/status endpoints
  metrics_recorder.rs        # Prometheus metrics, track_metrics and request_timeout middleware
  replay_worker.rs           # Binary: replays disk fallback files to Kafka
  auth/
    authorization.rs         # Token validation and extraction
    http_tokens.rs           # Token fetching from HTTP_TOKENS_URL
  consumers/
    simple_consumer.rs       # Production Kafka consumer with enrichment
    async_consumer.rs        # Alternative async consumer
  enrichment/
    handler.rs               # Event enrichment orchestration
    bot_resolver.rs          # Bot detection
    ua_resolver.rs           # User-agent parsing
    privacy_enrichment.rs    # Privacy mode (IP truncation)
  events/
    event.rs                 # Event struct, parsing from bytes/headers
    transform_event.rs       # TransformedEvent (post-enrichment)
    failed_event.rs          # FailedEvent for error topic
  sinks/
    mod.rs                   # EventSink trait, EventTypes enum
    kafka_event_sink.rs      # KafkaSink + KafkaContext (ClientContext stats callback)
    disk_sink.rs             # DiskSink with rotating JSONL segments
    fallback_sink.rs         # FallbackSink (Kafka primary, disk fallback)
    print_sink.rs            # PrintSink for development
  geo/                       # MaxMind GeoIP resolution
  ip2location/               # IP2Proxy detection
  utils/
    kafka_config.rs          # Kafka producer/consumer ClientConfig builders
    time.rs                  # TimeSource trait
```

## Key patterns

### Error handling
All capture-path errors use `CaptureError` enum (src/api.rs). Variants map to HTTP status codes:
- `RetryableSinkError` -> 503 (client should retry)
- `NonRetryableSinkError` -> 400 (don't retry)
- `TokenValidationError` -> 401
- `RequestDecodingError` -> 400
- `PayloadTooLarge` / `EventTooBig` -> 413

Never use `.unwrap()` or `panic!()` in the capture/sink path. Use proper error propagation.

### Sink architecture
`EventSink` trait (src/sinks/mod.rs) has two methods: `send()` and `send_batch()`.
In production, the sink chain is: `FallbackSink(KafkaSink, DiskSink)`.
FallbackSink catches `RetryableSinkError` from KafkaSink and writes to DiskSink.
Non-retryable errors propagate directly.

### KafkaContext and health detection
`KafkaContext` (src/sinks/kafka_event_sink.rs) implements `rdkafka::ClientContext`.
The `stats()` callback fires every 10s (`statistics.interval.ms=10000`) and checks broker connectivity.
It calls `HealthRegistry.set_kafka_healthy()` which is exposed via `/health/status`.

### State
The axum `State` struct (src/router.rs) holds:
- `sink`: `Arc<dyn EventSink + Send + Sync>`
- `timesource`: `Arc<dyn TimeSource + Send + Sync>`
- `http_tokens`: `Arc<Mutex<HttpTokens>>`
- `health`: `HealthRegistry`

### Middleware stack (applied to capture routes only)
1. `request_timeout` - innermost route_layer, wraps handler in tokio timeout
2. `track_metrics` - outermost route_layer, records request count/latency/status
3. `DefaultBodyLimit` - layer, enforces max body size
4. `TraceLayer` - layer, structured request tracing
5. `CorsLayer` - layer, CORS headers
6. Sentry layers - error reporting

Health routes are merged separately with no middleware.

### Kafka config
Producer and consumer configs are built in `src/utils/kafka_config.rs`.
Auth is configured via env vars: `KAFKA_AUTH`, `KAFKA_SECURITY_PROTOCOL`, `KAFKA_SASL`.
TLS certs are mounted from k8s secrets at `data/tls/producer/` and `data/tls/consumer/`.

## Testing conventions

- Tests live in `#[cfg(test)] mod tests` within each source file
- Use `PrintSink` or custom mock sinks (e.g., `FailingSink`) for sink testing
- Use `HealthRegistry::new()` when constructing State in tests
- 2 pre-existing tests (`test_download_and_save`, `test_ip2proxy_download_and_save`) require env vars and are expected to fail locally

## Environment variables

See README.md for the full list. Key ones for development:
- `PRINT_SINK=true` to skip Kafka
- `LOG_LEVEL=DEBUG` for verbose logging
- `APP_ENV=dev` to skip MaxMind/IP2Proxy downloads in consumer

## K8s deployment notes

- Capture API and replay-worker run as sidecar containers in the same pod
- They share a `fallback-volume` (emptyDir) at `/app/data/fallback`
- Both must use the same Docker image since both binaries are built from this crate
- Health probes point to `/health/liveness` and `/health/readiness` on port 3000
- Metrics are scraped from port 3001
