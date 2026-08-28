use chrono::Utc;
use serde_json::to_string;
use std::error::Error as StdError;
use std::error::Error;
use std::fmt;

use crate::auth::authorization::CredentialKind;
use crate::commercial_event_catalog_generated::SERVER_ONLY_COMPANY_FIELDS;
use crate::events::event::ProcessedEvent;
use crate::events::failed_event::FailedEvent;
use crate::events::transform_event::TransformedEvent;
use crate::ip2location::ip2proxy::IP2ProxyWrapper;
use crate::ip2location::resolver::IP2ProxyResolver;
use crate::pipeline::normalize_custom_timestamp;
use crate::{
    enrichment::privacy_enrichment::PrivacyEnrichmentService,
    geo::{maxmind::MaxMindResolver, resolver::GeoResolver},
};

use super::bot_resolver::{BotClassification, BotResolver};
use super::ua_resolver::UaResolver;

#[derive(Debug)]
pub struct MyError {
    pub failed_event: FailedEvent,
    pub description: String,
}

impl fmt::Display for MyError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.description)
    }
}

impl StdError for MyError {}

#[derive(Debug, Clone)]
pub struct EnrichmentHandler;

impl EnrichmentHandler {
    pub fn new() -> Self {
        EnrichmentHandler
    }
    fn value_to_string(v: &serde_json::Value) -> Option<String> {
        v.as_str()
            .map(ToString::to_string)
            .or_else(|| v.as_i64().map(|num| num.to_string()))
    }

    fn determine_classification(is_bot: bool, proxy_type: &str) -> i32 {
        if is_bot {
            return 1;
        }

        match proxy_type {
            "NOPROXY" => 0,
            "VPN" => 21,
            "TOR" => 22,
            "DCH" => 23,
            "PUB" => 24,
            "WEB" => 25,
            "SES" => 26,
            _ => 0, // Default to normal visitor if unknown proxy type
        }
    }

    pub async fn process_payload(
        &self,
        payload: &str,
        geo_resolver: Option<&GeoResolver>,
        ip2proxy_resolver: Option<&IP2ProxyResolver>,
        bot_resolver: &BotResolver,
        ua_parser: &UaResolver,
    ) -> Result<TransformedEvent, Box<dyn std::error::Error + Send>> {
        if payload.is_empty() {
            return Err(Box::new(std::io::Error::new(
                std::io::ErrorKind::InvalidInput,
                "Payload is empty",
            )));
        }
        let data: ProcessedEvent =
            serde_json::from_str(payload).map_err(|e| Box::new(e) as Box<dyn Error + Send>)?;
        self.process_event_inner(
            data,
            Some(payload),
            geo_resolver,
            ip2proxy_resolver,
            bot_resolver,
            ua_parser,
        )
        .await
    }

    pub async fn process_event(
        &self,
        data: ProcessedEvent,
        geo_resolver: Option<&GeoResolver>,
        ip2proxy_resolver: Option<&IP2ProxyResolver>,
        bot_resolver: &BotResolver,
        ua_parser: &UaResolver,
    ) -> Result<TransformedEvent, Box<dyn std::error::Error + Send>> {
        self.process_event_inner(
            data,
            None,
            geo_resolver,
            ip2proxy_resolver,
            bot_resolver,
            ua_parser,
        )
        .await
    }

