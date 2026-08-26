use async_nats::connection::State as ConnectionState;
use async_nats::jetstream;
use async_nats::HeaderMap;
use serde::Serialize;
use std::env;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::{RwLock, Semaphore};

use crate::api::CaptureError;
use crate::events::failed_event::FailedEvent;
use crate::health::HealthRegistry;
use crate::json_spool::JsonSpool;
use crate::pipeline::{EnrichedEventEnvelopeV1, RawArchiveEnvelopeV1};

const RAW_ARCHIVE_SUBJECT: &str = "events.raw.v1";
const CAPTURE_DLQ_SUBJECT: &str = "events.dlq.v1.capture";
const NATS_CONNECT_TIMEOUT: Duration = Duration::from_secs(2);
const NATS_CONNECT_RETRY_DELAY: Duration = Duration::from_secs(1);
const DEFAULT_NATS_WORK_PUBLISH_TIMEOUT: Duration = Duration::from_millis(1_500);
const DEFAULT_NATS_ARCHIVE_PUBLISH_TIMEOUT: Duration = Duration::from_millis(250);
const DEFAULT_NATS_DLQ_PUBLISH_TIMEOUT: Duration = Duration::from_millis(1_500);
const DEFAULT_WORK_IN_FLIGHT_LIMIT: usize = 512;
const ARCHIVE_REPLAY_INTERVAL: Duration = Duration::from_secs(5);
const ARCHIVE_REPLAY_BATCH_SIZE: usize = 256;
const ARCHIVE_IN_FLIGHT_LIMIT: usize = 4_096;
const DEFAULT_ARCHIVE_SPILL_MAX_BYTES: u64 = 512 * 1024 * 1024;

#[derive(Clone)]
pub struct NatsEventPublisher {
    url: String,
    work_connection: Arc<RwLock<Option<NatsConnection>>>,
    archive_connection: Arc<RwLock<Option<NatsConnection>>>,
    dlq_connection: Arc<RwLock<Option<NatsConnection>>>,
    health: HealthRegistry,
    archive_spool: Option<JsonSpool>,
    archive_enabled: bool,
    work_in_flight_limit: usize,
    work_permits: Arc<Semaphore>,
    archive_permits: Arc<Semaphore>,
    publish_timeouts: PublishTimeouts,
}

#[derive(Clone)]
struct NatsConnection {
    client: async_nats::Client,
    jetstream: jetstream::Context,
}

#[derive(Clone, Copy)]
struct PublishTimeouts {
    work: Duration,
    archive: Duration,
    dlq: Duration,
}

impl PublishTimeouts {
    fn from_env() -> Self {
        Self {
            work: timeout_from_env(
                "NATS_WORK_PUBLISH_TIMEOUT_MS",
                DEFAULT_NATS_WORK_PUBLISH_TIMEOUT,
            ),
            archive: timeout_from_env(
                "NATS_ARCHIVE_PUBLISH_TIMEOUT_MS",
                DEFAULT_NATS_ARCHIVE_PUBLISH_TIMEOUT,
            ),
            dlq: timeout_from_env(
                "NATS_DLQ_PUBLISH_TIMEOUT_MS",
                DEFAULT_NATS_DLQ_PUBLISH_TIMEOUT,
            ),
        }
    }

    fn for_kind(self, kind: PublishKind) -> Duration {
        match kind {
            PublishKind::Work => self.work,
            PublishKind::Archive => self.archive,
            PublishKind::Dlq => self.dlq,
        }
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum PublishKind {
    Work,
    Archive,
    Dlq,
}

impl PublishKind {
    fn label(self) -> &'static str {
        match self {
            Self::Work => "work",
            Self::Archive => "archive",
            Self::Dlq => "dlq",
        }
    }

    fn client_name(self) -> &'static str {
        match self {
            Self::Work => "helpin-capture-work",
            Self::Archive => "helpin-capture-archive",
            Self::Dlq => "helpin-capture-dlq",
        }
    }
}

