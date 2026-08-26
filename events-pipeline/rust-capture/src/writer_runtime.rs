use std::collections::{BTreeSet, HashMap};
use std::env;
use std::ops::Range;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};

use anyhow::{anyhow, Context, Result};
use async_nats::jetstream;
use async_nats::jetstream::consumer::PullConsumer;
use async_nats::jetstream::stream::DirectGetErrorKind;
use async_nats::jetstream::{AckKind, Message};
use async_nats::HeaderMap;
use chrono::{DateTime, Utc};
use futures::StreamExt;
use serde::{Deserialize, Serialize};
use tokio::sync::oneshot;
use tokio::task::JoinHandle;

use crate::json_spool::JsonSpool;
use crate::pipeline::{
    delivery_disposition, encode_ingest_version, seed_received_scan_start, visitor_shard,
    DeliveryDisposition, EnrichedEventEnvelopeV1, Sessionizer, VISITOR_SHARD_COUNT,
};
use crate::writer::{
    seed_floor_from_metadata, shard_from_subject, writer_consumer_name, writer_owned_shards,
    ACK_PROGRESS_INTERVAL, WRITER_PULL_MAX_MESSAGES,
};
use crate::writer_store::{
    insert_with_isolation, ClickHouseEventStore, EventInsertRow, IsolationOutcome, SeedRequest,
    StoreError,
};

const WORK_STREAM_NAME: &str = "EVENTS_ENRICHED_V1";
const WRITER_DLQ_SUBJECT: &str = "events.dlq.v1.writer";
const INSERT_MAX_ROWS: usize = 8_192;
const INSERT_MAX_BYTES: usize = 16 * 1024 * 1024;
const INSERT_MAX_WAIT: Duration = Duration::from_secs(1);
const NATS_PUBLISH_TIMEOUT: Duration = Duration::from_secs(5);
const RETRY_INITIAL_DELAY: Duration = Duration::from_millis(250);
const RETRY_MAX_DELAY: Duration = Duration::from_secs(30);
const SESSION_CACHE_IDLE_RETENTION: Duration = Duration::from_secs(60 * 60);
const SESSION_CACHE_PRUNE_INTERVAL: Duration = Duration::from_secs(60);
const DEFAULT_SESSION_CACHE_MAX_ENTRIES_PER_SHARD: usize = 100_000;
const DEFAULT_POISON_SPILL_MAX_BYTES: u64 = 1024 * 1024 * 1024;
const POISON_REPLAY_INTERVAL: Duration = Duration::from_secs(5);

#[derive(Clone, Debug)]
pub struct WriterSettings {
    pub nats_url: String,
    pub clickhouse_http_url: String,
    pub clickhouse_database: String,
    pub clickhouse_user: String,
    pub clickhouse_password: String,
    pub ordinal: usize,
    pub replicas: usize,
    pub session_cache_max_entries_per_shard: usize,
    pub poison_spill_dir: PathBuf,
    pub poison_spill_max_bytes: u64,
}

impl WriterSettings {
    pub fn from_env() -> Result<Self> {
        let replicas = parse_env("WRITER_REPLICAS", 2_usize)?;
        anyhow::ensure!(replicas > 0, "WRITER_REPLICAS must be positive");
        anyhow::ensure!(
            replicas <= usize::from(VISITOR_SHARD_COUNT),
            "WRITER_REPLICAS cannot exceed {VISITOR_SHARD_COUNT} logical shards"
        );
        let ordinal = match env::var("WRITER_ORDINAL") {
            Ok(value) => value
                .parse::<usize>()
                .context("WRITER_ORDINAL must be an unsigned integer")?,
            Err(_) => ordinal_from_pod_name(
                &env::var("POD_NAME")
                    .context("WRITER_ORDINAL or a StatefulSet POD_NAME must be set")?,
            )?,
        };
        anyhow::ensure!(
            ordinal < replicas,
            "writer ordinal must be less than replicas"
        );
        let session_cache_max_entries_per_shard = parse_env(
            "SESSION_CACHE_MAX_ENTRIES_PER_SHARD",
            DEFAULT_SESSION_CACHE_MAX_ENTRIES_PER_SHARD,
        )?;
        anyhow::ensure!(
            session_cache_max_entries_per_shard > 0,
            "SESSION_CACHE_MAX_ENTRIES_PER_SHARD must be positive"
        );

        let poison_spill_max_bytes = parse_env(
            "WRITER_POISON_SPILL_MAX_BYTES",
            DEFAULT_POISON_SPILL_MAX_BYTES,
        )?;
        anyhow::ensure!(
            poison_spill_max_bytes > 0,
            "WRITER_POISON_SPILL_MAX_BYTES must be positive"
        );

        Ok(Self {
            nats_url: required_env("NATS_URL")?,
            clickhouse_http_url: required_env("CLICKHOUSE_HTTP_URL")?,
            clickhouse_database: env::var("CLICKHOUSE_DATABASE")
                .unwrap_or_else(|_| "usermaven".to_string()),
            clickhouse_user: required_env("CLICKHOUSE_USER")?,
            clickhouse_password: required_env("CLICKHOUSE_PASSWORD")?,
            ordinal,
            replicas,
            session_cache_max_entries_per_shard,
            poison_spill_dir: PathBuf::from(
                env::var("WRITER_POISON_SPILL_DIR")
                    .unwrap_or_else(|_| "data/writer-poison-spill".to_string()),
            ),
            poison_spill_max_bytes,
        })
    }

