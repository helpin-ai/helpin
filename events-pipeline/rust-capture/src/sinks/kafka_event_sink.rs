use std::hash::{Hash, Hasher};
use std::time::Duration;
use std::collections::hash_map::DefaultHasher;

use async_trait::async_trait;
use rdkafka::{
    producer::{FutureProducer, FutureRecord},
    util::Timeout,
    ClientConfig, ClientContext, Statistics,
};
use tokio::task::JoinSet;

use crate::api::CaptureError;
use crate::health::HealthRegistry;
use crate::utils::kafka_config::create_producer_kafka_config;

use super::{EventSink, EventTypes};

pub fn get_partition(project_id: &str, partitions: usize) -> i32 {
    let mut hasher = DefaultHasher::new();
    project_id.hash(&mut hasher);
    let hash_code = hasher.finish();
    let partition = (hash_code as u32) % (partitions as u32);
    partition as i32
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
        })
    }
}

impl KafkaSink {
    async fn kafka_send(
        producer: FutureProducer<KafkaContext>,
        topic: String,
        event: EventTypes,
        timeout: Duration,
    ) -> Result<(), CaptureError> {
        tracing::debug!("Sending event to kafka: {:?}", event);
        let payload = serde_json::to_string(&event).map_err(|e| {
            CaptureError::NonRetryableSinkError(format!("Failed to serialize event: {}", e))
        })?;
        let key = event.key();
        let partition = get_partition(&event.project_id(), 18);

        let record = FutureRecord::to(topic.as_str())
            .partition(partition)
            .key(&key)
            .payload(&payload);

        let result = producer.send(record, Timeout::After(timeout)).await;

        match result {
            Ok((partition, offset)) => {
                tracing::debug!(
                    "Message delivered to partition {} at offset {}",
                    partition,
                    offset
                );
                Ok(())
            }
            Err((err, _)) => {
                tracing::error!("Failed to produce event to Kafka: {}", err);
                Err(CaptureError::RetryableSinkError)
            }
        }
    }
}

#[async_trait]
impl EventSink for KafkaSink {
    async fn send(&self, event: EventTypes) -> Result<(), CaptureError> {
        Self::kafka_send(
            self.producer.clone(),
            self.topic.clone(),
            event,
            self.send_timeout,
        )
        .await
    }

    async fn send_batch(&self, events: Vec<EventTypes>) -> Result<(), CaptureError> {
        let mut set = JoinSet::new();

        for event in events {
            let producer = self.producer.clone();
            let topic = self.topic.clone();
            let timeout = self.send_timeout;

            set.spawn(Self::kafka_send(producer, topic, event, timeout));
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
