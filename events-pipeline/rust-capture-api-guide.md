# Event capture API reference

## Overview

Use this reference to send browser or server events to the capture service.
The [pipeline guide](README.md) explains tenancy, identity, and delivery. This
reference describes the checked-in handler, not the retired upstream API.

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

Capture authenticates against Helpin's token registry before decoding the body.
Supply a registered credential using `?api_key=...`, `?token=...`, or the
`x-auth-token` header. The header overrides the query token; a nonempty token
wins over `api_key`. Historical `p_*` query parameters are also accepted.

`Authorization: Bearer ...` and a body-only `api_key` do **not** authenticate this
handler. A request credential overrides the payload's `api_key` for every event.
The registry determines browser/server credential kind, installation, and
workspace; token punctuation and a caller-supplied `project_id` are not authority.

For browser integrations, prefer the [SDK](../packages/sdk-js/README.md), which
supplies the supported request format. Keep server credentials on your backend.

## Content Types

| Content-Type | Description |
|--------------|-------------|
| `application/json` | Standard JSON format (default) |
| `application/x-www-form-urlencoded` | Form data with base64-encoded `data` field |

## Request Format

Events can be sent as:
- **Single Event**: A JSON object containing one event
- **Batch Events**: A JSON array containing multiple events

The following examples are request bodies; send the credential separately as
described above.

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
| `api_key` | String | No in the body | Populated from the authenticated request credential; raw credentials are replaced before downstream storage |
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
| `referer` | String | No | Input field used to populate normalized `referrer`; the handler does not read the HTTP Referer header |
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
| `timestamp` | Integer | No | Server-credential event time in seconds or milliseconds; allowed range is now minus 7 days through now plus 1 hour. Browser events use receipt time. |
| `received_at` | String | Auto | Server timestamp when event was received (auto-generated) |
| `src` | String | No | Source/origin of the event |

### Privacy & Consent Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `cookie_policy` | String | No | Query parameter copied into each event; body-only values are overwritten |
| `ip_policy` | String | No | Query parameter copied into each event; body-only values are overwritten |

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
| `x-auth-token` | Registered credential; overrides query credentials |
| `Content-Type` | Request content type (application/json or application/x-www-form-urlencoded) |
| `x-forwarded-for` | Client IP address (automatically captured) |
| `user-agent` | Client user agent (automatically captured) |

## Response Format

### Success Response

```json
{
  "status": "Ok"
}
```

**HTTP Status Code**: `200 OK`

### Error Responses

| HTTP Status | Description |
|-------------|-------------|
| `400 Bad Request` | Invalid JSON payload or no events in batch |
| `401 Unauthorized` | Invalid or missing request credential |
| `403 Forbidden` | Browser credential attempted a server-only commercial event or company field |
| `413 Payload Too Large` | Request body or event exceeds its limit |
| `503 Service Unavailable` | Retryable sink failure |
| `504 Gateway Timeout` | Request exceeds the configured timeout |

Capture errors return a plain-text message, not a JSON string. See
[error mapping](rust-capture/src/api.rs) and [timeout middleware](rust-capture/src/metrics_recorder.rs).

## Automatic Enrichment

The following fields are automatically enriched by the server if not provided:

1. **IP Address**: Extracted from `x-forwarded-for` header
2. **User Agent**: Extracted from `user-agent` header
3. **Referrer**: Copied from the payload `referer` field
4. **Received At**: Server timestamp when event is received
5. **Geolocation**: Enriched based on IP address (in processing pipeline)
6. **User Agent Parsing**: Device, browser, and OS information (in processing pipeline)
7. **Bot Detection**: Automatic bot detection (in processing pipeline)

## Backfilling Historical Data

This endpoint accepts recent server events, not arbitrary historical imports.
The token registry must classify the credential as `server`. When a server event
supplies `timestamp`, enrichment accepts Unix seconds or milliseconds (absolute
values above 1,000,000,000,000 are interpreted as milliseconds) only within **now
minus 7 days through now plus 1 hour**. Browser credentials use receipt time.
Out-of-window timestamps fail enrichment rather than being clamped.

For a current server event, generate the timestamp at send time:

```sh
EVENT_TIMESTAMP=$(date +%s)
curl --fail-with-body 'http://localhost:3000/api/v1/s2s/event' \
  -H "x-auth-token: $HELPIN_SERVER_TOKEN" \
  -H 'Content-Type: application/json' \
  --data "{\"event_type\":\"purchase\",\"timestamp\":$EVENT_TIMESTAMP,\"user\":{\"id\":\"user123\"}}"
```

Set `HELPIN_SERVER_TOKEN` to an actual registered server credential and use the
capture origin for your environment. See [timestamp validation](rust-capture/src/pipeline.rs)
and [enrichment](rust-capture/src/enrichment/handler.rs) for the current bounds.

## Best Practices

1. **Always include user identification**: Provide at least one user identifier (id, email, or anonymous_id)
2. **Use batch endpoints for multiple events**: Send events in batches to reduce network overhead
3. **Include UTM parameters**: For marketing attribution tracking
4. **Consistent event naming**: Use a standardized naming convention for event_type
5. **Custom attributes**: Use `event_attributes` for event-specific data
6. **Recent server events**: Use `timestamp` only within the accepted time window
7. **Privacy compliance**: Always respect user consent via cookie_policy and ip_policy

## Rate Limiting

The router defaults to a 2 MiB request limit (`MAX_BODY_SIZE`) and the request
timeout defaults to 60 seconds (`REQUEST_TIMEOUT_SECS`). Ingress may impose
additional limits. For sustained ingestion:
- Batch events when possible
- Implement retry logic with exponential backoff
- Monitor response times and adjust accordingly

## Security Notes

1. **Server Secret**: Use server-to-server endpoints (`/api/v1/s2s/*`) with server secret tokens for backend events
2. **Client Secret**: Use client endpoints (`/api/v1/event*`) with client tokens for frontend events
3. **Token Security**: Never expose server secrets in client-side code
4. **CORS**: The API supports CORS for cross-origin requests
