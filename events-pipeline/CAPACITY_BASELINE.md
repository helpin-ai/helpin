# Event pipeline capacity baseline

This document is the sizing baseline for the Helpin event pipeline. It records
the reported historical load result, the deployment topology derived from it, and the
limits of that evidence. Re-run the qualification before raising the target or
changing JetStream storage, replication, ClickHouse, enrichment, or batching.

## Recorded baseline (2026-08-26)

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

These are recorded results from the original run, not a fresh verification of
the current checkout. This review found no checked-in raw output establishing
these exact measurements; retain new run artifacts when repeating qualification.

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

The current [harness](scripts/sustained-e2e.sh) defaults to a 25 ms p99 latency
threshold (`SUSTAINED_K6_P99_LIMIT_MS`), below the recorded 43.55 ms p99 above.
The historical command therefore does not establish a passing result under
current thresholds. Choose and record the required latency target before running;
do not loosen it merely to reproduce the old throughput figure.

## Initial production qualification

The launch ceiling is 300 events/s. A separate ten-minute run exercised the
complete production semantics: two captures, two writers, encrypted file-backed
R3 work, file-backed consumer state, S2 compression, and the R1 raw archive.

| Measure | Result |
| --- | ---: |
| Scheduled and accepted events | 180,010 / 180,010 |
| ClickHouse physical, logical, and unique rows | 180,010 |
| Effective event / HTTP request rate | 300.012 / 30.001 per second |
| Dropped k6 iterations / HTTP failures | 0 / 0 |
| Response latency p50 / p95 / p99 / max | 4.47 / 7.14 / 8.58 / 33.36 ms |
| Final drain | 1 second |
| JetStream redeliveries | 0 |

Observed resources at 300 events/s were far below the 15k ceiling. CPU values
use 1.00 as one logical core; replica ranges are per process/container.

| Layer | Replicas | CPU avg per replica | CPU peak per replica | Memory avg per replica | Memory peak per replica |
| --- | ---: | ---: | ---: | ---: | ---: |
| k6 generator | 1 | 0.036 | 0.061 | 58 MiB | 79 MiB |
| capture | 2 | 0.024-0.027 | 0.049-0.055 | 441 MiB | 443 MiB |
| replay sidecar | 2 | 0.028 | 0.92-0.94 startup | 246 MiB | 247 MiB |
| session writer | 2 | 0.018-0.019 | 0.027-0.030 | 33-34 MiB | 46-48 MiB |
| NATS | 3 | 0.038-0.077 | 0.068-0.115 | 76-105 MiB | 98-132 MiB |
| ClickHouse | 1 | 0.145 | 0.298 | 976 MiB | 1,331 MiB |

The replay CPU peaks are one cold-start sample, not sustained replay load.

## File-store compression measurement

Two identical capture-only runs retained 600,010 events in an encrypted,
file-backed R3 work stream. With consumers disabled, drain behavior could not
hide stored bytes. The harness compared JetStream logical bytes with `du` from
all three NATS data directories.

| Work storage | Logical bytes/event | Replicated logical bytes | Physical bytes across R3 | Physical/logical |
| --- | ---: | ---: | ---: | ---: |
| no compression | 2,228.24 | 4.011 GB | 4.025 GB | 100.36% |
| S2 | 2,228.24 | 4.011 GB | 271.5 MB | 6.77% |

S2 reduced physical work storage by 93.25%, or 14.8x. The production run also
measured the raw archive at 1,205.15 logical bytes/event and 12.52% physical to
logical, or about 8.0x compression. These ratios are payload-dependent; alert
on both filesystem use and logical stream bytes and repeat this measurement
after material event-envelope changes.

## Deployment sizing

The resource tables below reflect the checked-in
[production capture/replay manifest](../k8s/prod/events-pipeline/deployments/eventpipeline-web.yaml)
and [staging manifest](../k8s/stage/events-pipeline/deployments/eventpipeline-web.yaml),
including the increased production replay memory allocation. They do not report
live cluster resources.

Stage and production preserve the two-capture/two-writer topology for
availability and shard ownership. Production is sized for the measured 300/s
launch ceiling, with burst headroom:

