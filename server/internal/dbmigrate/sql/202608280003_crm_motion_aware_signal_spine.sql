-- Motion-aware CRM signal spine. Legacy signal data is deliberately removed;
-- it has no trustworthy detection-time motion and the replacement starts fresh.

-- Detach non-FK arrays, then let the next evaluator pass repopulate the live
-- corpus from fresh evidence under the immutable interpretation contract.
UPDATE crm_suggestions SET signal_ids = '{}' WHERE cardinality(signal_ids) > 0;
UPDATE crm_signal_external_evidence SET signal_id = NULL WHERE signal_id IS NOT NULL;
DELETE FROM crm_signal_feedback;
DELETE FROM crm_signal_deliveries;
DELETE FROM crm_buyer_signals;

ALTER TABLE crm_pipelines
    ADD COLUMN IF NOT EXISTS default_commercial_motion text;
ALTER TABLE crm_deals ADD COLUMN IF NOT EXISTS commercial_motion text;

-- Seed one motion per existing pipeline. Deals inherit this value, so a renewal
-- or expansion pipeline does not require a per-deal backfill. Ambiguous names
-- deliberately remain new_business and can be corrected once in pipeline settings.
UPDATE crm_pipelines
SET default_commercial_motion = CASE
    WHEN name ~* '(renew|recurr|contract[[:space:]_-]*extension)' THEN 'renewal'
    WHEN name ~* '(expand|upsell|cross[[:space:]_-]*sell|growth)' THEN 'expansion'
    ELSE 'new_business'
END
WHERE default_commercial_motion IS NULL;

ALTER TABLE crm_pipelines ALTER COLUMN default_commercial_motion SET DEFAULT 'new_business';
ALTER TABLE crm_pipelines ALTER COLUMN default_commercial_motion SET NOT NULL;

ALTER TABLE crm_pipelines DROP CONSTRAINT IF EXISTS crm_pipelines_default_commercial_motion_check;
ALTER TABLE crm_pipelines ADD CONSTRAINT crm_pipelines_default_commercial_motion_check
    CHECK (default_commercial_motion IN ('new_business', 'expansion', 'renewal'));
ALTER TABLE crm_deals DROP CONSTRAINT IF EXISTS crm_deals_commercial_motion_check;
ALTER TABLE crm_deals ADD CONSTRAINT crm_deals_commercial_motion_check
    CHECK (commercial_motion IS NULL OR commercial_motion IN ('new_business', 'expansion', 'renewal'));

CREATE INDEX IF NOT EXISTS idx_crm_pipelines_default_commercial_motion
    ON crm_pipelines (workspace_id, default_commercial_motion);
CREATE INDEX IF NOT EXISTS idx_crm_deals_commercial_motion
    ON crm_deals (workspace_id, commercial_motion);

ALTER TABLE crm_companies ADD COLUMN IF NOT EXISTS customer_success_owner_member_id uuid;
CREATE INDEX IF NOT EXISTS idx_crm_companies_customer_success_owner
    ON crm_companies (workspace_id, customer_success_owner_member_id);

CREATE TABLE IF NOT EXISTS crm_signal_observations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    contact_id uuid,
    deal_id uuid,
    company_id uuid,
    rule_key text NOT NULL,
    rule_version integer NOT NULL,
    detector_kind text NOT NULL,
    signal_domain text NOT NULL,
    source_type text NOT NULL,
    source_id text,
    source_thread_id text,
    summary text NOT NULL,
    evidence_excerpt text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    confidence double precision NOT NULL,
    observed_at timestamptz NOT NULL,
    window_started_at timestamptz,
    window_ended_at timestamptz,
    identity_method text NOT NULL,
    identity_trust text NOT NULL,
    evidence_fingerprint text NOT NULL,
    motions_at_detection jsonb NOT NULL DEFAULT '{}'::jsonb,
    context_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    replay_calibration_excluded boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_signal_observation_evidence
    ON crm_signal_observations (workspace_id, rule_key, rule_version, evidence_fingerprint,
        COALESCE(contact_id::text, ''), COALESCE(deal_id::text, ''), COALESCE(company_id::text, ''));

CREATE TABLE IF NOT EXISTS crm_signal_interpretation_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid,
    rule_key text NOT NULL,
    rule_version integer NOT NULL,
    motion text NOT NULL,
    observation_signal_type text NOT NULL DEFAULT '*',
    version integer NOT NULL,
    signal_type text NOT NULL,
    polarity text NOT NULL,
    business_weight double precision NOT NULL,
    half_life_days double precision NOT NULL,
    recommended_action_key text,
    recommended_action_label text,
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_signal_interpretation_version
    ON crm_signal_interpretation_configs
    (COALESCE(workspace_id::text, ''), rule_key, rule_version, motion,
     observation_signal_type, version);

