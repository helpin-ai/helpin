# NATS event pipeline

## Runtime path

Helpin's event pipeline is a clean-slate replacement for the unused Kafka
pipeline:

```text
capture -> inline stateless enrichment -> events.enriched.v1.<shard>
                                      \-> events.raw.v1

events.enriched.v1.<shard> -> sessionizer -> ClickHouse
```

The `session-writer` binary is the only work-stream consumer. It owns the
JetStream pull loops, session state, ClickHouse inserts, DLQ isolation, and
acknowledgements. The bootstrap job creates all 128 durable consumers before
capture starts. An environment must not accept capture traffic unless all
writer ordinals are ready; otherwise the WorkQueue eventually reaches
`MaxBytes` and capture spills locally.

Kafka, Kafka Streams, the ClickHouse Kafka engine, NATS KV, shard leases, and
an event-assignment ledger are not part of the production design.

Capture lowercases `project_id` before all identity-sensitive operations. The
visitor shard is:

```text
sha256(lower(project_id) + ":" + user_anonymous_id) % 128
```

The subject and envelope version are immutable contracts. An incompatible
envelope change uses `.v2` subjects and parallel consumers; `.v1` is never
changed in place because a rolled-back writer must still understand queued
messages.

## Validation and enrichment

Capture validates client timestamps before publishing. Accepted timestamps
are between `now - 7 days` and `now + 1 hour`, inclusive. Invalid or out-of-
range timestamps go to the capture DLQ and are never clamped. This preserves
the invariant that every version of an `event_id` has the same timestamp and
therefore the same ClickHouse month partition.

As in the Usermaven pipeline, capture and replay download a missing MaxMind or
IP2Proxy database in-process before processing events. Each process refreshes
MaxMind every 12 hours and IP2Proxy every 24 hours. Downloads are written beside
the target and atomically renamed, and a successful refresh immediately
hot-swaps the parsed resolver. A failed download or reload keeps the last valid
resolver. If no valid database is available, optional network enrichment fails
open with empty fields and an alert; it must never panic the HTTP process.
Availability, file age, database version, and reload-result
metrics are emitted for both databases. Packaged UA and bot assets are pinned
in the image. Licensed
MaxMind/IP2Proxy golden fixtures and their database checksums are a production
enablement gate; this clean-slate branch has no live traffic.

The availability-critical work publish and the privacy-normalized raw archive
publish run concurrently. Full IP addresses are removed or truncated in the
archive using the same privacy decision as enrichment. Only work-publish
failure invokes the work fallback. Archive-only failure is counted and fsynced
to its own bounded best-effort archive spool, which replays only to the archive
subject and cannot consume the work fallback's disk budget. A capture-DLQ
failure preserves the rejected source event in the work fallback. Pending or
disconnected async-NATS clients spill immediately. Connected work and DLQ
publishes wait up to 1.5 seconds for a JetStream acknowledgement; the
diagnostic archive retains a 250 ms budget. These deadlines are independently
configurable and are tuned from
`capture_publish_latency_seconds{stream,result}`. Initial connection uses a
separate two-second background budget.

## JetStream

Use a dedicated three-node, file-backed cluster pinned to NATS Server 2.14.5
by image digest. Enable TLS, S2 stream compression, and JetStream `chachapoly`
encryption at rest. Pin `sync_interval=2m`; changing it is a separate
durability/IO benchmark.

All streams use an explicit 30-second `Nats-Msg-Id` duplicate window.

| Stream | Replicas | Retention | Max age | Discard |
| --- | ---: | --- | --- | --- |
| enriched work | 3 | WorkQueue | 48 hours | new |
| raw debug archive | 1 | Limits | 48 hours | old |
| DLQ | 3 | Limits | 48 hours | old |

The R1 archive deliberately accepts loss with its owning NATS node. It is
diagnostic replay insurance, not an acknowledged-event durability source.

The work stream is durable only while the writer recovers inside 48 hours.
Page at six hours oldest-pending age and escalate at 24 hours. A full work
stream rejects new publishes so capture uses encrypted spill; archive and DLQ
capacity never take valid ingestion down.

