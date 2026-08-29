INSERT INTO crm_signal_scoring_configs
    (workspace_id, version, enabled, heuristic, parameters)
SELECT NULL,
       2,
       true,
       heuristic,
       parameters || '{"source_correlation":"canonical_source_v1"}'::jsonb
FROM crm_signal_scoring_configs
WHERE workspace_id IS NULL
  AND version = 1
ON CONFLICT DO NOTHING;