-- Only unambiguous mappings are seeded. Omission is intentional when the same
-- observation does not have a trustworthy action in that motion.
INSERT INTO crm_signal_interpretation_configs (
    workspace_id, rule_key, rule_version, motion, observation_signal_type,
    version, signal_type, polarity, business_weight, half_life_days,
    recommended_action_key, recommended_action_label, enabled
) VALUES
    (NULL, 'repeated_pricing_activity', 1, 'prospecting', '*', 1,
     'buying_intent', 'positive', 17, 7, 'contact_about_pricing', 'Follow up about pricing', true),
    (NULL, 'repeated_pricing_activity', 1, 'conversion', '*', 1,
     'buying_intent', 'positive', 17, 7, 'contact_about_pricing', 'Follow up about pricing', true),
    (NULL, 'repeated_pricing_activity', 1, 'expansion', '*', 1,
     'buying_intent', 'positive', 17, 7, 'review_upgrade_fit', 'Review upgrade fit', true),
    (NULL, 'repeated_pricing_activity', 1, 'retention', '*', 1,
     'risk_signal', 'negative', 20, 30, 'investigate_downgrade_risk', 'Investigate downgrade risk', true)
ON CONFLICT DO NOTHING;

-- Manual assertions are explicit context mappings rather than a code fallback.
-- They preserve the user-selected type/polarity and deliberately recommend no
-- automated action.
INSERT INTO crm_signal_interpretation_configs (
    workspace_id, rule_key, rule_version, motion, observation_signal_type,
    version, signal_type, polarity, business_weight, half_life_days,
    recommended_action_key, recommended_action_label, enabled
)
SELECT NULL, 'manual_signal', 1, motion, '*', 1, 'inherit', 'inherit', 8, 30,
       NULL, NULL, true
FROM unnest(ARRAY['prospecting','conversion','onboarding','adoption','expansion','renewal','retention']) AS motion
ON CONFLICT DO NOTHING;

