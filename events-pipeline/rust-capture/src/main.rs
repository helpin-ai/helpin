use dotenv::dotenv;
use events_pipeline::metrics_recorder::metrics_app;
use events_pipeline::utils::time::SystemTime;
use events_pipeline::{auth, enrichment, health, router, sinks};
use std::env;
use std::net::SocketAddr;
use std::path::PathBuf;
use std::sync::Arc;
use tokio::signal;
use tracing_subscriber::EnvFilter;

use tracing_subscriber::{fmt, prelude::*};

#[tokio::main]
async fn main() {
    dotenv().ok();
    let log_level = std::env::var("LOG_LEVEL").unwrap_or_else(|_| "INFO".to_string());
    let filter = format!("{},tower_http=ERROR", log_level);
    let traces_sample_rate: f32 = env::var("TRACES_SAMPLE_RATE")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(0.1);

    // Initialize Sentry from env var (single init)
    let sentry_dsn = env::var("SENTRY_DSN").unwrap_or_default();
    let _guard = sentry::init((
        sentry_dsn,
        sentry::ClientOptions {
            release: sentry::release_name!(),
            traces_sample_rate,
            ..Default::default()
        },
    ));

    // Initialize tracing
    tracing_subscriber::registry()
        .with(EnvFilter::new(filter))
        .with(fmt::layer())
        .with(sentry_tracing::layer())
        .init();

    tracing::info!("Application booting");

    let (_main_server, _metrics_server) = tokio::join!(start_main_server(), start_metrics_server());
}

async fn start_main_server() {
    let use_print_sink = env::var("PRINT_SINK").unwrap_or_else(|_| "false".to_string());
    tracing::info!("Application booting");

    let tokens = auth::http_tokens::HttpTokens::new().await;
    tracing::info!("tokens loaded: {:?}", tokens.len());

    let health_registry = health::HealthRegistry::new();

    let app = if use_print_sink == "true" {
        tracing::info!("Using print sink");
        router::router(
            SystemTime {},
            sinks::print_sink::PrintSink {},
            tokens,
            health_registry.clone(),
        )
    } else {
        tracing::info!("Using inline enrichment and NATS with durable disk fallback");
        let nats_url = env::var("NATS_URL").expect("Expected NATS_URL");
        let publisher =
            sinks::nats_event_sink::NatsEventPublisher::new(nats_url, health_registry.clone());

        let network_enrichment_enabled = env::var("NETWORK_ENRICHMENT_ENABLED")
            .map(|value| value != "false")
            .unwrap_or(true);
        let databases = enrichment::database_state::EnrichmentDatabaseState::initialize(
            network_enrichment_enabled,
        )
        .await;
        databases.start_refresh_loop();
        let preload_user_agents = env::var("USER_AGENT_CACHE_PRELOAD_ENABLED")
            .map(|value| value != "false")
            .unwrap_or(true);
        let nats_sink = Arc::new(if preload_user_agents {
            sinks::enriching_nats_sink::EnrichingNatsSink::new_with_preloaded_user_agents(
                publisher, databases,
            )
        } else {
            sinks::enriching_nats_sink::EnrichingNatsSink::new(publisher, databases)
        });

        let fallback_dir =
            PathBuf::from(env::var("FALLBACK_DIR").unwrap_or_else(|_| "data/fallback".to_string()));
        let disk_sink = Arc::new(sinks::disk_sink::DiskSink::new(fallback_dir));
        let fallback_sink = sinks::fallback_sink::FallbackSink::new(nats_sink, disk_sink);

        tracing::info!("NATS sink with disk fallback initialized");
        router::router(
            SystemTime {},
            fallback_sink,
            tokens,
            health_registry.clone(),
        )
    };

    let addr = SocketAddr::from(([0, 0, 0, 0], env_port("CAPTURE_HTTP_PORT", 3000)));
    tracing::info!("listening on {}", addr);

    axum::Server::bind(&addr)
        .serve(app.into_make_service())
        .with_graceful_shutdown(shutdown_signal(health_registry))
        .await
        .unwrap();
}

async fn start_metrics_server() {
    let app = metrics_app();

    let addr = SocketAddr::from(([0, 0, 0, 0], env_port("CAPTURE_METRICS_PORT", 3001)));
    tracing::debug!("listening on {} for metrics server", addr);
    axum::Server::bind(&addr)
        .serve(app.into_make_service())
        .with_graceful_shutdown(wait_for_signal())
        .await
        .unwrap()
}

fn env_port(name: &str, default: u16) -> u16 {
    env::var(name)
        .ok()
        .and_then(|value| value.parse::<u16>().ok())
        .filter(|port| *port > 0)
        .unwrap_or(default)
}

#[cfg(test)]
mod tests {
    use super::env_port;

    #[test]
    fn capture_ports_use_defaults_when_unconfigured() {
        assert_eq!(env_port("HELPIN_TEST_MISSING_CAPTURE_PORT", 3000), 3000);
    }
}

async fn wait_for_signal() {
    let ctrl_c = async {
        signal::ctrl_c()
            .await
            .expect("failed to install Ctrl+C handler");
    };

    #[cfg(unix)]
    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("failed to install signal handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {},
        _ = terminate => {},
    }
}

async fn shutdown_signal(health_registry: health::HealthRegistry) {
    wait_for_signal().await;

    tracing::info!("signal received, marking pod as not ready");
    health_registry.set_ready(false);

    // Give k8s time to update endpoints and stop routing new traffic (pre-stop grace period)
    tokio::time::sleep(std::time::Duration::from_secs(5)).await;

    tracing::info!("starting graceful shutdown");
}