    pub fn owned_shards(&self) -> Vec<u8> {
        writer_owned_shards(self.replicas, self.ordinal)
            .expect("validated writer replica and ordinal settings")
    }
}

pub struct SessionWriter {
    settings: WriterSettings,
    work_jetstream: jetstream::Context,
    dlq_jetstream: jetstream::Context,
    store: ClickHouseEventStore,
    sessions: HashMap<u8, Sessionizer>,
    last_session_prune: Instant,
    poison_spool: JsonSpool,
    health: WriterHealth,
}

#[derive(Clone, Default)]
pub struct WriterHealth {
    ready: Arc<AtomicBool>,
}

impl WriterHealth {
    pub fn is_ready(&self) -> bool {
        self.ready.load(Ordering::SeqCst)
    }

    pub fn set_ready(&self, ready: bool) {
        self.ready.store(ready, Ordering::SeqCst);
    }
}

impl SessionWriter {
    pub async fn connect(settings: WriterSettings) -> Result<Self> {
        Self::connect_with_health(settings, WriterHealth::default()).await
    }

    pub async fn connect_with_health(
        settings: WriterSettings,
        health: WriterHealth,
    ) -> Result<Self> {
        let work_client =
            crate::nats_client::connect_named(&settings.nats_url, "helpin-session-writer-work")
                .await
                .context("connect session writer work client to NATS")?;
        let dlq_client =
            crate::nats_client::connect_named(&settings.nats_url, "helpin-session-writer-dlq")
                .await
                .context("connect session writer DLQ client to NATS")?;
        let store = ClickHouseEventStore::new(
            &settings.clickhouse_http_url,
            &settings.clickhouse_database,
            &settings.clickhouse_user,
            &settings.clickhouse_password,
        );
        let poison_spool = JsonSpool::new(
            settings.poison_spill_dir.clone(),
            settings.poison_spill_max_bytes,
        )
        .context("initialize writer poison spill")?;
        Ok(Self {
            settings,
            work_jetstream: jetstream::new(work_client),
            dlq_jetstream: jetstream::new(dlq_client),
            store,
            sessions: HashMap::new(),
            last_session_prune: Instant::now(),
            poison_spool,
            health,
        })
    }

    pub fn health(&self) -> WriterHealth {
        self.health.clone()
    }