impl NatsEventPublisher {
    /// Connect in the background so capture starts immediately and fsyncs to
    /// fallback storage while NATS is unavailable. Once connected, async-nats
    /// owns reconnects; publish failures never discard the healing client.
    pub fn new(url: impl Into<String>, health: HealthRegistry) -> Self {
        let archive_enabled = bool_from_env("RAW_ARCHIVE_ENABLED", true);
        let archive_dir = PathBuf::from(
            env::var("ARCHIVE_SPILL_DIR").unwrap_or_else(|_| "data/archive-spill".to_string()),
        );
        let archive_max_bytes = env::var("ARCHIVE_SPILL_MAX_BYTES")
            .ok()
            .and_then(|value| value.parse().ok())
            .unwrap_or(DEFAULT_ARCHIVE_SPILL_MAX_BYTES);
        let archive_spool = if !archive_enabled {
            None
        } else {
            match JsonSpool::new(archive_dir, archive_max_bytes) {
                Ok(spool) => Some(spool),
                Err(error) => {
                    metrics::gauge!("capture_archive_spill_available", 0.0);
                    tracing::warn!(%error, "archive spill unavailable; archive failures remain best effort");
                    None
                }
            }
        };
        Self::new_with_config(
            url,
            health,
            archive_spool,
            archive_enabled,
            positive_usize_from_env("NATS_WORK_MAX_IN_FLIGHT", DEFAULT_WORK_IN_FLIGHT_LIMIT),
        )
    }

    #[cfg(test)]
    fn new_with_spool(
        url: impl Into<String>,
        health: HealthRegistry,
        archive_spool: Option<JsonSpool>,
    ) -> Self {
        Self::new_with_config(
            url,
            health,
            archive_spool,
            true,
            DEFAULT_WORK_IN_FLIGHT_LIMIT,
        )
    }

    fn new_with_config(
        url: impl Into<String>,
        health: HealthRegistry,
        archive_spool: Option<JsonSpool>,
        archive_enabled: bool,
        work_in_flight_limit: usize,
    ) -> Self {
        let publisher = Self {
            url: url.into(),
            work_connection: Arc::new(RwLock::new(None)),
            archive_connection: Arc::new(RwLock::new(None)),
            dlq_connection: Arc::new(RwLock::new(None)),
            health,
            archive_spool,
            archive_enabled,
            work_in_flight_limit,
            work_permits: Arc::new(Semaphore::new(work_in_flight_limit)),
            archive_permits: Arc::new(Semaphore::new(ARCHIVE_IN_FLIGHT_LIMIT)),
            publish_timeouts: PublishTimeouts::from_env(),
        };
        publisher.health.set_nats_healthy(false);
        metrics::gauge!(
            "capture_archive_spill_available",
            if publisher.archive_enabled && publisher.archive_spool.is_some() {
                1.0
            } else {
                0.0
            }
        );
        metrics::gauge!(
            "capture_raw_archive_enabled",
            if publisher.archive_enabled { 1.0 } else { 0.0 }
        );
        metrics::gauge!("capture_work_publish_limit", work_in_flight_limit as f64);
        publisher.start_connector(PublishKind::Work);
        publisher.start_connector(PublishKind::Dlq);
        if publisher.archive_enabled {
            publisher.start_connector(PublishKind::Archive);
            publisher.start_archive_replayer();
        } else {
            tracing::info!("raw diagnostic archive publishing disabled");
        }
        publisher
    }

    fn connection_slot(&self, kind: PublishKind) -> Arc<RwLock<Option<NatsConnection>>> {
        match kind {
            PublishKind::Work => self.work_connection.clone(),
            PublishKind::Archive => self.archive_connection.clone(),
            PublishKind::Dlq => self.dlq_connection.clone(),
        }
    }

    fn start_connector(&self, kind: PublishKind) {
        let url = self.url.clone();
        let connection = self.connection_slot(kind);
        let health = self.health.clone();
        tokio::spawn(async move {
            loop {
                match tokio::time::timeout(
                    NATS_CONNECT_TIMEOUT,
                    crate::nats_client::connect_named(&url, kind.client_name()),
                )
                .await
                {
                    Ok(Ok(client)) => {
                        let context = jetstream::new(client.clone());
                        *connection.write().await = Some(NatsConnection {
                            client,
                            jetstream: context,
                        });
                        if kind == PublishKind::Work {
                            health.set_nats_healthy(true);
                        }
                        metrics::gauge!("capture_nats_connection_health", 1.0, "stream" => kind.label());
                        tracing::info!(%url, stream = kind.label(), "capture connected to NATS");
                        return;
                    }
                    Ok(Err(error)) => {
                        if kind == PublishKind::Work {
                            health.set_nats_healthy(false);
                        }
                        metrics::gauge!("capture_nats_connection_health", 0.0, "stream" => kind.label());
                        tracing::warn!(%error, %url, stream = kind.label(), "could not connect capture to NATS");
                    }
                    Err(_) => {
                        if kind == PublishKind::Work {
                            health.set_nats_healthy(false);
                        }
                        metrics::gauge!("capture_nats_connection_health", 0.0, "stream" => kind.label());
                        tracing::warn!(%url, stream = kind.label(), "NATS connection timed out");
                    }
                }
                tokio::time::sleep(NATS_CONNECT_RETRY_DELAY).await;
            }
        });
    }

