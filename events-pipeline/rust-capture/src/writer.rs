use std::collections::HashMap;
use std::time::Duration;

use async_nats::jetstream::consumer::{pull, AckPolicy};
use async_nats::jetstream::stream::{
    Compression, Config as StreamConfig, DiscardPolicy, RetentionPolicy, StorageType,
};

use anyhow::{ensure, Result};

use crate::pipeline::{DeliveryDisposition, ROW_BY_ROW_REJECTION_THRESHOLD, VISITOR_SHARD_COUNT};

pub const SHARD_PULL_BATCH_SIZE: usize = 256;
pub const WRITER_PULL_MAX_MESSAGES: usize = 8_192;
pub const SHARD_ACK_WAIT: Duration = Duration::from_secs(5 * 60);
pub const ACK_PROGRESS_INTERVAL: Duration = Duration::from_secs(60);
pub const STREAM_MAX_AGE: Duration = Duration::from_secs(48 * 60 * 60);
pub const PUBLISH_DEDUPLICATION_WINDOW: Duration = Duration::from_secs(30);
pub const WRITER_CONSUMER_PREFIX: &str = "events-writer-v2-";
pub const LEGACY_SHARD_CONSUMER_PREFIX: &str = "events-writer-v1-";
const SEED_FLOOR_METADATA_PREFIX: &str = "helpin.seed-floor.";

pub fn enriched_work_stream_config(max_bytes: i64) -> StreamConfig {
    StreamConfig {
        name: "EVENTS_ENRICHED_V1".to_string(),
        description: Some("sessionization work queue for enriched capture events".to_string()),
        subjects: vec!["events.enriched.v1.*".to_string()],
        retention: RetentionPolicy::WorkQueue,
        max_age: STREAM_MAX_AGE,
        max_bytes,
        discard: DiscardPolicy::New,
        storage: StorageType::File,
        num_replicas: 3,
        duplicate_window: PUBLISH_DEDUPLICATION_WINDOW,
        allow_direct: true,
        compression: Some(Compression::S2),
        ..Default::default()
    }
}

pub fn raw_archive_stream_config(max_bytes: i64) -> StreamConfig {
    StreamConfig {
        name: "EVENTS_RAW_V1".to_string(),
        description: Some("privacy-normalized short-lived event debug archive".to_string()),
        subjects: vec!["events.raw.v1".to_string()],
        retention: RetentionPolicy::Limits,
        max_age: STREAM_MAX_AGE,
        max_bytes,
        discard: DiscardPolicy::Old,
        storage: StorageType::File,
        num_replicas: 1,
        duplicate_window: PUBLISH_DEDUPLICATION_WINDOW,
        compression: Some(Compression::S2),
        ..Default::default()
    }
}

pub fn dlq_stream_config(max_bytes: i64) -> StreamConfig {
    StreamConfig {
        name: "EVENTS_DLQ_V1".to_string(),
        description: Some("bounded capture and writer rejection records".to_string()),
        subjects: vec!["events.dlq.v1.*".to_string()],
        retention: RetentionPolicy::Limits,
        max_age: STREAM_MAX_AGE,
        max_bytes,
        discard: DiscardPolicy::Old,
        storage: StorageType::File,
        num_replicas: 3,
        duplicate_window: PUBLISH_DEDUPLICATION_WINDOW,
        compression: Some(Compression::S2),
        ..Default::default()
    }
}

pub fn writer_consumer_name(replicas: usize, ordinal: usize) -> String {
    format!("{WRITER_CONSUMER_PREFIX}r{replicas:03}-o{ordinal:03}")
}

/// Assigns the permanent logical shards to a writer ordinal. Remainder shards
/// go to the lowest ordinals, so three writers own 34, 33, and 33 shards while
/// four writers own four contiguous groups of 25.
pub fn writer_owned_shards(replicas: usize, ordinal: usize) -> Result<Vec<u8>> {
    let shard_count = usize::from(VISITOR_SHARD_COUNT);
    ensure!(replicas > 0, "WRITER_REPLICAS must be positive");
    ensure!(
        replicas <= shard_count,
        "WRITER_REPLICAS cannot exceed {shard_count} logical shards"
    );
    ensure!(
        ordinal < replicas,
        "writer ordinal must be less than replicas"
    );

    let base = shard_count / replicas;
    let remainder = shard_count % replicas;
    let start = ordinal * base + ordinal.min(remainder);
    let len = base + usize::from(ordinal < remainder);
    Ok((start..start + len).map(|shard| shard as u8).collect())
}

pub fn shard_subject(shard: u8) -> String {
    format!("events.enriched.v1.{shard:03}")
}

pub fn shard_from_subject(subject: &str) -> Option<u8> {
    let suffix = subject.strip_prefix("events.enriched.v1.")?;
    if suffix.len() != 3 || !suffix.bytes().all(|byte| byte.is_ascii_digit()) {
        return None;
    }
    let shard = suffix.parse::<u8>().ok()?;
    (u16::from(shard) < VISITOR_SHARD_COUNT).then_some(shard)
}

pub fn seed_floor_metadata_key(shard: u8) -> String {
    format!("{SEED_FLOOR_METADATA_PREFIX}{shard:03}")
}

pub fn seed_floor_from_metadata(metadata: &HashMap<String, String>, shard: u8) -> u64 {
    metadata
        .get(&seed_floor_metadata_key(shard))
        .and_then(|value| value.parse::<u64>().ok())
        .unwrap_or_default()
}

