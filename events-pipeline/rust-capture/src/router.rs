use axum::{extract::DefaultBodyLimit, middleware};
use sentry::integrations::tower::{NewSentryLayer, SentryHttpLayer};
use std::sync::{Arc, Mutex};
use tower_http::trace::{self, TraceLayer};
use tracing::Level;

use axum::{
    http::{header, Method},
    routing::{get, post},
    Router,
};

use crate::{
    auth::http_tokens::HttpTokens, capture, health, health::HealthRegistry,
    metrics_recorder::{request_timeout, track_metrics},
    sinks, utils::time::TimeSource,
};
use tower_http::cors::{Any, CorsLayer};

#[derive(Clone)]
pub struct State {
    pub sink: Arc<dyn sinks::EventSink + Send + Sync>,
    pub timesource: Arc<dyn TimeSource + Send + Sync>,
    pub http_tokens: Arc<Mutex<HttpTokens>>,
    pub health: HealthRegistry,
}

async fn index() -> &'static str {
    "ok"
}

pub fn router<
    TZ: TimeSource + Send + Sync + 'static,
    S: sinks::EventSink + Send + Sync + 'static,
>(
    timesource: TZ,
    sink: S,
    http_tokens: Arc<Mutex<HttpTokens>>,
    health_registry: HealthRegistry,
) -> Router {
    let state = State {
        sink: Arc::new(sink),
        timesource: Arc::new(timesource),
        http_tokens,
        health: health_registry.clone(),
    };

    let cors = CorsLayer::new()
        .allow_methods([Method::GET, Method::POST, Method::OPTIONS])
        .allow_headers([
            header::ACCEPT,
            header::ACCEPT_LANGUAGE,
            header::AUTHORIZATION,
            header::CONTENT_LANGUAGE,
            header::CONTENT_TYPE,
        ])
        .allow_origin(Any);

    // 2MB body size limit
    let max_body_size: usize = std::env::var("MAX_BODY_SIZE")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(2 * 1024 * 1024);

    // Health check routes (no auth, no body limit, no middleware)
    let health_routes = Router::new()
        .route("/health/liveness", get(health::liveness))
        .route("/health/readiness", get(health::readiness))
        .route("/health/status", get(health::status))
        .with_state(health_registry);

    Router::new()
        .route("/", get(index))
        .route("/api/v1/event", post(capture::event))
        .route("/api/v1/events", post(capture::event))
        .route("/api/v1/s2s/event", post(capture::event))
        .route("/api/v1/s2s/event/", post(capture::event))
        .route("/api/v1/s2s/events", post(capture::event))
        .route("/api.:ignored", post(capture::event))
        .route_layer(middleware::from_fn(request_timeout))
        .route_layer(middleware::from_fn(track_metrics))
        .layer(DefaultBodyLimit::max(max_body_size))
        .layer(
            TraceLayer::new_for_http()
                .make_span_with(trace::DefaultMakeSpan::new().level(Level::INFO))
                .on_response(trace::DefaultOnResponse::new().level(Level::INFO)),
        )
        .layer(TraceLayer::new_for_http())
        .layer(cors)
        .layer(NewSentryLayer::new_from_top())
        .layer(SentryHttpLayer::new())
        .with_state(state)
        .merge(health_routes)
}
