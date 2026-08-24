CREATE DATABASE IF NOT EXISTS usermaven;

CREATE TABLE IF NOT EXISTS usermaven.events
(
    `_timestamp` DateTime64(3, 'UTC'),
    `_partition` UInt64,
    `_offset` UInt64,
    `api_key` String,
    `event_id` String,
    `event_type` LowCardinality(String),
    `project_id` String,
    `user_anonymous_id` String,
    `user_id` String,
    `user_email` String,
    `company_id` String,
    `identity_method` LowCardinality(String),
    `identity_trust` LowCardinality(String),
    `identity_verified_at` Nullable(DateTime64(3, 'UTC')),
    `identity_verifier_version` String,
    `session_id` String,
    `doc_path` String,
    `url` String,
    `event_attributes` String,
    `utm_source` String,
    `click_id_gclid` String,
    `raw_event` String
)
ENGINE = ReplacingMergeTree(_offset)
PARTITION BY toYYYYMM(_timestamp)
ORDER BY (project_id, toDate(_timestamp), event_type, user_anonymous_id, session_id, event_id)
TTL _timestamp + INTERVAL 400 DAY DELETE;

CREATE TABLE IF NOT EXISTS usermaven.events_queue
(
    `raw_event` String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'kafka:29092',
    kafka_topic_list = 'helpin.events.sessionized',
    kafka_group_name = 'helpin-clickhouse-local-v1',
    kafka_format = 'JSONAsString',
    kafka_num_consumers = 1;

CREATE MATERIALIZED VIEW IF NOT EXISTS usermaven.events_queue_mv
TO usermaven.events
AS SELECT
    parseDateTime64BestEffortOrZero(JSONExtractString(raw_event, 'timestamp'), 3, 'UTC') AS _timestamp,
    toUInt64(_partition) AS _partition,
    toUInt64(_offset) AS _offset,
    JSONExtractString(raw_event, 'api_key') AS api_key,
    JSONExtractString(raw_event, 'event_id') AS event_id,
    JSONExtractString(raw_event, 'event_type') AS event_type,
    lower(JSONExtractString(raw_event, 'project_id')) AS project_id,
    JSONExtractString(raw_event, 'user_anonymous_id') AS user_anonymous_id,
    JSONExtractString(raw_event, 'user_id') AS user_id,
    JSONExtractString(raw_event, 'user_email') AS user_email,
    JSONExtractString(raw_event, 'company_id') AS company_id,
    JSONExtractString(raw_event, 'identity_method') AS identity_method,
    JSONExtractString(raw_event, 'identity_trust') AS identity_trust,
    parseDateTime64BestEffortOrNull(JSONExtractString(raw_event, 'identity_verified_at'), 3, 'UTC') AS identity_verified_at,
    JSONExtractString(raw_event, 'identity_verifier_version') AS identity_verifier_version,
    JSONExtractString(raw_event, 'session_id') AS session_id,
    JSONExtractString(raw_event, 'doc_path') AS doc_path,
    JSONExtractString(raw_event, 'url') AS url,
    JSONExtractString(raw_event, 'event_attributes') AS event_attributes,
    JSONExtractString(raw_event, 'utm_source') AS utm_source,
    JSONExtractString(raw_event, 'click_id_gclid') AS click_id_gclid,
    raw_event
FROM usermaven.events_queue;
