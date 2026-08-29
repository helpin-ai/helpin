-- Treat the evidence fingerprint as the durable detector identity. Evaluator
-- windows overlap deliberately and must not make identical evidence unique.

DROP INDEX IF EXISTS idx_crm_rule_signal_evidence;

CREATE TEMP TABLE crm_signal_evidence_dedup_map ON COMMIT DROP AS
WITH normalized AS (
    SELECT
        id AS signal_id,
        workspace_id,
        rule_key,
        rule_version,
        COALESCE(contact_id::text, '') AS contact_key,
        COALESCE(deal_id::text, '') AS deal_key,
        COALESCE(company_id::text, '') AS company_key,
        evidence_fingerprint,
        created_at
    FROM crm_buyer_signals
    WHERE rule_key IS NOT NULL
      AND evidence_fingerprint <> ''
), ranked AS (
    SELECT
        signal_id,
        FIRST_VALUE(signal_id) OVER evidence_group AS canonical_id,
        ROW_NUMBER() OVER evidence_group AS evidence_rank,
        COUNT(*) OVER evidence_partition AS evidence_count
    FROM normalized
    WINDOW
        evidence_partition AS (
            PARTITION BY workspace_id, rule_key, rule_version,
                contact_key, deal_key, company_key, evidence_fingerprint
        ),
        evidence_group AS (
            PARTITION BY workspace_id, rule_key, rule_version,
                contact_key, deal_key, company_key, evidence_fingerprint
            ORDER BY created_at ASC NULLS LAST, signal_id ASC
        )
)
SELECT signal_id, canonical_id, evidence_rank
FROM ranked
WHERE evidence_count > 1;

-- Preserve the strongest lifecycle state recorded on any duplicate.
WITH merged AS (
    SELECT
        mapping.canonical_id,
        MIN(signal.reviewed_at) AS reviewed_at,
        MIN(signal.acted_at) AS acted_at,
        MIN(signal.dismissed_at) AS dismissed_at,
        (ARRAY_AGG(signal.dismissed_by_member_id ORDER BY signal.dismissed_at DESC NULLS LAST)
            FILTER (WHERE signal.dismissed_by_member_id IS NOT NULL))[1] AS dismissed_by_member_id,
        (ARRAY_AGG(signal.dismissal_reason ORDER BY signal.dismissed_at DESC NULLS LAST)
            FILTER (WHERE signal.dismissal_reason IS NOT NULL))[1] AS dismissal_reason
    FROM crm_signal_evidence_dedup_map mapping
    JOIN crm_buyer_signals signal ON signal.id = mapping.signal_id
    GROUP BY mapping.canonical_id
)
UPDATE crm_buyer_signals canonical
SET reviewed_at = COALESCE(canonical.reviewed_at, merged.reviewed_at),
    acted_at = COALESCE(canonical.acted_at, merged.acted_at),
    dismissed_at = COALESCE(canonical.dismissed_at, merged.dismissed_at),
    dismissed_by_member_id = COALESCE(canonical.dismissed_by_member_id, merged.dismissed_by_member_id),
    dismissal_reason = COALESCE(canonical.dismissal_reason, merged.dismissal_reason)
FROM merged
WHERE canonical.id = merged.canonical_id;

UPDATE crm_signal_feedback feedback
SET signal_id = mapping.canonical_id
FROM crm_signal_evidence_dedup_map mapping
WHERE feedback.signal_id = mapping.signal_id
  AND mapping.signal_id <> mapping.canonical_id;

UPDATE crm_signal_external_evidence evidence
SET signal_id = mapping.canonical_id
FROM crm_signal_evidence_dedup_map mapping
WHERE evidence.signal_id = mapping.signal_id
  AND mapping.signal_id <> mapping.canonical_id;

-- Delivery routes are unique per signal. Keep the most complete route before
-- repointing it so duplicate routes cannot violate the delivery constraint.
CREATE TEMP TABLE crm_signal_delivery_dedup_map ON COMMIT DROP AS
SELECT
    delivery.id AS delivery_id,
    mapping.canonical_id,
    ROW_NUMBER() OVER (
        PARTITION BY mapping.canonical_id, delivery.policy_id, delivery.channel,
            COALESCE(delivery.recipient_member_id::text, ''),
            COALESCE(delivery.destination_team_id::text, '')
        ORDER BY
            CASE delivery.status
                WHEN 'sent' THEN 0
                WHEN 'sending' THEN 1
                WHEN 'pending' THEN 2
                ELSE 3
            END,
            delivery.delivered_at DESC NULLS LAST,
            delivery.created_at ASC NULLS LAST,
            delivery.id ASC
    ) AS delivery_rank
FROM crm_signal_deliveries delivery
JOIN crm_signal_evidence_dedup_map mapping ON mapping.signal_id = delivery.signal_id;

DELETE FROM crm_signal_deliveries delivery
USING crm_signal_delivery_dedup_map mapping
WHERE delivery.id = mapping.delivery_id
  AND mapping.delivery_rank > 1;

UPDATE crm_signal_deliveries delivery
SET signal_id = mapping.canonical_id
FROM crm_signal_delivery_dedup_map mapping
WHERE delivery.id = mapping.delivery_id
  AND mapping.delivery_rank = 1
  AND delivery.signal_id <> mapping.canonical_id;

DELETE FROM crm_buyer_signals signal
USING crm_signal_evidence_dedup_map mapping
WHERE signal.id = mapping.signal_id
  AND mapping.evidence_rank > 1;

UPDATE crm_buyer_signals
SET summary = 'Deep browsing session',
    polarity = 'neutral'
WHERE rule_key = 'session_depth_spike';

CREATE UNIQUE INDEX idx_crm_rule_signal_evidence
    ON crm_buyer_signals (
        workspace_id,
        rule_key,
        rule_version,
        COALESCE(contact_id::text, ''),
        COALESCE(deal_id::text, ''),
        COALESCE(company_id::text, ''),
        evidence_fingerprint
    )
    WHERE rule_key IS NOT NULL AND evidence_fingerprint <> '';