-- Every existing rule gets a versioned applicability/action map. Most rules
-- preserve the detector's observation type and polarity; the exceptional
-- repeated-pricing rows above deliberately reinterpret those dimensions.
WITH mappings(rule_key, motion, action_key, action_label) AS (VALUES
    ('support_volume_spike', 'conversion', 'review_open_support_risk', 'Review open support risk'),
    ('support_volume_spike', 'retention', 'review_retention_risk', 'Review retention risk'),
    ('urgent_issue_open_deal', 'conversion', 'resolve_issue_for_deal', 'Resolve the issue blocking the deal'),
    ('urgent_issue_open_deal', 'expansion', 'resolve_issue_for_expansion', 'Resolve the issue blocking expansion'),
    ('urgent_issue_open_deal', 'renewal', 'resolve_issue_for_renewal', 'Resolve the issue before renewal'),
    ('support_ai_escalation', 'conversion', 'review_support_context', 'Review support context'),
    ('support_ai_escalation', 'retention', 'review_support_context', 'Review support context'),
    ('support_csat_deterioration', 'conversion', 'review_open_support_risk', 'Review open support risk'),
    ('support_csat_deterioration', 'retention', 'review_retention_risk', 'Review retention risk'),
    ('requested_feature_shipped', 'conversion', 'follow_up_on_shipped_feature', 'Follow up on the shipped feature'),
    ('requested_feature_shipped', 'adoption', 'drive_feature_adoption', 'Help the account adopt the shipped feature'),
    ('requested_feature_shipped', 'expansion', 'review_expansion_fit', 'Review expansion fit'),
    ('deal_stage_stalled', 'conversion', 'unstick_deal', 'Unstick the deal'),
    ('deal_stage_stalled', 'expansion', 'unstick_expansion', 'Unstick the expansion'),
    ('deal_stage_stalled', 'renewal', 'unstick_renewal', 'Unstick the renewal'),
    ('deal_gone_dark', 'conversion', 'reengage_deal', 'Re-engage the deal'),
    ('deal_gone_dark', 'expansion', 'reengage_expansion', 'Re-engage the expansion'),
    ('deal_gone_dark', 'renewal', 'reengage_renewal', 'Re-engage the renewal'),
    ('champion_quiet', 'conversion', 'reengage_champion', 'Re-engage the champion'),
    ('champion_quiet', 'expansion', 'reengage_champion', 'Re-engage the champion'),
    ('champion_quiet', 'renewal', 'reengage_champion', 'Re-engage the champion'),
    ('timeline_followup_lapsed', 'conversion', 'complete_followup', 'Complete the promised follow-up'),
    ('timeline_followup_lapsed', 'expansion', 'complete_followup', 'Complete the promised follow-up'),
    ('timeline_followup_lapsed', 'renewal', 'complete_followup', 'Complete the promised follow-up'),
    ('deal_single_threaded', 'conversion', 'multithread_deal', 'Build more relationships in the account'),
    ('deal_single_threaded', 'expansion', 'multithread_deal', 'Build more relationships in the account'),
    ('deal_single_threaded', 'renewal', 'multithread_deal', 'Build more relationships in the account'),
    ('renewal_approaching', 'renewal', 'prepare_renewal', 'Prepare the renewal conversation'),
    ('renewal_approaching', 'retention', 'review_retention_risk', 'Review retention risk'),
    ('buying_committee_expanded', 'conversion', 'engage_new_stakeholders', 'Engage the new stakeholders'),
    ('buying_committee_expanded', 'expansion', 'engage_new_stakeholders', 'Engage the new stakeholders'),
    ('buying_committee_expanded', 'renewal', 'engage_new_stakeholders', 'Engage the new stakeholders'),
    ('buying_committee_shrank', 'conversion', 'rebuild_buying_committee', 'Rebuild the buying committee'),
    ('buying_committee_shrank', 'expansion', 'rebuild_buying_committee', 'Rebuild the buying committee'),
    ('buying_committee_shrank', 'renewal', 'rebuild_buying_committee', 'Rebuild the buying committee'),
    ('procurement_page_activity', 'prospecting', 'follow_up_on_procurement', 'Follow up on procurement readiness'),
    ('procurement_page_activity', 'conversion', 'follow_up_on_procurement', 'Follow up on procurement readiness'),
    ('procurement_page_activity', 'expansion', 'review_expansion_procurement', 'Review expansion procurement needs'),
    ('known_contact_returned', 'prospecting', 'reengage_contact', 'Re-engage the returning contact'),
    ('known_contact_returned', 'conversion', 'reengage_contact', 'Re-engage the returning contact'),
    ('known_contact_returned', 'expansion', 'review_expansion_interest', 'Review expansion interest'),
    ('high_intent_product_event', 'conversion', 'follow_up_on_product_intent', 'Follow up on product intent'),
    ('high_intent_product_event', 'adoption', 'review_adoption', 'Review product adoption'),
    ('high_intent_product_event', 'expansion', 'review_expansion_interest', 'Review expansion interest'),
    ('session_depth_spike', 'prospecting', 'review_account_interest', 'Review account interest'),
    ('session_depth_spike', 'conversion', 'review_account_interest', 'Review account interest'),
    ('new_account_stakeholder', 'prospecting', 'review_new_stakeholder', 'Review the new stakeholder'),
    ('new_account_stakeholder', 'conversion', 'engage_new_stakeholder', 'Engage the new stakeholder'),
    ('anonymous_account_traffic', 'prospecting', 'review_account_interest', 'Review account interest'),
    ('anonymous_account_traffic', 'conversion', 'review_account_interest', 'Review account interest'),
    ('campaign_attributed_return', 'prospecting', 'follow_up_on_campaign_return', 'Follow up on the campaign return'),
    ('campaign_attributed_return', 'conversion', 'follow_up_on_campaign_return', 'Follow up on the campaign return'),
    ('pre_identification_history', 'prospecting', 'review_prior_interest', 'Review prior account interest'),
    ('pre_identification_history', 'conversion', 'review_prior_interest', 'Review prior account interest'),
    ('configured_form_submission', 'prospecting', 'follow_up_on_form', 'Follow up on the form submission'),
    ('configured_form_submission', 'conversion', 'follow_up_on_form', 'Follow up on the form submission'),
    ('configured_form_submission', 'expansion', 'review_expansion_interest', 'Review expansion interest'),
    ('identified_article_view', 'prospecting', 'review_content_interest', 'Review content interest'),
    ('identified_article_view', 'conversion', 'review_content_interest', 'Review content interest'),
    ('versioned_interaction', 'prospecting', 'review_interaction', 'Review the interaction'),
    ('versioned_interaction', 'conversion', 'review_interaction', 'Review the interaction'),
    ('external_provider_evidence', 'prospecting', 'review_external_evidence', 'Review external evidence'),
    ('external_provider_evidence', 'conversion', 'review_external_evidence', 'Review external evidence'),
    ('conversation_signal_extraction', 'prospecting', 'review_conversation_evidence', 'Review conversation evidence'),
    ('conversation_signal_extraction', 'conversion', 'follow_up_on_conversation', 'Follow up on the conversation'),
    ('conversation_signal_extraction', 'adoption', 'review_adoption', 'Review adoption'),
    ('conversation_signal_extraction', 'expansion', 'review_expansion', 'Review expansion opportunity'),
    ('conversation_signal_extraction', 'renewal', 'prepare_renewal', 'Prepare the renewal conversation'),
    ('conversation_signal_extraction', 'retention', 'review_retention_risk', 'Review retention risk'),
    ('conversation_signal_extraction', 'onboarding', 'review_onboarding', 'Review onboarding progress')
)
INSERT INTO crm_signal_interpretation_configs (
    workspace_id, rule_key, rule_version, motion, observation_signal_type,
    version, signal_type, polarity, business_weight, half_life_days,
    recommended_action_key, recommended_action_label, enabled
)
SELECT NULL, cfg.rule_key, cfg.version, mappings.motion, '*', 1,
       'inherit', 'inherit', cfg.business_weight, cfg.half_life_days,
       mappings.action_key, mappings.action_label, true
