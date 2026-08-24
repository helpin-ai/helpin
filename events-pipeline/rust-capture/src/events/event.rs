use std::collections::BTreeMap;
use std::collections::HashMap;

use axum::http::HeaderValue;
use serde::{Deserialize, Serialize};
use serde_json::Value;

use anyhow::Result;
use axum::http::HeaderMap;
use bytes::Bytes;
use serde_json::from_str;
use uuid::Uuid;

use crate::auth::authorization::AuthorizedCredential;
use crate::utils::time;
#[derive(Deserialize, Default)]
pub enum Compression {
    #[default]
    #[serde(rename = "gzip-js")]
    GzipJs,
}

#[derive(Deserialize, Default)]
pub struct EventQuery {
    // A map where each key is a query parameter and each value is a vector of strings,
    // representing the values for that parameter.
    #[serde(flatten)]
    pub query_params: BTreeMap<String, String>,
}

#[derive(Debug, Deserialize)]
pub struct EventFormData {
    pub data: String,
}

#[derive(Clone, Default, Debug, Deserialize, Serialize)]
pub struct Event {
    pub api_key: String,
    pub event_type: String,
    pub click_id: Option<HashMap<String, Value>>,
    pub ids: Option<HashMap<String, Value>>,
    pub utc_time: Option<String>,
    // pub local_tz_offset: Option<i64>,
    pub referrer: Option<String>,
    pub url: Option<String>,
    pub page_title: Option<String>,
    pub doc_path: Option<String>,
    pub doc_host: Option<String>,
    pub doc_search: Option<String>,
    pub screen_resolution: Option<String>,
    pub vp_size: Option<String>,
    pub user_agent: Option<String>,
    pub user_language: Option<String>,
    pub doc_encoding: Option<String>,
    pub user: HashMap<String, Value>,
    pub utm: Option<HashMap<String, Value>>,
    pub company: Option<HashMap<String, Value>>,
    pub event_attributes: Option<HashMap<String, Value>>,
    pub autocapture_attributes: Option<HashMap<String, Value>>,
    pub ip: Option<String>,
    pub cookie_policy: Option<String>,
    pub ip_policy: Option<String>,
    pub received_at: String,
    pub src: Option<String>,
    pub timestamp: Option<i64>, // New field for custom timestamp
}

fn extract_and_set_request_context(
    obj: &mut serde_json::Map<String, serde_json::Value>,
    ip_addr: &str,
    cookie_policy: &str,
    ip_policy: &str,
    user_agent: &str,
    token: &str,
) {
    let fields = [
        (
            "ip",
            obj.get("source_ip")
                .cloned()
                .unwrap_or_else(|| serde_json::json!(ip_addr)),
        ),
        (
            "referrer",
            obj.get("referer").cloned().unwrap_or_else(|| "".into()),
        ),
        ("cookie_policy", serde_json::json!(cookie_policy)),
        ("ip_policy", serde_json::json!(ip_policy)),
        (
            "user_agent",
            obj.get("user_agent")
                .cloned()
                .unwrap_or_else(|| serde_json::json!(user_agent)),
        ),
        ("received_at", serde_json::json!(time::get_current_time())),
    ];

    for (key, value) in &fields {
        obj.insert(key.to_string(), value.clone());
    }

    // Corner case if token value is present and different from body api_key value, override it.
    if !token.is_empty() {
        let current_api_key = obj
            .get("api_key")
            .map(|v| v.to_string())
            .unwrap_or_default();
        if current_api_key != serde_json::json!(token).to_string() {
            tracing::debug!("request credential overrides the payload credential");
            obj.insert("api_key".to_string(), serde_json::json!(token));
        }
    }
}

