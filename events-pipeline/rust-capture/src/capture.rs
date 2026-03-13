use std::sync::Arc;

use bytes::Bytes;

use axum::Json;
use axum::extract::{Query, State};
use axum::http::HeaderMap;
use axum_macros::debug_handler;
use base64::Engine;
use uuid::Uuid;

use crate::api::{CaptureError, CaptureResponse, CaptureResponseCode};
use crate::auth::authorization;
use crate::events::event::{Event, EventFormData, EventQuery, ProcessedEvent};
use crate::sinks;
use crate::router;

#[debug_handler]
pub async fn event(
    state: State<router::State>,
    Query(EventQuery { query_params }): Query<EventQuery>,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Json<CaptureResponse>, CaptureError> {
    tracing::debug!(len = body.len(), "new event request");
    tracing::debug!(headers = ?headers, "headers");

    let result = authorization::validate_token(
        query_params.clone(),
        headers.clone(),
        state.http_tokens.clone(),
    );
    if result.is_err() {
        tracing::debug!("Token is invalid");
        return Err(CaptureError::TokenValidationError);
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
                    "JSON payload is invalid: {}",
                    payload_str
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

    process_events(state.sink.clone(), &events).await?;

    Ok(Json(CaptureResponse {
        status: CaptureResponseCode::Ok,
    }))
}

pub fn process_single_event(_event: &Event) -> Result<ProcessedEvent, CaptureError> {
    Ok(ProcessedEvent {
        event: _event.clone(),
        event_id: Uuid::new_v4(),
    })
}

pub async fn process_events(
    sink: Arc<dyn sinks::EventSink + Send + Sync>,
    events: &[Event],
) -> Result<(), CaptureError> {
    tracing::debug!(events = ?events, "processing events");

    let events: Vec<ProcessedEvent> = events
        .iter()
        .map(process_single_event)
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
    use crate::auth::http_tokens::{HttpTokens, Token};
    use crate::health::HealthRegistry;
    use crate::sinks;
    use crate::sinks::print_sink::PrintSink;
    use crate::utils::time::SystemTime;
    use std::collections::HashMap;
    use std::sync::{Arc, Mutex};

    use serde_json::json;

    use super::process_events;
    use crate::events::event::Event;
    use crate::router::State;

    #[tokio::test]
    async fn all_events_have_same_token() {
        let http_tokens: Arc<Mutex<HttpTokens>> = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![Token {
                id: String::from("1"),
                client_secret: String::from("secret_api_key"),
                server_secret: String::from("secret_token"),
                origins: vec![String::from("localhost")],
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

        let processed = process_events(state.sink, &events).await;
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
        let result = process_events(sink, &events).await;
        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_process_events_empty_batch() {
        let sink: Arc<dyn sinks::EventSink + Send + Sync> = Arc::new(PrintSink {});
        let events: Vec<Event> = vec![];

        // process_events called with empty vec — no events to process, returns Ok
        // (the empty check happens in the capture handler, not process_events)
        let result = process_events(sink, &events).await;
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
        let result = process_events(sink, &events).await;
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

            let result = process_events(sink, &events).await;
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

            let result = process_events(sink, &events).await;
            assert!(result.is_err(), "Batch sink error should propagate");
        }
    }
}