    pub async fn run(mut self) -> Result<()> {
        let startup_started = std::time::Instant::now();
        let stream = self
            .work_jetstream
            .get_stream(WORK_STREAM_NAME)
            .await
            .context("get enriched work stream")?;
        spawn_poison_replayer(self.dlq_jetstream.clone(), self.poison_spool.clone());
        let durable_name = writer_consumer_name(self.settings.replicas, self.settings.ordinal);
        let mut consumer: PullConsumer = stream
            .get_consumer(&durable_name)
            .await
            .map_err(|error| anyhow!("get pre-provisioned consumer {durable_name}: {error}"))?;
        let consumer_info = consumer
            .info()
            .await
            .context("read writer consumer seed boundary")?
            .clone();
        let shared_ack_floor = consumer_info.ack_floor.stream_sequence;
        let seed_metadata = consumer_info.config.metadata.clone();
        let store = self.store.clone();
        let mut initializations = futures::stream::iter(self.settings.owned_shards())
            .map(|shard| {
                let stream = stream.clone();
                let store = store.clone();
                let seed_floor =
                    shared_ack_floor.max(seed_floor_from_metadata(&seed_metadata, shard));
                async move {
                    let sessionizer = seed_shard(&store, shard, &stream, seed_floor)
                        .await
                        .with_context(|| format!("seed shard {shard}"))?;
                    Result::<_>::Ok((shard, sessionizer))
                }
            })
            .buffer_unordered(8);

        while let Some(initialized) = initializations.next().await {
            let (shard, sessionizer) = initialized?;
            metrics::gauge!("event_writer_session_cache_entries", sessionizer.len() as f64, "shard" => shard.to_string());
            self.sessions.insert(shard, sessionizer);
        }
        self.health.set_ready(true);
        metrics::histogram!(
            "event_writer_startup_duration_seconds",
            startup_started.elapsed().as_secs_f64()
        );

        tracing::info!(
            ordinal = self.settings.ordinal,
            replicas = self.settings.replicas,
            shards = ?self.settings.owned_shards(),
            "session writer ready"
        );

        loop {
            let messages = match fetch_writer_round(&consumer).await {
                Ok(messages) => messages,
                Err(error) => {
                    metrics::counter!("event_writer_fetch_errors_total", 1);
                    tracing::warn!(%error, %durable_name, "writer consumer fetch failed");
                    tokio::time::sleep(Duration::from_secs(1)).await;
                    continue;
                }
            };
            if messages.is_empty() {
                continue;
            }
            let progress = ProgressGuard::start(messages.clone());
            let preparation = self.prepare_round(&messages)?;

            for terminal in &preparation.terminal_dlq {
                self.publish_dlq_or_spill(terminal).await?;
            }

            let isolation = self
                .insert_with_retry(&preparation.rows)
                .await
                .context("insert ClickHouse writer round")?;
            let rejected_count = isolation.rejected.len();
            for rejected in isolation.rejected {
                metrics::counter!("event_clickhouse_rejected_rows_total", 1);
                let source = preparation
                    .row_sources
                    .get(rejected.index)
                    .ok_or_else(|| anyhow!("missing source for rejected row {}", rejected.index))?;
                let record = WriterDlqRecord::from_source(
                    source,
                    "clickhouse_row_rejected",
                    &rejected.reason,
                );
                self.publish_dlq_or_spill(&record).await?;
            }

            self.commit_session_overlays(&preparation.session_overlays)?;
            let prune_idle = self.last_session_prune.elapsed() >= SESSION_CACHE_PRUNE_INTERVAL;
            for shard in &preparation.touched_shards {
                let Some(sessionizer) = self.sessions.get_mut(shard) else {
                    continue;
                };
                let evicted = if prune_idle {
                    sessionizer.prune_idle(SESSION_CACHE_IDLE_RETENTION)
                } else {
                    0
                } + sessionizer
                    .enforce_capacity(self.settings.session_cache_max_entries_per_shard);
                if evicted > 0 {
                    metrics::counter!("event_writer_session_cache_evictions_total", evicted as u64, "shard" => shard.to_string());
                }
                metrics::gauge!("event_writer_session_cache_entries", sessionizer.len() as f64, "shard" => shard.to_string());
            }
            if prune_idle {
                self.last_session_prune = Instant::now();
            }
            self.ack_round_with_retry(&messages).await;
            progress.stop().await;
            metrics::counter!(
                "event_writer_committed_rows_total",
                preparation.rows.len().saturating_sub(rejected_count) as u64
            );
        }
    }

    fn prepare_round(&self, messages: &[Message]) -> Result<PreparedRound> {
        // Only sessionize visitors touched by this insert round. Cloning all
        // warm shard caches here made every round O(total cache cardinality).
        let mut overlays = HashMap::<u8, Sessionizer>::new();
        let mut rows = Vec::new();
        let mut row_sources = Vec::new();
        let mut terminal_dlq = Vec::new();
        let mut touched_shards = BTreeSet::new();

        for message in messages {
            let shard = shard_from_subject(message.subject.as_str()).ok_or_else(|| {
                anyhow!(
                    "writer consumer delivered unexpected subject {}",
                    message.subject
                )
            })?;
            anyhow::ensure!(
                self.sessions.contains_key(&shard),
                "missing session state for shard {}",
                shard
            );
            touched_shards.insert(shard);
            let source = MessageSource::from_message(shard, message)?;
            if delivery_disposition(source.delivery_attempt)
                == DeliveryDisposition::TerminalDlqAndAck
            {
                terminal_dlq.push(WriterDlqRecord::from_source(
                    &source,
                    "delivery_attempt_exhausted",
                    "application delivery-attempt cap reached",
                ));
                continue;
            }

            let mut envelope = match parse_and_validate_envelope(shard, message) {
                Ok(envelope) => envelope,
                Err(error) => {
                    terminal_dlq.push(WriterDlqRecord::from_source(
                        &source,
                        "invalid_envelope",
                        &error.to_string(),
                    ));
                    continue;
                }
            };
            if let Err(error) =
                assign_with_overlay(&self.sessions, &mut overlays, shard, &mut envelope)
            {
                terminal_dlq.push(WriterDlqRecord::from_source(
                    &source,
                    "invalid_event_timestamp",
                    &error.to_string(),
                ));
                continue;
            }
            let ingest_version =
                encode_ingest_version(source.stream_sequence, 0, source.delivery_attempt)
                    .map_err(|error| anyhow!(error))?;
            rows.push(EventInsertRow {
                raw_event: serde_json::to_string(&envelope.event)
                    .context("serialize sessionized event")?,
                _nats_subject: source.subject.clone(),
                _nats_stream_sequence: source.stream_sequence,
                _nats_delivery_attempt: source.delivery_attempt.min(255) as u8,
                _retro_generation: 0,
                _ingest_version: ingest_version,
            });
            row_sources.push(source);
        }

        Ok(PreparedRound {
            rows,
            row_sources,
            session_overlays: overlays,
            terminal_dlq,
            touched_shards: touched_shards.into_iter().collect(),
        })
    }