impl Event {
    /// We post up _at least one_ event, so when decompressiong and deserializing there
    /// could be more than one. Hence this function has to return a Vec.
    /// TODO: Use an axum extractor for this
    pub fn from_bytes(
        bytes: Bytes,
        headers: HeaderMap,
        query_params: BTreeMap<String, String>,
        token: Option<String>,
    ) -> Result<Vec<Event>> {
        tracing::debug!(len = bytes.len(), "decoding new event");
        let payload = String::from_utf8(bytes.into())?;

        // Extract request context.

        let ip_addr = match headers.get("x-forwarded-for") {
            Some(value) => match value.to_str() {
                Ok(str) => str,
                Err(_) => "",
            },
            None => "",
        };

        let user_agent = match headers.get("user-agent") {
            Some(value) => match value.to_str() {
                Ok(str) => str,
                Err(_) => "",
            },
            None => "",
        };

        let json_value: serde_json::Value = serde_json::from_str(&payload)
            .map_err(|e| anyhow::anyhow!("Failed to parse JSON: {}", e))?;
        let mut cookie_policy = String::new();
        let mut ip_policy = String::new();

        // Check for the token in query params for api key and p_{random} for token
        for (k, v) in query_params {
            if k == "cookie_policy" {
                // tracing::debug!("Look for api_key query param: key: {}, Value: {:?}", k, v);
                cookie_policy = v.clone();
            } else if k == "ip_policy" {
                // tracing::debug!("Look for token query param: key: {}, Value: {:?}", k, v);
                ip_policy = v.clone();
            }
        }

        let mut json_values: Vec<serde_json::Value> = match json_value {
            serde_json::Value::Array(arr) => arr,
            _ => vec![json_value],
        };

        for json_value in json_values.iter_mut() {
            if let Some(obj) = json_value.as_object_mut() {
                extract_and_set_request_context(
                    obj,
                    &ip_addr,
                    &cookie_policy,
                    &ip_policy,
                    &user_agent,
                    &token.clone().unwrap_or_default(),
                );
            }
        }

        let payload_modified = serde_json::to_string(&json_values).unwrap();

        match serde_json::from_str::<Vec<Event>>(&payload_modified) {
            Ok(events) => Ok(events),
            Err(err) => Err(anyhow::Error::from(err)),
        }
    }
}

