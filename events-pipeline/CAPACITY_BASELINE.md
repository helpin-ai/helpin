# Event pipeline capacity baseline

This document is the sizing baseline for the Helpin event pipeline. It records
the verified load result, the deployment topology derived from it, and the
limits of that evidence. Re-run the qualification before raising the target or
changing JetStream storage, replication, ClickHouse, enrichment, or batching.

## Verified baseline

On 2026-08-26, the complete pipeline accepted and wrote every event in a
ten-minute, 15,000 events/second run:

| Measure | Result |
| --- | ---: |
| Scheduled and accepted events | 9,000,050 / 9,000,050 |
| ClickHouse physical, logical, and unique rows | 9,000,050 |
| Effective event rate | 14,999.767 events/s |
| HTTP request rate (10 events/request) | 1,499.977 requests/s |
| Dropped k6 iterations / HTTP failures | 0 / 0 |
| Response latency p50 / p95 / p99 / max | 2.48 / 24.00 / 43.55 / 293.52 ms |
| Final drain after load | 3 seconds |
| JetStream redeliveries | 0 |
| Fallback or spill data | 0 |

The topology was two capture processes, two shard-partitioned writers, a
three-node R3 JetStream cluster, and one ClickHouse server. Capture used 512
in-flight work publishes per process. Writers used the 8,192-row/16 MiB insert
boundary and one-second flush interval. k6 used an open arrival model with
2,048 preallocated and 4,096 maximum VUs; only 203 VUs were active at peak.

The full monitored-stack resource totals were 5.50 CPU cores and 4.31 GiB on
average, with synchronized peaks of 6.91 cores and 5.13 GiB. CPU values below
use 1.00 as one logical core. Memory is resident/container memory and includes
cold-start peaks where applicable.

| Layer | Replicas | CPU avg | CPU peak | Memory avg | Memory peak |
| --- | ---: | ---: | ---: | ---: | ---: |
| k6 generator | 1 | 0.48 | 0.69 | 0.84 GiB | 0.96 GiB |
| capture | 2 | 1.13 | 1.38 | 0.34 GiB | 1.05 GiB |
| replay sidecar | 2 | 0.005 | 0.018 | 0.06 GiB | 0.48 GiB |
| session writer | 2 | 0.91 | 1.26 | 0.69 GiB | 0.85 GiB |
| NATS JetStream | 3 | 1.77 | 1.98 | 1.13 GiB | 1.34 GiB |
| ClickHouse | 1 | 1.22 | 1.94 | 1.28 GiB | 1.91 GiB |
| **Whole monitored stack** | 11 processes/containers | **5.50** | **6.91** | **4.31 GiB** | **5.13 GiB** |

Per replica, capture averaged 0.56-0.57 cores and peaked at 0.67-0.71. Writers
averaged 0.43-0.48 cores and peaked at 0.64-0.65. NATS nodes averaged
0.55-0.62 cores and peaked at 0.62-0.70.

## Qualification configuration

Raise the shell open-file limit before the run. The OpenSSL prefix shown is for
Apple Silicon Homebrew installations.

```bash
ulimit -n 65536

PATH="/opt/homebrew/opt/openssl@3/bin:$PATH" \
SUSTAINED_LOAD_DRIVER=k6-arrival \
SUSTAINED_RATE=15000 \
SUSTAINED_DURATION_SECONDS=600 \
SUSTAINED_BATCH_SIZE=10 \
SUSTAINED_VISITORS=2000000 \
SUSTAINED_NETWORK_ENRICHMENT_ENABLED=true \
SUSTAINED_CAPTURE_ENV_FILE="$PWD/events-pipeline/rust-capture/.env" \
SUSTAINED_K6_PRE_ALLOCATED_VUS=2048 \
SUSTAINED_K6_MAX_VUS=4096 \
SUSTAINED_WORK_MAX_IN_FLIGHT=512 \
SUSTAINED_RAW_ARCHIVE_ENABLED=false \
SUSTAINED_CONSUMERS_ENABLED=true \
SUSTAINED_CAPTURE_REPLICAS=2 \
SUSTAINED_WRITER_REPLICAS=2 \
SUSTAINED_WORK_STORAGE=memory \
SUSTAINED_CONSUMER_MEMORY_STORAGE=true \
SUSTAINED_RESOURCE_MONITOR_ENABLED=true \
SUSTAINED_DOCKER_RESOURCE_MONITOR_ENABLED=true \
SUSTAINED_RESOURCE_SAMPLE_INTERVAL_SECONDS=15 \
SUSTAINED_RESULTS_DIR=/tmp/helpin-e2e-15000-baseline \
events-pipeline/scripts/sustained-e2e.sh
```

## Deployment sizing

Stage and production use the verified two-capture/two-writer application
topology for availability and shard ownership. The initial production target is
no more than 1,000 events/s, so production is intentionally smaller than the
15k/s qualification environment:

| Production container | Replicas | CPU request / limit | Memory request / limit |
| --- | ---: | ---: | ---: |
| capture | 2 | 0.5 / 1 core | 512 MiB / 1 GiB |
| replay sidecar | 2 | 0.2 / 0.5 cores | 256 / 512 MiB |
| session writer | 2 | 0.5 / 1 core | 512 MiB / 1 GiB |
| NATS | 3 | 1 / 2 cores | 2 / 4 GiB |

