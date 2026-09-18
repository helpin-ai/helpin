# Historical research: Rust session windowing with Kafka

**Date**: 2026-03-13
**Context**: Evaluating whether KStreams sessionization (`SessionEventWindowStream.java`) can be replaced with a Rust implementation.

> Status: superseded architecture research from March 2026. The recommendations,
> dependency versions, vendor comparisons and performance estimates below record
> that investigation; they are not current installation or upgrade guidance.
> Vendor facts have not been refreshed in this review.

As of the September 2026 code review, [Cargo.toml](../../events-pipeline/rust-capture/Cargo.toml)
uses `async-nats` and has no `rdkafka` dependency. The current
[sessionizer](../../events-pipeline/rust-capture/src/pipeline.rs) implements a
30-minute inactivity window in Rust; the
[writer store](../../events-pipeline/rust-capture/src/writer_store.rs) recovers
active-session evidence from ClickHouse `session_seed_events`. Kafka compacted
topics and Java KStreams are not the current recovery path. Start with the
[pipeline overview](../../events-pipeline/README.md) and
[capture guide](../../events-pipeline/rust-capture/README.md) for current operations.

---

## 1. Kafka Client Libraries in Rust

### rdkafka (rust-rdkafka) — RECOMMENDED
- **GitHub**: github.com/fede1024/rust-rdkafka | ~1.9k stars, 1,060 commits
- **Crate**: `rdkafka` on crates.io (latest 0.39.0; we use 0.31.0 in `rust-capture`)
- **Wrapper around**: librdkafka (same C library powering Python, Go, .NET Kafka clients)
- **Production users**: Materialize (maintains their own fork), Numberly, Sentry, Arroyo, Redpanda ecosystem
- **Key features**:
  - All Kafka versions since 0.8.x
  - Exactly-once semantics (EOS) via idempotent/transactional producers + read-committed consumers
  - Async API (tokio + async-std)
  - Consumer groups, auto rebalancing, offset management
  - SASL/SSL auth (already configured in our `rust-capture`)
  - Admin API (create/delete topics, alter configs)
- **Maturity**: Production-ready. The most widely used Rust Kafka client. Actively maintained.

### kafka-rust
- **GitHub**: github.com/kafka-rust/kafka-rust | ~1.4k stars, 606 commits
- **Pure Rust** implementation (no C dependency)
- **Status**: Much less active than rdkafka, missing many modern Kafka features
- **Verdict**: Not recommended for new projects. Missing EOS, limited protocol support.

### Materialize fork of rdkafka
- **GitHub**: github.com/MaterializeInc/rust-rdkafka
- Adds patches for Materialize's production needs
- Relevant if we need bleeding-edge fixes, but upstream rdkafka is sufficient for our use case

**Recommendation**: Continue with `rdkafka`. We already use it in `rust-capture` (v0.31.0, should upgrade to v0.39.0).

---

## 2. Stream Processing Frameworks in Rust

### Arroyo — MOST MATURE
- **GitHub**: github.com/ArroyoSystems/arroyo | ~4.8k stars, 815 commits, 88 contributors
- **License**: Apache 2.0 (remains open-source post-acquisition)
- **Acquired by Cloudflare** (2025) — now powers Cloudflare Pipelines
- **Architecture**: Distributed stream processing engine, SQL-first, ships as single binary
- **Window types**: Tumbling, sliding, **session windows** (native SQL support)
- **Session window syntax**:
  ```sql
  SELECT SESSION(interval '30 minutes') as window,
         COUNT(DISTINCT auction_id) AS num_auctions
  FROM bids
  GROUP BY 1
  ```
- **Exactly-once**: Yes, including Iceberg sink with two-phase commit
- **Kafka integration**: Native Kafka source/sink connectors, Confluent Schema Registry
- **Performance**: Claims 5x+ over Apache Flink, 10x faster sliding windows
- **State management**: Built-in checkpointing, not RocksDB-based (uses custom Rust engine)
- **Limitation**: Session windows capped at 24 hours max. Requires running Arroyo as a separate service (not an embeddable library).
- **Verdict**: Best option if you want a full framework. Overkill if you just need session windowing on a single topic.