#[derive(Clone, Default, Debug, Deserialize, Serialize)]
pub struct ProcessedEvent {
    pub event: Event,
    pub event_id: Uuid,
    #[serde(default)]
    pub authorization: Option<AuthorizedCredential>,
    #[serde(default)]
    pub identity_provenance: EventIdentityProvenance,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct EventIdentityProvenance {
    pub identity_method: String,
    pub identity_trust: String,
    pub verified_at: Option<String>,
    pub verifier_version: Option<String>,
}

impl Default for EventIdentityProvenance {
    fn default() -> Self {
        Self {
            identity_method: "anonymous".to_string(),
            identity_trust: "untrusted".to_string(),
            verified_at: None,
            verifier_version: None,
        }
    }
}

impl ProcessedEvent {
    pub fn key(&self) -> String {
        let project_id = self
            .authorization
            .as_ref()
            .map(|credential| credential.workspace_id.as_str())
            .unwrap_or_default();
        format!("{}:{}", project_id, self.event_id)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::http::HeaderValue;
    use bytes::Bytes;
    use std::collections::HashMap;
    use std::sync::Arc;

    // Function to create common headers for test cases
    fn get_common_headers() -> HeaderMap {
        let mut headers = HeaderMap::new();
        headers.insert(
            "x-forwarded-for",
            HeaderValue::from_static("123.123.123.123"),
        );
        headers.insert("referer", HeaderValue::from_static("http://localhost"));
        headers.insert("user-agent", HeaderValue::from_static("test-agent"));
        headers
    }

    // Function to create common query params for test cases
    fn get_common_query_params() -> BTreeMap<String, String> {
        let mut query_params = BTreeMap::new();
        query_params.insert(
            "cookie_policy".to_string(),
            "cookies_policy_test".to_string(),
        );
        query_params.insert("ip_policy".to_string(), "ip_policy_test".to_string());
        query_params
    }

    #[test]
    fn test_from_bytes() {
        // Setup bytes
        let bytes = Bytes::from_static(b"{\"api_key\": \"key\", \"event_type\": \"type\", \"url\": \"http://localhost\", \"user\": {\"id\": \"user\"}}");

        let headers = get_common_headers();
        let query_params = get_common_query_params();

        let result =
            Event::from_bytes(bytes, headers, query_params, Some("key".to_string())).unwrap();
        assert_eq!(result.len(), 1);

        let event = &result[0];
        println!("Deserialized Event: {:?}", event);
        println!("Deserialized url: {:?}", event.url.as_deref());
        assert_eq!(event.api_key, "key");
        assert_eq!(event.event_type, "type");
        assert_eq!(event.url.as_deref(), Some("http://localhost"));
        assert_eq!(event.ip.clone().unwrap(), "123.123.123.123");
        assert_eq!(event.user_agent.clone().unwrap(), "test-agent");
        assert_eq!(event.cookie_policy.clone().unwrap(), "cookies_policy_test");
        assert_eq!(event.ip_policy.clone().unwrap(), "ip_policy_test");
        assert!(event.received_at.len() > 0);
    }
    #[test]
    fn test_from_bytes_with_user_agent_in_payload() {
        let bytes = Bytes::from_static(
        b"{\"api_key\": \"key\", \"event_type\": \"type\", \"url\": \"http://localhost\", \"user\": {\"id\": \"user\"}, \"user_agent\": \"payload-user-agent\"}",
    );

        let headers = get_common_headers();
        let query_params = get_common_query_params();

        let result =
            Event::from_bytes(bytes, headers, query_params, Some("key".to_string())).unwrap();

        // Additional assertions
        let event = &result[0];
        assert_eq!(event.user_agent.clone().unwrap(), "payload-user-agent");
    }

    #[test]
    fn test_from_bytes_with_source_ip_in_payload() {
        let bytes = Bytes::from_static(
        b"{\"api_key\": \"key\", \"event_type\": \"type\", \"url\": \"http://localhost\", \"user\": {\"id\": \"user\"}, \"source_ip\": \"payload-ip\"}",
    );

        let headers = get_common_headers();
        let query_params = get_common_query_params();

        let result =
            Event::from_bytes(bytes, headers, query_params, Some("key".to_string())).unwrap();

        // Additional assertions
        let event = &result[0];
        assert_eq!(event.ip.clone().unwrap(), "payload-ip");
    }

    #[test]
    fn test_from_bytes_with_url() {
        // Setup bytes with url field
        let bytes = Bytes::from_static(b"{\"api_key\": \"key\", \"event_type\": \"type\", \"url\": \"http://example.com\", \"user\": {\"id\": \"user\"}}");

        let headers = get_common_headers();
        let query_params = get_common_query_params();

        let result =
            Event::from_bytes(bytes, headers, query_params, Some("key".to_string())).unwrap();
        assert_eq!(result.len(), 1);

        let event = &result[0];
        assert_eq!(event.url.clone().unwrap(), "http://example.com");
    }

    #[test]
    fn test_from_bytes_with_multiple_events() {
        // Setup bytes with multiple events
        let bytes = Bytes::from_static(b"[{\"api_key\": \"key1\", \"event_type\": \"type1\", \"user\": {\"id\": \"user1\"}}, {\"api_key\": \"key2\", \"event_type\": \"type2\", \"user\": {\"id\": \"user2\"}}]");

        let headers = get_common_headers();
        let query_params = get_common_query_params();

        // token will be override
        let result =
            Event::from_bytes(bytes, headers, query_params, Some("key".to_string())).unwrap();
        print!("{:?}", result);
        assert_eq!(result.len(), 2);

        let event1 = &result[0];
        assert_eq!(event1.api_key, "key");
        assert_eq!(event1.event_type, "type1");

        let event2 = &result[1];
        assert_eq!(event2.api_key, "key");
        assert_eq!(event2.event_type, "type2");
    }

    #[test]
    fn test_from_bytes_with_token_override() {
        // Setup bytes with api_key field
        let bytes = Bytes::from_static(
            b"{\"api_key\": \"key\", \"event_type\": \"type\", \"user\": {\"id\": \"user\"}}",
        );

        let headers = get_common_headers();
        let query_params = get_common_query_params();

        let result =
            Event::from_bytes(bytes, headers, query_params, Some("token".to_string())).unwrap();
        assert_eq!(result.len(), 1);

        let event = &result[0];
        assert_eq!(event.api_key, "token");
    }

    // --- New tests for error handling ---

    #[test]
    fn test_from_bytes_invalid_json_returns_error() {
        let bytes = Bytes::from_static(b"this is not json at all");
        let headers = get_common_headers();
        let query_params = get_common_query_params();

        let result = Event::from_bytes(bytes, headers, query_params, None);
        assert!(result.is_err(), "Invalid JSON should return an error");
        let err_msg = result.unwrap_err().to_string();
        assert!(
            err_msg.contains("Failed to parse JSON"),
            "Error should mention JSON parsing, got: {}",
            err_msg
        );
    }

    #[test]
    fn test_from_bytes_empty_json_object_returns_error() {
        // Empty object missing required fields like api_key, event_type, user
        let bytes = Bytes::from_static(b"{}");
        let headers = get_common_headers();
        let query_params = get_common_query_params();

        let result = Event::from_bytes(bytes, headers, query_params, None);
        // Should error because required fields (api_key, event_type, user) are missing
        assert!(
            result.is_err(),
            "Empty JSON object should fail deserialization"
        );
    }

    #[test]
    fn test_from_bytes_missing_api_key_no_token_override() {
        // Missing api_key, no token to override — extract_and_set_request_context should handle gracefully
        let bytes = Bytes::from_static(b"{\"event_type\": \"type\", \"user\": {\"id\": \"user\"}}");
        let headers = get_common_headers();
        let query_params = get_common_query_params();

        // No token override — api_key field missing from payload
        let result = Event::from_bytes(bytes, headers, query_params, None);
        // Deserialization fails because api_key is a required field in Event struct
        assert!(
            result.is_err(),
            "Missing api_key should fail deserialization"
        );
    }

    #[test]
    fn test_from_bytes_with_no_headers() {
        // No x-forwarded-for, no user-agent — should use empty strings
        let bytes = Bytes::from_static(
            b"{\"api_key\": \"key\", \"event_type\": \"type\", \"user\": {\"id\": \"user\"}}",
        );
        let headers = HeaderMap::new(); // Empty headers
        let query_params = BTreeMap::new();

        let result = Event::from_bytes(bytes, headers, query_params, Some("key".to_string()));
        assert!(result.is_ok(), "Should handle missing headers gracefully");

        let event = &result.unwrap()[0];
        // IP comes from x-forwarded-for which is missing, so it should be empty
        assert_eq!(event.ip.as_deref(), Some(""));
        // User agent comes from header which is missing, so should be empty
        assert_eq!(event.user_agent.as_deref(), Some(""));
    }

    #[test]
    fn test_from_bytes_with_empty_token_does_not_override() {
        let bytes = Bytes::from_static(
            b"{\"api_key\": \"original_key\", \"event_type\": \"type\", \"user\": {\"id\": \"user\"}}",
        );
        let headers = get_common_headers();
        let query_params = get_common_query_params();

        // Empty token should not override api_key
        let result = Event::from_bytes(bytes, headers, query_params, Some("".to_string())).unwrap();
        let event = &result[0];
        assert_eq!(
            event.api_key, "original_key",
            "Empty token should not override api_key"
        );
    }

    #[test]
    fn test_from_bytes_with_none_token_does_not_override() {
        let bytes = Bytes::from_static(
            b"{\"api_key\": \"original_key\", \"event_type\": \"type\", \"user\": {\"id\": \"user\"}}",
        );
        let headers = get_common_headers();
        let query_params = get_common_query_params();

        let result = Event::from_bytes(bytes, headers, query_params, None).unwrap();
        let event = &result[0];
        assert_eq!(
            event.api_key, "original_key",
            "None token should not override api_key"
        );
    }

    #[test]
    fn test_from_bytes_invalid_utf8_returns_error() {
        // Invalid UTF-8 bytes
        let bytes = Bytes::from(vec![0xFF, 0xFE, 0xFD]);
        let headers = get_common_headers();
        let query_params = get_common_query_params();

        let result = Event::from_bytes(bytes, headers, query_params, None);
        // from_utf8 will lossy-convert but JSON parse will fail
        assert!(result.is_err(), "Invalid UTF-8 should fail");
    }

    #[test]
    fn test_from_bytes_empty_array() {
        let bytes = Bytes::from_static(b"[]");
        let headers = get_common_headers();
        let query_params = get_common_query_params();

        let result = Event::from_bytes(bytes, headers, query_params, None);
        // An empty array should successfully parse but return empty vec
        assert!(result.is_ok());
        assert_eq!(result.unwrap().len(), 0);
    }
}
