# Event pipeline test environments

`scripts/e2e.sh` and `scripts/sustained-e2e.sh` start a disposable three-node
NATS 2.14.5 cluster and ClickHouse. Every run generates a one-day test CA and
client/server certificates, enables mutual TLS 1.3, and encrypts the JetStream
file store with `chachapoly`. No test private key is committed.

For network-enrichment qualification, provide pinned databases rather than a
downloader service that only resolves inside Kubernetes:

```bash
SUSTAINED_NETWORK_ENRICHMENT_ENABLED=true \
SUSTAINED_REQUIRE_IP2PROXY=true \
SUSTAINED_IP2PROXY_DB_PATH=/absolute/path/to/pinned-ip2proxy.BIN \
SUSTAINED_CAPTURE_ENV_FILE=/absolute/path/to/test.env \
events-pipeline/scripts/sustained-e2e.sh
```

The IP2Proxy fixture is not checked in because its redistribution terms must be
verified by the owner of the licence. Record its checksum with each golden
enrichment result.

Local compose results are contention tests, not production capacity evidence.
Run `scripts/run-k6-remote.sh` from a separate generator host against the
production-shaped environment, where NATS occupies distinct anti-affined nodes
and its PVCs use the operator-verified provisioned-IOPS NVMe StorageClass.

To exercise this disposable target from a separate load-generator machine,
start the target first. It waits up to ten minutes for the first remote event,
then monitors the configured test duration and requires all 3,000,000 expected
events to reach ClickHouse:

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

Once it prints `External generator target is ready`, run this from the remote
generator, replacing `TARGET_HOST` with the reachable IP or DNS name of the
target machine:

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

The arrival profile is the capacity test: it schedules 5,000 requests/s and
reports dropped iterations if the generator lacks VUs. Use
`PROFILE=connections CONNECTIONS=1000` as a separate closed-model connection
pressure test; it does not guarantee 5,000 completed requests/s.

Set `SUSTAINED_WORK_COMPRESSION=none` for the WorkQueue S2-off A/B run. Archive
compression remains enabled in both variants. Compare publish-ack latency,
NATS CPU, disk bytes/IOPS, fallback volume, and drain time before changing the
production stream.
