use async_trait::async_trait;
use serde::{Deserialize, Serialize};

use crate::api::CaptureError;
use crate::events::{
    event::ProcessedEvent, failed_event::FailedEvent, transform_event::TransformedEvent,
};

pub mod disk_sink;
pub mod fallback_sink;
pub mod kafka_event_sink;
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
            EventTypes::Processed(event) => {
                let api_key = event.event.api_key.clone();
                let first_element = api_key.split('.').next().unwrap_or("");
                first_element.to_string()
            }
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
