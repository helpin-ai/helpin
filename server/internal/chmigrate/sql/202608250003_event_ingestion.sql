-- Migration: event_ingestion
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
(`raw_event`, `_partition`, `_offset`, `_kafka_timestamp_ms`)
AS SELECT
    raw_event,
    toUInt64(_partition) AS _partition,
    toUInt64(_offset) AS _offset,
    _timestamp_ms AS _kafka_timestamp_ms
FROM usermaven.events_queue;
