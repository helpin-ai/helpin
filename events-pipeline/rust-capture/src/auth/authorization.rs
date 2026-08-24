use std::collections::BTreeMap;
use std::error::Error;
use std::fmt::Display;
use std::sync::{Arc, Mutex};

use crate::auth::http_tokens::{HttpTokens, Token};
use axum::http::HeaderMap;
use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum CredentialKind {
    Browser,
    Server,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct AuthorizedCredential {
    pub workspace_id: String,
    pub credential_kind: CredentialKind,
    pub installation_id: String,
    pub allowed_origins: Vec<String>,
    pub identity_verification_mode: String,
    #[serde(skip)]
    pub signing_secret: String,
    #[serde(skip)]
    pub browser_key: String,
}

// This enum is used to define possible reasons for a token to be invalid.
#[derive(Debug, PartialEq)]
pub enum InvalidTokenReason {
    Empty,
    ApiKey,
    Token,
}

impl InvalidTokenReason {
    // Returns a string description of the invalid token reason.
    pub fn reason(&self) -> &str {
        match *self {
            Self::Empty => "API Key or Server Side Token is empty",
            Self::ApiKey => "API key is not valid",
            Self::Token => "Token is not valid",
        }
    }
}

// This allows us to print an InvalidTokenReason directly.
impl Display for InvalidTokenReason {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "{}", self.reason())
    }
}

// This allows us to use an InvalidTokenReason as an Error.
impl Error for InvalidTokenReason {
    fn description(&self) -> &str {
        self.reason()
    }
}

// This enum represents constants we use to extract tokens.
pub enum ExtractTokenConst {
    ApiKey,
    Token,
    TokenHeaderName,
    RandomizeAPIKey,
}

impl ExtractTokenConst {
    // Returns the string representation of the constant.
    pub fn as_str(&self) -> &'static str {
        match *self {
            ExtractTokenConst::ApiKey => "api_key",
            ExtractTokenConst::Token => "token",
            ExtractTokenConst::TokenHeaderName => "x-auth-token",
            ExtractTokenConst::RandomizeAPIKey => "p_",
        }
    }
}
// This function is used to extract token based on api_key or _token.
pub fn extract_token_by_key(
    params: BTreeMap<std::string::String, std::string::String>,
    headers: HeaderMap,
) -> Option<String> {
    // Initialize the variables we'll use to store the api_key and _token.
    let mut api_key = String::new();
    let mut _token = String::new();

    // Check the parameters to see if we have an api_key or token.
    for (k, v) in params {
        if k == ExtractTokenConst::ApiKey.as_str() {
            api_key = v.clone();
        } else if k == ExtractTokenConst::Token.as_str()
            || k.starts_with(ExtractTokenConst::RandomizeAPIKey.as_str())
        {
            _token = v.clone();
        }
    }

    // Check the headers to see if we have a token.
    if let Some(token) = headers.get(ExtractTokenConst::TokenHeaderName.as_str()) {
        if let Ok(s) = token.to_str() {
            _token = s.to_string();
        }
    }

    // Return the token if it exists, otherwise return the api_key.
    if !_token.is_empty() {
        Some(_token)
    } else if !api_key.is_empty() {
        Some(api_key)
    } else {
        None
    }
}

// This function is used to validate tokens.
pub fn validate_token(
    params: BTreeMap<std::string::String, std::string::String>,
    headers: HeaderMap,
    tokens: Arc<Mutex<HttpTokens>>,
) -> Result<AuthorizedCredential, InvalidTokenReason> {
    // Initialize the variables we'll use to store the api_key and _token.
    let mut api_key = String::new();
    let mut _token = String::new();

    // Check the parameters to see if we have an api_key or token.
    for (k, v) in params {
        if k == ExtractTokenConst::ApiKey.as_str() {
            api_key = v.clone();
        } else if k == ExtractTokenConst::Token.as_str()
            || k.starts_with(ExtractTokenConst::RandomizeAPIKey.as_str())
        {
            _token = v.clone();
        }
    }

    // Check the headers to see if we have a token.
    if let Some(token) = headers.get(ExtractTokenConst::TokenHeaderName.as_str()) {
        if let Ok(s) = token.to_str() {
            _token = s.to_string();
        }
    }

    // Lock the mutex only once for efficiency.
    let tokens_guard = match tokens.lock() {
        Ok(guard) => guard,
        Err(e) => {
            tracing::error!("Failed to acquire lock on tokens: {:?}", e);
            return Err(InvalidTokenReason::Token);
        }
    };

    let effective = if !_token.is_empty() {
        &_token
    } else {
        &api_key
    };
    if effective.is_empty() {
        return Err(InvalidTokenReason::Empty);
    }

    let mut matched: Option<(&Token, CredentialKind)> = None;
    for token in &tokens_guard.tokens {
        let kind = if token.client_secret == *effective {
            Some(CredentialKind::Browser)
        } else if token.server_secret == *effective {
            Some(CredentialKind::Server)
        } else {
            None
        };
        let Some(kind) = kind else { continue };
        if let Some((previous, _)) = matched {
            if previous.workspace_id != token.workspace_id {
                return Err(InvalidTokenReason::Token);
            }
        }
        matched = Some((token, kind));
    }

    match matched {
        Some((token, credential_kind)) if !token.workspace_id.trim().is_empty() => {
            Ok(AuthorizedCredential {
                workspace_id: token.workspace_id.to_lowercase(),
                credential_kind,
                installation_id: token.id.clone(),
                allowed_origins: token.origins.clone(),
                identity_verification_mode: token.identity_verification_mode.clone(),
                signing_secret: token.server_secret.clone(),
                browser_key: token.client_secret.clone(),
            })
        }
        Some(_) => Err(InvalidTokenReason::Token),
        None if !api_key.is_empty() => Err(InvalidTokenReason::ApiKey),
        None => Err(InvalidTokenReason::Token),
    }
}

