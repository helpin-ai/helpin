use chrono::{DateTime, Duration, SecondsFormat, Utc};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::time::{Duration as StdDuration, Instant};

use crate::events::event::ProcessedEvent;
use crate::events::transform_event::TransformedEvent;

pub const VISITOR_SHARD_COUNT: u16 = 100;
pub const MAX_EVENT_AGE_DAYS: i64 = 7;
pub const MAX_EVENT_FUTURE_HOURS: i64 = 1;
pub const MAX_EXPECTED_REPLAY_DELAY_HOURS: i64 = 48;
pub const MAX_STREAM_SEQUENCE: u64 = (1_u64 << 48) - 1;
pub const SESSION_INACTIVITY_MINUTES: i64 = 30;
pub const TERMINAL_DELIVERY_ATTEMPT: u64 = 256;
pub const ROW_BY_ROW_REJECTION_THRESHOLD: usize = 8;

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct EnrichedEventEnvelopeV1 {
    pub schema_version: u8,
    pub event_id: String,
    pub event_received_at: String,
    pub visitor_shard: u8,
    pub event: TransformedEvent,
}

impl EnrichedEventEnvelopeV1 {
    pub fn new(mut event: TransformedEvent) -> Self {
        let visitor_shard = visitor_shard(&event.project_id, &event.user_anonymous_id);
        event.project_id = event.project_id.to_lowercase();
        event.visitor_shard = visitor_shard;
        Self {
            schema_version: 1,
            event_id: event.event_id.clone(),
            event_received_at: event.event_received_at.clone(),
            visitor_shard,
            event,
        }
    }

    pub fn work_subject(&self) -> String {
        format!("events.enriched.v1.{:03}", self.visitor_shard)
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct RawArchiveEnvelopeV1 {
    pub schema_version: u8,
    pub event_id: String,
    pub event_received_at: String,
    pub event: ProcessedEvent,
}

pub fn canonical_visitor_key(project_id: &str, user_anonymous_id: &str) -> String {
    format!("{}:{}", project_id.to_lowercase(), user_anonymous_id)
}

pub fn visitor_shard(project_id: &str, user_anonymous_id: &str) -> u8 {
    let digest = Sha256::digest(canonical_visitor_key(project_id, user_anonymous_id).as_bytes());
    let prefix = u64::from_be_bytes(digest[..8].try_into().expect("SHA-256 prefix is 8 bytes"));
    (prefix % u64::from(VISITOR_SHARD_COUNT)) as u8
}

pub fn normalize_custom_timestamp(
    timestamp: i64,
    now: DateTime<Utc>,
) -> Result<String, TimestampValidationError> {
    let milliseconds = timestamp.unsigned_abs() > 1_000_000_000_000_u64;
    let (seconds, nanoseconds) = if milliseconds {
        (
            timestamp.div_euclid(1_000),
            timestamp.rem_euclid(1_000) as u32 * 1_000_000,
        )
    } else {
        (timestamp, 0)
    };
    let parsed = DateTime::<Utc>::from_timestamp(seconds, nanoseconds)
        .ok_or(TimestampValidationError::Unrepresentable)?;
    if parsed < now - Duration::days(MAX_EVENT_AGE_DAYS) {
        return Err(TimestampValidationError::TooOld);
    }
    if parsed > now + Duration::hours(MAX_EVENT_FUTURE_HOURS) {
        return Err(TimestampValidationError::TooFarInFuture);
    }
    Ok(parsed.to_rfc3339_opts(SecondsFormat::Nanos, true))
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TimestampValidationError {
    Unrepresentable,
    TooOld,
    TooFarInFuture,
}

impl std::fmt::Display for TimestampValidationError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Unrepresentable => formatter.write_str("timestamp is not representable"),
            Self::TooOld => formatter.write_str("timestamp is more than 7 days old"),
            Self::TooFarInFuture => {
                formatter.write_str("timestamp is more than 1 hour in the future")
            }
        }
    }
}

pub fn encode_ingest_version(
    stream_sequence: u64,
    retro_generation: u8,
    delivery_attempt: u64,
) -> Result<u64, &'static str> {
    if stream_sequence > MAX_STREAM_SEQUENCE {
        return Err("stream sequence exceeds the 48-bit version field");
    }
    Ok((stream_sequence << 16)
        | (u64::from(retro_generation) << 8)
        | delivery_attempt.min(u64::from(u8::MAX)))
}

