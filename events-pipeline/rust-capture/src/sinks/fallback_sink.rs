use super::disk_sink::DiskSink;
use super::{EventSink, EventTypes};
use crate::api::CaptureError;
use std::sync::Arc;

use async_trait::async_trait;

pub struct FallbackSink {
    primary: Arc<dyn EventSink + Send + Sync>,
    fallback: Arc<DiskSink>,
}

impl FallbackSink {
    pub fn new(primary: Arc<dyn EventSink + Send + Sync>, fallback: Arc<DiskSink>) -> Self {
        FallbackSink { primary, fallback }
    }

    pub fn disk_sink(&self) -> &DiskSink {
        &self.fallback
    }
}

impl Clone for FallbackSink {
    fn clone(&self) -> Self {
        FallbackSink {
            primary: self.primary.clone(),
            fallback: self.fallback.clone(),
        }
    }
}

#[async_trait]
impl EventSink for FallbackSink {
    async fn send(&self, event: EventTypes) -> Result<(), CaptureError> {
        let fallback_event = event.clone();
        match self.primary.send(event).await {
            Ok(()) => Ok(()),
            Err(CaptureError::RetryableSinkError) => {
                tracing::warn!("Primary sink failed with retryable error, falling back to disk");
                metrics::counter!("capture_fallback_failovers_total", 1);
                self.fallback.send(fallback_event).await
            }
            Err(e) => Err(e),
        }
    }

    async fn send_batch(&self, events: Vec<EventTypes>) -> Result<(), CaptureError> {
        let fallback_events = events.clone();
        match self.primary.send_batch(events).await {
            Ok(()) => Ok(()),
            Err(CaptureError::RetryableSinkError) => {
                tracing::warn!(
                    "Primary sink batch failed with retryable error, falling back to disk"
                );
                metrics::counter!("capture_fallback_failovers_total", 1);
                self.fallback.send_batch(fallback_events).await
            }
            Err(e) => Err(e),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::events::event::{Event, ProcessedEvent};
    use std::collections::HashMap;
    use uuid::Uuid;

    fn make_test_event(api_key: &str) -> EventTypes {
        EventTypes::Processed(ProcessedEvent {
            event: Event {
                api_key: api_key.to_string(),
                event_type: "test".to_string(),
                user: HashMap::new(),
                ..Default::default()
            },
            event_id: Uuid::new_v4(),
        })
    }

    struct SuccessSink;

    #[async_trait]
    impl EventSink for SuccessSink {
        async fn send(&self, _event: EventTypes) -> Result<(), CaptureError> {
            Ok(())
        }
        async fn send_batch(&self, _events: Vec<EventTypes>) -> Result<(), CaptureError> {
            Ok(())
        }
    }

    struct RetryableFailSink;

    #[async_trait]
    impl EventSink for RetryableFailSink {
        async fn send(&self, _event: EventTypes) -> Result<(), CaptureError> {
            Err(CaptureError::RetryableSinkError)
        }
        async fn send_batch(&self, _events: Vec<EventTypes>) -> Result<(), CaptureError> {
            Err(CaptureError::RetryableSinkError)
        }
    }

    struct NonRetryableFailSink;

    #[async_trait]
    impl EventSink for NonRetryableFailSink {
        async fn send(&self, _event: EventTypes) -> Result<(), CaptureError> {
            Err(CaptureError::NonRetryableSinkError("bad event".to_string()))
        }
        async fn send_batch(&self, _events: Vec<EventTypes>) -> Result<(), CaptureError> {
            Err(CaptureError::NonRetryableSinkError("bad event".to_string()))
        }
    }

    #[tokio::test]
    async fn test_primary_success_no_fallback() {
        let dir = std::env::temp_dir().join(format!("fallback_success_{}", Uuid::new_v4()));
        let primary: Arc<dyn EventSink + Send + Sync> = Arc::new(SuccessSink);
        let fallback = Arc::new(DiskSink::new(dir.clone()));
        let sink = FallbackSink::new(primary, fallback.clone());

        let result = sink.send(make_test_event("test")).await;
        assert!(result.is_ok());

        // No events should be written to disk
        fallback.close().await.unwrap();
        let pending = dir.join("pending");
        let files: Vec<_> = std::fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();
        assert_eq!(files.len(), 0, "No fallback files when primary succeeds");

        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_retryable_error_falls_to_disk() {
        let dir = std::env::temp_dir().join(format!("fallback_retry_{}", Uuid::new_v4()));
        let primary: Arc<dyn EventSink + Send + Sync> = Arc::new(RetryableFailSink);
        let fallback = Arc::new(DiskSink::new(dir.clone()));
        let sink = FallbackSink::new(primary, fallback.clone());

        let result = sink.send(make_test_event("fallback_event")).await;
        assert!(result.is_ok(), "Should succeed via fallback");

        fallback.close().await.unwrap();
        let pending = dir.join("pending");
        let files: Vec<_> = std::fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();
        assert_eq!(files.len(), 1, "Event should be written to disk fallback");

        let contents = std::fs::read_to_string(files[0].path()).unwrap();
        assert!(contents.contains("fallback_event"));

        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_non_retryable_error_propagates() {
        let dir = std::env::temp_dir().join(format!("fallback_nonretry_{}", Uuid::new_v4()));
        let primary: Arc<dyn EventSink + Send + Sync> = Arc::new(NonRetryableFailSink);
        let fallback = Arc::new(DiskSink::new(dir.clone()));
        let sink = FallbackSink::new(primary, fallback.clone());

        let result = sink.send(make_test_event("test")).await;
        assert!(result.is_err(), "Non-retryable errors should propagate");

        // No events should be written to disk for non-retryable errors
        fallback.close().await.unwrap();
        let pending = dir.join("pending");
        let files: Vec<_> = std::fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();
        assert_eq!(
            files.len(),
            0,
            "Non-retryable errors should not write to disk"
        );

        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_batch_retryable_falls_to_disk() {
        let dir = std::env::temp_dir().join(format!("fallback_batch_retry_{}", Uuid::new_v4()));
        let primary: Arc<dyn EventSink + Send + Sync> = Arc::new(RetryableFailSink);
        let fallback = Arc::new(DiskSink::new(dir.clone()));
        let sink = FallbackSink::new(primary, fallback.clone());

        let events = vec![
            make_test_event("b1"),
            make_test_event("b2"),
            make_test_event("b3"),
        ];

        let result = sink.send_batch(events).await;
        assert!(result.is_ok(), "Batch should succeed via fallback");

        fallback.close().await.unwrap();
        let pending = dir.join("pending");
        let files: Vec<_> = std::fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();
        assert_eq!(files.len(), 1);

        let contents = std::fs::read_to_string(files[0].path()).unwrap();
        assert_eq!(contents.lines().count(), 3);

        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_batch_non_retryable_propagates() {
        let dir = std::env::temp_dir().join(format!("fallback_batch_nonretry_{}", Uuid::new_v4()));
        let primary: Arc<dyn EventSink + Send + Sync> = Arc::new(NonRetryableFailSink);
        let fallback = Arc::new(DiskSink::new(dir.clone()));
        let sink = FallbackSink::new(primary, fallback.clone());

        let events = vec![make_test_event("b1"), make_test_event("b2")];

        let result = sink.send_batch(events).await;
        assert!(
            result.is_err(),
            "Non-retryable batch errors should propagate"
        );

        std::fs::remove_dir_all(&dir).unwrap();
    }
}
