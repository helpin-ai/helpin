ALTER TABLE crm_signal_rule_configs
    ADD COLUMN IF NOT EXISTS business_weight double precision NOT NULL DEFAULT 10,
    ADD COLUMN IF NOT EXISTS half_life_days double precision NOT NULL DEFAULT 30;

CREATE TABLE IF NOT EXISTS crm_signal_scoring_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid,
    version integer NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    heuristic boolean NOT NULL DEFAULT true,
    parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_signal_scoring_global_version
    ON crm_signal_scoring_configs (version) WHERE workspace_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_signal_scoring_workspace_version
    ON crm_signal_scoring_configs (workspace_id, version) WHERE workspace_id IS NOT NULL;

INSERT INTO crm_signal_scoring_configs (version, heuristic, parameters)
SELECT 1, true, '{
  "signal_weights": {"buying_intent": 15, "budget_signal": 12, "timeline_signal": 10, "champion_signal": 12, "objection": 12, "competitor_mention": 8, "risk_signal": 20},
  "half_lives_days": {"buying_intent": 7, "budget_signal": 21, "timeline_signal": 10, "champion_signal": 180, "objection": 30, "competitor_mention": 45, "risk_signal": 30},
  "domain_weights": {"conversation": 1, "web_behavior": 1.1, "product_usage": 1.1, "support": 1.15, "delivery": 1, "relationship": 1.1, "market": 0.7},
  "identity_trust": {"verified": 1, "probabilistic": 0.65, "untrusted": 0.35, "unknown": 0.5},
  "compound_window_days": 14,
  "compound_boost_per_domain": 0.15,
  "max_compound_boost": 0.45
}'::jsonb
WHERE NOT EXISTS (SELECT 1 FROM crm_signal_scoring_configs WHERE workspace_id IS NULL AND version = 1);

UPDATE crm_signal_rule_configs SET business_weight = CASE rule_key
    WHEN 'urgent_issue_open_deal' THEN 20 WHEN 'support_csat_deterioration' THEN 18
    WHEN 'deal_gone_dark' THEN 18 WHEN 'timeline_followup_lapsed' THEN 16
    WHEN 'repeated_pricing_activity' THEN 17 WHEN 'procurement_page_activity' THEN 18
    WHEN 'high_intent_product_event' THEN 17 WHEN 'requested_feature_shipped' THEN 14
    WHEN 'buying_committee_expanded' THEN 13 WHEN 'buying_committee_shrank' THEN 15
    ELSE 10 END,
    half_life_days = CASE rule_key
    WHEN 'repeated_pricing_activity' THEN 7 WHEN 'procurement_page_activity' THEN 10
    WHEN 'known_contact_returned' THEN 7 WHEN 'high_intent_product_event' THEN 7
    WHEN 'session_depth_spike' THEN 3 WHEN 'renewal_approaching' THEN 30
    WHEN 'champion_quiet' THEN 21 WHEN 'deal_gone_dark' THEN 21
    ELSE 30 END
WHERE version = 1;
