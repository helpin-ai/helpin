# Rust-Capture API Documentation

## Overview

The Rust-Capture API is an event tracking service that captures, processes, and forwards analytics events. It supports both client-side and server-to-server event tracking.

## Endpoints

All endpoints accept POST requests and return the same response format:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/event` | POST | Single or batch event capture (client-side) |
| `/api/v1/events` | POST | Single or batch event capture (client-side) |
| `/api/v1/s2s/event` | POST | Server-to-server event capture |
| `/api/v1/s2s/events` | POST | Server-to-server event capture |
| `/api.:ignored` | POST | Legacy endpoint support |

## Authentication

Authentication is required for all endpoints. The API key can be provided in three ways:

1. **Query Parameter**: `?api_key=YOUR_API_KEY` or `?token=YOUR_API_KEY`
2. **HTTP Header**: `Authorization: Bearer YOUR_API_KEY`
3. **Request Body**: Include `api_key` field in the JSON payload

**Note**: If the token is provided via header/query and differs from the body's `api_key`, the header/query token will override the body value.

## Content Types

| Content-Type | Description |
|--------------|-------------|
| `application/json` | Standard JSON format (default) |
| `application/x-www-form-urlencoded` | Form data with base64-encoded `data` field |

## Request Format

Events can be sent as:
- **Single Event**: A JSON object containing one event
- **Batch Events**: A JSON array containing multiple events

### Example - Single Event
```json
{
  "api_key": "your_api_key",
  "event_type": "pageview",
  "user": {
    "id": "user123",
    "email": "user@example.com"
  },
  "url": "https://example.com/page"
}
```

### Example - Batch Events
```json
[
  {
    "api_key": "your_api_key",
    "event_type": "pageview",
    "user": {"id": "user123"}
  },
  {
    "api_key": "your_api_key",
    "event_type": "click",
    "user": {"id": "user123"}
  }
]
```

## Event Fields

### Core Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `api_key` | String | Yes | Your project API key (can be provided via query/header) |
| `event_type` | String | Yes | Type of event being tracked (e.g., "pageview", "click", "custom_event") |
| `user` | Object | Yes | User identification and attributes object |

### User Object Fields

The `user` object is a flexible HashMap that can contain any custom fields. Common fields include:

| Field | Type | Description |
|-------|------|-------------|
| `id` | String | Unique user identifier |
| `email` | String | User's email address |
| `first_name` | String | User's first name |
| `last_name` | String | User's last name |
| `created_at` | String | User account creation timestamp |
| Custom fields | Any | Any additional user attributes as key-value pairs |

### Company Object Fields (Optional)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `company` | Object | No | Company/organization information associated with the user |

The `company` object is a flexible HashMap that can contain:

| Field | Type | Description |
|-------|------|-------------|
| `id` | String | Unique company identifier |
| `name` | String | Company name |
| `created_at` | String | Company creation timestamp |
| Custom fields | Any | Any additional company attributes |

### Page Tracking Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | String | No | Full URL of the page |
| `page_title` | String | No | Title of the page |
| `referrer` | String | No | Referring URL (HTTP Referer) |
| `doc_path` | String | No | Document path component of the URL |
| `doc_host` | String | No | Document host/domain |
| `doc_search` | String | No | Query string parameters |
| `doc_encoding` | String | No | Document character encoding |

### Device & Browser Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_agent` | String | No | User agent string (auto-captured from headers if not provided) |
| `user_language` | String | No | User's browser language |
| `screen_resolution` | String | No | Screen resolution (e.g., "1920x1080") |
| `vp_size` | String | No | Viewport size (e.g., "1280x720") |

### UTM & Marketing Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `utm` | Object | No | UTM campaign parameters |

The `utm` object can contain:

| Field | Type | Description |
|-------|------|-------------|
| `source` | String | UTM source parameter |
| `medium` | String | UTM medium parameter |
| `campaign` | String | UTM campaign parameter |
| `term` | String | UTM term parameter |
| `content` | String | UTM content parameter |

### Click ID Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `click_id` | Object | No | Click tracking IDs from various platforms |

The `click_id` object can contain:

| Field | Type | Description |
|-------|------|-------------|
| `gclid` | String | Google Click ID |
| `fbclid` | String | Facebook Click ID |

