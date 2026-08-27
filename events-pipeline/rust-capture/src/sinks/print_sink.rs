pub struct PrintSink {}
use super::{EventSink, EventTypes};
use crate::api::CaptureError;
use async_trait::async_trait;

#[async_trait]
impl EventSink for PrintSink {
    async fn send(&self, event: EventTypes) -> Result<(), CaptureError> {
        log_event(&event);
        Ok(())
    }
    async fn send_batch(&self, events: Vec<EventTypes>) -> Result<(), CaptureError> {
        let span = tracing::span!(tracing::Level::INFO, "batch of events");
        let _enter = span.enter();

        for event in events {
            log_event(&event);
        }

        Ok(())
    }
}

fn log_event(event: &EventTypes) {
    let event_type = match event {
        EventTypes::Processed(event) => event.event.event_type.as_str(),
        EventTypes::Transformed(event) => event.event_type.as_str(),
        EventTypes::Failed(_) => "failed",
    };
    tracing::info!(
        project_id = %event.project_id(),
        event_key = %event.key(),
        event_type,
        "event accepted by print sink"
    );
}
