use std::path::Path;
use std::sync::Arc;
use std::time::{Duration, SystemTime};

use tokio::sync::RwLock;

use crate::geo::resolver::GeoResolver;
use crate::ip2location::resolver::IP2ProxyResolver;

const DEFAULT_MAXMIND_REFRESH_INTERVAL: Duration = Duration::from_secs(12 * 60 * 60);
const DEFAULT_IP2PROXY_REFRESH_INTERVAL: Duration = Duration::from_secs(24 * 60 * 60);
const METRIC_INTERVAL: Duration = Duration::from_secs(60);

#[derive(Clone)]
pub struct EnrichmentDatabaseState {
    enabled: bool,
    maxmind_path: String,
    ip2proxy_path: String,
    geo: Arc<RwLock<Option<GeoResolver>>>,
    ip2proxy: Arc<RwLock<Option<IP2ProxyResolver>>>,
}

impl EnrichmentDatabaseState {
    /// Download missing databases before constructing the resolvers, matching
    /// the original Usermaven consumer lifecycle. Download failures fail open:
    /// capture still starts, and the periodic refresh loop retries later.
    pub async fn initialize(enabled: bool) -> Self {
        if enabled {
            let maxmind_path = crate::geo::downloader::target_path();
            let ip2proxy_path = crate::ip2location::downloader::target_path();
            tokio::join!(
                download_maxmind_if_missing(&maxmind_path),
                download_ip2proxy_if_missing(&ip2proxy_path),
            );
        }

        Self::load(enabled)
    }

    pub fn load(enabled: bool) -> Self {
        let maxmind_path = crate::geo::downloader::target_path();
        let ip2proxy_path = crate::ip2location::downloader::target_path();
        let geo = enabled.then(|| GeoResolver::new(&maxmind_path)).and_then(
            |result| match result {
                Ok(resolver) => Some(resolver),
                Err(error) => {
                    tracing::warn!(%error, "MaxMind database unavailable; enrichment is failing open");
                    None
                }
            },
        );
        let ip2proxy = enabled
            .then(|| IP2ProxyResolver::new(&ip2proxy_path))
            .and_then(|result| match result {
                Ok(resolver) => Some(resolver),
                Err(error) => {
                    tracing::warn!(%error, "IP2Proxy database unavailable; enrichment is failing open");
                    None
                }
            });
        emit_database_metrics(
            "maxmind",
            &maxmind_path,
            geo.as_ref()
                .map(|resolver| resolver.database_build_epoch() as f64),
        );
        emit_database_metrics(
            "ip2proxy",
            &ip2proxy_path,
            ip2proxy
                .as_ref()
                .and_then(|resolver| numeric_database_version(&resolver.database_version())),
        );
        let state = Self {
            enabled,
            maxmind_path,
            ip2proxy_path,
            geo: Arc::new(RwLock::new(geo)),
            ip2proxy: Arc::new(RwLock::new(ip2proxy)),
        };
        state
    }

    pub async fn snapshot(&self) -> (Option<GeoResolver>, Option<IP2ProxyResolver>) {
        let geo = self.geo.read().await.clone();
        let ip2proxy = self.ip2proxy.read().await.clone();
        (geo, ip2proxy)
    }

    pub fn start_refresh_loop(&self) {
        if !self.enabled {
            metrics::gauge!("capture_enrichment_database_enabled", 0.0);
            return;
        }
        metrics::gauge!("capture_enrichment_database_enabled", 1.0);

        let maxmind_updater = self.clone();
        tokio::spawn(async move {
            let refresh_interval = refresh_interval(
                "MAXMIND_DB_REFRESH_INTERVAL_SECS",
                DEFAULT_MAXMIND_REFRESH_INTERVAL,
            );
            loop {
                tokio::time::sleep(refresh_interval).await;
                maxmind_updater.refresh_maxmind().await;
            }
        });

        let ip2proxy_updater = self.clone();
        tokio::spawn(async move {
            let refresh_interval = refresh_interval(
                "IP2PROXY_DB_REFRESH_INTERVAL_SECS",
                DEFAULT_IP2PROXY_REFRESH_INTERVAL,
            );
            loop {
                tokio::time::sleep(refresh_interval).await;
                ip2proxy_updater.refresh_ip2proxy().await;
            }
        });

        let reporter = self.clone();
        tokio::spawn(async move {
            loop {
                reporter.emit_metrics().await;
                tokio::time::sleep(METRIC_INTERVAL).await;
            }
        });
    }

    async fn refresh_maxmind(&self) {
        match crate::geo::downloader::download_and_save().await {
            Ok(()) => match GeoResolver::new(&self.maxmind_path) {
                Ok(resolver) => {
                    *self.geo.write().await = Some(resolver);
                    refresh_succeeded("maxmind");
                }
                Err(error) => self.refresh_failed("maxmind", &error.to_string()),
            },
            Err(error) => self.refresh_failed("maxmind", &error.to_string()),
        }
        self.emit_metrics().await;
    }

