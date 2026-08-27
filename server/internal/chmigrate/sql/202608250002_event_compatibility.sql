-- Migration: event_compatibility
ALTER TABLE helpin.events
    MODIFY COLUMN `parsed_ua_bot` Enum8('false' = 0, 'true' = 1, 'vpn' = 21, 'tor' = 22, 'dch' = 23, 'pub' = 24, 'web' = 25, 'ses' = 26)
    MATERIALIZED CAST(toInt8(JSONExtractInt(raw_event, 'parsed_ua_bot')), 'Enum8(\'false\' = 0, \'true\' = 1, \'vpn\' = 21, \'tor\' = 22, \'dch\' = 23, \'pub\' = 24, \'web\' = 25, \'ses\' = 26)')
    CODEC(T64, ZSTD(1));

ALTER TABLE helpin.events
    ADD COLUMN IF NOT EXISTS `parsed_referer_channel` LowCardinality(String)
    MATERIALIZED multiIf(empty(referer), 'Direct', multiSearchAnyCaseInsensitive(parsed_referer, ['google', 'bing', 'yahoo', 'duckduckgo']) > 0, 'search', multiSearchAnyCaseInsensitive(parsed_referer, ['facebook', 'instagram', 'linkedin', 'twitter', 'tiktok', 'reddit']) > 0, 'social', 'referral')
    CODEC(ZSTD(1)) AFTER parsed_referer;

ALTER TABLE helpin.events
    ADD COLUMN IF NOT EXISTS `parsed_referer_source` LowCardinality(String)
    MATERIALIZED if(empty(referer), 'Direct', parsed_referer)
    CODEC(ZSTD(1)) AFTER parsed_referer_channel;

ALTER TABLE helpin.events MATERIALIZE COLUMN parsed_referer_channel;
ALTER TABLE helpin.events MATERIALIZE COLUMN parsed_referer_source;
