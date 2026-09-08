-- SELECT-only recovery preview. No secret values or customer messages are read.
-- Existing ledger entries and posted charges remain in their original periods.
WITH ledger AS (
 SELECT period_id, SUM(final_charged_microusd) AS ledger_used,
        SUM(final_charged_microusd) FILTER (WHERE feature_key = 'ask_chat') AS ask_chat_used
 FROM billing_ai_usage_ledger GROUP BY period_id
), holds AS (
 SELECT period_id, COUNT(*) AS active_reservations, SUM(reserved_microusd) AS active_holds
 FROM billing_ai_usage_reservations WHERE status = 'active' GROUP BY period_id
)
SELECT w.slug, p.id AS period_id, p.period_start, p.period_end,
       p.enforcement_mode, p.allowance_microusd, p.used_microusd,
       COALESCE(l.ledger_used, 0) AS ledger_used_microusd,
       p.used_microusd - COALESCE(l.ledger_used, 0) AS counter_difference_microusd,
       COALESCE(l.ask_chat_used, 0) AS ask_chat_used_microusd,
       p.reserved_microusd, COALESCE(h.active_holds, 0) AS holds_to_transfer_microusd,
       COALESCE(h.active_reservations, 0) AS reservations_to_transfer,
       CASE WHEN p.enforcement_mode = 'extra' THEN p.overage_microusd ELSE 0 END AS settlement_to_snapshot_microusd,
       s.status AS existing_settlement_status,
       wb.current_period_start AS subscription_anchor,
       wb.current_period_end AS subscription_end
FROM billing_ai_usage_periods p
JOIN workspaces w ON w.id = p.workspace_id
JOIN workspace_billing wb ON wb.workspace_id = p.workspace_id
LEFT JOIN ledger l ON l.period_id = p.id
LEFT JOIN holds h ON h.period_id = p.id
LEFT JOIN billing_ai_usage_settlements s ON s.period_id = p.id
WHERE p.status = 'open' AND p.period_end <= now()
ORDER BY p.period_end, p.id;

-- Entries omitted by the former date-only billing report.
SELECT w.slug, p.id AS period_id, l.feature_key, COUNT(*) AS entries,
       SUM(l.final_charged_microusd) AS previously_hidden_microusd,
       MIN(l.created_at) AS first_posted_at, MAX(l.created_at) AS last_posted_at
FROM billing_ai_usage_ledger l
JOIN billing_ai_usage_periods p ON p.id = l.period_id
JOIN workspaces w ON w.id = l.workspace_id
WHERE l.entry_kind IN ('usage', 'estimate')
  AND (l.created_at < p.period_start OR l.created_at >= p.period_end)
GROUP BY w.slug, p.id, l.feature_key
ORDER BY w.slug, p.id, l.feature_key;
