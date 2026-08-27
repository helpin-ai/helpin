use serde::{Deserialize, Serialize};

#[derive(Default, Debug, Clone, Serialize, Deserialize)]
#[serde(default)]
pub struct TransformedEvent {
    pub timestamp: String,
    pub event_received_at: String,
    pub visitor_shard: u8,
    pub _is_deleted: u32,
    pub api_key: String,
    pub autocapture_attributes: String,
    pub click_id_fbclid: Option<String>,
    pub click_id_gclid: Option<String>,
    pub company_created_at: Option<String>,
    pub company_custom: String,
    pub company_id: Option<String>,
    pub company_name: Option<String>,
    pub doc_encoding: Option<String>,
    pub doc_host: Option<String>,
    pub doc_path: Option<String>,
    pub doc_search: Option<String>,
    pub event_attributes: String,
    pub event_id: String,
    pub event_type: String,
    pub ids_ajs_anonymous_id: Option<String>,
    pub ids_ajs_user_id: Option<String>,
    pub ids_fbp: Option<String>,
    pub ids_ga: Option<String>,
    pub identity_method: String,
    pub identity_trust: String,
    pub identity_verified_at: Option<String>,
    pub identity_verifier_version: Option<String>,
    pub local_tz_offset: Option<i64>,
    pub location_city: Option<String>,
    pub location_continent: Option<String>,
    pub location_country: Option<String>,
    pub location_country_name: Option<String>,
    pub location_region: Option<String>,
    pub location_region_name: Option<String>,
    pub location_zip: Option<String>,
    pub location_lat: Option<f64>,
    pub location_lon: Option<f64>,
    pub page_title: Option<String>,
    pub parsed_ua_bot: i32,
    pub parsed_ua_bot_category: String,
    pub parsed_ua_bot_name: String,
    pub parsed_ua_bot_provider: String,
    pub parsed_ua_device_brand: Option<String>,
    pub parsed_ua_device_family: Option<String>,
    pub parsed_ua_device_model: Option<String>,
    pub parsed_ua_os_family: Option<String>,
    pub parsed_ua_os_version: Option<String>,
    pub parsed_ua_ua_family: Option<String>,
    pub parsed_ua_ua_version: Option<String>,
    pub project_id: String,
    pub referer: Option<String>,
    pub screen_resolution: Option<String>,
    pub session_id: Option<String>,
    pub source_ip: String,
    pub src: Option<String>,
    pub url: Option<String>,
    pub user_agent: Option<String>,
    pub user_anonymous_id: String,
    pub user_created_at: Option<String>,
    pub user_custom: String,
    pub user_email: Option<String>,
    pub user_first_name: Option<String>,
    pub user_hashed_anonymous_id: String,
    pub user_id: Option<String>,
    pub user_language: Option<String>,
    pub user_last_name: Option<String>,
    pub utc_time: Option<String>,
    pub utm_campaign: Option<String>,
    pub utm_content: Option<String>,
    pub utm_medium: Option<String>,
    pub utm_source: Option<String>,
    pub utm_term: Option<String>,
    pub vp_size: Option<String>,
}

impl TransformedEvent {
    pub fn key(&self) -> String {
        format!("{}", self.event_id)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json;

    #[test]
    fn test_transformed_event_default() {
        // Test that the default TransformedEvent has expected values
        let event = TransformedEvent::default();

        assert_eq!(event.timestamp, "");
        assert_eq!(event.event_received_at, "");
        assert_eq!(event.visitor_shard, 0);
        assert_eq!(event._is_deleted, 0);
        assert_eq!(event.api_key, "");
        assert_eq!(event.parsed_ua_bot_category, "");
        assert_eq!(event.parsed_ua_bot_name, "");
        assert_eq!(event.parsed_ua_bot_provider, "");
        // Repeat for all fields in your struct
    }

    #[test]
    fn test_transformed_event_key() {
        // Test that the key method returns expected value
        let mut event = TransformedEvent::default();
        event.event_id = "12345".to_string();

        assert_eq!(event.key(), "12345");
    }
    #[test]
    fn test_transformed_event_deserialization() {
        // Test that a TransformedEvent can be correctly deserialized from JSON
        let json = r#"{
        "timestamp": "2023-07-15T10:00:00Z",
        "_is_deleted": 0,
        "api_key": "test_api_key",
        "autocapture_attributes": "",
        "event_attributes": "",
        "event_id": "event_id",
        "event_type": "event_type",
        "project_id": "project_id",
        "source_ip": "source_ip",
        "user_anonymous_id": "anonymous_id",
        "user_custom": "{}",
        "company_custom":"{}",
        "parsed_ua_bot": 0,
        "user_hashed_anonymous_id": "hashed_id"
    }"#;

        let event: TransformedEvent = serde_json::from_str(json).unwrap();

        assert_eq!(event.timestamp, "2023-07-15T10:00:00Z");
        assert_eq!(event._is_deleted, 0);
        assert_eq!(event.api_key, "test_api_key");
        assert_eq!(event.parsed_ua_bot_category, "");
        // Add similar assertions for the rest of your fields
    }
}
