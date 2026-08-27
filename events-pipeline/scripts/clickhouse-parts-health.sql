WITH
    (
        SELECT toUInt64(value)
        FROM system.merge_tree_settings
        WHERE name = 'parts_to_throw_insert'
    ) AS default_throw_threshold
SELECT
    database,
    table,
    partition,
    count() AS active_parts,
    default_throw_threshold,
    round(active_parts / default_throw_threshold, 4) AS default_threshold_ratio,
    sum(rows) AS active_rows,
    formatReadableSize(sum(bytes_on_disk)) AS active_bytes
FROM system.parts
WHERE active
  AND database = 'helpin'
  AND table IN ('events', 'session_seed_events')
GROUP BY database, table, partition
ORDER BY active_parts DESC;

-- Alert on sustained growth before default_threshold_ratio reaches 0.5.
-- If SHOW CREATE TABLE reports a per-table parts_to_throw_insert override,
-- the monitoring rule must use that override instead of the default above.
