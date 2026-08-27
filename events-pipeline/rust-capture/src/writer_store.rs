use std::ops::Range;
use std::time::Duration;

use async_trait::async_trait;
use chrono::{DateTime, Utc};
use clickhouse::{Client, Row};
use serde::{Deserialize, Serialize};

use crate::pipeline::{SessionState, ROW_BY_ROW_REJECTION_THRESHOLD};

const INSERT_SEND_TIMEOUT: Duration = Duration::from_secs(30);
const INSERT_END_TIMEOUT: Duration = Duration::from_secs(60);

#[derive(Clone, Debug, Row, Serialize)]
pub struct EventInsertRow {
    pub raw_event: String,
    pub _nats_subject: String,
    pub _nats_stream_sequence: u64,
    pub _nats_delivery_attempt: u8,
    pub _retro_generation: u8,
    pub _ingest_version: u64,
}

#[derive(Clone, Debug)]
pub struct SeedRequest {
    pub visitor_shard: u8,
    pub anchor_received: DateTime<Utc>,
    pub anchor_event: DateTime<Utc>,
    pub received_scan_start: DateTime<Utc>,
    pub acked_stream_sequence: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SeededSession {
    pub project_id: String,
    pub user_anonymous_id: String,
    pub state: SessionState,
}

#[derive(Debug, Row, Deserialize)]
struct SeedRow {
    project_id: String,
    user_anonymous_id: String,
    session_id: String,
    last_event_at_millis: i64,
}

#[derive(Clone)]
pub struct ClickHouseEventStore {
    client: Client,
}

impl ClickHouseEventStore {
    pub fn new(
        url: impl Into<String>,
        database: impl Into<String>,
        user: impl Into<String>,
        password: impl Into<String>,
    ) -> Self {
        let client = Client::default()
            .with_url(url)
            .with_database(database)
            .with_user(user)
            .with_password(password)
            .with_option("async_insert", "0")
            .with_option("wait_end_of_query", "1");
        Self { client }
    }