Staging preserves topology and correctness coverage at considerably lower
cost. It is not continuously provisioned as a 15k/s performance environment:

| Staging container | Replicas | CPU request / limit | Memory request / limit |
| --- | ---: | ---: | ---: |
| capture | 2 | 0.5 / 1 core | 512 MiB / 1 GiB |
| replay sidecar | 2 | 0.2 / 0.5 cores | 256 / 512 MiB |
| session writer | 2 | 0.5 / 1 core | 512 MiB / 1 GiB |
| NATS | 3 | 0.5 / 2 cores | 1 / 3 GiB |

Temporarily apply the measured qualification envelope (2 CPU/2 GiB per capture
and writer, 4 CPU/8 GiB per NATS node) and a provisioned-IOPS storage class
before using stage for a 15k/s qualification, then restore the stage envelope
after recording the run.

NATS retains more memory than its observed 15k/s process RSS because the
deployed streams are file-backed, encrypted, and R3, and operating-system page
cache and storage latency affect their behavior. NATS pods must remain on
separate nodes and use provisioned-IOPS NVMe-class volumes. ClickHouse is not
owned by these manifests; for the initial 1k/s target, allocate 1/2 CPU
request/limit and 2/4 GiB memory request/limit, then apply the measured 15k
qualification envelope before raising the traffic target.

Changing `WRITER_REPLICAS` changes durable-consumer shard ownership. Stop all
writers, set `EVENTS_CONSUMER_REBALANCE_FROM` to the exact active topology, run
the bootstrap job, and then start the new StatefulSet. The current manifest
performs the guarded `r003` to `r002` transition.

For the first rollout of these manifests, pause automatic GitOps reconciliation,
scale the writer StatefulSet to zero, wait for its pods to terminate, run the
bootstrap job with `EVENTS_CONSUMER_REBALANCE_FROM=r003`, apply the two-replica
StatefulSet, and then resume reconciliation. If the live consumer topology is
not exactly `r003`, stop and use the topology reported by the bootstrap guard;
do not bypass it while writers are running.

## Forty-eight-hour JetStream capacity

The work stream has a 48-hour maximum age. At the initial production ceiling:

```text
1,000 events/s * 172,800 s = 172,800,000 events
```

The monitored work-stream backlog measured 2,212.5 bytes/event on average and
2,213.8 bytes/event at peak. Using the peak footprint:

| Capacity component | Size |
| --- | ---: |
| 48-hour logical enriched work payload at 1k/s | 382.5 GB (356.3 GiB) |
| Logical work allocation with 25% headroom | 478.2 GB (445.3 GiB) |
| R3 physical payload before headroom | 1.148 TB (1.044 TiB) |
| R3 physical allocation with 25% headroom | 1.435 TB (1.305 TiB) |

Production uses a 512 GiB logical work-stream cap, a 900 GB JetStream file cap,
and a 1 TiB PVC on each NATS node. The work cap is 549.8 GB, covering the
478.2 GB requirement above. The configured work, bounded 256 GiB R1 raw
archive, and 32 GiB R3 DLQ budgets total 800 GiB / 859.0 GB. That leaves 41 GB
inside the JetStream cap and about 200 GB between the cap and each 1 TiB
filesystem. Alert before any node reaches 70% of its filesystem or JetStream
file-store allocation; capacity must be expanded before 80%.

At 15k/s, the same calculation is 5.738 TB logical, 7.173 TB logical with 25%
headroom, and 21.518 TB physical across R3. Before raising production to that
rate, expand every NATS PVC to at least 8 TiB and raise `max_file_store` to 8 TB
and `EVENTS_WORK_MAX_BYTES` to 7.173 TB. StatefulSet volume-claim templates are
immutable on many clusters, so plan the retained-PVC expansion/recreation as an
explicit storage migration.

This is outage-buffer sizing. A healthy WorkQueue drains continuously: the
qualification run's peak backlog was only 28,754 messages / 63.7 MB. Stage
keeps its smaller 200 GiB-per-node qualification volume and does not promise a
48-hour 15k/s outage buffer.

The raw archive was disabled during the qualification, so its bytes/event were
not measured. Its 512 GiB production limit is a bounded diagnostic budget, not
evidence of 48-hour raw retention. Measure representative production payloads
with the archive enabled before increasing that budget. The capture fallback
and archive-spill volumes are pod-local `emptyDir` volumes; they survive a
container restart but not pod replacement or node loss.

## Evidence boundary and next qualification

This baseline proves the application topology and 15k/s end-to-end correctness
on one host. It does **not** prove the complete production durability path: the
work stream and consumer state were memory-backed, the raw archive was off,
and IP2Proxy was unavailable and followed the fail-open path.

Before treating 15k/s as the production durability SLO, repeat the run on
separate production-like nodes with file-backed R3 work storage, file-backed
consumer state, raw archive enabled, pinned MaxMind/IP2Proxy databases, and the
production storage class. Record disk latency, IOPS, throughput, and NATS
filesystem growth alongside the existing per-layer CPU and memory summary.