    fn commit_session_overlays(&mut self, overlays: &HashMap<u8, Sessionizer>) -> Result<()> {
        commit_session_overlays_into(&mut self.sessions, overlays)
    }

    async fn insert_with_retry(
        &self,
        rows: &[EventInsertRow],
    ) -> Result<IsolationOutcome, StoreError> {
        let mut combined = IsolationOutcome::default();
        for range in insert_ranges(rows, INSERT_MAX_ROWS, INSERT_MAX_BYTES) {
            let mut delay = RETRY_INITIAL_DELAY;
            loop {
                match insert_with_isolation(&self.store, &rows[range.clone()]).await {
                    Ok(mut outcome) => {
                        for rejected in &mut outcome.rejected {
                            rejected.index += range.start;
                        }
                        combined.rejected.extend(outcome.rejected);
                        break;
                    }
                    Err(StoreError::Retryable(error)) => {
                        metrics::counter!("event_clickhouse_insert_retries_total", 1);
                        tracing::warn!(%error, ?delay, "ClickHouse insert retry");
                        tokio::time::sleep(delay).await;
                        delay = (delay * 2).min(RETRY_MAX_DELAY);
                    }
                    Err(error) => return Err(error),
                }
            }
        }
        Ok(combined)
    }

    async fn publish_dlq_or_spill(&self, record: &WriterDlqRecord) -> Result<()> {
        match publish_dlq(&self.dlq_jetstream, record).await {
            Ok(()) => Ok(()),
            Err(error) => {
                metrics::counter!("event_writer_dlq_publish_failures_total", 1);
                tracing::warn!(%error, event_id = %record.event_id, "writer DLQ unavailable; fsyncing poison spill");
                self.poison_spool
                    .append(record)
                    .await
                    .context("fsync writer poison spill")?;
                metrics::counter!("event_writer_poison_spill_written_total", 1);
                Ok(())
            }
        }
    }

    async fn ack_round_with_retry(&self, messages: &[Message]) {
        // The writer consumer spans many subjects, so acknowledgements must
        // stay in delivery order. This keeps its single ack floor a valid seed
        // boundary for every logical shard it owns.
        let mut next = 0;
        let mut delay = RETRY_INITIAL_DELAY;
        while let Some(message) = messages.get(next) {
            match message.ack().await {
                Ok(()) => {
                    next += 1;
                    delay = RETRY_INITIAL_DELAY;
                }
                Err(error) => {
                    let shard = shard_from_subject(message.subject.as_str());
                    metrics::counter!("event_writer_ack_retries_total", 1, "shard" => shard.map(|value| value.to_string()).unwrap_or_else(|| "unknown".to_string()));
                    tracing::warn!(%error, ?shard, "JetStream message ack failed");
                    tokio::time::sleep(delay).await;
                    delay = (delay * 2).min(RETRY_MAX_DELAY);
                }
            }
        }
    }
}

fn commit_session_overlays_into(
    sessions: &mut HashMap<u8, Sessionizer>,
    overlays: &HashMap<u8, Sessionizer>,
) -> Result<()> {
    for (shard, overlay) in overlays {
        sessions
            .get_mut(shard)
            .ok_or_else(|| anyhow!("missing session state for shard {shard}"))?
            .merge_overlay(overlay);
    }
    Ok(())
}