#[cfg(test)]
mod tests {
    use crate::auth::http_tokens::Token;

    use super::*;
    use axum::http::header::HeaderValue;
    use axum::http::HeaderMap;
    use std::collections::BTreeMap;
    use std::sync::{Arc, Mutex};

    #[test]
    fn test_validate_token() {
        let mut params = BTreeMap::new();
        params.insert("api_key".to_string(), "secret_api_key".to_string());
        params.insert("token".to_string(), "secret_token".to_string());

        let mut headers = HeaderMap::new();
        headers.insert("x-auth-token", HeaderValue::from_static("secret_token"));

        let token = Token {
            id: "1".to_string(),
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_token".to_string(),
            origins: vec!["localhost".to_string()],
            identity_verification_mode: "report_only".to_string(),
        };

        let tokens = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![token],
        }));

        let authorized = validate_token(params, headers.clone(), tokens).unwrap();
        assert_eq!(authorized.credential_kind, CredentialKind::Server);
        assert_eq!(
            authorized.workspace_id,
            "00000000-0000-0000-0000-000000000001"
        );
    }

    #[test]
    fn test_validate_token_invalid_api_key_and_token() {
        let mut params = BTreeMap::new();
        params.insert("api_key".to_string(), "invalid_api_key".to_string());
        params.insert("token".to_string(), "secret_token".to_string());

        let mut headers = HeaderMap::new();
        headers.insert("x-auth-token", HeaderValue::from_static("secret_token"));

        let token = Token {
            id: "1".to_string(),
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_toaken".to_string(),
            origins: vec!["localhost".to_string()],
            identity_verification_mode: "report_only".to_string(),
        };

        let tokens = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![token],
        }));

        assert_eq!(
            validate_token(params, headers.clone(), tokens).unwrap_err(),
            InvalidTokenReason::ApiKey
        );
    }

    #[test]
    fn test_validate_token_invalid_token() {
        let mut params = BTreeMap::new();
        params.insert("api_key".to_string(), "secret_api_key".to_string());
        params.insert("token".to_string(), "invalid_token".to_string());

        let mut headers = HeaderMap::new();
        headers.insert("x-auth-token", HeaderValue::from_static("invalid_token"));

        let token = Token {
            id: "1".to_string(),
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_token".to_string(),
            origins: vec!["localhost".to_string()],
            identity_verification_mode: "report_only".to_string(),
        };

        let tokens = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![token],
        }));

        assert_eq!(
            validate_token(params, headers.clone(), tokens).unwrap_err(),
            InvalidTokenReason::ApiKey
        );
    }

    #[test]
    fn test_validate_token_empty() {
        let params = BTreeMap::new();
        let headers = HeaderMap::new();

        let token = Token {
            id: "1".to_string(),
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_token".to_string(),
            origins: vec!["localhost".to_string()],
            identity_verification_mode: "report_only".to_string(),
        };

        let tokens = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![token],
        }));

        assert_eq!(
            validate_token(params, headers.clone(), tokens).unwrap_err(),
            InvalidTokenReason::Empty
        );
    }

    #[test]
    fn test_extract_token_by_key_from_params() {
        let mut params = BTreeMap::new();
        params.insert(
            String::from(ExtractTokenConst::ApiKey.as_str()),
            String::from("test_api_key"),
        );
        params.insert(
            String::from(ExtractTokenConst::Token.as_str()),
            String::from("test_token"),
        );

        let headers = HeaderMap::new();

        assert_eq!(extract_token_by_key(params, headers).unwrap(), "test_token");
    }

    #[test]
    fn test_extract_token_by_key_from_headers() {
        let params = BTreeMap::new();

        let mut headers = HeaderMap::new();
        headers.insert(
            ExtractTokenConst::TokenHeaderName.as_str(),
            "test_token".parse().unwrap(),
        );

        assert_eq!(extract_token_by_key(params, headers).unwrap(), "test_token");
    }

    #[test]
    fn test_extract_token_by_key_no_token() {
        let params = BTreeMap::new();
        let headers = HeaderMap::new();

        assert_eq!(extract_token_by_key(params, headers), None);
    }

    #[test]
    fn test_extract_token_by_key_api_key_only() {
        let mut params = BTreeMap::new();
        params.insert(
            String::from(ExtractTokenConst::ApiKey.as_str()),
            String::from("test_api_key"),
        );

        let headers = HeaderMap::new();

        assert_eq!(
            extract_token_by_key(params, headers).unwrap(),
            "test_api_key"
        );
    }

    // --- New tests for graceful error handling ---

    #[test]
    fn test_extract_token_non_utf8_header_ignored() {
        // Non-UTF-8 header values should be silently ignored, falling back to params
        let mut params = BTreeMap::new();
        params.insert("api_key".to_string(), "param_key".to_string());

        let mut headers = HeaderMap::new();
        // Insert a header with bytes that are valid for HTTP but not valid UTF-8 string
        headers.insert(
            ExtractTokenConst::TokenHeaderName.as_str(),
            HeaderValue::from_bytes(&[0x80, 0x81]).unwrap(),
        );

        // Should fall back to api_key from params since header is non-UTF-8
        let result = extract_token_by_key(params, headers);
        assert_eq!(result, Some("param_key".to_string()));
    }

    #[test]
    fn test_validate_token_non_utf8_header_ignored() {
        let mut params = BTreeMap::new();
        params.insert("api_key".to_string(), "secret_api_key".to_string());

        let mut headers = HeaderMap::new();
        // Non-UTF-8 x-auth-token header
        headers.insert(
            ExtractTokenConst::TokenHeaderName.as_str(),
            HeaderValue::from_bytes(&[0x80, 0x81]).unwrap(),
        );

        let token = Token {
            id: "1".to_string(),
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_token".to_string(),
            origins: vec!["localhost".to_string()],
            identity_verification_mode: "report_only".to_string(),
        };

        let tokens = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![token],
        }));

        // Should still validate via api_key param, ignoring the bad header
        assert!(validate_token(params, headers, tokens).is_ok());
    }

    #[test]
    fn test_validate_token_poisoned_mutex_returns_error() {
        let params = BTreeMap::new();
        let headers = HeaderMap::new();

        let token = Token {
            id: "1".to_string(),
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_token".to_string(),
            origins: vec!["localhost".to_string()],
            identity_verification_mode: "report_only".to_string(),
        };

        let tokens = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![token],
        }));

        // Poison the mutex by panicking while holding the lock
        let tokens_clone = tokens.clone();
        let _ = std::panic::catch_unwind(|| {
            let _guard = tokens_clone.lock().unwrap();
            panic!("intentional panic to poison mutex");
        });

        // Mutex is now poisoned — validate_token should return error, not panic
        let result = validate_token(params, headers, tokens);
        assert!(
            result.is_err(),
            "Poisoned mutex should return error, not panic"
        );
    }

    #[test]
    fn test_extract_token_with_randomized_api_key_prefix() {
        let mut params = BTreeMap::new();
        params.insert("p_abc123".to_string(), "randomized_token".to_string());

        let headers = HeaderMap::new();

        let result = extract_token_by_key(params, headers);
        assert_eq!(result, Some("randomized_token".to_string()));
    }

    #[test]
    fn test_validate_token_via_server_secret_as_api_key() {
        // Tests the found_api_key_in_server_secret path
        let mut params = BTreeMap::new();
        params.insert("api_key".to_string(), "server_secret_value".to_string());

        let headers = HeaderMap::new();

        let token = Token {
            id: "1".to_string(),
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            client_secret: "client_key".to_string(),
            server_secret: "server_secret_value".to_string(),
            origins: vec![],
            identity_verification_mode: "report_only".to_string(),
        };

        let tokens = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![token],
        }));

        let authorized = validate_token(params, headers, tokens).unwrap();
        assert_eq!(authorized.credential_kind, CredentialKind::Server);
        assert_eq!(
            authorized.workspace_id,
            "00000000-0000-0000-0000-000000000001"
        );
    }

    #[test]
    fn punctuation_in_browser_key_does_not_grant_server_privileges() {
        let mut params = BTreeMap::new();
        params.insert("api_key".to_string(), "browser.key.with.dots".to_string());
        let token = Token {
            id: "1".to_string(),
            workspace_id: "00000000-0000-0000-0000-000000000001".to_string(),
            client_secret: "browser.key.with.dots".to_string(),
            server_secret: "server-secret".to_string(),
            origins: vec![],
            identity_verification_mode: "report_only".to_string(),
        };
        let tokens = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![token],
        }));

        let authorized = validate_token(params, HeaderMap::new(), tokens).unwrap();
        assert_eq!(authorized.credential_kind, CredentialKind::Browser);
    }
}
