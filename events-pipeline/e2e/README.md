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

Resource sampling is also platform-aware: Linux reads `/proc/meminfo`, while
macOS uses `vm_stat` and `sysctl vm.swapusage`. The macOS
`mem_available_kib` value is an approximation based on free, inactive, and
speculative VM pages; use Activity Monitor for additional host-level diagnosis.

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

The current fully monitored single-host baseline is 15,000 events/s for ten
minutes with batches of ten, two captures, and two writers. It achieved exact
9,000,050-event ClickHouse parity with no drops or HTTP failures. See
[`../CAPACITY_BASELINE.md`](../CAPACITY_BASELINE.md) for the reproducible
command, latency, per-layer resources, storage calculation, and evidence
boundary. Do not compare that result directly with the single-event closed
connection profile above: batching and the open arrival model test different
boundaries.

The initial production qualification is 300 events/s for ten minutes with the
same application topology, file-backed R3 work, file-backed consumer state, S2,
and raw archive enabled. It achieved exact 180,010-event parity, a one-second
drain, and p99 8.58 ms. The capacity baseline records its complete resource and
disk results.

## Realistic enrichment profile

Set `SUSTAINED_K6_DATA_PROFILE=realistic` on a `k6-arrival` or
`k6-connections` run to replace the minimal payload with a deterministic mixed
workload. Each HTTP batch represents one browser: its events share a visitor,
IP, and user agent, while successive requests rotate through 65,536 IPs and a
weighted 9,999-UA fixture. The payload varies event types, URLs, referrers,
privacy policies, UTM attribution, Segment/GA/Facebook IDs, click IDs, user and
company properties, event attributes, and autocapture attributes.

The profile keeps the simple workload as the historical throughput baseline.
It intentionally increases capture CPU, payload bytes, JetStream bytes,
ClickHouse row width, and object-storage traffic. Override its cardinalities
with `SUSTAINED_K6_REALISTIC_USER_AGENTS` and
`SUSTAINED_K6_REALISTIC_IPS`. It also reserves traffic for generic crawlers and
all configured AI crawler/fetcher families. `clickhouse-realistic-coverage.tsv`
and the `clickhouse.realistic_coverage` summary object prove that IP, UA, geo,
UTM, ID, company, autocapture, and AI bot classification fields reached
ClickHouse.

A 60-second direct-R2 qualification at 15,000 events/s with two captures and
three writers stored all 900,020 accepted events with no drops, HTTP failures,
redeliveries, duplicates, or session errors. It covered 69,816 stored IPs,
8,654 UAs, and 897,130 geo-enriched rows; latency was p50 3.65 ms, p95 85.76
ms, and p99 180.90 ms, with a 13-second writer drain. Capture preloads the
shipped UA fixture into the UA and bot caches at startup; compared with the
same cold-cache run, that reduced p99 from 274.80 ms and peak RSS from roughly
2.41 GiB to 2.02 GiB per capture. MaxMind was available during this run, but
IP2Proxy was unavailable and followed the fail-open path, so a pinned
IP2Proxy-enabled run is still required for complete network-enrichment
qualification.

## Cloudflare R2-backed ClickHouse test

ClickHouse authenticates to R2 through its S3-compatible API. Create an R2
**Object Read & Write** token scoped only to the `helpin-clickhouse` bucket and
use the generated **Access Key ID** and **Secret Access Key**. A general
Cloudflare API bearer token is not accepted by the ClickHouse S3 disk.

Keep credentials in the ignored local file:

```bash
cp events-pipeline/e2e/.env.r2.example events-pipeline/e2e/.env.r2
chmod 600 events-pipeline/e2e/.env.r2
```

Populate `CLICKHOUSE_R2_ACCESS_KEY_ID` and
`CLICKHOUSE_R2_SECRET_ACCESS_KEY`; leave the checked-in EU endpoint unchanged.
Then add `SUSTAINED_CLICKHOUSE_STORAGE=r2` to a sustained command. The harness
adds a unique `helpin-sustained/<run-id>/` object prefix, loads an
environment-backed ClickHouse storage configuration, and verifies that
`helpin.events` uses the R2-backed `events` policy while
`helpin.session_seed_events` remains on local storage for fast recovery.
The R2 profile gives ClickHouse 4 CPUs and 6 GiB, enables a 4 GiB local
write-through cache, and forces compact event parts to avoid one
remote object per column during inserts and immediate merges. Override these
diagnostic defaults with `CLICKHOUSE_R2_CPUS`, `CLICKHOUSE_R2_MEMORY_LIMIT`, or
`SUSTAINED_CLICKHOUSE_R2_CACHE_MAX_SIZE` when comparing another resource shape.