async fn seed_shard(
    store: &ClickHouseEventStore,
    shard: u8,
    stream: &jetstream::stream::Stream,
    ack_floor: u64,
) -> Result<Sessionizer> {
    let started = std::time::Instant::now();
    let now = Utc::now();
    let subject = format!("events.enriched.v1.{shard:03}");
    let (anchor_received, anchor_event) = match stream
        .direct_get_next_for_subject(&subject, Some(ack_floor.saturating_add(1)))
        .await
    {
        Ok(first) => seed_anchors_from_payload(&first.payload).unwrap_or_else(|error| {
            metrics::counter!("event_writer_seed_anchor_fallback_total", 1, "shard" => shard.to_string());
            tracing::warn!(%error, shard, "first pending event cannot anchor seed; using current time");
            (now, now)
        }),
        Err(error) if error.kind() == DirectGetErrorKind::NotFound => (now, now),
        Err(error) => return Err(error).context("direct-read first pending shard event"),
    };

    let request = SeedRequest {
        visitor_shard: shard,
        anchor_received,
        anchor_event,
        received_scan_start: seed_received_scan_start(anchor_received),
        acked_stream_sequence: ack_floor,
    };
    let seeded = store.seed_sessions(&request).await?;
    let mut sessionizer = Sessionizer::default();
    for row in seeded {
        sessionizer.seed(&row.project_id, &row.user_anonymous_id, row.state);
    }
    metrics::histogram!("event_writer_seed_duration_seconds", started.elapsed().as_secs_f64(), "shard" => shard.to_string());
    Ok(sessionizer)
}

struct PreparedRound {
    rows: Vec<EventInsertRow>,
    row_sources: Vec<MessageSource>,
    session_overlays: HashMap<u8, Sessionizer>,
    terminal_dlq: Vec<WriterDlqRecord>,
    touched_shards: Vec<u8>,
}

fn assign_with_overlay(
    base: &HashMap<u8, Sessionizer>,
    overlays: &mut HashMap<u8, Sessionizer>,
    shard: u8,
    envelope: &mut EnrichedEventEnvelopeV1,
) -> Result<()> {
    let base_shard = base
        .get(&shard)
        .ok_or_else(|| anyhow!("missing session state for shard {shard}"))?;
    let overlay = overlays.entry(shard).or_default();
    let project_id = envelope.event.project_id.clone();
    let user_anonymous_id = envelope.event.user_anonymous_id.clone();
    if overlay.state(&project_id, &user_anonymous_id).is_none() {
        if let Some(state) = base_shard.state(&project_id, &user_anonymous_id) {
            overlay.seed(&project_id, &user_anonymous_id, state);
        }
    }
    overlay
        .assign(envelope)
        .context("assign session in round overlay")?;
    Ok(())
}

async fn fetch_writer_round(consumer: &PullConsumer) -> Result<Vec<Message>> {
    let started = tokio::time::Instant::now();
    let mut messages = Vec::with_capacity(WRITER_PULL_MAX_MESSAGES);
    let mut bytes = 0;

    'round: loop {
        if should_flush_insert_round(messages.len(), bytes, started.elapsed()) {
            break;
        }
        let deadline = started + INSERT_MAX_WAIT;
        let Some(expires) = deadline.checked_duration_since(tokio::time::Instant::now()) else {
            break;
        };
        let max_messages = next_pull_max_messages(messages.len());
        let mut fetched = consumer
            .batch()
            .max_messages(max_messages)
            .expires(expires.max(Duration::from_millis(1)))
            .messages()
            .await
            .context("start writer consumer fetch")?;
        let before = messages.len();
        while let Some(result) = fetched.next().await {
            let message = match result {
                Ok(message) => message,
                // NATS reports a completed pull as 409. The messages already
                // yielded by this pull are complete and must still be committed.
                // Do not add a server-side max_bytes limit here. JetStream can
                // mark the message crossing that boundary as delivered even
                // though async-nats only yields the completion status, leaving
                // it invisible until AckWait expires. ClickHouse byte limits
                // are enforced locally by insert_ranges instead.
                Err(error) if batch_completed(&error.to_string()) => break 'round,
                Err(error) => return Err(anyhow!("writer consumer fetch stream: {error}")),
            };
            bytes += message.payload.len();
            messages.push(message);
        }
        if messages.len() == before && started.elapsed() >= INSERT_MAX_WAIT {
            break;
        }
    }
    Ok(messages)
}

fn next_pull_max_messages(buffered: usize) -> usize {
    WRITER_PULL_MAX_MESSAGES.saturating_sub(buffered)
}

fn batch_completed(error: &str) -> bool {
    error.contains("409") && error.contains("Batch Completed")
}

fn should_flush_insert_round(rows: usize, bytes: usize, elapsed: Duration) -> bool {
    rows >= INSERT_MAX_ROWS || bytes >= INSERT_MAX_BYTES || elapsed >= INSERT_MAX_WAIT
}

fn insert_ranges(rows: &[EventInsertRow], max_rows: usize, max_bytes: usize) -> Vec<Range<usize>> {
    assert!(max_rows > 0);
    assert!(max_bytes > 0);

    let mut ranges = Vec::new();
    let mut start = 0;
    let mut bytes = 0;
    for (index, row) in rows.iter().enumerate() {
        let row_bytes = row.raw_event.len() + row._nats_subject.len() + 32;
        if index > start && (index - start >= max_rows || bytes + row_bytes > max_bytes) {
            ranges.push(start..index);
            start = index;
            bytes = 0;
        }
        bytes += row_bytes;
    }
    if start < rows.len() {
        ranges.push(start..rows.len());
    }
    ranges
}