    fn start_archive_replayer(&self) {
        let Some(spool) = self.archive_spool.clone() else {
            return;
        };
        let publisher = self.clone();
        tokio::spawn(async move {
            loop {
                tokio::time::sleep(ARCHIVE_REPLAY_INTERVAL).await;
                let pending = match spool.pending().await {
                    Ok(pending) => pending,
                    Err(error) => {
                        metrics::counter!("capture_archive_spill_replay_total", 1, "result" => "scan_failure");
                        tracing::warn!(%error, "scan archive spill failed");
                        continue;
                    }
                };
                for path in pending.into_iter().take(ARCHIVE_REPLAY_BATCH_SIZE) {
                    let archive = match spool.read::<RawArchiveEnvelopeV1>(&path).await {
                        Ok(archive) => archive,
                        Err(error) => {
                            metrics::counter!("capture_archive_spill_replay_total", 1, "result" => "decode_failure");
                            tracing::warn!(%error, ?path, "dropping corrupt archive spill record");
                            let _ = spool.remove(&path).await;
                            continue;
                        }
                    };
                    match publisher
                        .publish_json(
                            RAW_ARCHIVE_SUBJECT,
                            &archive.event_id,
                            &archive,
                            PublishKind::Archive,
                        )
                        .await
                    {
                        Ok(()) => {
                            if let Err(error) = spool.remove(&path).await {
                                tracing::warn!(%error, ?path, "remove replayed archive spill failed");
                            } else {
                                metrics::counter!("capture_archive_spill_replay_total", 1, "result" => "success");
                            }
                        }
                        Err(_) => break,
                    }
                }
            }
        });
    }

    async fn connection(&self, kind: PublishKind) -> Result<NatsConnection, CaptureError> {
        self.connection_slot(kind)
            .read()
            .await
            .clone()
            .ok_or(CaptureError::RetryableSinkError)
    }

    pub async fn publish_event(
        &self,
        enriched: &EnrichedEventEnvelopeV1,
        raw: &RawArchiveEnvelopeV1,
    ) -> Result<(), CaptureError> {
        let started = std::time::Instant::now();
        let deadline = started + self.publish_timeouts.work;
        let work_subject = enriched.work_subject();
        let permit_wait = deadline.saturating_duration_since(std::time::Instant::now());
        let (work_result, saturated) = match tokio::time::timeout(
            permit_wait,
            self.work_permits.clone().acquire_owned(),
        )
        .await
        {
            Ok(Ok(permit)) => {
                let in_flight = self.work_permits.available_permits();
                metrics::gauge!(
                    "capture_work_publishes_in_flight",
                    (self.work_in_flight_limit.saturating_sub(in_flight)) as f64
                );
                let result = self
                    .publish_json_until(
                        &work_subject,
                        &enriched.event_id,
                        enriched,
                        PublishKind::Work,
                        started,
                        deadline,
                    )
                    .await;
                drop(permit);
                metrics::gauge!(
                    "capture_work_publishes_in_flight",
                    (self
                        .work_in_flight_limit
                        .saturating_sub(self.work_permits.available_permits()))
                        as f64
                );
                (result, false)
            }
            Err(_) => {
                metrics::counter!("capture_work_publish_saturated_total", 1);
                record_publish_latency(PublishKind::Work, "saturated", started.elapsed());
                (Err(CaptureError::RetryableSinkError), true)
            }
            Ok(Err(_)) => {
                record_publish_latency(PublishKind::Work, "unavailable", started.elapsed());
                (Err(CaptureError::RetryableSinkError), false)
            }
        };
        if work_result.is_ok() {
            self.publish_archive_best_effort(raw);
        }
        if let Err(error) = &work_result {
            metrics::counter!("capture_work_publish_failures_total", 1);
            if saturated {
                tracing::debug!(event_id = %enriched.event_id, "work publish limit reached; using durable fallback");
            } else {
                tracing::warn!(%error, event_id = %enriched.event_id, "work publish failed");
            }
        }
        work_result
    }