    async fn process_event_inner(
        &self,
        data: ProcessedEvent,
        original_payload: Option<&str>,
        geo_resolver: Option<&GeoResolver>,
        ip2proxy_resolver: Option<&IP2ProxyResolver>,
        bot_resolver: &BotResolver,
        ua_parser: &UaResolver,
    ) -> Result<TransformedEvent, Box<dyn std::error::Error + Send>> {
        // Only rejection records need the original JSON. Avoid serializing every
        // successful inline event just to preserve a payload for uncommon failures.
        let rejection_payload =
            if data.event.event_type == "user_identify" || data.event.timestamp.is_some() {
                match original_payload {
                    Some(payload) => payload.to_string(),
                    None => serde_json::to_string(&data)
                        .map_err(|error| Box::new(error) as Box<dyn Error + Send>)?,
                }
            } else {
                String::new()
            };

        // enrich with geo data, privacy and default event values.

        let mut transformed_event: TransformedEvent = TransformedEvent::default();

        let mut service = PrivacyEnrichmentService::new(&data.event);
        let result = match service.enrich(geo_resolver) {
            Ok(r) => r,
            Err(e) => {
                tracing::warn!("Privacy enrichment failed: {:?}, using defaults", e);
                crate::enrichment::privacy_enrichment::IPGeoData {
                    ip: data.event.ip.clone().unwrap_or_default(),
                    anonymous_id: String::new(),
                    hashed_anonymous_id: format!(
                        "{:x}",
                        md5::compute(
                            data.event.ip.clone().unwrap_or_default()
                                + &data.event.user_agent.clone().unwrap_or_default()
                        )
                    ),
                }
            }
        };
        let location_data = geo_resolver
            .map(|resolver| MaxMindResolver::new(resolver).resolve(&result.ip))
            .transpose()
            .unwrap_or_else(|e| {
                tracing::warn!(
                    "Could not resolve location data for ip {:?}: {:?}, proceeding with empty geo data",
                    result.ip,
                    e
                );
                None
            })
            .unwrap_or_default();

        // check ip2proxy

        let ip2proxy_result = ip2proxy_resolver
            .map(|resolver| IP2ProxyWrapper::new(resolver).resolve(&result.ip))
            .transpose()
            .unwrap_or_else(|e| {
                tracing::warn!(
                    "Could not resolve IP2Proxy data for ip {:?}: {:?}, defaulting to NOPROXY",
                    result.ip,
                    e
                );
                None
            })
            .unwrap_or_default();
        let proxy_type = ip2proxy_result.proxy_type.as_deref().unwrap_or("NOPROXY");

        let bot_classification = data
            .event
            .user_agent
            .as_deref()
            .map(|user_agent| bot_resolver.classify(user_agent))
            .unwrap_or_else(BotClassification::missing_user_agent);
        let classification = Self::determine_classification(bot_classification.is_bot, proxy_type);
        if bot_classification.is_bot {
            let provider = if bot_classification.provider.is_empty() {
                "unknown".to_string()
            } else {
                bot_classification.provider.clone()
            };
            metrics::counter!(
                "capture_bot_classifications_total",
                1,
                "category" => bot_classification.category.clone(),
                "provider" => provider
            );
        }

        transformed_event.event_id = data.event_id.to_string();
        transformed_event.identity_method = data.identity_provenance.identity_method.clone();
        transformed_event.identity_trust = data.identity_provenance.identity_trust.clone();
        transformed_event.identity_verified_at = data.identity_provenance.verified_at.clone();
        transformed_event.identity_verifier_version =
            data.identity_provenance.verifier_version.clone();
        transformed_event.src = data.event.src.or_else(|| Some("usermaven".to_string()));
        // payload attributes
        transformed_event.project_id = data
            .authorization
            .as_ref()
            .map(|credential| credential.workspace_id.clone())
            .unwrap_or_else(|| {
                data.event
                    .api_key
                    .split('.')
                    .next()
                    .unwrap_or("")
                    .to_string()
            })
            .to_lowercase();
        transformed_event.api_key = data
            .authorization
            .as_ref()
            .map(|credential| credential.installation_id.clone())
            .unwrap_or_default();
        transformed_event.event_type = data.event.event_type.clone();
        transformed_event.utc_time = data.event.utc_time;
        transformed_event.local_tz_offset = Some(0);
        transformed_event.referer = data.event.referrer;
        transformed_event.url = data.event.url.clone();
        transformed_event.page_title = data.event.page_title;
        transformed_event.source_ip = result.ip;

        // If doc_host/doc_path/doc_search are missing, parse them from the url
        let (parsed_host, parsed_path, parsed_search) = data
            .event
            .url
            .as_deref()
            .and_then(|u| url::Url::parse(u).ok())
            .map(|parsed| {
                let host = parsed.host_str().map(|h| h.to_string());
                let path = Some(parsed.path().to_string());
                let query = parsed.query().map(|q| format!("?{}", q));
                (host, path, query)
            })
            .unwrap_or((None, None, None));

        transformed_event.doc_host = data.event.doc_host.or(parsed_host);
        transformed_event.doc_path = data.event.doc_path.or(parsed_path);
        transformed_event.doc_search = data.event.doc_search.or(parsed_search);
        transformed_event.screen_resolution = data
            .event
            .screen_resolution
            .or_else(|| Some("0".to_string()));
        transformed_event.user_agent = data.event.user_agent;
        transformed_event.user_language = data.event.user_language;
        transformed_event.doc_encoding = data.event.doc_encoding;
        transformed_event.vp_size = data.event.vp_size;

        // UA resolver using UAP.
        let default = "".to_string();
        let resolved_ua = ua_parser
            .resolve(transformed_event.user_agent.as_ref().unwrap_or(&default))
            .unwrap();

        transformed_event.parsed_ua_device_brand = resolved_ua.device_brand;
        transformed_event.parsed_ua_device_family = resolved_ua.device_family;
        transformed_event.parsed_ua_device_model = resolved_ua.device_model;
        transformed_event.parsed_ua_os_family = resolved_ua.os_family;
        transformed_event.parsed_ua_os_version = resolved_ua.os_version;
        transformed_event.parsed_ua_ua_family = resolved_ua.ua_family;
        transformed_event.parsed_ua_ua_version = resolved_ua.ua_version;

        // user and geo attributes

        transformed_event.user_hashed_anonymous_id = result.hashed_anonymous_id;
        transformed_event.user_anonymous_id = result.anonymous_id;
        transformed_event.location_continent = location_data.continent;
        transformed_event.location_country = location_data.country;
        transformed_event.location_city = location_data.city;
        transformed_event.location_country_name = location_data.country_name;
        transformed_event.location_region = location_data.region;
        transformed_event.location_region_name = location_data.region_name;
        transformed_event.location_lat = location_data.lat;
        transformed_event.location_lon = location_data.lon;
        transformed_event.location_zip = location_data.zip;
        transformed_event.timestamp = data.event.received_at.clone();
        transformed_event.event_received_at = data.event.received_at.clone();
        transformed_event.parsed_ua_bot = classification;
        transformed_event.parsed_ua_bot_category = bot_classification.category;
        transformed_event.parsed_ua_bot_name = bot_classification.name;
        transformed_event.parsed_ua_bot_provider = bot_classification.provider;

        // Check if the API key is a server-side token
        let is_server_side_token = data
            .authorization
            .as_ref()
            .map(|credential| credential.credential_kind == CredentialKind::Server)
            .unwrap_or_else(|| data.event.api_key.contains('.'));

        // Look for the timestamp in the event data
        let event_timestamp = if is_server_side_token {
            if let Some(custom_timestamp) = data.event.timestamp {
                normalize_custom_timestamp(custom_timestamp, Utc::now()).map_err(|error| {
                    let description = error.to_string();
                    Box::new(MyError {
                        failed_event: FailedEvent {
                            eventn_ctx_event_id: data.event_id.to_string(),
                            project_id: transformed_event.project_id.clone(),
                            error_description: description.clone(),
                            error: 5,
                            payload: rejection_payload.clone(),
                        },
                        description,
                    }) as Box<dyn Error + Send>
                })?
            } else {
                data.event.received_at.clone()
            }
        } else {
            data.event.received_at.clone()
        };
        transformed_event.timestamp = event_timestamp;

        // User object transformations

        let user = &data.event.user;
        let default_json = serde_json::json!({});
        let user_custom = user.get("custom").unwrap_or(&default_json);
        let user_custom_str = match user_custom {
            serde_json::Value::Object(obj) if obj.is_empty() => "{}".to_string(),
            _ => to_string(user_custom).unwrap_or_else(|e| {
                eprintln!("Failed to serialize custom user data: {}", e);
                "".to_string()
            }),
        };
        transformed_event.user_custom = user_custom_str;

        if let Some(created_at) = user.get("created_at").and_then(|v| v.as_str()) {
            transformed_event.user_created_at = Some(created_at.to_string());
        }
        if let Some(email) = user.get("email").and_then(|v| v.as_str()) {
            transformed_event.user_email = Some(email.to_string());
        }
        if let Some(first_name) = user.get("first_name").and_then(|v| v.as_str()) {
            transformed_event.user_first_name = Some(first_name.to_string());
        }
        if let Some(last_name) = user.get("last_name").and_then(|v| v.as_str()) {
            transformed_event.user_last_name = Some(last_name.to_string());
        }
        if let Some(user_id) = user.get("id").and_then(Self::value_to_string) {
            transformed_event.user_id = Some(user_id.to_string());
        }

        // Company object transformations

        if let Some(company) = &data.event.company {
            let mut company_custom = company
                .get("custom")
                .cloned()
                .unwrap_or(default_json.clone());
            if !company_custom.is_object() {
                company_custom = serde_json::json!({});
            }
            if let Some(custom) = company_custom.as_object_mut() {
                for field in SERVER_ONLY_COMPANY_FIELDS {
                    if let Some(value) = company.get(*field) {
                        custom.insert((*field).to_string(), value.clone());
                    }
                }
            }
            let company_custom_str = match &company_custom {
                serde_json::Value::Object(obj) if obj.is_empty() => "{}".to_string(),
                _ => to_string(&company_custom).unwrap_or_else(|e| {
                    eprintln!("Failed to serialize custom company data: {}", e);
                    "".to_string()
                }),
            };
            transformed_event.company_custom = company_custom_str;

            if let Some(created_at) = company.get("created_at").and_then(|v| v.as_str()) {
                transformed_event.company_created_at = Some(created_at.to_string());
            }
            if let Some(company_id) = company.get("id").and_then(Self::value_to_string) {
                transformed_event.company_id = Some(company_id.to_string());
            }
            if let Some(name) = company.get("name").and_then(|v| v.as_str()) {
                transformed_event.company_name = Some(name.to_string());
            }
        }

        // Event attributes

        let event_attributes_str = if let Some(event_attributes) = &data.event.event_attributes {
            if event_attributes.is_empty() {
                "{}".to_string()
            } else {
                match to_string(event_attributes) {
                    Ok(event_attributes_str) => event_attributes_str,
                    Err(e) => {
                        eprintln!("Failed to serialize event attributes: {}", e);
                        "".to_string()
                    }
                }
            }
        } else {
            "{}".to_string()
        };

        transformed_event.event_attributes = event_attributes_str;

        let autocapture_attributes_str =
            if let Some(autocapture_attributes) = &data.event.autocapture_attributes {
                if autocapture_attributes.is_empty() {
                    "{}".to_string()
                } else {
                    match to_string(autocapture_attributes) {
                        Ok(autocapture_attributes_str) => autocapture_attributes_str,
                        Err(e) => {
                            eprintln!("Failed to serialize autocapture attributes: {}", e);
                            "".to_string()
                        }
                    }
                }
            } else {
                "{}".to_string()
            };

        transformed_event.autocapture_attributes = autocapture_attributes_str;

        // utms

        if let Some(utm) = &data.event.utm {
            if let Some(campaign) = utm.get("campaign").and_then(Self::value_to_string) {
                transformed_event.utm_campaign = Some(campaign.to_string());
            }
            if let Some(content) = utm.get("content").and_then(Self::value_to_string) {
                transformed_event.utm_content = Some(content.to_string());
            }
            if let Some(medium) = utm.get("medium").and_then(Self::value_to_string) {
                transformed_event.utm_medium = Some(medium.to_string());
            }
            if let Some(source) = utm.get("source").and_then(Self::value_to_string) {
                transformed_event.utm_source = Some(source.to_string());
            }
            if let Some(term) = utm.get("term").and_then(Self::value_to_string) {
                transformed_event.utm_term = Some(term.to_string());
            }
        }

        // ids

        if let Some(ids) = &data.event.ids {
            if let Some(ajs_anonymous_id) =
                ids.get("ajs_anonymous_id").and_then(Self::value_to_string)
            {
                transformed_event.ids_ajs_anonymous_id = Some(ajs_anonymous_id.to_string());
            }

            if let Some(ajs_user_id) = ids.get("ajs_user_id").and_then(Self::value_to_string) {
                transformed_event.ids_ajs_user_id = Some(ajs_user_id.to_string());
            }
            if let Some(fbp) = ids.get("fbp").and_then(Self::value_to_string) {
                transformed_event.ids_fbp = Some(fbp.to_string());
            }
            if let Some(ga) = ids.get("ga").and_then(Self::value_to_string) {
                transformed_event.ids_ga = Some(ga.to_string());
            }
        }

        // clicks

        if let Some(click_id) = &data.event.click_id {
            if let Some(fbclid) = click_id.get("fbclid").and_then(Self::value_to_string) {
                transformed_event.click_id_fbclid = Some(fbclid.to_string());
            }
            if let Some(gclid) = click_id.get("gclid").and_then(Self::value_to_string) {
                transformed_event.click_id_gclid = Some(gclid.to_string());
            }
        }

        if &data.event.event_type == "user_identify" {
            // Check if user.id exists
            if data.event.user.get("id").is_none() {
                let mut failed = FailedEvent::default();
                failed.eventn_ctx_event_id = data.event_id.to_string();
                failed.project_id = transformed_event.project_id;
                failed.error_description = "user.id required".to_string();
                failed.error = 1;

                failed.payload = rejection_payload.clone();

                return Err(Box::new(MyError {
                    failed_event: failed.clone(),
                    description: failed.error_description,
                }));
            }

            // Check if company exists and is not empty
            if let Some(company) = &data.event.company {
                // Skip validation if the company object is empty (no fields)
                let is_empty_object = company.is_empty();

                // Check if company.id exists when company is provided and not empty
                if !is_empty_object && company.get("id").is_none() {
                    let mut failed = FailedEvent::default();
                    failed.eventn_ctx_event_id = data.event_id.to_string();
                    failed.project_id = transformed_event.project_id;
                    failed.error_description = "company.id required".to_string();
                    failed.error = 3;
                    failed.payload = rejection_payload.clone();
                    return Err(Box::new(MyError {
                        failed_event: failed.clone(),
                        description: failed.error_description,
                    }));
                }
                // Check if company.name exists
                // else if company.get("name").is_none() {
                //     data.error = Some(4);
                //     data.error_description = Some("company.name required".to_string());
                // }
            }
        }

        Ok(transformed_event)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_event_enrichment() {
        // Create necessary dependencies
        let geo_resolver: Option<GeoResolver> = None;
        let ip2proxy_resolver: Option<IP2ProxyResolver> = None;
        let bot_resolver = BotResolver::new();
        let ua_parser = UaResolver::new();
        ua_parser.seed_to_lru_cache().unwrap();
        let handler = EnrichmentHandler::new();

        // Define test cases
        let test_cases = vec![
            // Test case 1: Valid event
            (
                r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"first_space_activity","referrer":"","url":"https://app.futy.nl","user_agent":"curl/8.1.2","user":{"created_at":"2023-07-11 14:36:23","anonymous_id":"lebbuqvhk","first_name":"Camylle","id":"jk9bE4Y13xvL8vJL8d5waOr6","last_name":"Shields","email":"tess30@example.com"},"company":{"id":"xWVJR6maYAyprg94P7E1L8qp","custom":{"activity_status":"active","last_activity_at":null,"on_trial":false,"plan":"standard"},"created_at":"2023-07-11 14:36:23","name":"Rippin and Sons"},"event_attributes":{"space_id":"xWVJR6maYAyprg94P7E1L8qp","active_domain":"test.com","space_code":"64ad4cc733099","space_name":"In perferendis"},"ip":"95.10.187.240","received_at":"2024-05-10T09:08:20.443126000Z","src":"futy-api"},"event_id":"9b8faa58-1ef4-44b4-879f-21813cc6e75e"}"#,
                true,
            ),
            // Test case 2: Invalid event - missing user.id
            (
                r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"user_identify","referrer":"","url":"https://app.futy.nl","user_agent":"curl/8.1.2","user":{"created_at":"2023-07-11 14:36:23","anonymous_id":"lebbuqvhk","first_name":"Camylle","last_name":"Shields","email":"tess30@example.com"},"company":{"id":"xWVJR6maYAyprg94P7E1L8qp","custom":{"activity_status":"active","last_activity_at":null,"on_trial":false,"plan":"standard"},"created_at":"2023-07-11 14:36:23","name":"Rippin and Sons"},"event_attributes":{"space_id":"xWVJR6maYAyprg94P7E1L8qp","active_domain":"test.com","space_code":"64ad4cc733099","space_name":"In perferendis"},"ip":"95.10.187.240","received_at":"2024-05-10T09:08:20.443126000Z","src":"futy-api"},"event_id":"9b8faa58-1ef4-44b4-879f-21813cc6e75e"}"#,
                false,
            ),
            // Test case 3: Valid event - with utm parameters
            (
                r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"page_view","referrer":"","url":"https://app.futy.nl","user_agent":"curl/8.1.2","user":{"created_at":"2023-07-11 14:36:23","anonymous_id":"lebbuqvhk","first_name":"Camylle","id":"jk9bE4Y13xvL8vJL8d5waOr6","last_name":"Shields","email":"tess30@example.com"},"company":{"id":"xWVJR6maYAyprg94P7E1L8qp","custom":{"activity_status":"active","last_activity_at":null,"on_trial":false,"plan":"standard"},"created_at":"2023-07-11 14:36:23","name":"Rippin and Sons"},"utm":{"campaign":"summer_sale","source":"newsletter","medium":"email","term":"discount","content":"image_link"},"ip":"95.10.187.240","received_at":"2024-05-10T09:08:20.443126000Z","src":"futy-api"},"event_id":"9b8faa58-1ef4-44b4-879f-21813cc6e75e"}"#,
                true,
            ),
            // Test case 4: Invalid event - missing company.id for user_identify event
            (
                r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"user_identify","referrer":"","url":"https://app.futy.nl","user_agent":"curl/8.1.2","user":{"created_at":"2023-07-11 14:36:23","anonymous_id":"lebbuqvhk","first_name":"Camylle","id":"jk9bE4Y13xvL8vJL8d5waOr6","last_name":"Shields","email":"tess30@example.com"},"company":{"custom":{"activity_status":"active","last_activity_at":null,"on_trial":false,"plan":"standard"},"created_at":"2023-07-11 14:36:23","name":"Rippin and Sons"},"ip":"95.10.187.240","received_at":"2024-05-10T09:08:20.443126000Z","src":"futy-api", "timestamp": 1720253671},"event_id":"9b8faa58-1ef4-44b4-879f-21813cc6e75e"}"#,
                false,
            ),
            // Test case 5: Valid event - user_identify with empty company object
            (
                r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"user_identify","referrer":"","url":"https://app.futy.nl","user_agent":"curl/8.1.2","user":{"created_at":"2023-07-11 14:36:23","anonymous_id":"lebbuqvhk","first_name":"Camylle","id":"jk9bE4Y13xvL8vJL8d5waOr6","last_name":"Shields","email":"tess30@example.com"},"company":{},"ip":"95.10.187.240","received_at":"2024-05-10T09:08:20.443126000Z","src":"futy-api"},"event_id":"9b8faa58-1ef4-44b4-879f-21813cc6e75e"}"#,
                true,
            ),
            // Add more test cases as needed
        ];

        // Run test cases

        for (payload, expected_result) in test_cases {
            let result = handler
                .process_payload(
                    payload,
                    geo_resolver.as_ref(),
                    ip2proxy_resolver.as_ref(),
                    &bot_resolver,
                    &ua_parser,
                )
                .await;

            match (result, expected_result) {
                (Ok(transformed_event), true) => {
                    // println!("Test case passed");
                    println!("{:?}", transformed_event.timestamp);
                    // Check the transformed event
                    // println!("{:?}", transformed_event);

                    assert_eq!(
                        transformed_event.event_id,
                        "9b8faa58-1ef4-44b4-879f-21813cc6e75e"
                    );
                    assert_eq!(transformed_event.project_id, "umywi4ukqf");
                    assert_eq!(transformed_event.api_key, "");

                    // Check utm parameters
                    if transformed_event.event_type == "page_view" {
                        assert_eq!(
                            transformed_event.utm_campaign,
                            Some("summer_sale".to_string())
                        );
                        assert_eq!(transformed_event.utm_source, Some("newsletter".to_string()));
                        assert_eq!(transformed_event.utm_medium, Some("email".to_string()));
                        assert_eq!(transformed_event.utm_term, Some("discount".to_string()));
                        assert_eq!(
                            transformed_event.utm_content,
                            Some("image_link".to_string())
                        );
                    }
                }
                (Err(_), false) => println!("Test case passed"),
                _ => panic!("Test case failed"),
            }
        }
    }

    /// Tests that events with IPs not found in MaxMind/IP2Proxy databases
    /// are still processed successfully with empty geo data instead of being dropped.
    #[tokio::test]
    async fn test_event_with_unknown_ip_succeeds() {
        let geo_resolver: Option<GeoResolver> = None;
        let ip2proxy_resolver: Option<IP2ProxyResolver> = None;
        let bot_resolver = BotResolver::new();
        let ua_parser = UaResolver::new();
        ua_parser.seed_to_lru_cache().unwrap();
        let handler = EnrichmentHandler::new();

        // Use a private IP that won't be in any GeoIP database
        let payload = r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"page_view","referrer":"","url":"https://example.com","user_agent":"Mozilla/5.0","user":{"anonymous_id":"test123","id":"user1"},"ip":"192.168.1.1","received_at":"2024-05-10T09:08:20.443126000Z","src":"test"},"event_id":"a1b2c3d4-e5f6-7890-abcd-ef1234567890"}"#;

        let result = handler
            .process_payload(
                payload,
                geo_resolver.as_ref(),
                ip2proxy_resolver.as_ref(),
                &bot_resolver,
                &ua_parser,
            )
            .await;
        assert!(
            result.is_ok(),
            "Event with unknown IP should not be dropped, got: {:?}",
            result.err()
        );

        let event = result.unwrap();
        assert_eq!(event.event_id, "a1b2c3d4-e5f6-7890-abcd-ef1234567890");
        assert_eq!(event.project_id, "umywi4ukqf");
        // Geo fields should be None (graceful fallback)
        assert!(
            event.location_country.is_none(),
            "Country should be None for unknown IP"
        );
        assert!(
            event.location_city.is_none(),
            "City should be None for unknown IP"
        );
        assert!(
            event.location_lat.is_none(),
            "Lat should be None for unknown IP"
        );
        assert!(
            event.location_lon.is_none(),
            "Lon should be None for unknown IP"
        );
    }

    /// Tests with the exact IP that was causing production event drops.
    #[tokio::test]
    async fn test_event_with_production_failing_ip() {
        let geo_resolver: Option<GeoResolver> = None;
        let ip2proxy_resolver: Option<IP2ProxyResolver> = None;
        let bot_resolver = BotResolver::new();
        let ua_parser = UaResolver::new();
        ua_parser.seed_to_lru_cache().unwrap();
        let handler = EnrichmentHandler::new();

        // IP 102.204.88.22 was causing "AddressNotFoundError" in production
        let payload = r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"page_view","referrer":"","url":"https://example.com","user_agent":"Mozilla/5.0","user":{"anonymous_id":"test456","id":"user2"},"ip":"102.204.88.22","received_at":"2024-05-10T09:08:20.443126000Z","src":"test"},"event_id":"b2c3d4e5-f6a7-8901-bcde-f12345678901"}"#;

        let result = handler
            .process_payload(
                payload,
                geo_resolver.as_ref(),
                ip2proxy_resolver.as_ref(),
                &bot_resolver,
                &ua_parser,
            )
            .await;
        assert!(
            result.is_ok(),
            "Event with IP 102.204.88.22 should not be dropped, got: {:?}",
            result.err()
        );

        let event = result.unwrap();
        assert_eq!(event.event_id, "b2c3d4e5-f6a7-8901-bcde-f12345678901");
        // Event should be fully processed even if geo data is empty
        assert_eq!(event.event_type, "page_view");
        assert!(!event.source_ip.is_empty(), "Source IP should still be set");
    }

    /// Tests that events with cookie_policy="comply" and an unknown IP
    /// don't panic during EU compliance check.
    #[tokio::test]
    async fn test_event_with_cookie_comply_and_unknown_ip() {
        let geo_resolver: Option<GeoResolver> = None;
        let ip2proxy_resolver: Option<IP2ProxyResolver> = None;
        let bot_resolver = BotResolver::new();
        let ua_parser = UaResolver::new();
        ua_parser.seed_to_lru_cache().unwrap();
        let handler = EnrichmentHandler::new();

        // Event with cookie_policy=comply and a private IP
        let payload = r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"page_view","referrer":"","url":"https://example.com","user_agent":"Mozilla/5.0","user":{"anonymous_id":"test789","id":"user3"},"cookie_policy":"comply","ip_policy":"comply","ip":"192.168.1.1","received_at":"2024-05-10T09:08:20.443126000Z","src":"test"},"event_id":"c3d4e5f6-a7b8-9012-cdef-123456789012"}"#;

        let result = handler
            .process_payload(
                payload,
                geo_resolver.as_ref(),
                ip2proxy_resolver.as_ref(),
                &bot_resolver,
                &ua_parser,
            )
            .await;
        assert!(
            result.is_ok(),
            "Event with comply policy and unknown IP should not fail, got: {:?}",
            result.err()
        );
    }

    /// Tests that doc_host, doc_path, and doc_search are parsed from the url
    /// when not explicitly provided (server-side events).
    #[tokio::test]
    async fn test_server_side_event_parses_url_fields() {
        let geo_resolver: Option<GeoResolver> = None;
        let ip2proxy_resolver: Option<IP2ProxyResolver> = None;
        let bot_resolver = BotResolver::new();
        let ua_parser = UaResolver::new();
        ua_parser.seed_to_lru_cache().unwrap();
        let handler = EnrichmentHandler::new();

        // Server-side event: has url but no doc_host/doc_path/doc_search
        let payload = r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"page_view","referrer":"","url":"https://app.example.com/dashboard/analytics?period=30d&filter=active","user_agent":"Mozilla/5.0","user":{"anonymous_id":"srv123","id":"user1"},"ip":"95.10.187.240","received_at":"2024-05-10T09:08:20.443126000Z","src":"usermaven-server"},"event_id":"d4e5f6a7-b8c9-0123-def0-234567890123"}"#;

        let result = handler
            .process_payload(
                payload,
                geo_resolver.as_ref(),
                ip2proxy_resolver.as_ref(),
                &bot_resolver,
                &ua_parser,
            )
            .await;
        assert!(
            result.is_ok(),
            "Server-side event should succeed, got: {:?}",
            result.err()
        );

        let event = result.unwrap();
        assert_eq!(event.doc_host, Some("app.example.com".to_string()));
        assert_eq!(event.doc_path, Some("/dashboard/analytics".to_string()));
        assert_eq!(
            event.doc_search,
            Some("?period=30d&filter=active".to_string())
        );
    }

    /// Tests that explicitly provided doc_host/doc_path/doc_search are NOT overridden
    /// by URL parsing (client-side events).
    #[tokio::test]
    async fn test_client_side_event_preserves_explicit_doc_fields() {
        let geo_resolver: Option<GeoResolver> = None;
        let ip2proxy_resolver: Option<IP2ProxyResolver> = None;
        let bot_resolver = BotResolver::new();
        let ua_parser = UaResolver::new();
        ua_parser.seed_to_lru_cache().unwrap();
        let handler = EnrichmentHandler::new();

        // Client-side event: has explicit doc_host/doc_path/doc_search
        let payload = r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"page_view","referrer":"","url":"https://app.example.com/page","doc_host":"custom-host.com","doc_path":"/custom-path","doc_search":"?custom=true","user_agent":"Mozilla/5.0","user":{"anonymous_id":"cli123","id":"user2"},"ip":"95.10.187.240","received_at":"2024-05-10T09:08:20.443126000Z","src":"usermaven"},"event_id":"e5f6a7b8-c9d0-1234-ef01-345678901234"}"#;

        let result = handler
            .process_payload(
                payload,
                geo_resolver.as_ref(),
                ip2proxy_resolver.as_ref(),
                &bot_resolver,
                &ua_parser,
            )
            .await;
        assert!(
            result.is_ok(),
            "Client-side event should succeed, got: {:?}",
            result.err()
        );

        let event = result.unwrap();
        // Explicit values should be preserved, NOT overridden by URL parsing
        assert_eq!(event.doc_host, Some("custom-host.com".to_string()));
        assert_eq!(event.doc_path, Some("/custom-path".to_string()));
        assert_eq!(event.doc_search, Some("?custom=true".to_string()));
    }

    /// Tests URL parsing with various URL formats.
    #[tokio::test]
    async fn test_url_parsing_edge_cases() {
        let geo_resolver: Option<GeoResolver> = None;
        let ip2proxy_resolver: Option<IP2ProxyResolver> = None;
        let bot_resolver = BotResolver::new();
        let ua_parser = UaResolver::new();
        ua_parser.seed_to_lru_cache().unwrap();
        let handler = EnrichmentHandler::new();

        // URL with no path or query
        let payload = r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"page_view","referrer":"","url":"https://example.com","user_agent":"Mozilla/5.0","user":{"anonymous_id":"edge1","id":"user3"},"ip":"95.10.187.240","received_at":"2024-05-10T09:08:20.443126000Z","src":"test"},"event_id":"f6a7b8c9-d0e1-2345-f012-456789012345"}"#;

        let result = handler
            .process_payload(
                payload,
                geo_resolver.as_ref(),
                ip2proxy_resolver.as_ref(),
                &bot_resolver,
                &ua_parser,
            )
            .await;
        assert!(result.is_ok());

        let event = result.unwrap();
        assert_eq!(event.doc_host, Some("example.com".to_string()));
        assert_eq!(event.doc_path, Some("/".to_string()));
        assert!(
            event.doc_search.is_none(),
            "No query string means doc_search should be None"
        );
    }

    #[tokio::test]
    async fn test_ai_user_fetcher_enrichment() {
        let bot_resolver = BotResolver::new();
        let ua_parser = UaResolver::new();
        ua_parser.seed_to_lru_cache().unwrap();
        let handler = EnrichmentHandler::new();
        let payload = r#"{"event":{"api_key":"UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7","event_type":"page_view","url":"https://example.com","user_agent":"Mozilla/5.0 (compatible; ChatGPT-User/1.0; +https://openai.com/bot)","user":{"anonymous_id":"ai-fetcher"},"ip":"192.168.1.1","received_at":"2024-05-10T09:08:20.443126000Z","src":"usermaven"},"event_id":"a7a7a7a7-a7a7-47a7-a7a7-a7a7a7a7a7a7"}"#;

        let event = handler
            .process_payload(payload, None, None, &bot_resolver, &ua_parser)
            .await
            .expect("AI user fetcher event should be enriched");

        assert_eq!(event.parsed_ua_bot, 1);
        assert_eq!(event.parsed_ua_bot_category, "ai_user_fetcher");
        assert_eq!(event.parsed_ua_bot_provider, "openai");
        assert_eq!(event.parsed_ua_bot_name, "ChatGPT-User");
    }
}
