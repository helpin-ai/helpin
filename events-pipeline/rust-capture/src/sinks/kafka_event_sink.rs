use std::collections::hash_map::DefaultHasher;
use std::hash::{Hash, Hasher};
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc,
};
use std::time::Duration;

use async_trait::async_trait;
use rdkafka::{
    error::{KafkaError, RDKafkaErrorCode},
    metadata::MetadataTopic,
    producer::{FutureProducer, FutureRecord, Producer},
    util::Timeout,
    ClientConfig, ClientContext, Statistics,
};
use tokio::task::JoinSet;

use crate::api::CaptureError;
use crate::health::HealthRegistry;
use crate::utils::kafka_config::create_producer_kafka_config;

use super::{EventSink, EventTypes};

const UNKNOWN_PARTITION_COUNT: usize = 0;
const DEFAULT_METADATA_TIMEOUT_SECS: u64 = 5;

pub fn get_partition(project_id: &str, partitions: usize) -> i32 {
    let mut hasher = DefaultHasher::new();
    project_id.hash(&mut hasher);
    let hash_code = hasher.finish();
    let partition_count = partitions.max(1);
    let partition = (hash_code as u32) % (partition_count as u32);
    partition as i32
}

fn should_refresh_partition_metadata(err: &KafkaError) -> bool {
    matches!(
        err.rdkafka_error_code(),
        Some(RDKafkaErrorCode::UnknownPartition | RDKafkaErrorCode::UnknownTopicOrPartition)
    )
}

pub struct KafkaContext {
    health: HealthRegistry,
}

impl ClientContext for KafkaContext {
    fn stats(&self, stats: Statistics) {
        let brokers_up = stats.brokers.values().any(|broker| broker.state == "UP");
        self.health.set_kafka_healthy(brokers_up);

        let up = stats.brokers.values().filter(|b| b.state == "UP").count();
        metrics::gauge!("capture_kafka_brokers_up", up as f64);
        metrics::gauge!("capture_kafka_callback_queue_depth", stats.replyq as f64);
        metrics::gauge!("capture_kafka_producer_queue_depth", stats.msg_cnt as f64);
    }
}

#[derive(Clone)]
pub struct KafkaSink {
    producer: FutureProducer<KafkaContext>,
    topic: String,
    send_timeout: Duration,
    partition_count: Arc<AtomicUsize>,
}

impl KafkaSink {
    pub fn new(
        topic: String,
        brokers: String,
        health_registry: HealthRegistry,
    ) -> Result<KafkaSink, CaptureError> {
        let timeout_secs: u64 = std::env::var("KAFKA_SEND_TIMEOUT_SECS")
            .ok()
            .and_then(|v| v.parse().ok())
            .unwrap_or(20);

        tracing::debug!(
            "Producer: kafka topic and brokers are: {} {}",
            topic,
            brokers
        );
        let config: ClientConfig = create_producer_kafka_config(brokers, topic.clone());

        let context = KafkaContext {
            health: health_registry,
        };
        let producer: FutureProducer<KafkaContext> =
            config.create_with_context(context).map_err(|e| {
                tracing::error!("Failed to create Kafka producer: {:?}", e);
                CaptureError::RetryableSinkError
            })?;

        Ok(KafkaSink {
            producer,
            topic,
            send_timeout: Duration::from_secs(timeout_secs),
            partition_count: Arc::new(AtomicUsize::new(UNKNOWN_PARTITION_COUNT)),
        })
    }

    fn metadata_timeout(&self) -> Duration {
        self.send_timeout
            .min(Duration::from_secs(DEFAULT_METADATA_TIMEOUT_SECS))
    }

    fn refresh_partition_count(&self) -> Result<usize, CaptureError> {
        let metadata = self
            .producer
            .client()
            .fetch_metadata(
                Some(self.topic.as_str()),
                Timeout::After(self.metadata_timeout()),
            )
            .map_err(|err| {
                tracing::error!(
                    "Failed to fetch Kafka metadata for topic {}: {}",
                    self.topic,
                    err
                );
                CaptureError::RetryableSinkError
            })?;

        let topic = metadata
            .topics()
            .iter()
            .find(|topic: &&MetadataTopic| topic.name() == self.topic.as_str())
            .ok_or_else(|| {
                tracing::error!(
                    "Kafka metadata response did not include topic {}",
                    self.topic
                );
                CaptureError::RetryableSinkError
            })?;

        if let Some(err) = topic.error() {
            tracing::error!(
                "Kafka metadata for topic {} returned error {:?}",
                self.topic,
                err
            );
            return Err(CaptureError::RetryableSinkError);
        }

        let partition_count = topic.partitions().len();
        if partition_count == 0 {
            tracing::error!(
                "Kafka metadata for topic {} returned zero partitions",
                self.topic
            );
            return Err(CaptureError::RetryableSinkError);
        }

        self.partition_count
            .store(partition_count, Ordering::Relaxed);
        metrics::gauge!(
            "capture_kafka_topic_partition_count",
            partition_count as f64,
            "topic" => self.topic.clone()
        );

        tracing::info!(
            "Kafka topic metadata refreshed: topic={} partitions={}",
            self.topic,
            partition_count
        );

        Ok(partition_count)
    }