struct ProgressGuard {
    stop: Option<oneshot::Sender<()>>,
    task: JoinHandle<()>,
}

impl ProgressGuard {
    fn start(messages: Vec<Message>) -> Self {
        let (stop, mut stopped) = oneshot::channel();
        let task = tokio::spawn(async move {
            let mut interval = tokio::time::interval(ACK_PROGRESS_INTERVAL);
            interval.tick().await;
            loop {
                tokio::select! {
                    _ = interval.tick() => {
                        for message in &messages {
                            if let Err(error) = message.ack_with(AckKind::Progress).await {
                                metrics::counter!("event_writer_ack_progress_errors_total", 1);
                                tracing::warn!(%error, "AckProgress failed");
                            }
                        }
                    }
                    _ = &mut stopped => return,
                }
            }
        });
        Self {
            stop: Some(stop),
            task,
        }
    }

    async fn stop(mut self) {
        if let Some(stop) = self.stop.take() {
            let _ = stop.send(());
        }
        let _ = self.task.await;
    }
}

#[derive(Clone, Debug)]
struct MessageSource {
    shard: u8,
    subject: String,
    stream_sequence: u64,
    delivery_attempt: u64,
    event_id: String,
    payload: String,
}

impl MessageSource {
    fn from_message(shard: u8, message: &Message) -> Result<Self> {
        let info = message
            .info()
            .map_err(|error| anyhow!("decode JetStream message metadata: {error}"))?;
        let event_id = serde_json::from_slice::<EnrichedEventEnvelopeV1>(&message.payload)
            .map(|envelope| envelope.event_id)
            .unwrap_or_default();
        Ok(Self {
            shard,
            subject: message.subject.to_string(),
            stream_sequence: info.stream_sequence,
            delivery_attempt: info.delivered.max(1) as u64,
            event_id,
            payload: String::from_utf8_lossy(&message.payload).into_owned(),
        })
    }
}

#[derive(Debug, Deserialize, Serialize)]
struct WriterDlqRecord {
    schema_version: u8,
    rejected_at: String,
    reason_code: String,
    reason: String,
    event_id: String,
    visitor_shard: u8,
    nats_subject: String,
    nats_stream_sequence: u64,
    nats_delivery_attempt: u64,
    payload: String,
}

impl WriterDlqRecord {
    fn from_source(source: &MessageSource, reason_code: &str, reason: &str) -> Self {
        Self {
            schema_version: 1,
            rejected_at: Utc::now().to_rfc3339(),
            reason_code: reason_code.to_string(),
            reason: reason.to_string(),
            event_id: source.event_id.clone(),
            visitor_shard: source.shard,
            nats_subject: source.subject.clone(),
            nats_stream_sequence: source.stream_sequence,
            nats_delivery_attempt: source.delivery_attempt,
            payload: source.payload.clone(),
        }
    }
}

async fn publish_dlq(context: &jetstream::Context, record: &WriterDlqRecord) -> Result<()> {
    let payload = serde_json::to_vec(record).context("serialize writer DLQ record")?;
    let mut headers = HeaderMap::new();
    headers.insert(
        "Nats-Msg-Id",
        format!(
            "writer-{}-{}-{}",
            record.nats_stream_sequence, record.nats_delivery_attempt, record.reason_code
        ),
    );
    tokio::time::timeout(NATS_PUBLISH_TIMEOUT, async {
        context
            .publish_with_headers(WRITER_DLQ_SUBJECT, headers, payload.into())
            .await
            .context("publish writer DLQ")?
            .await
            .context("wait for writer DLQ acknowledgement")?;
        Result::<()>::Ok(())
    })
    .await
    .context("writer DLQ publish timed out")??;
    metrics::counter!("event_writer_dlq_total", 1, "reason" => record.reason_code.clone());
    Ok(())
}

