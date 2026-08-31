use std::{env, net::SocketAddr};

use anyhow::anyhow;
use anyhow::Result;
use axum::{extract::State, http::StatusCode, routing::get, Router};
use dotenv::dotenv;
use events_pipeline::metrics_recorder::metrics_app;
use events_pipeline::writer_runtime::{SessionWriter, WriterHealth, WriterSettings};
use tracing_subscriber::EnvFilter;

#[tokio::main]
async fn main() -> Result<()> {
    dotenv().ok();
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::from_default_env())
        .init();

    let settings = WriterSettings::from_env()?;
    let health_port = env_port("WRITER_HEALTH_PORT", 3000)?;
    let metrics_port = env_port("WRITER_METRICS_PORT", 3001)?;
    let health = WriterHealth::default();
    let health_router = Router::new()
        .route("/health/liveness", get(|| async { StatusCode::OK }))
        .route("/health/readiness", get(readiness))
        .with_state(health.clone());

    let health_server = axum::Server::bind(&SocketAddr::from(([0, 0, 0, 0], health_port)))
        .serve(health_router.into_make_service());
    let metrics_server = axum::Server::bind(&SocketAddr::from(([0, 0, 0, 0], metrics_port)))
        .serve(metrics_app().into_make_service());
    let writer_runtime = async move {
        loop {
            health.set_ready(false);
            match SessionWriter::connect_with_health(settings.clone(), health.clone()).await {
                Ok(writer) => {
                    if let Err(error) = writer.run().await {
                        tracing::error!(%error, "session writer stopped; reconnecting");
                    }
                }
                Err(error) => {
                    tracing::warn!(%error, "session writer dependencies unavailable; retrying");
                }
            }
            tokio::time::sleep(std::time::Duration::from_secs(1)).await;
        }
    };

    tokio::select! {
        _ = writer_runtime => Err(anyhow!("writer runtime stopped")),
        result = health_server => match result {
            Ok(()) => Err(anyhow!("writer health server stopped")),
            Err(error) => Err(error.into()),
        },
        result = metrics_server => match result {
            Ok(()) => Err(anyhow!("writer metrics server stopped")),
            Err(error) => Err(error.into()),
        },
    }
}

fn env_port(name: &str, default: u16) -> Result<u16> {
    match env::var(name) {
        Ok(value) => value
            .parse()
            .map_err(|error| anyhow!("invalid {name}={value:?}: {error}")),
        Err(env::VarError::NotPresent) => Ok(default),
        Err(error) => Err(anyhow!("read {name}: {error}")),
    }
}

async fn readiness(State(health): State<WriterHealth>) -> StatusCode {
    if health.is_ready() {
        StatusCode::OK
    } else {
        StatusCode::SERVICE_UNAVAILABLE
    }
}
