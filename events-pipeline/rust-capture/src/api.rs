use std::collections::HashMap;

use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Debug, Deserialize, Serialize)]
pub struct CaptureRequest {
    #[serde(alias = "$token", alias = "api_key")]
    pub token: String,

    pub event: String,
    pub properties: HashMap<String, Value>,
}

#[derive(Debug, PartialEq, Eq, Deserialize, Serialize)]
pub enum CaptureResponseCode {
    Ok = 1,
}

#[derive(Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct CaptureResponse {
    pub status: CaptureResponseCode,
}

#[derive(Debug)]
pub enum CaptureError {
    RequestDecodingError(String),
    EmptyBatch,
    NoTokenError,
    TokenValidationError,
    RetryableSinkError,
    NonRetryableSinkError(String),
    EventTooBig(String),
    PayloadTooLarge,
}

impl CaptureError {
    pub fn to_metric_tag(&self) -> &'static str {
        match self {
            CaptureError::RequestDecodingError(_) => "request_decoding_error",
            CaptureError::EmptyBatch => "empty_batch",
            CaptureError::NoTokenError => "no_token",
            CaptureError::TokenValidationError => "token_validation_error",
            CaptureError::RetryableSinkError => "retryable_sink_error",
            CaptureError::NonRetryableSinkError(_) => "non_retryable_sink_error",
            CaptureError::EventTooBig(_) => "event_too_big",
            CaptureError::PayloadTooLarge => "payload_too_large",
        }
    }
}

impl std::fmt::Display for CaptureError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            CaptureError::RequestDecodingError(msg) => write!(f, "Request decoding error: {}", msg),
            CaptureError::EmptyBatch => write!(f, "No events in batch"),
            CaptureError::NoTokenError => write!(f, "No API key provided"),
            CaptureError::TokenValidationError => write!(f, "API key is not valid"),
            CaptureError::RetryableSinkError => {
                write!(f, "Service temporarily unavailable, please retry")
            }
            CaptureError::NonRetryableSinkError(msg) => write!(f, "Sink error: {}", msg),
            CaptureError::EventTooBig(msg) => write!(f, "Event too big: {}", msg),
            CaptureError::PayloadTooLarge => write!(f, "Payload too large"),
        }
    }
}

