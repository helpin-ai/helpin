use axum::http::StatusCode;
use axum::response::IntoResponse;
use axum::Json;
use serde::Serialize;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

#[derive(Clone)]
pub struct HealthRegistry {
    ready: Arc<AtomicBool>,
    nats_healthy: Arc<AtomicBool>,
}

#[derive(Serialize)]
pub struct HealthStatus {
    pub status: &'static str,
    pub ready: bool,
    pub components: ComponentHealth,
}

#[derive(Serialize)]
pub struct ComponentHealth {
    pub nats: ComponentStatus,
}

#[derive(Serialize)]
pub struct ComponentStatus {
    pub healthy: bool,
    pub detail: &'static str,
}

impl HealthRegistry {
    pub fn new() -> Self {
        Self {
            ready: Arc::new(AtomicBool::new(true)),
            nats_healthy: Arc::new(AtomicBool::new(true)),
        }
    }

    pub fn set_ready(&self, ready: bool) {
        self.ready.store(ready, Ordering::SeqCst);
    }

    pub fn set_nats_healthy(&self, healthy: bool) {
        self.nats_healthy.store(healthy, Ordering::SeqCst);
        metrics::gauge!("capture_nats_health", if healthy { 1.0 } else { 0.0 });
    }

    pub fn is_ready(&self) -> bool {
        self.ready.load(Ordering::SeqCst)
    }

    pub fn is_nats_healthy(&self) -> bool {
        self.nats_healthy.load(Ordering::SeqCst)
    }
}

/// Liveness probe: returns 200 if the process is alive and responding.
/// K8s uses this to decide whether to restart the container.
pub async fn liveness() -> impl IntoResponse {
    (StatusCode::OK, "ok")
}

/// Readiness probe: returns 200 when the pod can accept traffic, 503 during shutdown.
/// K8s uses this to decide whether to route traffic to the pod.
/// Note: We stay ready when NATS is down because FallbackSink fsyncs to disk.
pub async fn readiness(health: axum::extract::State<HealthRegistry>) -> impl IntoResponse {
    if health.is_ready() {
        (StatusCode::OK, "ready")
    } else {
        (StatusCode::SERVICE_UNAVAILABLE, "shutting down")
    }
}

/// Detailed health status for debugging and dashboards.
pub async fn status(health: axum::extract::State<HealthRegistry>) -> impl IntoResponse {
    let nats_healthy = health.is_nats_healthy();
    let ready = health.is_ready();

    let overall = if ready { "healthy" } else { "shutting_down" };

    let status = HealthStatus {
        status: overall,
        ready,
        components: ComponentHealth {
            nats: ComponentStatus {
                healthy: nats_healthy,
                detail: if nats_healthy {
                    "connected"
                } else {
                    "unreachable"
                },
            },
        },
    };

    (StatusCode::OK, Json(status))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_health_registry_defaults() {
        let registry = HealthRegistry::new();
        assert!(registry.is_ready());
        assert!(registry.is_nats_healthy());
    }

    #[test]
    fn test_set_ready() {
        let registry = HealthRegistry::new();
        registry.set_ready(false);
        assert!(!registry.is_ready());
        registry.set_ready(true);
        assert!(registry.is_ready());
    }

    #[test]
    fn test_set_nats_healthy() {
        let registry = HealthRegistry::new();
        registry.set_nats_healthy(false);
        assert!(!registry.is_nats_healthy());
        registry.set_nats_healthy(true);
        assert!(registry.is_nats_healthy());
    }

    #[test]
    fn test_clone_shares_state() {
        let registry = HealthRegistry::new();
        let clone = registry.clone();
        registry.set_ready(false);
        assert!(!clone.is_ready());
    }

    #[tokio::test]
    async fn test_liveness_returns_200() {
        let response = liveness().await.into_response();
        assert_eq!(response.status(), StatusCode::OK);
    }
}
