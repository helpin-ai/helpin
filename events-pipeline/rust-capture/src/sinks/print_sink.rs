pub struct PrintSink {}
use super::{EventSink, EventTypes};
use crate::api::CaptureError;
use async_trait::async_trait;

#[async_trait]
impl EventSink for PrintSink {
    async fn send(&self, event: EventTypes) -> Result<(), CaptureError> {
        tracing::info!("single event: {:?}", event);
        Ok(())
    }
    async fn send_batch(&self, events: Vec<EventTypes>) -> Result<(), CaptureError> {
        let span = tracing::span!(tracing::Level::INFO, "batch of events");
        let _enter = span.enter();

        for event in events {
            tracing::info!("event: {:?}", event);
        }

        Ok(())
    }
}
