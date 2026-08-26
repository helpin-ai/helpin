# Event pipeline test runbook

Run all commands from the repository root. The test scripts create disposable
infrastructure; they never use the developer NATS or ClickHouse containers.

## What the test environment contains

`events-pipeline/scripts/e2e.sh` and
`events-pipeline/scripts/sustained-e2e.sh` create:

- three file-backed NATS 2.14.5 nodes;
- a disposable ClickHouse server with the repository migrations applied;
- the Rust capture API, session writer, and NATS bootstrap binaries;
- a one-day test CA and client/server certificates with mutual TLS 1.3;
- JetStream `chachapoly` encryption at rest; and
- resource, JetStream, application-metric, and ClickHouse result files.

The scripts require Docker with Compose, Rust/Cargo, Go, Python 3, and `curl`.
The sustained scripts run the pinned `grafana/k6:1.8.1` image, so a host k6
installation is not required. Reserve ports 3000, 3001, 3010, 3011, 14222,
18123, 18222-18224, and 19000.

### macOS OpenSSL requirement

The certificate helper requires OpenSSL 3 because it uses
`x509 -copy_extensions copy`. `/usr/bin/openssl` on macOS is LibreSSL and does
not support this option. Older versions of the helper hid OpenSSL stderr, which
made this failure look like a silent script exit before infrastructure startup.
The helper now reports the unsupported implementation explicitly.

Install and select Homebrew OpenSSL 3 before running either E2E script:

```bash
brew install openssl@3
export PATH="$(brew --prefix openssl@3)/bin:$PATH"
openssl version
```

The last command must report OpenSSL 3.x, not LibreSSL. On an Apple Silicon
Homebrew installation, the equivalent one-command prefix is:

```bash
PATH="/opt/homebrew/opt/openssl@3/bin:$PATH" \
just events-e2e
```

Use the same `PATH=...` prefix on the first line of a sustained-test command if
you do not export it in the shell.

## Fast functional E2E test

```bash
just events-e2e
```

This test checks inline enrichment, R3 work and DLQ streams, the R1 diagnostic
archive, WorkQueue acknowledgements, session windows, ClickHouse writes, and
session-cache reconstruction after a writer restart. It cleans up its
containers and volumes automatically.

## Complete single-host sustained test

Use the closed k6 model when the purpose is to exercise a fixed number of
simultaneous clients. This one command starts the complete stack and 5,000 k6
VUs, targets 5,000 single-event requests/s for ten minutes, drains the stream,
and validates the final ClickHouse and session results:

```bash
SUSTAINED_LOAD_DRIVER=k6-connections \
SUSTAINED_RATE=5000 \
SUSTAINED_CONNECTIONS=5000 \
SUSTAINED_DURATION_SECONDS=600 \
SUSTAINED_BATCH_SIZE=1 \
SUSTAINED_VISITORS=2000000 \
SUSTAINED_NETWORK_ENRICHMENT_ENABLED=true \
SUSTAINED_CAPTURE_ENV_FILE="$PWD/events-pipeline/rust-capture/.env" \
SUSTAINED_RESULTS_DIR=/tmp/helpin-e2e-5000 \
events-pipeline/scripts/sustained-e2e.sh
```

The connection profile is a closed model. If the target slows down, each VU
waits for its response and achieved throughput falls below the requested rate.
That makes it useful for connection pressure, but it is not an open-loop
capacity result.

For a credential-free local run, set
`SUSTAINED_NETWORK_ENRICHMENT_ENABLED=false` and omit
`SUSTAINED_CAPTURE_ENV_FILE`. For a smaller workstation, reduce
`SUSTAINED_RATE` and `SUSTAINED_CONNECTIONS` together.

Single-host results measure contention among the generator, capture, three
NATS nodes, ClickHouse, and the writer. They are useful for correctness and
regression testing, not as production capacity evidence. In particular, a
four-core host can become CPU-bound well below 5,000 events/s even when disk and
memory still have headroom.

## Split target and remote generator

Use this layout for an open-loop capacity test. It keeps k6 CPU, sockets, and
network state off the target host.

### 1. Start the target

On the target host:

```bash
SUSTAINED_LOAD_DRIVER=external \
SUSTAINED_RATE=5000 \
SUSTAINED_DURATION_SECONDS=600 \
SUSTAINED_VISITORS=2000000 \
SUSTAINED_SOURCE_LABEL=k6-capture \
SUSTAINED_NETWORK_ENRICHMENT_ENABLED=true \
SUSTAINED_CAPTURE_ENV_FILE="$PWD/events-pipeline/rust-capture/.env" \
SUSTAINED_RESULTS_DIR=/tmp/helpin-remote-target-5000 \
events-pipeline/scripts/sustained-e2e.sh
```

Wait for `External generator target is ready`. The target waits up to ten
minutes for the first `k6-capture` event, then starts its ten-minute observation
window. Port 3000 must be reachable from the generator, either directly or
through the target's reverse proxy.

### 2. Start the generator

On the generator host, using the target's direct endpoint:

```bash
TARGET_URL=http://TARGET_HOST:3000/api/v1/event \
TOKEN=e2e-server-secret \
PROFILE=arrival \
EVENT_RATE=5000 \
DURATION=10m \
BATCH_SIZE=1 \
VISITORS=2000000 \
PRE_ALLOCATED_VUS=2000 \
MAX_VUS=5000 \
RESULTS_DIR=/tmp/helpin-k6-remote-5000 \
events-pipeline/scripts/run-k6-remote.sh
```

To include Caddy and public TLS in the test, set `TARGET_URL` to the public
route instead, for example
`https://helpin-dev.tryunhide.com/api/v1/event`. The token above belongs only
to the disposable E2E fixture.

The arrival profile is the capacity test: it schedules 5,000 requests/s even
when responses slow down. A non-zero `dropped_iterations` count means the
generator reached `MAX_VUS`; it does not prove that the target rejected those
iterations. Request timeouts or `dial: i/o timeout` must be correlated with the
target metrics before assigning the bottleneck. Run the generator on a host
with sufficient CPU, memory, socket capacity, and outbound bandwidth.

Use `PROFILE=connections CONNECTIONS=5000` as a separate closed-model test of
5,000 simultaneous VUs. It does not guarantee 5,000 completed requests/s.
Connections are reused by default to match SDK keep-alive behavior. Set
`NO_VU_CONNECTION_REUSE=true` only to test connection churn deliberately.

## Network-enrichment data

Never commit the capture `.env` file or licensed databases. The environment
file may provide the MaxMind account/license and IP2Proxy downloader URL used
by the in-process startup and refresh path.

For a reproducible qualification run, use pinned database files rather than a
downloader hostname that only resolves inside Kubernetes:

```bash
SUSTAINED_NETWORK_ENRICHMENT_ENABLED=true \
SUSTAINED_REQUIRE_IP2PROXY=true \
SUSTAINED_IP2PROXY_DB_PATH=/absolute/path/to/pinned-ip2proxy.BIN \
SUSTAINED_CAPTURE_ENV_FILE=/absolute/path/to/test.env \
events-pipeline/scripts/sustained-e2e.sh
```

The IP2Proxy fixture is not checked in because its redistribution terms must be
verified by the licence owner. Record the MaxMind/IP2Proxy file checksums with
golden enrichment results. Without `SUSTAINED_REQUIRE_IP2PROXY=true`, a missing
database follows the production fail-open behavior and is visible in capture
logs and enrichment metrics.

## Results and acceptance

`SUSTAINED_RESULTS_DIR` is retained after a sustained run. Important files are:

| File | Contents |
| --- | --- |
| `summary.json` | Combined load, ClickHouse, session, process, and container summary |
| `k6-summary.json` | Local k6 configuration, accepted events, dropped iterations, latency, and thresholds |
| `process-resources.csv` | Capture/writer CPU and RSS plus host memory and swap |
| `docker-resources.csv` | NATS and ClickHouse CPU, memory, and process samples |
| `capture-metrics.prom` | Work/archive publish latency, failure, spill, and enrichment metrics |
| `writer-metrics.prom` | Writer batches, pending work, rejection, seed, and session metrics |
| `jetstream-*.json` | Per-node JetStream stream and consumer state |
| `clickhouse-final.tsv` | Logical FINAL row, event, visitor, session, and empty-session counts |
| `clickhouse-parts-health.tsv` | Active-part and partition health |

The script succeeds only when k6 succeeds, every accepted event reaches
ClickHouse, session IDs are present, each generated visitor has the expected
single session, and work/archive/poison spools have drained. Physical duplicates
are reported separately from logical `FINAL` rows. In split-host mode, the k6
summary lives in the generator's `RESULTS_DIR`; copy it alongside the target
results when preserving a qualification run.

During a run, distinguish the layers:

- `capture_work_publish_failures_total` and the work fallback indicate the
  availability-critical path is failing.
- Archive failures, archive spill, or `capture_raw_archive_dropped_total` affect
  only the diagnostic copy and must not delay valid capture responses.
- Rising work publish-ack latency with host CPU near 100% indicates target
  contention; inspect `iostat`, container CPU, and capture CPU together.
- k6 `dropped_iterations` with healthy target latency indicates generator VU
  exhaustion.
- Compare ClickHouse row growth with k6 `events_accepted`, rather than with
  attempted or scheduled iterations.

## Compression A/B test

The default work stream uses S2 compression. Repeat the same run with
`SUSTAINED_WORK_COMPRESSION=none` to test it disabled; the diagnostic archive
remains compressed. Compare work publish-ack latency, NATS CPU, disk bytes and
IOPS, fallback volume, and drain time before changing production configuration.

## Cleanup

Normal completion and most failures remove the disposable containers and
volumes. After an interrupted or killed shell, clean only this test project:

```bash
docker compose \
  --project-name helpin-event-sustained \
  --file events-pipeline/e2e/compose.yaml \
  down --volumes --remove-orphans
```

This command does not remove the developer `helpin-nats`, Postgres, Redis, or
other unrelated containers.
