-- Migration: ai_bot_classification
ALTER TABLE usermaven.events
    ADD COLUMN IF NOT EXISTS `parsed_ua_bot_category` LowCardinality(String)
    MATERIALIZED JSONExtractString(raw_event, 'parsed_ua_bot_category')
    CODEC(ZSTD(1)) AFTER parsed_ua_bot;

ALTER TABLE usermaven.events
    ADD COLUMN IF NOT EXISTS `parsed_ua_bot_name` LowCardinality(String)
    MATERIALIZED JSONExtractString(raw_event, 'parsed_ua_bot_name')
    CODEC(ZSTD(1)) AFTER parsed_ua_bot_category;

ALTER TABLE usermaven.events
    ADD COLUMN IF NOT EXISTS `parsed_ua_bot_provider` LowCardinality(String)
    MATERIALIZED JSONExtractString(raw_event, 'parsed_ua_bot_provider')
    CODEC(ZSTD(1)) AFTER parsed_ua_bot_name;

ALTER TABLE usermaven.events
    ADD INDEX IF NOT EXISTS `idx_parsed_ua_bot_category`
    parsed_ua_bot_category TYPE set(16) GRANULARITY 1;
