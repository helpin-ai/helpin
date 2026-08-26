use std::sync::Arc;

use bytes::Bytes;

use axum::extract::{Query, State};
use axum::http::HeaderMap;
use axum::Json;
use axum_macros::debug_handler;
use base64::Engine;
use chrono::{TimeZone, Utc};
use hmac::{Hmac, Mac};
use sha2::Sha256;
use uuid::Uuid;

use crate::api::{CaptureError, CaptureResponse, CaptureResponseCode};
use crate::auth::authorization::{self, AuthorizedCredential, CredentialKind};
use crate::events::event::{
    Event, EventFormData, EventIdentityProvenance, EventQuery, ProcessedEvent,
};
use crate::router;
use crate::sinks;

#[debug_handler]
pub async fn event(
    state: State<router::State>,
    Query(EventQuery { query_params }): Query<EventQuery>,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Json<CaptureResponse>, CaptureError> {
    tracing::debug!(len = body.len(), "new event request");

    let authorized = authorization::validate_token(
        query_params.clone(),
        headers.clone(),
        state.http_tokens.clone(),
    )
    .map_err(|reason| {
        metrics::increment_counter!("capture_authorization_failures_total", "reason" => reason.to_string());
        CaptureError::TokenValidationError
    })?;

    if authorized.credential_kind == CredentialKind::Browser {
        let origin = headers
            .get("origin")
            .and_then(|value| value.to_str().ok())
            .unwrap_or_default();
        if origin.is_empty()
            || !authorized
                .allowed_origins
                .iter()
                .any(|allowed| allowed == origin)
        {
            metrics::increment_counter!(
                "capture_origin_mismatches_total",
                "installation_id" => authorized.installation_id.clone()
            );
        }
    }

    let token = authorization::extract_token_by_key(query_params.clone(), headers.clone());

    let events = match headers
        .get("content-type")
        .map_or("", |v| v.to_str().unwrap_or(""))
    {
        "application/x-www-form-urlencoded" => {
            let input: EventFormData = serde_urlencoded::from_bytes(&body).map_err(|e| {
                CaptureError::RequestDecodingError(format!("Invalid form data: {}", e))
            })?;
            let payload = base64::engine::general_purpose::STANDARD
                .decode(input.data)
                .map_err(|e| {
                    CaptureError::RequestDecodingError(format!("Invalid base64 data: {}", e))
                })?;
            Event::from_bytes(payload.into(), headers, query_params, token)
        }
        _ => {
            let payload_str = String::from_utf8_lossy(&body);
            if serde_json::from_str::<serde_json::Value>(&payload_str).is_ok() {
                Event::from_bytes(body, headers, query_params, token)
            } else {
                return Err(CaptureError::RequestDecodingError(format!(
                    "JSON payload is invalid ({} bytes)",
                    body.len()
                )));
            }
        }
    };

    let events = events.map_err(|e| {
        tracing::error!("JSON payload is invalid: {:?}", e);
        CaptureError::RequestDecodingError(format!("JSON payload is invalid: {:?}", e))
    })?;

    if events.is_empty() {
        return Err(CaptureError::EmptyBatch);
    }

    process_events(state.sink.clone(), &events, &authorized).await?;

    Ok(Json(CaptureResponse {
        status: CaptureResponseCode::Ok,
    }))
}

pub fn process_single_event(
    event: &Event,
    authorization: &AuthorizedCredential,
) -> Result<ProcessedEvent, CaptureError> {
    let identity_provenance = event_identity_provenance(event, authorization);
    let mut sanitized_event = event.clone();
    sanitized_event.user.remove("identity_verification");
    sanitized_event.api_key = authorization.installation_id.clone();
    Ok(ProcessedEvent {
        event: sanitized_event,
        event_id: Uuid::new_v4(),
        authorization: Some(authorization.clone()),
        identity_provenance,
    })
}

fn event_identity_provenance(
    event: &Event,
    authorization: &AuthorizedCredential,
) -> EventIdentityProvenance {
    if authorization.credential_kind == CredentialKind::Server {
        return EventIdentityProvenance {
            identity_method: "server_event".to_string(),
            identity_trust: "verified".to_string(),
            verified_at: Some(Utc::now().to_rfc3339()),
            verifier_version: Some("transport:v1".to_string()),
        };
    }

    let email = event
        .user
        .get("email")
        .and_then(|value| value.as_str())
        .unwrap_or_default()
        .trim()
        .to_lowercase();
    let external_user_id = event
        .user
        .get("id")
        .and_then(event_identity_value)
        .unwrap_or_default();
    let company_external_id = event
        .company
        .as_ref()
        .and_then(|company| company.get("id"))
        .and_then(event_identity_value)
        .unwrap_or_default();
    if email.is_empty() && external_user_id.is_empty() && company_external_id.is_empty() {
        return EventIdentityProvenance::default();
    }

    let proof = event.user.get("identity_verification");
    let valid = proof.is_some_and(|proof| {
        verify_event_identity_proof(
            proof,
            authorization,
            &email,
            &external_user_id,
            &company_external_id,
        )
    });
    if valid {
        EventIdentityProvenance {
            identity_method: "signed_widget".to_string(),
            identity_trust: "verified".to_string(),
            verified_at: Some(Utc::now().to_rfc3339()),
            verifier_version: Some("v1".to_string()),
        }
    } else {
        EventIdentityProvenance {
            identity_method: "browser_claim".to_string(),
            identity_trust: "untrusted".to_string(),
            verified_at: None,
            verifier_version: None,
        }
    }
}

fn event_identity_value(value: &serde_json::Value) -> Option<String> {
    value
        .as_str()
        .map(ToString::to_string)
        .or_else(|| value.as_i64().map(|number| number.to_string()))
}

fn verify_event_identity_proof(
    proof: &serde_json::Value,
    authorization: &AuthorizedCredential,
    email: &str,
    external_user_id: &str,
    company_external_id: &str,
) -> bool {
    let Some(proof) = proof.as_object() else {
        return false;
    };
    if proof.get("version").and_then(|value| value.as_str()) != Some("v1") {
        return false;
    }
    let Some(issued_at) = proof.get("issued_at").and_then(|value| value.as_i64()) else {
        return false;
    };
    let Some(expires_at) = proof.get("expires_at").and_then(|value| value.as_i64()) else {
        return false;
    };
    let now = Utc::now().timestamp();
    if expires_at < now
        || issued_at > now + 60
        || expires_at <= issued_at
        || expires_at - issued_at > 900
    {
        return false;
    }
    if Utc.timestamp_opt(issued_at, 0).single().is_none()
        || Utc.timestamp_opt(expires_at, 0).single().is_none()
    {
        return false;
    }
    let Some(signature) = proof.get("signature").and_then(|value| value.as_str()) else {
        return false;
    };
    if signature != signature.to_lowercase() {
        return false;
    }
    let Ok(provided) = hex::decode(signature) else {
        return false;
    };
    let message = format!(
        "helpin-widget-identity:v1\n{}\n{}\n{}\n{}\n{}\n{}",
        event_widget_key(authorization),
        email,
        external_user_id,
        company_external_id,
        issued_at,
        expires_at
    );
    let Ok(mut mac) = Hmac::<Sha256>::new_from_slice(authorization.signing_secret.as_bytes())
    else {
        return false;
    };
    mac.update(message.as_bytes());
    mac.verify_slice(&provided).is_ok()
}

fn event_widget_key(authorization: &AuthorizedCredential) -> &str {
    // The browser credential secret is not serialized into NATS or disk spill. It is needed only while
    // validating the domain-separated proof at capture time.
    &authorization.browser_key
}

pub async fn process_events(
    sink: Arc<dyn sinks::EventSink + Send + Sync>,
    events: &[Event],
    authorization: &AuthorizedCredential,
) -> Result<(), CaptureError> {
    tracing::debug!(count = events.len(), "processing events");

    let events: Vec<ProcessedEvent> = events
        .iter()
        .map(|event| process_single_event(event, authorization))
        .collect::<Result<Vec<_>, _>>()?;

    if events.len() == 1 {
        let event = sinks::EventTypes::Processed(events[0].clone());
        sink.send(event).await?;
    } else {
        let event_batch = events
            .into_iter()
            .map(sinks::EventTypes::Processed)
            .collect::<Vec<_>>();
        sink.send_batch(event_batch).await?;
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use crate::auth::authorization::{AuthorizedCredential, CredentialKind};
    use crate::auth::http_tokens::{HttpTokens, Token};
    use crate::health::HealthRegistry;
    use crate::sinks;
    use crate::sinks::print_sink::PrintSink;
    use crate::utils::time::SystemTime;
    use std::collections::HashMap;
    use std::sync::{Arc, Mutex};

    use serde_json::json;

    use super::{process_events, process_single_event};
    use crate::events::event::Event;
    use crate::router::State;

    fn test_authorization() -> AuthorizedCredential {
        AuthorizedCredential {
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            credential_kind: CredentialKind::Browser,
            installation_id: "installation-1".to_string(),
            allowed_origins: vec!["http://localhost".to_string()],
            identity_verification_mode: "report_only".to_string(),
            signing_secret: "secret_token".to_string(),
            browser_key: "secret_api_key".to_string(),
        }
    }

    #[tokio::test]
    async fn all_events_have_same_token() {
        let http_tokens: Arc<Mutex<HttpTokens>> = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![Token {
                id: String::from("1"),
                workspace_id: String::from("00000000-0000-0000-0000-000000000001"),
                client_secret: String::from("secret_api_key"),
                server_secret: String::from("secret_token"),
                origins: vec![String::from("localhost")],
                identity_verification_mode: String::from("report_only"),
            }],
        }));
        let state = State {
            http_tokens: http_tokens.clone(),
            sink: Arc::new(PrintSink {}),
            timesource: Arc::new(SystemTime {}),
            health: HealthRegistry::new(),
        };

        let events = vec![
            Event {
                api_key: String::from("hello"),
                event_type: String::new(),
                user: HashMap::new(),
                ..Default::default()
            },
            Event {
                api_key: String::from("hello"),
                event_type: String::new(),
                user: HashMap::from([(String::from("token"), json!("hello"))]),
                ..Default::default()
            },
        ];

        let processed = process_events(state.sink, &events, &test_authorization()).await;
        assert!(processed.is_ok());
    }

    #[tokio::test]
    async fn test_process_single_event() {
        let sink: Arc<dyn sinks::EventSink + Send + Sync> = Arc::new(PrintSink {});
        let events = vec![Event {
            api_key: String::from("test_key"),
            event_type: String::from("page_view"),
            user: HashMap::new(),
            ..Default::default()
        }];

        // Single event goes through send() path
        let result = process_events(sink, &events, &test_authorization()).await;
        assert!(result.is_ok());
    }

    #[test]
    fn processed_events_do_not_retain_credentials_or_identity_proofs() {
        let authorization = test_authorization();
        let event = Event {
            api_key: authorization.browser_key.clone(),
            event_type: String::from("identify"),
            user: HashMap::from([
                (String::from("email"), json!("person@example.com")),
                (
                    String::from("identity_verification"),
                    json!({"version": "v1", "signature": "secret-proof"}),
                ),
            ]),
            ..Default::default()
        };

        let processed = process_single_event(&event, &authorization).unwrap();

        assert_eq!(processed.event.api_key, authorization.installation_id);
        assert!(!processed.event.user.contains_key("identity_verification"));
    }

    #[tokio::test]
    async fn test_process_events_empty_batch() {
        let sink: Arc<dyn sinks::EventSink + Send + Sync> = Arc::new(PrintSink {});
        let events: Vec<Event> = vec![];

        // process_events called with empty vec — no events to process, returns Ok
        // (the empty check happens in the capture handler, not process_events)
        let result = process_events(sink, &events, &test_authorization()).await;
        // Empty iter collects to empty vec, then events.len() == 0, neither branch executes
        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_process_events_batch_path() {
        let sink: Arc<dyn sinks::EventSink + Send + Sync> = Arc::new(PrintSink {});
        let events = vec![
            Event {
                api_key: String::from("key1"),
                event_type: String::from("ev1"),
                user: HashMap::new(),
                ..Default::default()
            },
            Event {
                api_key: String::from("key2"),
                event_type: String::from("ev2"),
                user: HashMap::new(),
                ..Default::default()
            },
            Event {
                api_key: String::from("key3"),
                event_type: String::from("ev3"),
                user: HashMap::new(),
                ..Default::default()
            },
        ];

        // Multiple events go through send_batch() path
        let result = process_events(sink, &events, &test_authorization()).await;
        assert!(result.is_ok());
    }

    /// Test that a failing sink propagates CaptureError correctly.
    mod failing_sink_tests {
        use super::*;
        use crate::api::CaptureError;
        use crate::sinks::{EventSink, EventTypes};
        use async_trait::async_trait;

        struct FailingSink;

        #[async_trait]
        impl EventSink for FailingSink {
            async fn send(&self, _event: EventTypes) -> Result<(), CaptureError> {
                Err(CaptureError::RetryableSinkError)
            }
            async fn send_batch(&self, _events: Vec<EventTypes>) -> Result<(), CaptureError> {
                Err(CaptureError::RetryableSinkError)
            }
        }

        #[tokio::test]
        async fn test_single_event_sink_error_propagates() {
            let sink: Arc<dyn sinks::EventSink + Send + Sync> = Arc::new(FailingSink);
            let events = vec![Event {
                api_key: String::from("key"),
                event_type: String::from("ev"),
                user: HashMap::new(),
                ..Default::default()
            }];

            let result = process_events(sink, &events, &test_authorization()).await;
            assert!(result.is_err(), "Sink error should propagate");
        }

        #[tokio::test]
        async fn test_batch_sink_error_propagates() {
            let sink: Arc<dyn sinks::EventSink + Send + Sync> = Arc::new(FailingSink);
            let events = vec![
                Event {
                    api_key: String::from("k1"),
                    event_type: String::from("e1"),
                    user: HashMap::new(),
                    ..Default::default()
                },
                Event {
                    api_key: String::from("k2"),
                    event_type: String::from("e2"),
                    user: HashMap::new(),
                    ..Default::default()
                },
            ];

            let result = process_events(sink, &events, &test_authorization()).await;
            assert!(result.is_err(), "Batch sink error should propagate");
        }
    }
}