fn spawn_poison_replayer(context: jetstream::Context, spool: JsonSpool) {
    tokio::spawn(async move {
        loop {
            tokio::time::sleep(POISON_REPLAY_INTERVAL).await;
            let pending = match spool.pending().await {
                Ok(pending) => pending,
                Err(error) => {
                    metrics::counter!("event_writer_poison_spill_replay_total", 1, "result" => "scan_failure");
                    tracing::warn!(%error, "scan writer poison spill failed");
                    continue;
                }
            };
            for path in pending {
                let record = match spool.read::<WriterDlqRecord>(&path).await {
                    Ok(record) => record,
                    Err(error) => {
                        metrics::counter!("event_writer_poison_spill_replay_total", 1, "result" => "decode_failure");
                        tracing::error!(%error, ?path, "writer poison spill record is unreadable");
                        continue;
                    }
                };
                match publish_dlq(&context, &record).await {
                    Ok(()) => {
                        if let Err(error) = spool.remove(&path).await {
                            tracing::warn!(%error, ?path, "remove replayed poison spill failed");
                        } else {
                            metrics::counter!("event_writer_poison_spill_replay_total", 1, "result" => "success");
                        }
                    }
                    Err(error) => {
                        tracing::warn!(%error, "writer DLQ still unavailable; retaining poison spill");
                        break;
                    }
                }
            }
        }
    });
}

fn parse_and_validate_envelope(shard: u8, message: &Message) -> Result<EnrichedEventEnvelopeV1> {
    let envelope: EnrichedEventEnvelopeV1 =
        serde_json::from_slice(&message.payload).context("decode enriched envelope")?;
    anyhow::ensure!(envelope.schema_version == 1, "unsupported envelope version");
    anyhow::ensure!(
        envelope.visitor_shard == shard,
        "envelope shard does not match consumer"
    );
    anyhow::ensure!(
        envelope.event.visitor_shard == shard,
        "event shard does not match envelope"
    );
    anyhow::ensure!(
        visitor_shard(
            &envelope.event.project_id,
            &envelope.event.user_anonymous_id
        ) == shard,
        "visitor hash does not match shard"
    );
    anyhow::ensure!(
        envelope.event_id == envelope.event.event_id,
        "envelope event ID does not match event"
    );
    Ok(envelope)
}

fn seed_anchors_from_payload(payload: &[u8]) -> Result<(DateTime<Utc>, DateTime<Utc>)> {
    let envelope: EnrichedEventEnvelopeV1 =
        serde_json::from_slice(payload).context("decode seed envelope")?;
    let received = DateTime::parse_from_rfc3339(&envelope.event_received_at)
        .context("parse seed received time")?
        .with_timezone(&Utc);
    let event = DateTime::parse_from_rfc3339(&envelope.event.timestamp)
        .context("parse seed event time")?
        .with_timezone(&Utc);
    Ok((received, event))
}

fn ordinal_from_pod_name(pod_name: &str) -> Result<usize> {
    pod_name
        .rsplit_once('-')
        .and_then(|(_, suffix)| suffix.parse::<usize>().ok())
        .ok_or_else(|| anyhow!("POD_NAME must end in a StatefulSet ordinal"))
}

fn parse_env<T>(name: &str, default: T) -> Result<T>
where
    T: std::str::FromStr,
    T::Err: std::error::Error + Send + Sync + 'static,
{
    env::var(name)
        .map(|value| value.parse::<T>().with_context(|| format!("parse {name}")))
        .unwrap_or(Ok(default))
}

fn required_env(name: &str) -> Result<String> {
    env::var(name).with_context(|| format!("{name} must be set"))
}

#[cfg(test)]
mod tests {
    use chrono::TimeZone;

    use super::*;

    #[test]
    fn writer_ordinals_partition_every_shard_once() {
        let mut shards = Vec::new();
        for ordinal in 0..4 {
            let settings = WriterSettings {
                nats_url: String::new(),
                clickhouse_http_url: String::new(),
                clickhouse_database: String::new(),
                clickhouse_user: String::new(),
                clickhouse_password: String::new(),
                ordinal,
                replicas: 4,
                session_cache_max_entries_per_shard: DEFAULT_SESSION_CACHE_MAX_ENTRIES_PER_SHARD,
                poison_spill_dir: PathBuf::from("data/writer-poison-spill"),
                poison_spill_max_bytes: DEFAULT_POISON_SPILL_MAX_BYTES,
            };
            shards.extend(settings.owned_shards());
        }
        assert_eq!(shards, (0..100).collect::<Vec<_>>());
    }

    #[test]
    fn recognizes_nats_batch_completion_as_a_successful_pull_boundary() {
        assert!(batch_completed(
            "error while processing messages from the stream: 409, Some(\"Batch Completed\")"
        ));
        assert!(!batch_completed(
            "error while processing messages from the stream: 409, Some(\"Message Size Exceeds MaxBytes\")"
        ));
        assert!(!batch_completed(
            "error while processing messages from the stream: 409, Some(\"Consumer Deleted\")"
        ));
        assert!(!batch_completed("connection reset"));
    }

    #[test]
    fn parses_statefulset_ordinal_only_from_the_last_suffix() {
        assert_eq!(ordinal_from_pod_name("helpin-event-writer-3").unwrap(), 3);
        assert!(ordinal_from_pod_name("helpin-event-writer").is_err());
    }