FROM crm_signal_rule_configs cfg
JOIN mappings ON mappings.rule_key = cfg.rule_key
WHERE cfg.enabled = true
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS crm_signal_motion_states (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    resolver_version integer NOT NULL,
    motions jsonb NOT NULL DEFAULT '{}'::jsonb,
    input_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    effective_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_crm_signal_motion_state_entity
    ON crm_signal_motion_states (workspace_id, entity_type, entity_id, effective_at DESC);
CREATE INDEX IF NOT EXISTS idx_crm_signal_motion_state_retention
    ON crm_signal_motion_states (effective_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_signal_motion_state_history
    ON crm_signal_motion_states (workspace_id, entity_type, entity_id, resolver_version, effective_at);

CREATE TABLE IF NOT EXISTS crm_signal_routing_settings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL UNIQUE,
    default_signal_owner_member_id uuid,
    minimum_lane_priority double precision NOT NULL DEFAULT 5,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS crm_signal_rollout_settings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL UNIQUE,
    mode text NOT NULL DEFAULT 'shadow' CHECK (mode IN ('shadow', 'live')),
    activated_at timestamptz,
    activated_by_member_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE crm_buyer_signals ADD COLUMN IF NOT EXISTS observation_id uuid;
ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS commercial_motion text NOT NULL DEFAULT 'conversion';
ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS interpretation_version integer NOT NULL DEFAULT 1;
ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS business_weight_snapshot double precision NOT NULL DEFAULT 0;
ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS half_life_days_snapshot double precision NOT NULL DEFAULT 0;
ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS interpretation_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE crm_buyer_signals ADD COLUMN IF NOT EXISTS meaning_fingerprint text NOT NULL DEFAULT '';
ALTER TABLE crm_buyer_signals ADD COLUMN IF NOT EXISTS recommended_action_key text;
ALTER TABLE crm_buyer_signals ADD COLUMN IF NOT EXISTS recommended_action_label text;
ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS replay_calibration_excluded boolean NOT NULL DEFAULT false;
ALTER TABLE crm_buyer_signals ADD COLUMN IF NOT EXISTS superseded_at timestamptz;
ALTER TABLE crm_buyer_signals ADD COLUMN IF NOT EXISTS superseded_reason text;
ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS direction_changed_by_supersession boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_crm_buyer_signals_motion_active
    ON crm_buyer_signals (workspace_id, commercial_motion, detected_at DESC)
    WHERE dismissed_at IS NULL AND superseded_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_crm_buyer_signals_observation_id
    ON crm_buyer_signals (observation_id);
CREATE INDEX IF NOT EXISTS idx_crm_buyer_signals_meaning_fingerprint
    ON crm_buyer_signals (workspace_id, meaning_fingerprint);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_buyer_signals_unique_meaning
    ON crm_buyer_signals (
        workspace_id,
        meaning_fingerprint,
        COALESCE(contact_id::text, ''),
        COALESCE(deal_id::text, ''),
        COALESCE(company_id::text, '')
    )
    WHERE meaning_fingerprint <> '';

DROP INDEX IF EXISTS idx_crm_rule_signal_evidence;
CREATE UNIQUE INDEX idx_crm_rule_signal_evidence
    ON crm_buyer_signals (
        workspace_id,
        rule_key,
        rule_version,
        commercial_motion,
        COALESCE(contact_id::text, ''),
        COALESCE(deal_id::text, ''),
        COALESCE(company_id::text, ''),
        evidence_fingerprint
    )
    WHERE rule_key IS NOT NULL AND evidence_fingerprint <> '';
