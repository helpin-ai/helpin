use maxminddb::geoip2;

// Performs an enrichment to the event i.e user anonymous id, session id, and more.
const COMPLY_VALUE: &str = "comply";
const KEEP_VALUE: &str = "keep";
const STRICT_VALUE: &str = "strict";

use crate::{events::event::Event, geo::resolver::GeoResolver};

enum CookiePolicy {
    Comply,
    Keep,
    Strict,
}

enum IPPolicy {
    Comply,
    Keep,
    Strict,
}

pub struct PrivacyEnrichmentService {
    event: Event,
}

#[derive(Default, Debug, Clone)]
pub struct IPGeoData {
    pub anonymous_id: String,
    pub hashed_anonymous_id: String,
    pub ip: String,
}

impl PrivacyEnrichmentService {
    pub fn new(event: Event) -> Self {
        Self { event }
    }

    fn comply_with_cookie_laws(&self, geo_resolver: &GeoResolver, client_ip: &str) -> bool {
        // Look up the IP address in the database
        let country = match geo_resolver.lookup_country(client_ip) {
            Ok(c) => c,
            Err(e) => {
                tracing::warn!("Could not look up country for ip {:?}: {:?}, defaulting to non-EU", client_ip, e);
                return false;
            }
        };
        // Check if the country is part of the EU
        match country.country {
            Some(country) => match country.is_in_european_union {
                Some(is_in_eu) => is_in_eu,
                None => false,
            },
            None => false,
        }
    }

    fn get_three_octets(&self, client_ip: &str) -> String {
        let mut ip_parts: Vec<&str> = client_ip.split('.').collect();
        if let Some(last) = ip_parts.last_mut() {
            *last = "1";
        }
        ip_parts.join(".")
    }

    pub fn enrich(
        &mut self,
        geo_resolver: &GeoResolver,
    ) -> Result<IPGeoData, Box<dyn std::error::Error>> {
        let mut cookies_law_compliant = true;
        let mut compliant: Option<bool> = None;
        let hashed_anonymous_id = format!(
            "{:x}",
            md5::compute(
                self.event.ip.clone().unwrap_or_default()
                    + &self.event.user_agent.clone().unwrap_or_default()
            )
        );
        let mut data = IPGeoData::default();

        data.hashed_anonymous_id = hashed_anonymous_id.clone();

        if let Some(anonymous_id) = self.event.user.get("anonymous_id").and_then(|v| v.as_str()) {
            data.anonymous_id = anonymous_id.to_string();
        }

        // If anonymous_id is not set, use hashed_anonymous_id

        if data.anonymous_id.is_empty() {
            data.anonymous_id = hashed_anonymous_id.clone();
        }

        data.ip = self.event.ip.clone().unwrap();
        if let Some(cookie_policy_str) = &self.event.cookie_policy {
            let cookie_policy = match cookie_policy_str.as_str() {
                COMPLY_VALUE => CookiePolicy::Comply,
                KEEP_VALUE => CookiePolicy::Keep,
                STRICT_VALUE => CookiePolicy::Strict,
                _ => {
                    tracing::debug!("Unknown value {} for cookie_policy", cookie_policy_str); // use your logging function here
                    return Ok(data);
                }
            };

            match cookie_policy {
                CookiePolicy::Comply => {
                    let value = self.comply_with_cookie_laws(
                        geo_resolver,
                        self.event.ip.as_deref().unwrap_or(""),
                    );
                    compliant = Some(value);
                    cookies_law_compliant = value;
                }
                CookiePolicy::Keep => {
                    cookies_law_compliant = true;
                }
                CookiePolicy::Strict => {
                    cookies_law_compliant = false;
                }
            }
        }

        if !cookies_law_compliant {
            data.anonymous_id = hashed_anonymous_id;
        }

        if let Some(ip_policy_str) = &self.event.ip_policy {
            let ip_policy = match ip_policy_str.as_str() {
                COMPLY_VALUE => IPPolicy::Comply,
                KEEP_VALUE => IPPolicy::Keep,
                STRICT_VALUE => IPPolicy::Strict,
                _ => {
                    tracing::debug!("Unknown value {} for ip_policy", ip_policy_str); // use your logging function here
                    return Ok(data);
                }
            };
            match ip_policy {
                IPPolicy::Comply => {
                    if compliant.is_none() {
                        let value = self.comply_with_cookie_laws(
                            geo_resolver,
                            self.event.ip.as_deref().unwrap_or(""),
                        );
                        compliant = Some(value);
                    }

                    if compliant.unwrap_or(false) {
                        data.ip = self.get_three_octets(self.event.ip.as_deref().unwrap_or(""))
                    }
                }
                IPPolicy::Keep => {}
                IPPolicy::Strict => {
                    data.ip = self.get_three_octets(self.event.ip.as_deref().unwrap_or(""))
                }
            }
        }
        Ok(data)
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use crate::{geo::maxmind::MaxMindResolver, utils::time};

    use super::*;

    #[test]
    fn test_ip_geo_enrichment() {
        let mut user = HashMap::new();
        user.insert("id".to_string(), serde_json::json!("xy123"));
        // Create a GeoResolver with the in-memory database
        let geo_resolver = GeoResolver::new("data/GeoLite2-City.mmdb").unwrap();
        let event = Event {
            autocapture_attributes: None,
            api_key: "test_key".to_string(),
            event_type: "test_event".to_string(),
            click_id: None,
            ids: None,
            utc_time: None,
            // local_tz_offset: Some(0),
            referrer: None,
            url: Some("test_url".to_string()),
            page_title: None,
            doc_path: None,
            doc_host: None,
            doc_search: None,
            screen_resolution: None,
            vp_size: None,
            user_agent: Some("test_agent".to_string()),
            user_language: None,
            doc_encoding: None,
            user: user,
            utm: None,
            company: None,
            event_attributes: None,
            ip: Some("157.90.23.154".to_string()),
            cookie_policy: Some(COMPLY_VALUE.to_string()),
            ip_policy: Some(COMPLY_VALUE.to_string()),
            received_at: time::get_current_time(),
            src: None,
            timestamp: None,
        };

        let mut service = PrivacyEnrichmentService::new(event.clone());
        let result = service.enrich(&geo_resolver);
        assert!(result.is_ok()); // First ensure the result is Ok
        let data = result.unwrap();
        println!("{:?}", data);
        assert_eq!(data.anonymous_id, "005d2b0ea5826dd58992e3d1616e4636");

        assert_eq!(
            Some(data.hashed_anonymous_id),
            Some("005d2b0ea5826dd58992e3d1616e4636".to_string())
        );
        assert_eq!(Some(data.ip.clone()), Some("157.90.23.1".to_string()));
        assert_ne!(Some(data.ip), Some("157.90.23.154".to_string()));
    }

}