## Sessionization and delivery

There are 128 permanent shard subjects and initially four StatefulSet writers.
Each writer owns 32 static shards. Supported writer counts are divisors of 128;
the next scale step is eight.

Each shard consumer uses `AckPolicy=Explicit` (required by WorkQueue
retention), `AckWait=5m`,
`MaxAckPending=256`, `MaxDeliver=-1`, and pulls 256 messages. Pull size plus
`MaxAckPending` makes JetStream enforce one pending shard batch. The writer
acks every message in stream order after the whole contiguous shard batch
commits, so the consumer ack floor advances contiguously. Independent shard
batches are acknowledged concurrently; acknowledgements within one shard stay
serial. NATS 2.14 rejects `AckPolicy=All` for a pull consumer on a WorkQueue
stream, so a single final-message acknowledgement is not available under this
retention contract.
During ClickHouse retry it sends `AckProgress` for every pending message every
60 seconds.

The application, not JetStream, owns the poison attempt threshold. At delivery
256 or later it durably writes the event to DLQ/poison spill and terminally
acks it. `MaxDeliver=-1` ensures a crash during the terminal attempt produces
another delivery rather than an unrecoverable shard stall.

Session state is an in-memory `(session_id, last_event_at)` map. Events reuse
the active session only when their event-time gap is within `[-30m, +30m]`;
otherwise the writer creates the legacy deterministic SHA-256 session ID. A
very late event gets its own historical session but does not replace or move
the visitor's active cached state backwards. Late events do not merge
already-created sessions.

Entries unused in writer wall-clock time for one hour are pruned. A configurable
per-shard hard cap provides bounded memory; crossing it evicts the least-recently
touched entries down to 90% of the cap and emits a metric. An event for an evicted
visitor may start a new session, so sustained eviction is an operational failure,
not normal flow.

The session seed window is measured in event time. On restart the writer:

1. Reads the consumer's contiguous ack floor.
2. Direct-reads the first message for the shard subject after that floor.
3. Uses its immutable `event_received_at` and event timestamp as anchors.
4. Scans received time from `anchor_received - 30m - 48h` through the anchor,
   then filters to the required event-time session window and ack floor.
5. Uses current time when the shard has no pending message.

`session_seed_events` carries a minmax index on event timestamp. Its materialized
view extracts JSON fields directly from `raw_event`; it does not reference
MATERIALIZED aliases from the source table. The view accepts only
`_retro_generation = 0`; identity retro rewrites never change session state and
must not create immediately-expired historical seed partitions.

## ClickHouse writes and poison isolation

One full writer round is `32 * 256 = 8,192` rows. With synchronous inserts and
`async_insert=0`, coalesce for one second at high volume and flush at 8,192
rows or 16 MiB. Do not flush fewer than 1,000 rows before the absolute
ten-second ceiling. Record exact insert attempts, rows, bytes, and duration so
parts pressure can be correlated to writer behavior.

Capture validation means ClickHouse row rejection should be approximately
zero. Any rejection increments `event_clickhouse_rejected_rows_total`; a
non-zero sustained rate pages independently of DLQ volume. As a backstop, the
writer bisects a rejected batch. Once a failing sub-batch has eight or fewer
rows it switches to row-by-row inserts, durably DLQs the bad rows, and releases
the valid rows. This avoids recursively stalling all 32 shards during a
systematic payload wave.

ClickHouse uses `ReplacingMergeTree(_ingest_version)` and
`ORDER BY (project_id, event_date, event_id)`. The version layout is:

```text
stream_sequence << 16 | retro_generation << 8 | min(delivery_attempt, 255)
```

Original ingestion uses retro generation zero. A retroactive identity rewrite
increments the generation, so it outranks every redelivery of the original
sequence. `FINAL` or query-specific `event_id` aggregation is the correctness
guarantee; insert deduplication tokens are not.

The Go ClickHouse event connection sets
`do_not_merge_across_partitions_select_final=1` and
`use_skip_indexes_if_final_exact_mode=1`. Timestamp immutability guarantees all
versions of an event remain in one monthly partition, making the partitioned
`FINAL` setting safe.

