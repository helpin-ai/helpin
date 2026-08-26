use async_trait::async_trait;
use serde::{Deserialize, Serialize};

use crate::api::CaptureError;
use crate::events::{
    event::ProcessedEvent, failed_event::FailedEvent, transform_event::TransformedEvent,
};

pub mod disk_sink;
pub mod enriching_nats_sink;
pub mod fallback_sink;
pub mod nats_event_sink;
pub mod print_sink;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum EventTypes {
    Processed(ProcessedEvent),
    Transformed(TransformedEvent),
    Failed(FailedEvent),
}

impl EventTypes {
    pub fn key(&self) -> String {
        match self {
            EventTypes::Processed(event) => event.key(),
            EventTypes::Transformed(event) => event.key(),
            EventTypes::Failed(event) => event.key(),
        }
    }

    pub fn project_id(&self) -> String {
        match self {
            EventTypes::Processed(event) => event
                .authorization
                .as_ref()
                .map(|credential| credential.workspace_id.clone())
                .unwrap_or_else(|| {
                    event
                        .event
                        .api_key
                        .split('.')
                        .next()
                        .unwrap_or("")
                        .to_string()
                }),
            EventTypes::Transformed(event) => event.project_id.clone(),
            EventTypes::Failed(event) => event.project_id.clone(),
        }
    }
}

#[async_trait]
pub trait EventSink {
    async fn send(&self, event: EventTypes) -> Result<(), CaptureError>;
    async fn send_batch(&self, events: Vec<EventTypes>) -> Result<(), CaptureError>;
}
