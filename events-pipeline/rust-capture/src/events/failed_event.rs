use serde::{Deserialize, Serialize};

#[derive(Default, Debug, Clone, Serialize, Deserialize)]
pub struct FailedEvent {
    pub error: i64,
    pub error_description: String,
    pub project_id: String,
    pub payload: String,
    pub eventn_ctx_event_id: String,
}

impl FailedEvent {
    pub fn key(&self) -> String {
        format!("{}", self.eventn_ctx_event_id)
    }
}

use serde_json;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_failed_event_default() {
        // Test that the default FailedEvent has expected values
        let event = FailedEvent::default();

        assert_eq!(event.error, 0);
        assert_eq!(event.error_description, "");
        assert_eq!(event.project_id, "");
        assert_eq!(event.payload, "");
        assert_eq!(event.eventn_ctx_event_id, "");
    }

    #[test]
    fn test_failed_event_key() {
        // Test that the key method returns expected value
        let mut event = FailedEvent::default();
        event.eventn_ctx_event_id = "12345".to_string();

        assert_eq!(event.key(), "12345");
    }

    #[test]
    fn test_failed_event_serialization() {
        // Test that a FailedEvent can be correctly serialized to JSON
        let mut event = FailedEvent::default();
        event.error = 1;
        event.error_description = "test error".to_string();
        event.project_id = "test project".to_string();
        event.payload = "test payload".to_string();
        event.eventn_ctx_event_id = "12345".to_string();

        let json = serde_json::to_string(&event).unwrap();

        assert_eq!(
            json,
            "{\"error\":1,\"error_description\":\"test error\",\"project_id\":\"test project\",\"payload\":\"test payload\",\"eventn_ctx_event_id\":\"12345\"}"
        );
    }

    #[test]
    fn test_failed_event_deserialization() {
        // Test that a FailedEvent can be correctly deserialized from JSON
        let json = "{\"error\":1,\"error_description\":\"test error\",\"project_id\":\"test project\",\"payload\":\"test payload\",\"eventn_ctx_event_id\":\"12345\"}";

        let event: FailedEvent = serde_json::from_str(json).unwrap();

        assert_eq!(event.error, 1);
        assert_eq!(event.error_description, "test error");
        assert_eq!(event.project_id, "test project");
        assert_eq!(event.payload, "test payload");
        assert_eq!(event.eventn_ctx_event_id, "12345");
    }
}