    /// The archive is a diagnostic copy, not part of the capture durability
    /// contract. Bound its background work so an archive outage cannot queue
    /// unbounded tasks or delay a successful work-stream response.
    fn publish_archive_best_effort(&self, raw: &RawArchiveEnvelopeV1) {
        if !self.archive_enabled {
            return;
        }
        let Ok(permit) = self.archive_permits.clone().try_acquire_owned() else {
            metrics::counter!("capture_raw_archive_dropped_total", 1, "reason" => "in_flight_limit");
            return;
        };
        let publisher = self.clone();
        let raw = raw.clone();
        tokio::spawn(async move {
            let _permit = permit;
            if publisher
                .publish_json(
                    RAW_ARCHIVE_SUBJECT,
                    &raw.event_id,
                    &raw,
                    PublishKind::Archive,
                )
                .await
                .is_ok()
            {
                return;
            }

            metrics::counter!("capture_raw_archive_publish_failures_total", 1);
            match &publisher.archive_spool {
                Some(spool) => match spool.append(&raw).await {
                    Ok(()) => {
                        metrics::counter!("capture_archive_spill_written_total", 1);
                    }
                    Err(error) => {
                        metrics::counter!("capture_archive_spill_write_failures_total", 1);
                        tracing::warn!(%error, event_id = %raw.event_id, "archive spill failed; diagnostic copy dropped");
                    }
                },
                None => {
                    metrics::counter!("capture_archive_spill_write_failures_total", 1);
                }
            }
        });
    }

    pub async fn publish_failed(&self, event: &FailedEvent) -> Result<(), CaptureError> {
        self.publish_json(
            CAPTURE_DLQ_SUBJECT,
            &event.eventn_ctx_event_id,
            event,
            PublishKind::Dlq,
        )
        .await
    }

    async fn publish_json<T: Serialize>(
        &self,
        subject: &str,
        message_id: &str,
        value: &T,
        kind: PublishKind,
    ) -> Result<(), CaptureError> {
        let started = std::time::Instant::now();
        let deadline = started + self.publish_timeouts.for_kind(kind);
        self.publish_json_until(subject, message_id, value, kind, started, deadline)
            .await
    }

    async fn publish_json_until<T: Serialize>(
        &self,
        subject: &str,
        message_id: &str,
        value: &T,
        kind: PublishKind,
        started: std::time::Instant,
        deadline: std::time::Instant,
    ) -> Result<(), CaptureError> {
        let payload = serde_json::to_vec(value).map_err(|error| {
            CaptureError::NonRetryableSinkError(format!("failed to serialize NATS event: {error}"))
        })?;
        let mut headers = HeaderMap::new();
        headers.insert("Nats-Msg-Id", message_id);
        let connection = match self.connection(kind).await {
            Ok(connection) => connection,
            Err(error) => {
                record_publish_latency(kind, "unavailable", started.elapsed());
                return Err(error);
            }
        };
        if connection.client.connection_state() != ConnectionState::Connected {
            if kind == PublishKind::Work {
                self.health.set_nats_healthy(false);
            }
            metrics::gauge!("capture_nats_connection_health", 0.0, "stream" => kind.label());
            record_publish_latency(kind, "disconnected", started.elapsed());
            return Err(CaptureError::RetryableSinkError);
        }
        let remaining = deadline.saturating_duration_since(std::time::Instant::now());
        if remaining.is_zero() {
            record_publish_latency(kind, "timeout", started.elapsed());
            return Err(CaptureError::RetryableSinkError);
        }
        let result = tokio::time::timeout(remaining, async {
            let acknowledgement = connection
                .jetstream
                .publish_with_headers(subject.to_string(), headers, payload.into())
                .await
                .map_err(|error| error.to_string())?;
            acknowledgement.await.map_err(|error| error.to_string())
        })
        .await;
        match result {
            Ok(Ok(_)) => {
                record_publish_latency(kind, "success", started.elapsed());
            }
            Ok(Err(error)) => {
                record_publish_latency(kind, "error", started.elapsed());
                if kind == PublishKind::Work {
                    self.health.set_nats_healthy(false);
                    tracing::warn!(%error, %subject, %message_id, "JetStream publish failed");
                }
                metrics::gauge!("capture_nats_connection_health", 0.0, "stream" => kind.label());
                return Err(CaptureError::RetryableSinkError);
            }
            Err(_) => {
                record_publish_latency(kind, "timeout", started.elapsed());
                if kind == PublishKind::Work {
                    self.health.set_nats_healthy(false);
                    tracing::warn!(%subject, %message_id, "JetStream publish timed out");
                }
                metrics::gauge!("capture_nats_connection_health", 0.0, "stream" => kind.label());
                return Err(CaptureError::RetryableSinkError);
            }
        }
        metrics::gauge!("capture_nats_connection_health", 1.0, "stream" => kind.label());
        if kind == PublishKind::Work {
            self.health.set_nats_healthy(true);
        }
        Ok(())
    }
}