pub fn seed_received_scan_start(anchor_received: DateTime<Utc>) -> DateTime<Utc> {
    anchor_received - Duration::minutes(30) - Duration::hours(MAX_EXPECTED_REPLAY_DELAY_HOURS)
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum DeliveryDisposition {
    Process,
    TerminalDlqAndAck,
}

pub fn delivery_disposition(delivery_attempt: u64) -> DeliveryDisposition {
    if delivery_attempt >= TERMINAL_DELIVERY_ATTEMPT {
        DeliveryDisposition::TerminalDlqAndAck
    } else {
        DeliveryDisposition::Process
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SessionState {
    pub session_id: String,
    pub last_event_at: DateTime<Utc>,
}

#[derive(Clone, Default)]
pub struct Sessionizer {
    sessions: HashMap<String, CachedSession>,
}

#[derive(Clone)]
struct CachedSession {
    state: SessionState,
    last_touched: Instant,
}

impl Sessionizer {
    pub fn seed(&mut self, project_id: &str, user_anonymous_id: &str, state: SessionState) {
        self.sessions.insert(
            canonical_visitor_key(project_id, user_anonymous_id),
            CachedSession {
                state,
                last_touched: Instant::now(),
            },
        );
    }

    pub fn assign(
        &mut self,
        event: &mut EnrichedEventEnvelopeV1,
    ) -> Result<SessionState, chrono::ParseError> {
        let event_timestamp =
            DateTime::parse_from_rfc3339(&event.event.timestamp)?.with_timezone(&Utc);
        let key = canonical_visitor_key(&event.event.project_id, &event.event.user_anonymous_id);
        let current = self.sessions.get(&key).map(|cached| cached.state.clone());
        let state = assign_session(
            current.as_ref(),
            event_timestamp,
            &event.event.project_id,
            &event.event.user_anonymous_id,
        );
        event.event.session_id = Some(state.session_id.clone());
        let active_state = current
            .filter(|current| {
                event_timestamp
                    < current.last_event_at - Duration::minutes(SESSION_INACTIVITY_MINUTES)
            })
            .unwrap_or_else(|| state.clone());
        self.sessions.insert(
            key,
            CachedSession {
                state: active_state,
                last_touched: Instant::now(),
            },
        );
        Ok(state)
    }

    pub fn len(&self) -> usize {
        self.sessions.len()
    }

    pub fn state(&self, project_id: &str, user_anonymous_id: &str) -> Option<SessionState> {
        self.sessions
            .get(&canonical_visitor_key(project_id, user_anonymous_id))
            .map(|cached| cached.state.clone())
    }

    pub fn is_empty(&self) -> bool {
        self.sessions.is_empty()
    }

    /// Merge the visitor states touched by a prepared writer round. The
    /// overlay is bounded by the round size, so this avoids cloning or
    /// recomputing the full warm session cache.
    pub fn merge_overlay(&mut self, overlay: &Sessionizer) {
        self.sessions.extend(overlay.sessions.clone());
    }

    pub fn prune_idle(&mut self, max_idle: StdDuration) -> usize {
        let before = self.sessions.len();
        self.sessions
            .retain(|_, cached| cached.last_touched.elapsed() <= max_idle);
        before - self.sessions.len()
    }

    pub fn enforce_capacity(&mut self, max_entries: usize) -> usize {
        if self.sessions.len() <= max_entries {
            return 0;
        }
        let before = self.sessions.len();
        let target = max_entries.saturating_mul(9) / 10;
        let mut newest = self
            .sessions
            .iter()
            .map(|(key, cached)| (key.clone(), cached.last_touched))
            .collect::<Vec<_>>();
        newest.sort_unstable_by_key(|(_, touched)| std::cmp::Reverse(*touched));
        newest.truncate(target);
        let retained = newest
            .into_iter()
            .map(|(key, _)| key)
            .collect::<std::collections::HashSet<_>>();
        self.sessions.retain(|key, _| retained.contains(key));
        before - self.sessions.len()
    }
}

pub fn assign_session(
    current: Option<&SessionState>,
    event_timestamp: DateTime<Utc>,
    project_id: &str,
    user_anonymous_id: &str,
) -> SessionState {
    if let Some(current) = current {
        let gap = event_timestamp - current.last_event_at;
        let window = Duration::minutes(SESSION_INACTIVITY_MINUTES);
        if -window <= gap && gap <= window {
            return SessionState {
                session_id: current.session_id.clone(),
                last_event_at: current.last_event_at.max(event_timestamp),
            };
        }
    }

    let timestamp_millis = event_timestamp.timestamp_millis();
    let input = format!(
        "{}:{}",
        timestamp_millis,
        canonical_visitor_key(project_id, user_anonymous_id)
    );
    SessionState {
        session_id: format!("{:x}", Sha256::digest(input.as_bytes())),
        last_event_at: event_timestamp,
    }
}

#[cfg(test)]
mod tests {
    use chrono::TimeZone;

    use super::*;

    #[test]
    fn visitor_shard_is_case_insensitive_for_project_id() {
        let lower = visitor_shard("project-a", "visitor-1");
        assert_eq!(lower, visitor_shard("PROJECT-A", "visitor-1"));
        assert_eq!(lower, visitor_shard("Project-A", "visitor-1"));
        assert!(lower < VISITOR_SHARD_COUNT as u8);
    }

    #[test]
    fn work_subject_contains_zero_padded_shard() {
        let mut event = TransformedEvent::default();
        event.project_id = "PROJECT-A".to_string();
        event.user_anonymous_id = "visitor-1".to_string();
        event.event_id = "event-1".to_string();
        event.event_received_at = "2026-08-25T00:00:00Z".to_string();
        let envelope = EnrichedEventEnvelopeV1::new(event);
        assert_eq!(envelope.event.project_id, "project-a");
        assert!(envelope.work_subject().starts_with("events.enriched.v1."));
        assert_eq!(
            envelope.work_subject().len(),
            "events.enriched.v1.000".len()
        );
    }

    #[test]
    fn timestamp_acceptance_window_is_bounded() {
        let now = Utc.with_ymd_and_hms(2026, 8, 25, 12, 0, 0).unwrap();
        assert!(normalize_custom_timestamp((now - Duration::days(7)).timestamp(), now).is_ok());
        assert_eq!(
            normalize_custom_timestamp(
                (now - Duration::days(7) - Duration::seconds(1)).timestamp(),
                now
            ),
            Err(TimestampValidationError::TooOld)
        );
        assert!(normalize_custom_timestamp((now + Duration::hours(1)).timestamp(), now).is_ok());
        assert_eq!(
            normalize_custom_timestamp(
                (now + Duration::hours(1) + Duration::seconds(1)).timestamp(),
                now
            ),
            Err(TimestampValidationError::TooFarInFuture)
        );
    }

    #[test]
    fn timestamp_supports_milliseconds() {
        let now = Utc.with_ymd_and_hms(2026, 8, 25, 12, 0, 0).unwrap();
        let value = normalize_custom_timestamp(now.timestamp_millis() - 123, now).unwrap();
        assert_eq!(value, "2026-08-25T11:59:59.877000000Z");
    }

    #[test]
    fn retro_generation_outranks_all_redeliveries() {
        let original_max = encode_ingest_version(42, 0, 10_000).unwrap();
        let retro = encode_ingest_version(42, 1, 1).unwrap();
        assert!(retro > original_max);
        assert_eq!(original_max & 0xff, 255);
    }

    #[test]
    fn stream_sequence_is_bounded_to_48_bits() {
        assert!(encode_ingest_version(MAX_STREAM_SEQUENCE, 255, 255).is_ok());
        assert!(encode_ingest_version(MAX_STREAM_SEQUENCE + 1, 0, 1).is_err());
    }

    #[test]
    fn application_owns_the_terminal_delivery_after_jetstream_redelivery() {
        assert_eq!(delivery_disposition(255), DeliveryDisposition::Process);
        assert_eq!(
            delivery_disposition(256),
            DeliveryDisposition::TerminalDlqAndAck
        );
        assert_eq!(
            delivery_disposition(10_000),
            DeliveryDisposition::TerminalDlqAndAck
        );
    }

    #[test]
    fn seed_scan_includes_replay_delay_and_event_window() {
        let anchor = Utc.with_ymd_and_hms(2026, 8, 25, 12, 0, 0).unwrap();
        assert_eq!(
            seed_received_scan_start(anchor),
            anchor - Duration::hours(48) - Duration::minutes(30)
        );
    }

    #[test]
    fn very_late_event_gets_a_separate_session() {
        let last_event_at = Utc.with_ymd_and_hms(2026, 8, 25, 12, 0, 0).unwrap();
        let current = SessionState {
            session_id: "session-1".to_string(),
            last_event_at,
        };
        let assigned = assign_session(
            Some(&current),
            last_event_at - Duration::hours(3),
            "PROJECT-A",
            "visitor-1",
        );
        assert_ne!(assigned.session_id, "session-1");
        assert_eq!(assigned.last_event_at, last_event_at - Duration::hours(3));
    }

    #[test]
    fn inactivity_boundary_is_inclusive() {
        let first = Utc.with_ymd_and_hms(2026, 8, 25, 12, 0, 0).unwrap();
        let current = assign_session(None, first, "PROJECT-A", "visitor-1");
        let at_boundary = assign_session(
            Some(&current),
            first + Duration::minutes(30),
            "project-a",
            "visitor-1",
        );
        assert_eq!(at_boundary.session_id, current.session_id);

        let outside = assign_session(
            Some(&at_boundary),
            at_boundary.last_event_at + Duration::minutes(30) + Duration::milliseconds(1),
            "project-a",
            "visitor-1",
        );
        assert_ne!(outside.session_id, current.session_id);
    }

    #[test]
    fn sessionizer_uses_case_insensitive_visitor_state_and_mutates_the_event() {
        let timestamp = Utc.with_ymd_and_hms(2026, 8, 25, 12, 0, 0).unwrap();
        let mut sessionizer = Sessionizer::default();
        sessionizer.seed(
            "PROJECT-A",
            "visitor-1",
            SessionState {
                session_id: "seed-session".to_string(),
                last_event_at: timestamp,
            },
        );
        let mut transformed = TransformedEvent::default();
        transformed.project_id = "project-a".to_string();
        transformed.user_anonymous_id = "visitor-1".to_string();
        transformed.event_id = "event-1".to_string();
        transformed.timestamp = (timestamp + Duration::minutes(1)).to_rfc3339();
        let mut envelope = EnrichedEventEnvelopeV1::new(transformed);

        let state = sessionizer.assign(&mut envelope).unwrap();

        assert_eq!(state.session_id, "seed-session");
        assert_eq!(envelope.event.session_id.as_deref(), Some("seed-session"));
        assert_eq!(sessionizer.len(), 1);
    }

    #[test]
    fn very_late_event_does_not_replace_the_active_session() {
        let active_at = Utc.with_ymd_and_hms(2026, 8, 25, 12, 0, 0).unwrap();
        let mut sessionizer = Sessionizer::default();
        sessionizer.seed(
            "project-a",
            "visitor-1",
            SessionState {
                session_id: "active-session".to_string(),
                last_event_at: active_at,
            },
        );

        let mut old = TransformedEvent::default();
        old.project_id = "project-a".to_string();
        old.user_anonymous_id = "visitor-1".to_string();
        old.event_id = "old-event".to_string();
        old.timestamp = (active_at - Duration::hours(3)).to_rfc3339();
        let old_state = sessionizer
            .assign(&mut EnrichedEventEnvelopeV1::new(old))
            .unwrap();
        assert_ne!(old_state.session_id, "active-session");

        let mut next = TransformedEvent::default();
        next.project_id = "project-a".to_string();
        next.user_anonymous_id = "visitor-1".to_string();
        next.event_id = "next-event".to_string();
        next.timestamp = (active_at + Duration::minutes(1)).to_rfc3339();
        let next_state = sessionizer
            .assign(&mut EnrichedEventEnvelopeV1::new(next))
            .unwrap();
        assert_eq!(next_state.session_id, "active-session");
    }

    #[test]
    fn session_cache_capacity_trims_below_the_hard_limit() {
        let now = Utc.with_ymd_and_hms(2026, 8, 25, 12, 0, 0).unwrap();
        let mut sessionizer = Sessionizer::default();
        for visitor in 0..11 {
            sessionizer.seed(
                "project-a",
                &format!("visitor-{visitor}"),
                SessionState {
                    session_id: visitor.to_string(),
                    last_event_at: now,
                },
            );
        }
        assert_eq!(sessionizer.enforce_capacity(10), 2);
        assert_eq!(sessionizer.len(), 9);
    }
}
