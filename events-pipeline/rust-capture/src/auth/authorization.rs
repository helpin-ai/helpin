use std::collections::{BTreeMap, HashSet};
use std::error::Error;
use std::fmt::Display;
use std::sync::{Arc, Mutex};

use crate::auth::http_tokens::HttpTokens;
use axum::http::HeaderMap;

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
) -> Result<(), InvalidTokenReason> {
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
            api_key = v.clone();
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

    // Convert the tokens list into a HashSet for efficient lookups.
    let token_set: HashSet<_> = tokens_guard.tokens.iter().collect();

    // Check if the api_key or _token is found in the tokens set.
    let found_api_key = token_set.iter().any(|token| token.client_secret == api_key);
    let found_token = token_set.iter().any(|token| token.server_secret == _token);
    let found_api_key_in_server_secret =
        token_set.iter().any(|token| token.server_secret == api_key);

    // If either the api_key or _token is found, set found to true.
    let found = found_api_key || found_token || found_api_key_in_server_secret;

    // If found is true, return Ok. Otherwise, return an appropriate error.
    match found {
        true => Ok(()),
        false if !api_key.is_empty() => Err(InvalidTokenReason::ApiKey),
        false if !_token.is_empty() => Err(InvalidTokenReason::Token),
        false => Err(InvalidTokenReason::Empty),
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
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_token".to_string(),
            origins: vec!["localhost".to_string()],
        };

        let tokens = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![token],
        }));

        assert_eq!(validate_token(params, headers.clone(), tokens).unwrap(), ());
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
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_toaken".to_string(),
            origins: vec!["localhost".to_string()],
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
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_token".to_string(),
            origins: vec!["localhost".to_string()],
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
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_token".to_string(),
            origins: vec!["localhost".to_string()],
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
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_token".to_string(),
            origins: vec!["localhost".to_string()],
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
            client_secret: "secret_api_key".to_string(),
            server_secret: "secret_token".to_string(),
            origins: vec!["localhost".to_string()],
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
            client_secret: "client_key".to_string(),
            server_secret: "server_secret_value".to_string(),
            origins: vec![],
        };

        let tokens = Arc::new(Mutex::new(HttpTokens {
            tokens: vec![token],
        }));

        // Should pass via found_api_key_in_server_secret
        assert!(validate_token(params, headers, tokens).is_ok());
    }
}