    pub async fn seed_sessions(
        &self,
        request: &SeedRequest,
    ) -> Result<Vec<SeededSession>, StoreError> {
        let rows = self
            .client
            .query(
                "SELECT project_id, user_anonymous_id,\
                 argMax(session_id, tuple(event_timestamp, ingest_version)) AS session_id,\
                 max(toUnixTimestamp64Milli(event_timestamp)) AS last_event_at_millis \
                 FROM session_seed_events \
                 WHERE visitor_shard = ? \
                 AND event_received_at >= parseDateTime64BestEffort(?, 3, 'UTC') \
                 AND event_received_at <= parseDateTime64BestEffort(?, 3, 'UTC') \
                 AND event_timestamp >= parseDateTime64BestEffort(?, 3, 'UTC') \
                 AND event_timestamp <= parseDateTime64BestEffort(?, 3, 'UTC') \
                 AND nats_stream_sequence <= ? \
                 GROUP BY project_id, user_anonymous_id",
            )
            .bind(request.visitor_shard)
            .bind(format_timestamp(request.received_scan_start))
            .bind(format_timestamp(request.anchor_received))
            .bind(format_timestamp(
                request.anchor_event - chrono::Duration::minutes(30),
            ))
            .bind(format_timestamp(request.anchor_event))
            .bind(request.acked_stream_sequence)
            .fetch_all::<SeedRow>()
            .await
            .map_err(StoreError::from_clickhouse)?;

        rows.into_iter()
            .map(|row| {
                let last_event_at = DateTime::<Utc>::from_timestamp_millis(
                    row.last_event_at_millis,
                )
                .ok_or_else(|| {
                    StoreError::Permanent(format!(
                        "ClickHouse returned invalid seed timestamp {}",
                        row.last_event_at_millis
                    ))
                })?;
                Ok(SeededSession {
                    project_id: row.project_id,
                    user_anonymous_id: row.user_anonymous_id,
                    state: SessionState {
                        session_id: row.session_id,
                        last_event_at,
                    },
                })
            })
            .collect()
    }
}

#[async_trait]
pub trait EventRowInserter: Send + Sync {
    async fn insert_rows(&self, rows: &[EventInsertRow]) -> Result<(), StoreError>;
}

#[async_trait]
impl EventRowInserter for ClickHouseEventStore {
    async fn insert_rows(&self, rows: &[EventInsertRow]) -> Result<(), StoreError> {
        if rows.is_empty() {
            return Ok(());
        }
        let started = std::time::Instant::now();
        let bytes = rows
            .iter()
            .map(|row| row.raw_event.len() + row._nats_subject.len() + 32)
            .sum::<usize>();
        let result = async {
            let mut insert = self
                .client
                .insert::<EventInsertRow>("events")
                .map_err(StoreError::from_clickhouse)?
                .with_timeouts(Some(INSERT_SEND_TIMEOUT), Some(INSERT_END_TIMEOUT));
            for row in rows {
                insert
                    .write(row)
                    .await
                    .map_err(StoreError::from_clickhouse)?;
            }
            insert.end().await.map_err(StoreError::from_clickhouse)
        }
        .await;
        let outcome = match &result {
            Ok(()) => "success",
            Err(StoreError::Retryable(_)) => "retryable_error",
            Err(StoreError::RowRejected(_)) => "row_rejected",
            Err(StoreError::Permanent(_)) => "permanent_error",
        };
        metrics::counter!("event_clickhouse_insert_attempts_total", 1, "result" => outcome);
        metrics::histogram!("event_clickhouse_insert_rows", rows.len() as f64, "result" => outcome);
        metrics::histogram!("event_clickhouse_insert_bytes", bytes as f64, "result" => outcome);
        metrics::histogram!(
            "event_clickhouse_insert_duration_seconds",
            started.elapsed().as_secs_f64(),
            "result" => outcome
        );
        result
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum StoreError {
    Retryable(String),
    RowRejected(String),
    Permanent(String),
}

impl StoreError {
    fn from_clickhouse(error: clickhouse::error::Error) -> Self {
        use clickhouse::error::Error;

        match error {
            Error::Network(_) | Error::TimedOut => Self::Retryable(error.to_string()),
            Error::BadResponse(ref response) if is_row_rejection_response(response) => {
                Self::RowRejected(error.to_string())
            }
            Error::BadResponse(_) => Self::Retryable(error.to_string()),
            Error::InvalidParams(_)
            | Error::Compression(_)
            | Error::Decompression(_)
            | Error::SequenceMustHaveLength
            | Error::DeserializeAnyNotSupported
            | Error::NotEnoughData
            | Error::InvalidUtf8Encoding(_)
            | Error::InvalidTagEncoding(_)
            | Error::Custom(_)
            | Error::RowNotFound => Self::Permanent(error.to_string()),
            _ => Self::Permanent(error.to_string()),
        }
    }
}

impl std::fmt::Display for StoreError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Retryable(message) => write!(formatter, "retryable ClickHouse error: {message}"),
            Self::RowRejected(message) => write!(formatter, "ClickHouse row rejection: {message}"),
            Self::Permanent(message) => write!(formatter, "permanent ClickHouse error: {message}"),
        }
    }
}