impl IntoResponse for CaptureError {
    fn into_response(self) -> Response {
        let status = match &self {
            CaptureError::RequestDecodingError(_) => StatusCode::BAD_REQUEST,
            CaptureError::EmptyBatch => StatusCode::BAD_REQUEST,
            CaptureError::NoTokenError => StatusCode::UNAUTHORIZED,
            CaptureError::TokenValidationError => StatusCode::UNAUTHORIZED,
            CaptureError::RetryableSinkError => StatusCode::SERVICE_UNAVAILABLE,
            CaptureError::NonRetryableSinkError(_) => StatusCode::BAD_REQUEST,
            CaptureError::EventTooBig(_) => StatusCode::PAYLOAD_TOO_LARGE,
            CaptureError::PayloadTooLarge => StatusCode::PAYLOAD_TOO_LARGE,
        };

        metrics::increment_counter!("capture_errors_total", "error" => self.to_metric_tag());

        (status, self.to_string()).into_response()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::http::StatusCode;

    // --- to_metric_tag tests ---

    #[test]
    fn test_metric_tag_request_decoding_error() {
        let err = CaptureError::RequestDecodingError("bad json".into());
        assert_eq!(err.to_metric_tag(), "request_decoding_error");
    }

    #[test]
    fn test_metric_tag_empty_batch() {
        assert_eq!(CaptureError::EmptyBatch.to_metric_tag(), "empty_batch");
    }

    #[test]
    fn test_metric_tag_no_token() {
        assert_eq!(CaptureError::NoTokenError.to_metric_tag(), "no_token");
    }

    #[test]
    fn test_metric_tag_token_validation() {
        assert_eq!(
            CaptureError::TokenValidationError.to_metric_tag(),
            "token_validation_error"
        );
    }

    #[test]
    fn test_metric_tag_retryable_sink() {
        assert_eq!(
            CaptureError::RetryableSinkError.to_metric_tag(),
            "retryable_sink_error"
        );
    }

    #[test]
    fn test_metric_tag_non_retryable_sink() {
        let err = CaptureError::NonRetryableSinkError("serialize fail".into());
        assert_eq!(err.to_metric_tag(), "non_retryable_sink_error");
    }

    #[test]
    fn test_metric_tag_event_too_big() {
        let err = CaptureError::EventTooBig("5MB".into());
        assert_eq!(err.to_metric_tag(), "event_too_big");
    }

    #[test]
    fn test_metric_tag_payload_too_large() {
        assert_eq!(
            CaptureError::PayloadTooLarge.to_metric_tag(),
            "payload_too_large"
        );
    }

    // --- Display tests ---

    #[test]
    fn test_display_request_decoding_error() {
        let err = CaptureError::RequestDecodingError("invalid utf8".into());
        assert_eq!(err.to_string(), "Request decoding error: invalid utf8");
    }

    #[test]
    fn test_display_empty_batch() {
        assert_eq!(CaptureError::EmptyBatch.to_string(), "No events in batch");
    }

    #[test]
    fn test_display_no_token() {
        assert_eq!(
            CaptureError::NoTokenError.to_string(),
            "No API key provided"
        );
    }

    #[test]
    fn test_display_token_validation() {
        assert_eq!(
            CaptureError::TokenValidationError.to_string(),
            "API key is not valid"
        );
    }

    #[test]
    fn test_display_retryable_sink() {
        assert_eq!(
            CaptureError::RetryableSinkError.to_string(),
            "Service temporarily unavailable, please retry"
        );
    }

    #[test]
    fn test_display_non_retryable_sink() {
        let err = CaptureError::NonRetryableSinkError("bad data".into());
        assert_eq!(err.to_string(), "Sink error: bad data");
    }

    #[test]
    fn test_display_event_too_big() {
        let err = CaptureError::EventTooBig("10MB".into());
        assert_eq!(err.to_string(), "Event too big: 10MB");
    }

    #[test]
    fn test_display_payload_too_large() {
        assert_eq!(
            CaptureError::PayloadTooLarge.to_string(),
            "Payload too large"
        );
    }

    // --- IntoResponse status code tests ---

    fn extract_status(err: CaptureError) -> StatusCode {
        let response = err.into_response();
        response.status()
    }

    #[test]
    fn test_status_request_decoding_error() {
        assert_eq!(
            extract_status(CaptureError::RequestDecodingError("x".into())),
            StatusCode::BAD_REQUEST
        );
    }

    #[test]
    fn test_status_empty_batch() {
        assert_eq!(
            extract_status(CaptureError::EmptyBatch),
            StatusCode::BAD_REQUEST
        );
    }

    #[test]
    fn test_status_no_token() {
        assert_eq!(
            extract_status(CaptureError::NoTokenError),
            StatusCode::UNAUTHORIZED
        );
    }

    #[test]
    fn test_status_token_validation() {
        assert_eq!(
            extract_status(CaptureError::TokenValidationError),
            StatusCode::UNAUTHORIZED
        );
    }

    #[test]
    fn test_status_retryable_sink() {
        assert_eq!(
            extract_status(CaptureError::RetryableSinkError),
            StatusCode::SERVICE_UNAVAILABLE
        );
    }

    #[test]
    fn test_status_non_retryable_sink() {
        assert_eq!(
            extract_status(CaptureError::NonRetryableSinkError("x".into())),
            StatusCode::BAD_REQUEST
        );
    }

    #[test]
    fn test_status_event_too_big() {
        assert_eq!(
            extract_status(CaptureError::EventTooBig("x".into())),
            StatusCode::PAYLOAD_TOO_LARGE
        );
    }

    #[test]
    fn test_status_payload_too_large() {
        assert_eq!(
            extract_status(CaptureError::PayloadTooLarge),
            StatusCode::PAYLOAD_TOO_LARGE
        );
    }
}
