-- A confirmed CRM transition is native evidence, not an AI-generated signal.
ALTER TABLE crm_situations DROP CONSTRAINT IF EXISTS crm_situation_origin_contract;
ALTER TABLE crm_situations ADD CONSTRAINT crm_situation_origin_contract CHECK (
    (origin_kind = 'manual' AND created_by_member_id IS NOT NULL AND
        (company_id IS NOT NULL OR contact_id IS NOT NULL OR deal_id IS NOT NULL))
    OR origin_kind IN ('signal', 'suggestion')
    OR (origin_kind = 'deal_won' AND deal_id IS NOT NULL AND commercial_motion = 'onboarding')
);
ALTER TABLE crm_situation_changes DROP CONSTRAINT IF EXISTS crm_situation_changes_actor_kind_check;
ALTER TABLE crm_situation_changes ADD CONSTRAINT crm_situation_changes_actor_kind_check
    CHECK (actor_kind IN ('member', 'signal', 'suggestion', 'deal_won'));
