-- Version 1 used evaluator windows as identification boundaries and must not
-- become effective again if a newer workspace override is later disabled.

UPDATE crm_signal_rule_configs
SET enabled = FALSE,
    updated_at = NOW()
WHERE rule_key = 'pre_identification_history'
  AND version = 1
  AND enabled = TRUE;