### Callysto
- **GitHub**: github.com/vertexclique/callysto (now psila-ai/callysto) | ~163 stars, 256 commits
- **Concept**: "Kafka Streams mentality in Rust" — agents, tables, services
- **Dependencies**: rdkafka + optional RocksDB for state
- **Windowing**: No evidence of built-in session window support. Focuses on agents/tables/service patterns (like Python's Faust).
- **Documentation**: Only 34.63% documented
- **Verdict**: Interesting concept but immature. No session windowing. Small community.

### Fluvio (InfiniYon)
- **GitHub**: github.com/infinyon/fluvio | active development
- **Concept**: Kafka + Flink replacement in one product, Rust + WASM
- **Features**: SmartModules (WASM-based inline processing), time/event-based windows
- **Session windows**: Not explicitly documented
- **Verdict**: More of a Kafka replacement than a Kafka companion. Would require migrating off Kafka entirely. Not suitable for our use case.

### SeaStreamer
- **Homepage**: sea-ql.org/SeaStreamer
- **Concept**: Generic streaming toolkit (Kafka + Redis + File backends)
- **Features**: Consumer/producer/processor abstractions, backend-agnostic
- **Windowing**: None built-in
- **Verdict**: Good for simple consume-process-produce patterns. No windowing support.

### Timely Dataflow / Differential Dataflow
- **GitHub**: github.com/TimelyDataflow/timely-dataflow
- **Concept**: Academic-grade data-parallel computation framework
- **Used by**: Materialize (internally), Pathway (Python+Rust hybrid)
- **Kafka integration**: Possible but requires manual wiring
- **Windowing**: Can implement any windowing logic, but requires deep framework knowledge
- **Verdict**: Extremely powerful but steep learning curve. Best for building a streaming database, not for a single sessionization task.

---

## 3. Session Windowing in Rust — Manual Implementation

If no framework is adopted, session windowing can be built manually on top of `rdkafka`. Here's an assessment:

### What the KStreams code did at the time
(from `SessionEventWindowStream.java`):
1. Consumes from `KAFKA_TRANSFORMATION_TOPIC`
2. Re-keys events by `project_id:user_anonymous_id`
3. Groups by key, applies 30-minute session window
4. Aggregates: first event in session generates `session_id` (SHA-256 of timestamp + key), subsequent events inherit it
5. Produces to `KAFKA_SESSIONIZED_TOPIC`

### Manual Rust Implementation Approach

```
rdkafka consumer → re-key → in-memory session state → produce with session_id
```

**State management options**:
| Option | Pros | Cons |
|--------|------|------|
| In-memory `HashMap` | Fastest, simplest | Lost on restart, memory-bounded |
| `HashMap` + periodic checkpointing to Kafka compacted topic | Recoverable, Kafka-native | More complex, checkpoint lag |
| RocksDB (`rust-rocksdb` crate) | Persistent, handles large state | Disk I/O, operational complexity |
| `moka` cache (already in our deps) | TTL-based expiry fits session gap | Not persistent across restarts |

**Recommended approach for our use case**: `HashMap<String, SessionState>` with TTL-based eviction (30 min inactivity gap), backed by a Kafka compacted topic for recovery.

### Implementation Complexity

| Component | Difficulty | Notes |
|-----------|-----------|-------|
| Consume + produce with rdkafka | Easy | Already done in `rust-capture` |
| Re-keying events (JSON parse, extract fields) | Easy | Already using `serde_json` |
| Session state tracking (HashMap + TTL) | Medium | ~200 lines: track per-key last-seen timestamp, session_id |
| Window merging on rebalance | Hard | KStreams handles this automatically; manual impl needs careful offset management |
| Exactly-once semantics | Hard | Requires transactional producer + read-committed consumer + offset-in-transaction pattern |
| State recovery on restart | Medium | Replay from Kafka changelog topic or re-consume from earliest |
| Late event handling | Medium | Need watermark logic or accept best-effort ordering |

**Estimated effort**: 2-3 weeks for a production-grade implementation with EOS and state recovery, vs. ~0 for KStreams which provides all of this out of the box.

---

## 4. Production Readiness Summary

| Library/Framework | Stars | Last Active | Session Windows | EOS | Production Users | Verdict |
|-------------------|-------|-------------|-----------------|-----|------------------|---------|
| **rdkafka** | 1.9k | Active (2025) | No (client only) | Yes | Materialize, Numberly, Sentry | Production-ready client |
| **Arroyo** | 4.8k | Active (2026) | Yes (native SQL) | Yes | Cloudflare Pipelines | Production-ready framework |
| **Callysto** | 163 | Sporadic | No | Unclear | Limited | Not ready |
| **Fluvio** | Active | Active (2025) | Not documented | Unclear | InfiniYon Cloud | Kafka replacement, not companion |
| **SeaStreamer** | Small | Active | No | No | Limited | Too basic |
| **kafka-rust** | 1.4k | Low activity | No | No | Unknown | Not recommended |
| **Timely Dataflow** | ~2k | Active | Build-your-own | Build-your-own | Materialize | Over-engineered for this |

---

## 5. Trade-offs: KStreams (Java) vs Rust

### What We Gain with Rust
- **Resource efficiency**: Numberly replaced 54 Python pods with 20 Rust pods (2.7x reduction). Java→Rust would see similar gains (no JVM overhead, no GC pauses).
- **Consistent latency**: No GC stop-the-world pauses. Predictable p99 latency.
- **Lower memory footprint**: No JVM heap overhead. Our KStreams pod likely needs 512MB-1GB heap; Rust equivalent would use 50-100MB.
- **Unified codebase**: `rust-capture` already exists in Rust. Session windowing in Rust means one language for the entire events pipeline.
- **Smaller container images**: Rust static binary (~20MB) vs JVM + dependencies (~200MB+).
- **Faster startup**: Milliseconds vs seconds (no JVM warmup, no JIT compilation).

### What We Lose with Rust
- **KStreams session window is free**: 5 lines of Java (`.windowedBy(SessionWindows.with(INACTIVITY_GAP))`). Building this in Rust is 500+ lines.
- **Window merging**: KStreams handles session merging on consumer rebalance automatically. Manual implementation is the hardest part.
- **Exactly-once for free**: KStreams EOS is a config flag. In Rust, you wire transactional producers + offset commits manually.
- **RocksDB state store**: KStreams manages RocksDB lifecycle, compaction, and changelog topics automatically.
- **Operational maturity**: KStreams has 10+ years of production use across thousands of companies. Any Rust approach is newer.
- **Debugging/monitoring**: KStreams has built-in JMX metrics, state store querying, topology visualization. Rust requires custom observability.

### What Stays the Same
- Kafka protocol handling (both use librdkafka under the hood for Rust, native for Java)
- Message serialization (JSON in both cases)
- SASL/SSL auth (both support it)

---

## 6. Recommendations from the original investigation

### Option A: Keep KStreams (Recommended for now)
- **Effort**: 0
- **Risk**: Low
- Current implementation works. The 30-minute session window with SHA-256 session ID generation is straightforward KStreams usage.
- JVM resource overhead is the main cost (~512MB-1GB per pod).

### Option B: Arroyo as a Session Windowing Service
- **Effort**: 1-2 weeks
- **Risk**: Medium
- Deploy Arroyo alongside Kafka. Define session windowing in SQL.
- Gains: SQL-based, distributed, exactly-once, session windows native.
- Costs: New infrastructure component to operate. Cloudflare acquisition may affect open-source roadmap.

### Option C: Custom Rust Sessionizer in `rust-capture`
- **Effort**: 2-3 weeks
- **Risk**: Medium-High
- Add a new binary target to `rust-capture/Cargo.toml` (like existing `worker`, `consumer`, `replay-worker`).
- Use rdkafka transactional producer for EOS.
- Implement session state with `HashMap` + moka TTL cache + Kafka changelog.
- Gains: Unified Rust codebase, lower resource usage, no JVM.
- Costs: Must implement and maintain session merging, state recovery, watermarks manually.

### Option D: Hybrid — Rust Consumer with Simple Sessionization (no EOS)
- **Effort**: 1 week
- **Risk**: Medium
- Simplest Rust approach: consume, track sessions in-memory with TTL, produce.
- Accept at-least-once semantics (duplicate session events on restart/rebalance).
- If downstream consumers are idempotent (e.g., ClickHouse upserts by session_id), this is acceptable.
- Best if the goal is primarily to eliminate JVM overhead.

### Bottom Line
If the primary motivation is **eliminating the JVM** from the events pipeline, **Option D** is the best cost/benefit. If **correctness guarantees** matter, **keep KStreams (Option A)** — it handles the hard parts (window merging, EOS, state recovery) that would take weeks to reimplement correctly in Rust.
