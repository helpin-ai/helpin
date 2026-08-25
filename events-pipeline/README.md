Usermaven's Event Pipeline
---

Usermaven event pipeline is built on top of Rust and Java.
Rust handles event ingestion, enrichment, and reliable delivery to Kafka.
Java Kafka Streams handles sessionization.

The ingestion and transformation code is under `rust-capture/` and sessionization under `kafka-streams/`.

## What the pipeline does

- Collect events from frontend and backend SDKs
- Authenticate incoming requests by API key and server secret
- Transform and enrich events:
  - Geo-location enrichment (MaxMind)
  - IP2Proxy detection
  - Session ID generation
  - Anonymous ID generation
  - Privacy mode compliance (first 3 octets only)
  - Bot detection
  - User-agent parsing
- Produce events to Kafka with disk fallback for reliability
- Replay disk-buffered events when Kafka recovers
- Sessionize events using Kafka Streams

## Components

| Component | Language | Path | Description |
|-----------|----------|------|-------------|
| Capture API | Rust | `rust-capture/` | Event ingestion HTTP server |
| Consumer | Rust | `rust-capture/src/consumers/` | Kafka consumer with enrichment |
| Replay Worker | Rust | `rust-capture/src/replay_worker.rs` | Replays disk fallback to Kafka |
| Sessionization | Java | `kafka-streams/` | Kafka Streams session windows |
| Retroactive | Python | `eventpipeline-retroactive/` | Retroactive event processing |
| Upsert | Python | `eventpipeline-upsert/` | ClickHouse upsert worker |

## Getting started

From the Helpin repository root, the complete local stack is available through:

```bash
cp events-pipeline/.env.events.example events-pipeline/.env.events
just events-up
just events-smoke
```

`events-up` runs Kafka, capture, enrichment, sessionization, replay, and ClickHouse. It loads the internal API secret from `server/.env`; no event credential is stored in Compose or committed files. The smoke test verifies the authenticated project ID at every pipeline stage.

ClickHouse schema changes are versioned under `server/internal/chmigrate/sql` and
applied by the Go migration runner during `events-up`. They can also be inspected
directly:

```bash
cd server
go run ./cmd/clickhouse-migrate status
go run ./cmd/clickhouse-migrate validate
```

Stage and production run the same command as an Argo CD `PreSync` job, before
event consumers are updated. The `helpin-secrets` secret must provide
`CLICKHOUSE_DSN` and the pipeline's `KAFKA_*` connection variables.

### Prerequisites

- Rust toolchain ([install](https://www.rust-lang.org/learn/get-started))
- JDK 20 ([download](https://www.oracle.com/java/technologies/downloads/))
- Maven ([download](https://maven.apache.org))
- Docker + Docker Compose

### Quick start

```bash
git clone https://github.com/usermaven/events-pipeline
cd events-pipeline/rust-capture
cp .env.example .env
cargo build
```

### Development with PrintSink (no Kafka needed)

```bash
cd rust-capture
PRINT_SINK=true cargo run
```

### Development with Kafka

```bash
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
# Sample curl scripts in bin-request-examples/
./bin-request-examples/send-event.sh
./bin-request-examples/send-http-event.sh
```

### Validating events in Kafka

```bash
# Inside the Kafka container
kafka-run-class kafka.tools.GetOffsetShell --broker-list localhost:9092 --topic incoming_events --time -1
```

## Reliability features

- Typed error handling (retryable vs non-retryable)
- Kafka health detection via rdkafka stats callback
- Disk fallback when Kafka is unavailable
- Replay worker to recover buffered events
- Manual consumer offset commits (at-least-once)
- Request timeout middleware
- Body size limits
- Health check endpoints for k8s probes
- Graceful shutdown with readiness drain

See `rust-capture/README.md` for detailed documentation.