| Production container | Replicas | CPU request / limit | Memory request / limit |
| --- | ---: | ---: | ---: |
| capture | 2 | 0.25 / 1 core | 512 MiB / 1.5 GiB |
| replay sidecar | 2 | 0.2 / 0.5 cores | 768 MiB / 2 GiB |
| session writer | 2 | 0.25 / 1 core | 512 MiB / 1 GiB |
| NATS | 3 | 0.5 / 2 cores | 1 / 3 GiB |
| ClickHouse | 2 | 0.5 / 4 cores | 4 / 12 GiB |
| ClickHouse Keeper | 3 | 0.25 / 1 core | 512 MiB / 2 GiB |

Staging preserves topology at a lower cost and is not continuously provisioned
as a performance environment:

| Staging container | Replicas | CPU request / limit | Memory request / limit |
| --- | ---: | ---: | ---: |
| capture | 2 | 0.1 / 0.5 core | 256 / 768 MiB |
| replay sidecar | 2 | 0.1 / 0.25 cores | 256 / 512 MiB |
| session writer | 2 | 0.1 / 0.5 core | 256 / 512 MiB |
| NATS | 3 | 0.25 / 1 core | 512 MiB / 2 GiB |
| ClickHouse | 2 | 0.25 / 2 cores | 2 / 6 GiB |
| ClickHouse Keeper | 3 | 0.1 / 0.5 core | 256 MiB / 1 GiB |

Staging disables the capture user-agent cache preload to avoid paying its
cold-start memory cost continuously. Production keeps the preload and uses a
1.5 GiB limit because the measured cold-start peak exceeded 1 GiB. The
ClickHouse limits include headroom for merges, two replicas, and the local R2
cache; the single-host 300/s measurement is not a direct HA sizing result.

Changing `WRITER_REPLICAS` changes durable-consumer shard ownership. Stop all
writers, set `EVENTS_CONSUMER_REBALANCE_FROM` to the exact active topology, run
the bootstrap job, and then start the new StatefulSet. A new cluster requires
no rebalance override.

## Six-hour JetStream capacity

Work, raw, and DLQ streams have a six-hour maximum age. At the launch ceiling:

```text
300 events/s * 21,600 s = 6,480,000 events
```

| Capacity component | Logical | Measured S2 physical | Physical with 25% headroom |
| --- | ---: | ---: | ---: |
| R3 work, per NATS node | 14.439 GB | 0.978 GB | 1.222 GB |
| R3 work, cluster total | 43.317 GB | 2.933 GB | 3.666 GB |
| R1 raw archive | 7.809 GB | 0.978 GB | 1.222 GB |

Production sets logical caps of 20 GiB for work, 12 GiB for raw, and 2 GiB for
DLQ. NATS therefore needs a 40 GB logical file-store allowance, while measured
compressed data for a complete six-hour outage is about 2.4 GB on the busiest
node including 25% headroom. Each production NATS node receives a 20 GiB
retained HCloud PVC; stage uses 10 GiB. This retains more than 6x physical
headroom at the measured mix without provisioning against uncompressed logical
bytes.

`max_file_store` is a logical JetStream allowance and is intentionally larger
than the compressed filesystem. S2 is explicit in the bootstrap manifests.
Alert at 70% of either PVC or logical allowance and expand before 80%. If the
physical/logical ratio rises above 25%, reassess the PVC immediately.

The capture fallback remains 10 GiB per capture pod. At 300/s, evenly balanced
captures can hold roughly six hours of uncompressed enriched envelopes. It is
a retained HCloud volume attached to each capture StatefulSet replica, so it
survives pod replacement and can be reattached after node loss. The raw archive
is diagnostic and may drop after its smaller archive-spill budget during a NATS
outage; it must never delay capture.

Before increasing the rate, recalculate six-hour logical caps, run the same S2
disk measurement with representative payloads, expand retained PVCs if needed,
and repeat the production-semantics test. StatefulSet volume-claim templates
are immutable on many clusters, so treat expansion as an explicit migration.

## Evidence boundary

The 15k result proves application headroom on one host but used memory-backed
work/consumer state with raw archive disabled. The 300/s launch qualification
used the complete file-backed and archive path, but still shared one host and
used IP2Proxy's fail-open path. A future higher production ceiling requires
separate production-like nodes, the production storage class, pinned enrichment
databases, and recorded disk latency, IOPS, throughput, compression, CPU, and
memory.
