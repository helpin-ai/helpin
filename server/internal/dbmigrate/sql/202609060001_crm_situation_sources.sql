-- Add source provenance without fabricating customer targets or human creators.
ALTER TABLE crm_situations ADD COLUMN IF NOT EXISTS origin_kind text NOT NULL DEFAULT 'manual';
ALTER TABLE crm_situations ALTER COLUMN created_by_member_id DROP NOT NULL;
-- These names belong to the target/motion checks in 202609050002.
ALTER TABLE crm_situations DROP CONSTRAINT IF EXISTS crm_situations_check;
ALTER TABLE crm_situations DROP CONSTRAINT IF EXISTS crm_situations_commercial_motion_check;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'crm_situations'::regclass AND conname = 'crm_situation_origin_contract') THEN
        ALTER TABLE crm_situations ADD CONSTRAINT crm_situation_origin_contract CHECK (
            (origin_kind = 'manual' AND created_by_member_id IS NOT NULL AND
                (company_id IS NOT NULL OR contact_id IS NOT NULL OR deal_id IS NOT NULL))
            OR origin_kind IN ('signal', 'suggestion'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'crm_situations'::regclass AND conname = 'crm_situation_motion_contract') THEN
        ALTER TABLE crm_situations ADD CONSTRAINT crm_situation_motion_contract CHECK (
            commercial_motion IN ('prospecting','conversion','onboarding','adoption','expansion','renewal','retention')
            OR (origin_kind = 'suggestion' AND commercial_motion = 'needs_context'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS crm_situation_source_links (
    workspace_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('signal', 'suggestion')),
    source_id uuid NOT NULL,
    situation_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, kind, source_id),
    FOREIGN KEY (workspace_id, situation_id) REFERENCES crm_situations(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_crm_situation_source_destination
    ON crm_situation_source_links(workspace_id, situation_id);
