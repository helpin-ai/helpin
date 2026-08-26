use std::time::Duration;

use async_nats::jetstream::consumer::{pull, AckPolicy};
use async_nats::jetstream::stream::{
    Compression, Config as StreamConfig, DiscardPolicy, RetentionPolicy, StorageType,
};

use crate::pipeline::{DeliveryDisposition, ROW_BY_ROW_REJECTION_THRESHOLD};

pub const SHARD_PULL_BATCH_SIZE: usize = 256;
pub const SHARD_ACK_WAIT: Duration = Duration::from_secs(5 * 60);
pub const ACK_PROGRESS_INTERVAL: Duration = Duration::from_secs(60);
pub const STREAM_MAX_AGE: Duration = Duration::from_secs(48 * 60 * 60);
pub const PUBLISH_DEDUPLICATION_WINDOW: Duration = Duration::from_secs(30);

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

/// The durable consumer contract is deliberately encoded in code rather than
/// left to deployment defaults. In particular, unlimited server redelivery is
/// required so a crash on the application's terminal attempt cannot strand the
/// shard behind an exhausted MaxDeliver message.
pub fn shard_consumer_config(shard: u8) -> pull::Config {
    let durable_name = format!("events-writer-v1-{shard:03}");
    pull::Config {
        durable_name: Some(durable_name.clone()),
        name: Some(durable_name),
        description: Some(format!(
            "sessionizing ClickHouse writer for visitor shard {shard}"
        )),
        // NATS requires AckExplicit for pull consumers on WorkQueue streams.
        // MaxAckPending=256 and pull batches of 256 enforce one outstanding
        // contiguous batch per shard.
        ack_policy: AckPolicy::Explicit,
        ack_wait: SHARD_ACK_WAIT,
        max_deliver: -1,
        filter_subject: format!("events.enriched.v1.{shard:03}"),
        max_ack_pending: SHARD_PULL_BATCH_SIZE as i64,
        max_batch: SHARD_PULL_BATCH_SIZE as i64,
        ..Default::default()
    }
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
    fn consumer_enforces_one_contiguous_batch_with_unlimited_redelivery() {
        let config = shard_consumer_config(7);
        assert_eq!(config.ack_policy, AckPolicy::Explicit);
        assert_eq!(config.ack_wait, Duration::from_secs(300));
        assert_eq!(config.max_deliver, -1);
        assert_eq!(config.max_ack_pending, 256);
        assert_eq!(config.max_batch, 256);
        assert_eq!(config.filter_subject, "events.enriched.v1.007");
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
