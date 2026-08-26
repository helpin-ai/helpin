use std::env;
use std::sync::Arc;
use std::time::{Duration, Instant};

use arc_swap::ArcSwap;
use serde::Deserialize;
use tokio::task::spawn;
use tokio::time::sleep;

lazy_static::lazy_static! {
    static ref TOKEN_HTTP_CLIENT: reqwest::Client = reqwest::Client::new();
}

#[derive(Deserialize, Debug, PartialEq, Eq, Hash, Clone)]
pub struct Token {
    pub id: String,
    pub workspace_id: String,
    pub client_secret: String,
    pub server_secret: String,
    pub origins: Vec<String>,
    #[serde(default = "default_identity_verification_mode")]
    pub identity_verification_mode: String,
}

fn default_identity_verification_mode() -> String {
    "report_only".to_string()
}

#[derive(Deserialize, Debug)]
struct Tokens {
    tokens: Vec<Token>,
}

pub struct HttpTokens {
    tokens: ArcSwap<Vec<Token>>,
}

impl HttpTokens {
    pub fn from_tokens(tokens: Vec<Token>) -> Self {
        Self {
            tokens: ArcSwap::from_pointee(tokens),
        }
    }

    pub fn snapshot(&self) -> Arc<Vec<Token>> {
        self.tokens.load_full()
    }

    pub fn len(&self) -> usize {
        self.tokens.load().len()
    }

    pub(crate) fn replace(&self, tokens: Vec<Token>) {
        self.tokens.store(Arc::new(tokens));
    }

    pub async fn new() -> Arc<Self> {
        tracing::info!("Loading authorization tokens");

        // Retry with backoff on startup (3 attempts: 2s, 5s, 10s)
        let backoff_delays = [
            Duration::from_secs(2),
            Duration::from_secs(5),
            Duration::from_secs(10),
        ];
        let mut tokens: Option<Vec<Token>> = None;

        for (attempt, delay) in backoff_delays.iter().enumerate() {
            match Self::fetch_tokens().await {
                Ok(t) if !t.is_empty() => {
                    tokens = Some(t);
                    break;
                }
                Ok(_) => {
                    tracing::warn!(
                        "Token fetch attempt {} returned empty tokens, retrying in {:?}",
                        attempt + 1,
                        delay
                    );
                }
                Err(err) => {
                    tracing::warn!(
                        "Token fetch attempt {} failed: {:?}, retrying in {:?}",
                        attempt + 1,
                        err,
                        delay
                    );
                }
            }
            if attempt < backoff_delays.len() - 1 {
                sleep(*delay).await;
            }
        }

        // Last attempt if all retries failed
        let tokens = match tokens {
            Some(t) => t,
            None => match Self::fetch_tokens().await {
                Ok(t) => {
                    if t.is_empty() {
                        tracing::error!(
                            "All token fetch attempts returned empty. Starting with no tokens."
                        );
                    }
                    t
                }
                Err(err) => {
                    tracing::error!(
                        "All token fetch attempts failed: {:?}. Starting with no tokens.",
                        err
                    );
                    vec![]
                }
            },
        };

        tracing::info!("Tokens loaded: {:?} tokens", tokens.len());
        let tokens_list = Arc::new(HttpTokens::from_tokens(tokens));
        let cloned_tokens = Arc::clone(&tokens_list);
        spawn(Self::update_tokens(cloned_tokens));
        tokens_list
    }

    async fn update_tokens(tokens_list: Arc<Self>) {
        tracing::debug!("Running background task to update tokens every 10 seconds.");
        let mut etag: Option<String> = None;
        let mut last_success = Instant::now();
        loop {
            sleep(Duration::from_secs(10)).await;
            let refresh_started = Instant::now();
            metrics::gauge!(
                "token_registry_refresh_age_seconds",
                last_success.elapsed().as_secs_f64()
            );
            let (new_tokens, next_etag) = match Self::fetch_tokens_with_etag(etag.as_deref()).await
            {
                Ok(result) => result,
                Err(err) => {
                    metrics::increment_counter!("token_registry_fetch_failures_total");
                    metrics::increment_counter!("token_registry_stale_retentions_total");
                    tracing::warn!(
                        "Failed to fetch new tokens: {:?}. Keeping stale tokens.",
                        err
                    );
                    metrics::histogram!(
                        "token_registry_refresh_duration_seconds",
                        refresh_started.elapsed().as_secs_f64(),
                        "result" => "failure"
                    );
                    continue;
                }
            };
            last_success = Instant::now();
            if let Some(next_etag) = next_etag {
                etag = Some(next_etag);
            }
            let Some(new_tokens) = new_tokens else {
                metrics::histogram!(
                    "token_registry_refresh_duration_seconds",
                    refresh_started.elapsed().as_secs_f64(),
                    "result" => "not_modified"
                );
                continue;
            };

            if new_tokens.is_empty() {
                metrics::increment_counter!("token_registry_stale_retentions_total");
                tracing::warn!("Token fetch returned empty list. Keeping stale tokens.");
                metrics::histogram!(
                    "token_registry_refresh_duration_seconds",
                    refresh_started.elapsed().as_secs_f64(),
                    "result" => "empty"
                );
                continue;
            }

            tracing::info!(
                "Authorization HTTP tokens updated. Total tokens: {:?}",
                new_tokens.len()
            );
            tokens_list.replace(new_tokens);
            metrics::histogram!(
                "token_registry_refresh_duration_seconds",
                refresh_started.elapsed().as_secs_f64(),
                "result" => "updated"
            );
        }
    }