    fn resolve_partition_count(&self, force_refresh: bool) -> Result<usize, CaptureError> {
        if !force_refresh {
            let cached = self.partition_count.load(Ordering::Relaxed);
            if cached > 0 {
                return Ok(cached);
            }
        }

        self.refresh_partition_count()
    }

    async fn kafka_send(&self, event: EventTypes) -> Result<(), CaptureError> {
        tracing::debug!("Sending event to kafka: {:?}", event);
        let payload = serde_json::to_string(&event).map_err(|e| {
            CaptureError::NonRetryableSinkError(format!("Failed to serialize event: {}", e))
        })?;
        let key = event.key();
        let project_id = event.project_id();
        let mut force_refresh = false;

        loop {
            let partition_count = self.resolve_partition_count(force_refresh)?;
            let partition = get_partition(&project_id, partition_count);
            let record = FutureRecord::to(self.topic.as_str())
                .partition(partition)
                .key(&key)
                .payload(&payload);

            match self
                .producer
                .send(record, Timeout::After(self.send_timeout))
                .await
            {
                Ok((partition, offset)) => {
                    tracing::debug!(
                        "Message delivered to partition {} at offset {}",
                        partition,
                        offset
                    );
                    return Ok(());
                }
                Err((err, _)) if !force_refresh && should_refresh_partition_metadata(&err) => {
                    tracing::warn!(
                        "Kafka produce hit stale partition metadata for topic {}: {}. Refreshing metadata and retrying once.",
                        self.topic,
                        err
                    );
                    force_refresh = true;
                }
                Err((err, _)) => {
                    tracing::error!("Failed to produce event to Kafka: {}", err);
                    return Err(CaptureError::RetryableSinkError);
                }
            }
        }
    }
}

#[async_trait]
impl EventSink for KafkaSink {
    async fn send(&self, event: EventTypes) -> Result<(), CaptureError> {
        self.kafka_send(event).await
    }

    async fn send_batch(&self, events: Vec<EventTypes>) -> Result<(), CaptureError> {
        let mut set = JoinSet::new();

        for event in events {
            let sink = self.clone();
            set.spawn(async move { sink.kafka_send(event).await });
        }

        while let Some(res) = set.join_next().await {
            match res {
                Ok(Ok(_)) => {}
                Ok(Err(err)) => {
                    tracing::error!("Kafka send failed in batch: {:?}", err);
                    set.abort_all();
                    return Err(CaptureError::RetryableSinkError);
                }
                Err(join_err) => {
                    tracing::error!("Kafka send task panicked: {:?}", join_err);
                    set.abort_all();
                    return Err(CaptureError::RetryableSinkError);
                }
            }
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_get_partition_uses_supplied_partition_count() {
        let partition = get_partition("project_123", 6);
        assert!((0..6).contains(&partition));

        let smaller_partition = get_partition("project_123", 3);
        assert!((0..3).contains(&smaller_partition));
    }

    #[test]
    fn test_get_partition_defaults_zero_partition_count_to_partition_zero() {
        assert_eq!(get_partition("project_123", 0), 0);
    }

    #[test]
    fn test_unknown_partition_errors_trigger_metadata_refresh() {
        assert!(should_refresh_partition_metadata(
            &KafkaError::MessageProduction(RDKafkaErrorCode::UnknownPartition,)
        ));
        assert!(should_refresh_partition_metadata(
            &KafkaError::MessageProduction(RDKafkaErrorCode::UnknownTopicOrPartition,)
        ));
        assert!(!should_refresh_partition_metadata(
            &KafkaError::MessageProduction(RDKafkaErrorCode::MessageTimedOut,)
        ));
    }
}