/// The durable consumer contract is deliberately encoded in code rather than
/// left to deployment defaults. One replicated consumer state machine is
/// created per writer and filters all of that writer's logical shard subjects.
pub fn writer_consumer_config(
    replicas: usize,
    ordinal: usize,
    memory_storage: bool,
    seed_floors: &HashMap<u8, u64>,
) -> Result<pull::Config> {
    let durable_name = writer_consumer_name(replicas, ordinal);
    let shards = writer_owned_shards(replicas, ordinal)?;
    let metadata = shards
        .iter()
        .filter_map(|shard| {
            seed_floors
                .get(shard)
                .copied()
                .filter(|floor| *floor > 0)
                .map(|floor| (seed_floor_metadata_key(*shard), floor.to_string()))
        })
        .collect();
    Ok(pull::Config {
        durable_name: Some(durable_name.clone()),
        name: Some(durable_name),
        description: Some(format!(
            "sessionizing ClickHouse writer {ordinal} of {replicas} for {} visitor shards",
            shards.len()
        )),
        // NATS requires AckExplicit for pull consumers on WorkQueue streams.
        // A writer holds at most one bounded insert round at a time and acks it
        // in delivery order, keeping the shared consumer ack floor contiguous.
        ack_policy: AckPolicy::Explicit,
        ack_wait: SHARD_ACK_WAIT,
        max_deliver: -1,
        filter_subjects: shards.into_iter().map(shard_subject).collect(),
        max_ack_pending: WRITER_PULL_MAX_MESSAGES as i64,
        max_batch: WRITER_PULL_MAX_MESSAGES as i64,
        memory_storage,
        metadata,
        ..Default::default()
    })
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RejectionIsolation {
    Bisect,
    RowByRow,
}

pub fn rejection_isolation(row_count: usize) -> RejectionIsolation {
    if row_count <= ROW_BY_ROW_REJECTION_THRESHOLD {
        RejectionIsolation::RowByRow
    } else {
        RejectionIsolation::Bisect
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct DeliveryPlan {
    pub disposition: DeliveryDisposition,
    pub encoded_attempt: u8,
}

pub fn delivery_plan(delivery_attempt: u64) -> DeliveryPlan {
    DeliveryPlan {
        disposition: crate::pipeline::delivery_disposition(delivery_attempt),
        encoded_attempt: delivery_attempt.min(u64::from(u8::MAX)) as u8,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn consumer_filters_all_writer_shards_with_unlimited_redelivery() {
        let config = writer_consumer_config(4, 1, false, &HashMap::new()).unwrap();
        assert_eq!(config.ack_policy, AckPolicy::Explicit);
        assert_eq!(config.ack_wait, Duration::from_secs(300));
        assert_eq!(config.max_deliver, -1);
        assert_eq!(config.max_ack_pending, 8_192);
        assert_eq!(config.max_batch, 8_192);
        assert!(config.filter_subject.is_empty());
        assert_eq!(config.filter_subjects.len(), 25);
        assert_eq!(
            config.filter_subjects.first().unwrap(),
            "events.enriched.v1.025"
        );
        assert_eq!(
            config.filter_subjects.last().unwrap(),
            "events.enriched.v1.049"
        );
        assert!(!config.memory_storage);
        assert!(
            writer_consumer_config(4, 1, true, &HashMap::new())
                .unwrap()
                .memory_storage
        );
    }

    #[test]
    fn three_writers_own_every_shard_once_with_balanced_counts() {
        let assignments = (0..3)
            .map(|ordinal| writer_owned_shards(3, ordinal).unwrap())
            .collect::<Vec<_>>();
        assert_eq!(
            assignments.iter().map(Vec::len).collect::<Vec<_>>(),
            [34, 33, 33]
        );
        assert_eq!(assignments.concat(), (0..100).collect::<Vec<_>>());
    }

    #[test]
    fn consumer_carries_per_shard_migration_seed_floors() {
        let config = writer_consumer_config(3, 2, false, &HashMap::from([(67, 42_u64)])).unwrap();
        assert_eq!(seed_floor_from_metadata(&config.metadata, 67), 42);
        assert_eq!(seed_floor_from_metadata(&config.metadata, 68), 0);
    }

    #[test]
    fn work_capacity_rejects_new_events_but_debug_streams_drop_old_data() {
        let work = enriched_work_stream_config(10_000);
        assert_eq!(work.retention, RetentionPolicy::WorkQueue);
        assert_eq!(work.discard, DiscardPolicy::New);
        assert_eq!(work.max_age, STREAM_MAX_AGE);
        assert_eq!(work.num_replicas, 3);
        assert_eq!(work.duplicate_window, Duration::from_secs(30));
        assert_eq!(work.compression, Some(Compression::S2));
        assert!(work.allow_direct);

        assert_eq!(raw_archive_stream_config(1_000).discard, DiscardPolicy::Old);
        assert_eq!(raw_archive_stream_config(1_000).num_replicas, 1);
        assert_eq!(dlq_stream_config(1_000).discard, DiscardPolicy::Old);
    }

    #[test]
    fn terminal_attempt_remains_terminal_after_the_version_field_saturates() {
        let plan = delivery_plan(256);
        assert_eq!(plan.disposition, DeliveryDisposition::TerminalDlqAndAck);
        assert_eq!(plan.encoded_attempt, 255);
        assert_eq!(delivery_plan(10_000), plan);
    }

    #[test]
    fn small_rejected_sub_batches_switch_to_row_by_row() {
        assert_eq!(rejection_isolation(9), RejectionIsolation::Bisect);
        assert_eq!(rejection_isolation(8), RejectionIsolation::RowByRow);
        assert_eq!(rejection_isolation(1), RejectionIsolation::RowByRow);
    }
}