    #[test]
    fn seed_anchor_uses_both_envelope_clocks() {
        let mut event = crate::events::transform_event::TransformedEvent::default();
        event.event_id = "event-1".to_string();
        event.event_received_at = "2026-08-25T12:00:00Z".to_string();
        event.timestamp = "2026-08-25T10:00:00Z".to_string();
        let envelope = EnrichedEventEnvelopeV1 {
            schema_version: 1,
            event_id: event.event_id.clone(),
            event_received_at: event.event_received_at.clone(),
            visitor_shard: 0,
            event,
        };
        let payload = serde_json::to_vec(&envelope).unwrap();
        let (received, event) = seed_anchors_from_payload(&payload).unwrap();
        assert_eq!(
            received,
            Utc.with_ymd_and_hms(2026, 8, 25, 12, 0, 0).unwrap()
        );
        assert_eq!(event, Utc.with_ymd_and_hms(2026, 8, 25, 10, 0, 0).unwrap());
    }

    #[test]
    fn clickhouse_insert_ranges_enforce_row_and_byte_limits() {
        let rows = (0..5)
            .map(|index| EventInsertRow {
                raw_event: "x".repeat(20 + index),
                _nats_subject: "events.enriched.v1.000".to_string(),
                _nats_stream_sequence: index as u64,
                _nats_delivery_attempt: 1,
                _retro_generation: 0,
                _ingest_version: index as u64,
            })
            .collect::<Vec<_>>();

        assert_eq!(insert_ranges(&rows, 2, usize::MAX), vec![0..2, 2..4, 4..5]);
        assert_eq!(
            insert_ranges(&rows, usize::MAX, 155),
            vec![0..2, 2..4, 4..5]
        );
        assert!(insert_ranges(&[], 2, 100).is_empty());
    }

    #[test]
    fn insert_round_fills_capacity_or_flushes_after_one_second() {
        assert_eq!(next_pull_max_messages(0), WRITER_PULL_MAX_MESSAGES);
        assert_eq!(next_pull_max_messages(999), 7_193);
        assert!(!should_flush_insert_round(
            999,
            1_000,
            Duration::from_millis(999)
        ));
        assert!(should_flush_insert_round(
            999,
            1_000,
            Duration::from_secs(1)
        ));
        assert!(should_flush_insert_round(
            INSERT_MAX_ROWS,
            1_000,
            Duration::ZERO
        ));
        assert!(should_flush_insert_round(
            1,
            INSERT_MAX_BYTES,
            Duration::ZERO
        ));
    }

    #[test]
    fn prepared_overlay_preserves_the_inserted_session_when_the_first_row_is_rejected() {
        let shard = 7;
        let project_id = "project-a";
        let visitor = "visitor-a";
        let mut sessions = HashMap::from([(shard, Sessionizer::default())]);
        let mut overlays = HashMap::new();

        let envelope_at = |event_id: &str, timestamp: &str| {
            let mut event = crate::events::transform_event::TransformedEvent::default();
            event.project_id = project_id.to_string();
            event.user_anonymous_id = visitor.to_string();
            event.event_id = event_id.to_string();
            event.timestamp = timestamp.to_string();
            event.event_received_at = event.timestamp.clone();
            event.visitor_shard = shard;
            EnrichedEventEnvelopeV1 {
                schema_version: 1,
                event_id: event.event_id.clone(),
                event_received_at: event.event_received_at.clone(),
                visitor_shard: shard,
                event,
            }
        };

        let mut first = envelope_at("event-a", "2026-08-25T12:00:00Z");
        assign_with_overlay(&sessions, &mut overlays, shard, &mut first).unwrap();
        let inserted_session = first.event.session_id.clone().unwrap();

        let mut second = envelope_at("event-b", "2026-08-25T12:05:00Z");
        assign_with_overlay(&sessions, &mut overlays, shard, &mut second).unwrap();
        assert_eq!(second.event.session_id.as_deref(), Some(&*inserted_session));

        // ClickHouse rejected `first`, but `second` was already inserted with
        // the session produced by the complete prepared overlay.
        commit_session_overlays_into(&mut sessions, &overlays).unwrap();
        let committed = sessions[&shard].state(project_id, visitor).unwrap();
        assert_eq!(committed.session_id, inserted_session);
        assert_eq!(
            committed.last_event_at,
            Utc.with_ymd_and_hms(2026, 8, 25, 12, 5, 0).unwrap()
        );

        let mut third = envelope_at("event-c", "2026-08-25T12:07:00Z");
        sessions
            .get_mut(&shard)
            .unwrap()
            .assign(&mut third)
            .unwrap();
        assert_eq!(third.event.session_id, Some(committed.session_id));
    }
}