    async fn fetch_tokens() -> Result<Vec<Token>, String> {
        let (tokens, _) = Self::fetch_tokens_with_etag(None).await?;
        Ok(tokens.unwrap_or_default())
    }

    async fn fetch_tokens_with_etag(
        etag: Option<&str>,
    ) -> Result<(Option<Vec<Token>>, Option<String>), String> {
        let url = env::var("HTTP_TOKENS_URL")
            .map_err(|_| "HTTP_TOKENS_URL env var is not set".to_string())?;

        let mut request = TOKEN_HTTP_CLIENT.get(&url);

        // Attach bearer token if INTERNAL_API_SECRET is set
        if let Ok(secret) = env::var("INTERNAL_API_SECRET") {
            request = request.bearer_auth(secret);
        }
        if let Some(etag) = etag {
            request = request.header(reqwest::header::IF_NONE_MATCH, etag);
        }

        let response = request
            .send()
            .await
            .map_err(|e| format!("HTTP request to {} failed: {}", url, e))?;

        if response.status() == reqwest::StatusCode::NOT_MODIFIED {
            return Ok((None, etag.map(ToString::to_string)));
        }
        if !response.status().is_success() {
            return Err(format!(
                "HTTP tokens endpoint returned status {}",
                response.status()
            ));
        }

        let response_etag = response
            .headers()
            .get(reqwest::header::ETAG)
            .and_then(|value| value.to_str().ok())
            .map(ToString::to_string);
        let tokens_response: Tokens = response
            .json()
            .await
            .map_err(|e| format!("Failed to parse tokens response: {}", e))?;

        tracing::debug!("Fetched {} tokens", tokens_response.tokens.len());
        Ok((Some(tokens_response.tokens), response_etag))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    static ENV_LOCK: tokio::sync::Mutex<()> = tokio::sync::Mutex::const_new(());

    struct FakeHttpTokens {
        tokens: Vec<Token>,
    }

    impl FakeHttpTokens {
        fn new(tokens: Vec<Token>) -> Self {
            FakeHttpTokens { tokens }
        }

        async fn fetch_tokens(&self) -> Result<Vec<Token>, ()> {
            Ok(self.tokens.clone())
        }
    }

    #[tokio::test]
    async fn test_new() {
        let fake_tokens = vec![Token {
            id: "1".to_string(),
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            client_secret: "secret".to_string(),
            server_secret: "secret".to_string(),
            origins: vec!["localhost".to_string()],
            identity_verification_mode: "report_only".to_string(),
        }];

        let fake_http_tokens = FakeHttpTokens::new(fake_tokens.clone());

        let http_tokens = Arc::new(HttpTokens::from_tokens(fake_tokens));

        let result = fake_http_tokens.fetch_tokens().await.unwrap();
        assert_eq!(result, *http_tokens.snapshot());
    }

    #[tokio::test]
    async fn test_fetch_tokens() {
        let fake_tokens = vec![Token {
            id: "1".to_string(),
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            client_secret: "secret".to_string(),
            server_secret: "secret".to_string(),
            origins: vec!["localhost".to_string()],
            identity_verification_mode: "report_only".to_string(),
        }];

        let fake_http_tokens = FakeHttpTokens::new(fake_tokens.clone());

        let result = fake_http_tokens.fetch_tokens().await.unwrap();
        assert_eq!(result, fake_tokens);
    }

    // --- Tests for real fetch_tokens error handling ---

    #[tokio::test]
    async fn test_fetch_tokens_missing_env_var() {
        let _guard = ENV_LOCK.lock().await;
        // Ensure HTTP_TOKENS_URL is not set for this test
        env::remove_var("HTTP_TOKENS_URL");

        let result = HttpTokens::fetch_tokens().await;
        assert!(result.is_err(), "Should error when HTTP_TOKENS_URL not set");
        assert!(
            result.unwrap_err().contains("HTTP_TOKENS_URL"),
            "Error should mention the missing env var"
        );
    }

    #[tokio::test]
    async fn test_fetch_tokens_http_server_error() {
        let _guard = ENV_LOCK.lock().await;
        let mut server = mockito::Server::new_async().await;
        let mock = server
            .mock("GET", "/tokens")
            .with_status(500)
            .with_body("Internal Server Error")
            .create_async()
            .await;

        env::set_var("HTTP_TOKENS_URL", format!("{}/tokens", server.url()));

        let result = HttpTokens::fetch_tokens().await;
        assert!(result.is_err(), "Should error on 500 response");
        assert!(
            result.unwrap_err().contains("500"),
            "Error should contain the status code"
        );

        mock.assert_async().await;
        env::remove_var("HTTP_TOKENS_URL");
    }

    #[tokio::test]
    async fn test_fetch_tokens_http_not_found() {
        let _guard = ENV_LOCK.lock().await;
        let mut server = mockito::Server::new_async().await;
        let mock = server
            .mock("GET", "/tokens")
            .with_status(404)
            .create_async()
            .await;

        env::set_var("HTTP_TOKENS_URL", format!("{}/tokens", server.url()));

        let result = HttpTokens::fetch_tokens().await;
        assert!(result.is_err(), "Should error on 404 response");

        mock.assert_async().await;
        env::remove_var("HTTP_TOKENS_URL");
    }

    #[tokio::test]
    async fn test_fetch_tokens_invalid_json_response() {
        let _guard = ENV_LOCK.lock().await;
        let mut server = mockito::Server::new_async().await;
        let mock = server
            .mock("GET", "/tokens")
            .with_status(200)
            .with_header("content-type", "application/json")
            .with_body("this is not json")
            .create_async()
            .await;

        env::set_var("HTTP_TOKENS_URL", format!("{}/tokens", server.url()));

        let result = HttpTokens::fetch_tokens().await;
        assert!(result.is_err(), "Should error on invalid JSON response");
        assert!(
            result.unwrap_err().contains("parse"),
            "Error should mention parsing failure"
        );

        mock.assert_async().await;
        env::remove_var("HTTP_TOKENS_URL");
    }

    #[tokio::test]
    async fn test_fetch_tokens_success() {
        let _guard = ENV_LOCK.lock().await;
        let mut server = mockito::Server::new_async().await;
        let mock = server
            .mock("GET", "/tokens")
            .with_status(200)
            .with_header("content-type", "application/json")
            .with_body(r#"{"tokens": [{"id": "1", "workspace_id": "00000000-0000-0000-0000-000000000001", "client_secret": "cs1", "server_secret": "ss1", "origins": ["localhost"]}]}"#)
            .create_async()
            .await;

        env::set_var("HTTP_TOKENS_URL", format!("{}/tokens", server.url()));

        let result = HttpTokens::fetch_tokens().await;
        assert!(result.is_ok(), "Should succeed with valid response");
        let tokens = result.unwrap();
        assert_eq!(tokens.len(), 1);
        assert_eq!(tokens[0].id, "1");
        assert_eq!(tokens[0].client_secret, "cs1");
        assert_eq!(tokens[0].server_secret, "ss1");

        mock.assert_async().await;
        env::remove_var("HTTP_TOKENS_URL");
    }

    #[tokio::test]
    async fn test_fetch_tokens_empty_list() {
        let _guard = ENV_LOCK.lock().await;
        let mut server = mockito::Server::new_async().await;
        let mock = server
            .mock("GET", "/tokens")
            .with_status(200)
            .with_header("content-type", "application/json")
            .with_body(r#"{"tokens": []}"#)
            .create_async()
            .await;

        env::set_var("HTTP_TOKENS_URL", format!("{}/tokens", server.url()));

        let result = HttpTokens::fetch_tokens().await;
        assert!(result.is_ok(), "Empty tokens list is valid");
        assert_eq!(result.unwrap().len(), 0);

        mock.assert_async().await;
        env::remove_var("HTTP_TOKENS_URL");
    }
}