### Tracking IDs

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ids` | Object | No | Various third-party tracking identifiers |

The `ids` object can contain:

| Field | Type | Description |
|-------|------|-------------|
| `ga` | String | Google Analytics ID |
| `fbp` | String | Facebook Pixel ID |
| `ajs_anonymous_id` | String | Segment.io anonymous ID |
| `ajs_user_id` | String | Segment.io user ID |

### Custom Event Data

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `event_attributes` | Object | No | Custom attributes specific to this event |
| `autocapture_attributes` | Object | No | Automatically captured element attributes (for click events) |

Both objects are flexible HashMaps that can contain any custom key-value pairs.

### Technical Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source_ip` | String | No | User's IP address (auto-captured from `x-forwarded-for` header if not provided) |
| `utc_time` | String | No | UTC timestamp of when the event occurred |
| `timestamp` | Integer | No | Custom Unix timestamp for backfilling historical data. Only works with server-side tokens (tokens containing a dot). Accepts both seconds and milliseconds (values > 1,000,000,000,000 are treated as milliseconds). Must be after 1971-01-01. |
| `received_at` | String | Auto | Server timestamp when event was received (auto-generated) |
| `src` | String | No | Source/origin of the event |

### Privacy & Consent Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `cookie_policy` | String | No | Cookie consent policy status (can be passed as query parameter) |
| `ip_policy` | String | No | IP tracking policy status (can be passed as query parameter) |

## Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `api_key` | String | API key for authentication |
| `token` | String | Alternative to api_key |
| `cookie_policy` | String | Cookie consent policy |
| `ip_policy` | String | IP tracking policy |

## Request Headers

| Header | Description |
|--------|-------------|
| `Authorization` | Bearer token for authentication |
| `Content-Type` | Request content type (application/json or application/x-www-form-urlencoded) |
| `x-forwarded-for` | Client IP address (automatically captured) |
| `user-agent` | Client user agent (automatically captured) |
| `referer` | Referring URL (automatically captured) |

## Response Format

### Success Response

```json
{
  "status": 1
}
```

**HTTP Status Code**: `200 OK`

### Error Responses

| HTTP Status | Description |
|-------------|-------------|
| `400 Bad Request` | Invalid JSON payload or no events in batch |
| `401 Unauthorized` | Invalid or missing API key |

Error response format:
```json
"Error message describing the issue"
```

## Automatic Enrichment

The following fields are automatically enriched by the server if not provided:

1. **IP Address**: Extracted from `x-forwarded-for` header
2. **User Agent**: Extracted from `user-agent` header
3. **Referrer**: Extracted from `referer` header
4. **Received At**: Server timestamp when event is received
5. **Geolocation**: Enriched based on IP address (in processing pipeline)
6. **User Agent Parsing**: Device, browser, and OS information (in processing pipeline)
7. **Bot Detection**: Automatic bot detection (in processing pipeline)

## Backfilling Historical Data

To backfill historical events, use the `timestamp` field with the following requirements:

1. **Server-side token required**: The `timestamp` field only works with server-side API keys (tokens containing a dot, e.g., `UMYwi4UKqF.18954a1e-95fb-43d9-9808-fe828f85cad7`)
2. **Timestamp format**: Accepts Unix timestamps in either:
   - **Seconds**: Values ≤ 1,000,000,000,000 (e.g., `1609459200` for Jan 1, 2021)
   - **Milliseconds**: Values > 1,000,000,000,000 (e.g., `1609459200000` for Jan 1, 2021)
3. **Date validation**: Timestamp must represent a date after January 1, 1971
4. **Use S2S endpoints**: Always use server-to-server endpoints (`/api/v1/s2s/event` or `/api/v1/s2s/events`) for backfilling

### Example - Backfilling Event
```json
{
  "api_key": "your_server_secret.uuid-here",
  "event_type": "purchase",
  "timestamp": 1609459200000,
  "user": {
    "id": "user123",
    "email": "user@example.com"
  },
  "event_attributes": {
    "amount": 99.99,
    "product": "Premium Plan"
  }
}
```

## Best Practices

1. **Always include user identification**: Provide at least one user identifier (id, email, or anonymous_id)
2. **Use batch endpoints for multiple events**: Send events in batches to reduce network overhead
3. **Include UTM parameters**: For marketing attribution tracking
4. **Consistent event naming**: Use a standardized naming convention for event_type
5. **Custom attributes**: Use `event_attributes` for event-specific data
6. **Historical data**: Use the `timestamp` field with server-side tokens for backfilling
7. **Privacy compliance**: Always respect user consent via cookie_policy and ip_policy

## Rate Limiting

No explicit rate limits are documented, but it's recommended to:
- Batch events when possible
- Implement retry logic with exponential backoff
- Monitor response times and adjust accordingly

## Security Notes

1. **Server Secret**: Use server-to-server endpoints (`/api/v1/s2s/*`) with server secret tokens for backend events
2. **Client Secret**: Use client endpoints (`/api/v1/event*`) with client tokens for frontend events
3. **Token Security**: Never expose server secrets in client-side code
4. **CORS**: The API supports CORS for cross-origin requests