## Operations and acceptance

Capture credentials are publish-only. Writer credentials may consume, ack,
direct-read their work stream, and publish writer DLQ messages. Only operations
credentials may manage streams and consumers.

Alert on pending age/capacity, spill bytes and age, ClickHouse rejection and
retry rates, DLQ rate, enrichment fail-open/database age, session-cache size,
seed duration, archive retention, and logical duplicate counts.

ClickHouse monitoring also counts active parts per table and partition and
pages on sustained growth well before the deployed `parts_to_throw_insert`
setting. The runbook queries the deployed setting instead of assuming a
version default and tracks merge backlog alongside parts growth.

`message.ack()` publishes the acknowledgement but does not wait for server
confirmation. A lost ack therefore causes redelivery. The durability contract
is ClickHouse commit before ack plus redelivery convergence through
`_ingest_version`, not a confirmed ack round trip.

Benchmark the retroactive identity job separately before its first real-volume
run. It performs three bounded scans over the six-month rewrite window and is
not covered by the per-query behavioral repository benchmark.

15,000 events/s is a maximum load-test target, not the expected sustained
production rate. The sustained-rate forecast must be recorded before final
`MaxBytes` sizing. In particular, 512 GiB provides only about 200 bytes/event
for 48 hours at 15,000/s; measured compressed payload percentiles determine
the actual archive window. At the maximum target, capture must remain below
10 ms p50 and 25 ms p99. A ten-minute ClickHouse backlog must clear within 30
minutes while traffic continues. Failure tests cover node loss, writer crash before ack,
delivery attempts beyond 255, poison rows, old spill replay, empty and populated
session seeds, and timestamp/shard casing invariants.

## Deployment contract

The checked-in manifests run three NATS 2.14.5 nodes and four writer ordinals.
The NATS image is pinned to its Linux amd64 manifest digest, so the StatefulSet
also selects amd64 nodes. Server and client certificates must be issued by the
same private CA. The server certificate needs both client and server extended
key usage because every node accepts and initiates route connections. Its SANs
must cover the client service and the three StatefulSet pod DNS names.

The deployment system must provision these secrets before applying the stack:

- `helpin-eventpipeline-nats-secrets`: `jetstream-encryption-key` of at least
  32 random bytes plus independent `capture-password`, `writer-password`, and
  `ops-password` values.
- `helpin-eventpipeline-nats-server-tls`: `ca.crt`, `tls.crt`, and `tls.key`.
- `helpin-eventpipeline-nats-capture-tls`,
  `helpin-eventpipeline-nats-writer-tls`, and
  `helpin-eventpipeline-nats-ops-tls`: role-specific client certificates.
- `helpin-eventpipeline-writer`: `CLICKHOUSE_HTTP_URL`, `CLICKHOUSE_USER`, and
  `CLICKHOUSE_PASSWORD`; `CLICKHOUSE_DATABASE` defaults to `usermaven`.

Rotating `jetstream-encryption-key` in place is not permitted. NATS uses it to
decrypt the existing file store; rotation requires the documented JetStream
key migration procedure. Password and certificate rotation is independent.

Performance qualification uses TLS, client-certificate verification,
`chachapoly`, pinned database fixtures, NATS nodes on distinct machines with
provisioned-IOPS NVMe volumes, and an off-box load generator. The local
four-core compose stack is a correctness and contention test; its throughput
is not a production capacity result. Required pod anti-affinity is checked in,
but the environment-specific NVMe `storageClassName` must be supplied and
verified by the cluster operator before qualification.

Changing the work stream from WorkQueue to Limits retention remains an A/B
investigation. Limits avoids replicated stream deletion after every ack, but
still replicates consumer ack state, retains processed traffic until eviction,
makes `DiscardNew` capacity depend on total retained volume, and removes the
WorkQueue guard against overlapping consumers. Do not change retention without
measuring ack latency, storage growth, recovery, and duplicate-consumer safety.