impl std::error::Error for StoreError {}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct IsolationOutcome {
    pub rejected: Vec<RejectedRow>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RejectedRow {
    pub index: usize,
    pub reason: String,
}

pub async fn insert_with_isolation<I: EventRowInserter>(
    inserter: &I,
    rows: &[EventInsertRow],
) -> Result<IsolationOutcome, StoreError> {
    let mut pending = vec![0..rows.len()];
    let mut outcome = IsolationOutcome::default();

    while let Some(range) = pending.pop() {
        match inserter.insert_rows(&rows[range.clone()]).await {
            Ok(()) => {}
            Err(StoreError::RowRejected(reason)) if range.len() == 1 => {
                outcome.rejected.push(RejectedRow {
                    index: range.start,
                    reason,
                });
            }
            Err(StoreError::RowRejected(_)) if range.len() <= ROW_BY_ROW_REJECTION_THRESHOLD => {
                push_individual_ranges(&mut pending, range);
            }
            Err(StoreError::RowRejected(_)) => {
                let middle = range.start + range.len() / 2;
                pending.push(middle..range.end);
                pending.push(range.start..middle);
            }
            Err(error) => return Err(error),
        }
    }

    outcome.rejected.sort_by_key(|rejected| rejected.index);
    Ok(outcome)
}

fn push_individual_ranges(pending: &mut Vec<Range<usize>>, range: Range<usize>) {
    for index in (range.start..range.end).rev() {
        pending.push(index..index + 1);
    }
}

fn format_timestamp(timestamp: DateTime<Utc>) -> String {
    timestamp.to_rfc3339_opts(chrono::SecondsFormat::Millis, true)
}

fn is_row_rejection_response(response: &str) -> bool {
    const ROW_REJECTION_CODES: &[u32] = &[6, 27, 41, 53, 69, 117, 349];
    response
        .split("Code:")
        .nth(1)
        .and_then(|tail| {
            tail.trim_start()
                .split(|character: char| !character.is_ascii_digit())
                .next()
        })
        .and_then(|code| code.parse::<u32>().ok())
        .is_some_and(|code| ROW_REJECTION_CODES.contains(&code))
}

#[cfg(test)]
mod tests {
    use std::collections::HashSet;
    use std::sync::Mutex;

    use super::*;

    struct RejectingInserter {
        rejected: HashSet<String>,
        calls: Mutex<Vec<Vec<String>>>,
    }

    #[async_trait]
    impl EventRowInserter for RejectingInserter {
        async fn insert_rows(&self, rows: &[EventInsertRow]) -> Result<(), StoreError> {
            self.calls
                .lock()
                .unwrap()
                .push(rows.iter().map(|row| row.raw_event.clone()).collect());
            if rows
                .iter()
                .any(|row| self.rejected.contains(&row.raw_event))
            {
                Err(StoreError::RowRejected("Code: 117 bad row".to_string()))
            } else {
                Ok(())
            }
        }
    }

    fn rows(count: usize) -> Vec<EventInsertRow> {
        (0..count)
            .map(|index| EventInsertRow {
                raw_event: index.to_string(),
                _nats_subject: "events.enriched.v1.000".to_string(),
                _nats_stream_sequence: index as u64,
                _nats_delivery_attempt: 1,
                _retro_generation: 0,
                _ingest_version: index as u64,
            })
            .collect()
    }

    #[tokio::test]
    async fn bisects_large_batches_then_uses_rows_for_the_small_tail() {
        let inserter = RejectingInserter {
            rejected: HashSet::from(["3".to_string(), "17".to_string()]),
            calls: Mutex::new(Vec::new()),
        };
        let outcome = insert_with_isolation(&inserter, &rows(32)).await.unwrap();
        assert_eq!(
            outcome
                .rejected
                .iter()
                .map(|row| row.index)
                .collect::<Vec<_>>(),
            vec![3, 17]
        );
        assert!(inserter
            .calls
            .lock()
            .unwrap()
            .iter()
            .any(|call| call.len() == 1));
    }

    #[tokio::test]
    async fn retryable_errors_do_not_trigger_bisection() {
        struct Offline;
        #[async_trait]
        impl EventRowInserter for Offline {
            async fn insert_rows(&self, _rows: &[EventInsertRow]) -> Result<(), StoreError> {
                Err(StoreError::Retryable("offline".to_string()))
            }
        }

        assert!(matches!(
            insert_with_isolation(&Offline, &rows(32)).await,
            Err(StoreError::Retryable(_))
        ));
    }

    #[test]
    fn only_known_clickhouse_data_errors_are_row_rejections() {
        assert!(is_row_rejection_response(
            "Code: 117. DB::Exception: bad data"
        ));
        assert!(is_row_rejection_response("Code: 41. Cannot parse datetime"));
        assert!(!is_row_rejection_response(
            "Code: 241. Memory limit exceeded"
        ));
        assert!(!is_row_rejection_response("gateway timeout"));
    }
}