The unique R2 prefix is recorded in `r2-prefix.txt`. By default the cleanup trap
drops the test database synchronously before stopping ClickHouse. If cleanup
cannot reach ClickHouse, delete that recorded prefix manually. Set
`SUSTAINED_CLICKHOUSE_R2_KEEP_DATA=true` only when the objects must remain for
inspection.

The 15,000-event/s direct-R2 qualification uses three writer replicas. A
manually shortened 3m46s slice accepted and stored 3,388,020 events with exact
row and event-ID parity, no dropped iterations or redeliveries, and no consumer
pending backlog. HTTP latency was p50 2.84 ms, p95 79.22 ms, and p99 188.16 ms.
Two writers were slightly below this diagnostic rate, but remain ample for the
initial 300-event/s production allocation. The writer path therefore batches
up to 16,384 rows or 48 MiB and waits at most two seconds; three replicas are a
capacity-test setting, not the launch default.

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
| `jetstream-disk.csv` | Per-node total, work, raw, and DLQ physical filesystem samples |
| `clickhouse-final.tsv` | Logical FINAL row, event, visitor, session, and empty-session counts |
| `clickhouse-parts-health.tsv` | Active-part and partition health |
| `clickhouse-realistic-coverage.tsv` | Distinct IP/UA/path and populated enrichment/AI-bot field counts for the realistic profile |

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

The default work stream uses S2 compression. Repeat the same capture-only,
file-backed run with `SUSTAINED_WORK_COMPRESSION=none` to test it disabled; the
diagnostic archive remains compressed. `summary.json` reports logical bytes,
per-node physical bytes, and the physical/logical ratio when consumers are
disabled. Compare those values plus publish-ack latency, NATS CPU, disk IOPS,
fallback volume, and drain time before changing production configuration.

Capture-only mode always records a final physical-disk sample. Recurring `du`
sampling is disabled by default because it is an intrusive disk observer; set
`SUSTAINED_JETSTREAM_DISK_MONITOR_ENABLED=true` only when disk-growth samples
are part of the experiment.

## NATS contention isolation sequence

The capture process always uses separate work, archive, and DLQ connections.
`NATS_WORK_MAX_IN_FLIGHT` defaults to 512; the sustained harness exposes it as
`SUSTAINED_WORK_MAX_IN_FLIGHT` and records it in `test.env`.

For a 10,000 events/s qualification, keep the generator remote and repeat the
same target run first with the diagnostic archive disabled, then enabled. Use a
fresh results directory for each run:

```bash
SUSTAINED_LOAD_DRIVER=external \
SUSTAINED_RATE=10000 \
SUSTAINED_DURATION_SECONDS=600 \
SUSTAINED_VISITORS=4000000 \
SUSTAINED_RAW_ARCHIVE_ENABLED=false \
SUSTAINED_WRITER_REPLICAS=3 \
SUSTAINED_WORK_MAX_IN_FLIGHT=512 \
SUSTAINED_RESULTS_DIR=/tmp/helpin-10k-no-archive \
events-pipeline/scripts/sustained-e2e.sh
```

Repeat with `SUSTAINED_RAW_ARCHIVE_ENABLED=true` and a different result path.
Drive each target from the remote-generator command above with
`EVENT_RATE=10000`; keep every other input identical.

If work acknowledgements still stall, isolate stream replication from the
replicated consumer state machines. This mode provisions no consumers, does
not start the writer, and succeeds when every accepted event is present in the
unconsumed work stream:

```bash
SUSTAINED_LOAD_DRIVER=external \
SUSTAINED_RATE=10000 \
SUSTAINED_DURATION_SECONDS=600 \
SUSTAINED_VISITORS=4000000 \
SUSTAINED_RAW_ARCHIVE_ENABLED=false \
SUSTAINED_CONSUMERS_ENABLED=false \
SUSTAINED_RESULTS_DIR=/tmp/helpin-10k-no-consumers \
events-pipeline/scripts/sustained-e2e.sh
```

Finally, compare the normal per-writer durable consumer with memory-backed consumer
state by setting `SUSTAINED_CONSUMER_MEMORY_STORAGE=true`. Memory-backed state
remains replicated but removes consumer-state disk I/O; it is an isolation
setting, not a production recommendation without recovery testing.

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
