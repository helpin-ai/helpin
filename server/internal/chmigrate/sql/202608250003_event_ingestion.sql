-- Migration: event_ingestion
-- Narrow restart index for reconstructing active sessions without scanning the
-- wide events table. It intentionally retains one hour beyond the work stream.
CREATE TABLE IF NOT EXISTS usermaven.session_seed_events
(
    `visitor_shard` UInt8 CODEC(T64, ZSTD(1)),
    `event_received_at` DateTime64(3, 'UTC') CODEC(Delta(4), ZSTD(1)),
    `event_timestamp` DateTime64(3, 'UTC') CODEC(Delta(4), ZSTD(1)),
    `project_id` String CODEC(ZSTD(1)),
    `user_anonymous_id` String CODEC(ZSTD(1)),
    `session_id` String CODEC(ZSTD(1)),
    `event_id` String CODEC(ZSTD(1)),
    `nats_subject` String CODEC(ZSTD(1)),
    `nats_stream_sequence` UInt64 CODEC(T64, ZSTD(1)),
    `nats_delivery_attempt` UInt8 CODEC(T64, ZSTD(1)),
    `retro_generation` UInt8 CODEC(T64, ZSTD(1)),
    `ingest_version` UInt64 CODEC(T64, ZSTD(1)),

    INDEX `idx_seed_event_timestamp` event_timestamp TYPE minmax GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY toDate(event_received_at)
ORDER BY (visitor_shard, event_received_at, project_id, user_anonymous_id, event_id)
TTL event_received_at + INTERVAL 49 HOUR DELETE
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS usermaven.session_seed_events_mv
TO usermaven.session_seed_events
AS SELECT
    toUInt8(JSONExtractUInt(raw_event, 'visitor_shard')) AS visitor_shard,
    parseDateTime64BestEffort(JSONExtractString(raw_event, 'event_received_at'), 3, 'UTC') AS event_received_at,
    parseDateTime64BestEffort(JSONExtractString(raw_event, 'timestamp'), 3, 'UTC') AS event_timestamp,
    lower(JSONExtractString(raw_event, 'project_id')) AS project_id,
    JSONExtractString(raw_event, 'user_anonymous_id') AS user_anonymous_id,
    JSONExtractString(raw_event, 'session_id') AS session_id,
    JSONExtractString(raw_event, 'event_id') AS event_id,
    _nats_subject AS nats_subject,
    _nats_stream_sequence AS nats_stream_sequence,
    _nats_delivery_attempt AS nats_delivery_attempt,
    _retro_generation AS retro_generation,
    _ingest_version AS ingest_version
FROM usermaven.events
WHERE _retro_generation = 0;