fn timeout_from_env(name: &str, default: Duration) -> Duration {
    env::var(name)
        .ok()
        .and_then(|value| value.parse::<u64>().ok())
        .filter(|millis| *millis > 0)
        .map(Duration::from_millis)
        .unwrap_or(default)
}

fn positive_usize_from_env(name: &str, default: usize) -> usize {
    env::var(name)
        .ok()
        .and_then(|value| value.parse::<usize>().ok())
        .filter(|value| *value > 0)
        .unwrap_or(default)
}

fn bool_from_env(name: &str, default: bool) -> bool {
    env::var(name)
        .ok()
        .and_then(|value| match value.to_ascii_lowercase().as_str() {
            "true" | "1" | "yes" | "on" => Some(true),
            "false" | "0" | "no" | "off" => Some(false),
            _ => None,
        })
        .unwrap_or(default)
}

fn record_publish_latency(kind: PublishKind, result: &'static str, elapsed: Duration) {
    metrics::histogram!(
        "capture_publish_latency_seconds",
        elapsed.as_secs_f64(),
        "stream" => kind.label(),
        "result" => result
    );
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn construction_connects_in_background_so_capture_can_start_during_an_outage() {
        let publisher = NatsEventPublisher::new_with_spool(
            "nats://unavailable.invalid:4222",
            HealthRegistry::new(),
            None,
        );
        assert!(publisher.work_connection.read().await.is_none());
        assert!(publisher.archive_connection.read().await.is_none());
        assert!(publisher.dlq_connection.read().await.is_none());
        assert!(!Arc::ptr_eq(
            &publisher.work_connection,
            &publisher.archive_connection
        ));
        assert!(!Arc::ptr_eq(
            &publisher.work_connection,
            &publisher.dlq_connection
        ));
        assert_eq!(publisher.work_permits.available_permits(), 512);
        assert_eq!(
            publisher.publish_timeouts.work,
            DEFAULT_NATS_WORK_PUBLISH_TIMEOUT
        );
        assert_eq!(
            publisher.publish_timeouts.archive,
            DEFAULT_NATS_ARCHIVE_PUBLISH_TIMEOUT
        );
    }

    #[tokio::test]
    async fn saturated_work_waits_for_capacity_within_the_publish_budget() {
        let mut publisher = NatsEventPublisher::new_with_config(
            "nats://unavailable.invalid:4222",
            HealthRegistry::new(),
            None,
            false,
            1,
        );
        publisher.publish_timeouts.work = Duration::from_millis(100);
        let _permit = publisher
            .work_permits
            .clone()
            .acquire_owned()
            .await
            .unwrap();
        let event = EnrichedEventEnvelopeV1 {
            schema_version: 1,
            event_id: "event-1".to_string(),
            event_received_at: "2026-08-26T00:00:00Z".to_string(),
            visitor_shard: 0,
            event: Default::default(),
        };
        let raw = RawArchiveEnvelopeV1 {
            schema_version: 1,
            event_id: "event-1".to_string(),
            event_received_at: "2026-08-26T00:00:00Z".to_string(),
            event: Default::default(),
        };

        let publish = tokio::spawn({
            let publisher = publisher.clone();
            async move { publisher.publish_event(&event, &raw).await }
        });
        tokio::time::sleep(Duration::from_millis(20)).await;
        assert!(!publish.is_finished());
        drop(_permit);
        assert!(matches!(
            publish.await.unwrap(),
            Err(CaptureError::RetryableSinkError)
        ));
        assert!(!publisher.archive_enabled);
    }

    #[tokio::test]
    async fn saturated_work_falls_back_when_the_publish_budget_expires() {
        let mut publisher = NatsEventPublisher::new_with_config(
            "nats://unavailable.invalid:4222",
            HealthRegistry::new(),
            None,
            false,
            1,
        );
        publisher.publish_timeouts.work = Duration::from_millis(20);
        let _permit = publisher
            .work_permits
            .clone()
            .acquire_owned()
            .await
            .unwrap();
        let event = EnrichedEventEnvelopeV1 {
            schema_version: 1,
            event_id: "event-1".to_string(),
            event_received_at: "2026-08-26T00:00:00Z".to_string(),
            visitor_shard: 0,
            event: Default::default(),
        };
        let raw = RawArchiveEnvelopeV1 {
            schema_version: 1,
            event_id: "event-1".to_string(),
            event_received_at: "2026-08-26T00:00:00Z".to_string(),
            event: Default::default(),
        };

        let started = std::time::Instant::now();
        assert!(matches!(
            publisher.publish_event(&event, &raw).await,
            Err(CaptureError::RetryableSinkError)
        ));
        assert!(started.elapsed() >= Duration::from_millis(20));
    }
}