    async fn refresh_ip2proxy(&self) {
        match crate::ip2location::downloader::ip2proxy_download_and_save().await {
            Ok(()) => match IP2ProxyResolver::new(&self.ip2proxy_path) {
                Ok(resolver) => {
                    *self.ip2proxy.write().await = Some(resolver);
                    refresh_succeeded("ip2proxy");
                }
                Err(error) => self.refresh_failed("ip2proxy", &error.to_string()),
            },
            Err(error) => self.refresh_failed("ip2proxy", &error.to_string()),
        }
        self.emit_metrics().await;
    }

    fn refresh_failed(&self, database: &'static str, error: &str) {
        metrics::counter!("capture_enrichment_database_reload_total", 1, "database" => database, "result" => "failure");
        tracing::warn!(%error, database, "enrichment database refresh failed; retaining last valid resolver");
    }

    async fn emit_metrics(&self) {
        let geo = self.geo.read().await;
        let ip2proxy = self.ip2proxy.read().await;
        self.emit_metrics_from_loaded(&geo, &ip2proxy);
    }

    fn emit_metrics_from_loaded(
        &self,
        geo: &Option<GeoResolver>,
        ip2proxy: &Option<IP2ProxyResolver>,
    ) {
        emit_database_metrics(
            "maxmind",
            &self.maxmind_path,
            geo.as_ref()
                .map(|resolver| resolver.database_build_epoch() as f64),
        );
        emit_database_metrics(
            "ip2proxy",
            &self.ip2proxy_path,
            ip2proxy
                .as_ref()
                .and_then(|resolver| numeric_database_version(&resolver.database_version())),
        );
    }
}

async fn download_maxmind_if_missing(path: &str) {
    if Path::new(path).exists() {
        tracing::info!(path, "Using existing MaxMind database");
        return;
    }

    tracing::info!(path, "Downloading MaxMind database at startup");
    match crate::geo::downloader::download_and_save().await {
        Ok(()) => refresh_succeeded("maxmind"),
        Err(error) => {
            metrics::counter!("capture_enrichment_database_reload_total", 1, "database" => "maxmind", "result" => "failure");
            tracing::warn!(%error, "MaxMind startup download failed; enrichment will fail open");
        }
    }
}

async fn download_ip2proxy_if_missing(path: &str) {
    if Path::new(path).exists() {
        tracing::info!(path, "Using existing IP2Proxy database");
        return;
    }

    tracing::info!(path, "Downloading IP2Proxy database at startup");
    match crate::ip2location::downloader::ip2proxy_download_and_save().await {
        Ok(()) => refresh_succeeded("ip2proxy"),
        Err(error) => {
            metrics::counter!("capture_enrichment_database_reload_total", 1, "database" => "ip2proxy", "result" => "failure");
            tracing::warn!(%error, "IP2Proxy startup download failed; enrichment will fail open");
        }
    }
}

fn refresh_interval(variable: &str, default: Duration) -> Duration {
    std::env::var(variable)
        .ok()
        .and_then(|value| value.parse::<u64>().ok())
        .map(Duration::from_secs)
        .filter(|duration| !duration.is_zero())
        .unwrap_or(default)
}

fn refresh_succeeded(database: &'static str) {
    metrics::counter!("capture_enrichment_database_reload_total", 1, "database" => database, "result" => "success");
    tracing::info!(database, "Enrichment database refresh completed");
}

fn emit_database_metrics(database: &'static str, path: &str, version: Option<f64>) {
    let available = version.is_some();
    metrics::gauge!("capture_enrichment_database_available", if available { 1.0 } else { 0.0 }, "database" => database);
    if let Some(version) = version {
        metrics::gauge!("capture_enrichment_database_version", version, "database" => database);
    }
    let age = std::fs::metadata(Path::new(path))
        .and_then(|metadata| metadata.modified())
        .ok()
        .and_then(|modified| SystemTime::now().duration_since(modified).ok())
        .map(|duration| duration.as_secs_f64())
        .unwrap_or(-1.0);
    metrics::gauge!("capture_enrichment_database_age_seconds", age, "database" => database);
}

fn numeric_database_version(version: &str) -> Option<f64> {
    let parts = version
        .split(|character: char| !character.is_ascii_digit())
        .filter(|part| !part.is_empty())
        .map(str::parse::<u64>)
        .collect::<Result<Vec<_>, _>>()
        .ok()?;
    if parts.is_empty() {
        return None;
    }
    Some(
        parts
            .into_iter()
            .fold(0_u64, |encoded, part| encoded * 100 + part) as f64,
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn disabled_network_enrichment_has_an_empty_snapshot() {
        let state = EnrichmentDatabaseState::load(false);
        let (geo, ip2proxy) = state.snapshot().await;
        assert!(geo.is_none());
        assert!(ip2proxy.is_none());
    }

    #[test]
    fn database_versions_are_encoded_without_dynamic_metric_labels() {
        assert_eq!(numeric_database_version("2026.08.25"), Some(20260825.0));
        assert_eq!(numeric_database_version("unknown"), None);
    }
}
