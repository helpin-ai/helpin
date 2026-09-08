-- Additive customer-work foundation. No source backfill, activation or side effects.
CREATE TABLE IF NOT EXISTS crm_situations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    creation_key text NOT NULL CHECK (length(creation_key) BETWEEN 1 AND 200),
    creation_fingerprint text NOT NULL,
    title text NOT NULL CHECK (length(trim(title)) BETWEEN 1 AND 240),
    objective text NOT NULL CHECK (length(trim(objective)) BETWEEN 1 AND 4000),
    commercial_motion text NOT NULL CHECK (commercial_motion IN
        ('prospecting', 'conversion', 'onboarding', 'adoption', 'expansion', 'renewal', 'retention')),
    company_id uuid,
    contact_id uuid,
    deal_id uuid,
    owner_member_id uuid,
    next_action_owner_member_id uuid,
    lifecycle text NOT NULL DEFAULT 'open' CHECK (lifecycle IN ('open', 'paused', 'closed')),
    attention text NOT NULL DEFAULT 'needs_context' CHECK (attention IN
        ('needs_context', 'needs_approval', 'follow_up_due', 'waiting_customer', 'waiting_work', 'automation_failed')),
    next_step text NOT NULL DEFAULT '' CHECK (length(next_step) <= 1000),
    priority double precision NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 100),
    next_checkpoint_at timestamptz,
    outcome_kind text CHECK (outcome_kind IN ('achieved', 'not_pursued', 'invalid', 'duplicate')),
    outcome_summary text,
    closed_at timestamptz,
    closed_by_member_id uuid,
    created_by_member_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (company_id IS NOT NULL OR contact_id IS NOT NULL OR deal_id IS NOT NULL),
    CHECK ((lifecycle = 'closed' AND outcome_kind IS NOT NULL
                AND outcome_summary IS NOT NULL AND length(trim(outcome_summary)) > 0 AND closed_at IS NOT NULL)
        OR (lifecycle <> 'closed' AND outcome_kind IS NULL AND outcome_summary IS NULL AND closed_at IS NULL)),
    UNIQUE (workspace_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_situation_creation
    ON crm_situations(workspace_id, creation_key);
CREATE INDEX IF NOT EXISTS idx_crm_situation_queue
    ON crm_situations(workspace_id, lifecycle, priority DESC, created_at, id);
CREATE INDEX IF NOT EXISTS idx_crm_situation_owner
    ON crm_situations(workspace_id, owner_member_id, lifecycle);
CREATE INDEX IF NOT EXISTS idx_crm_situation_next_owner
    ON crm_situations(workspace_id, next_action_owner_member_id, lifecycle);
CREATE INDEX IF NOT EXISTS idx_crm_situation_motion
    ON crm_situations(workspace_id, commercial_motion, lifecycle);

-- Sources keep their existing deletion and permission policies. References may
-- outlive a deleted source; readers validate source workspace/existence instead
-- of exposing stale payloads or blocking the owning module's delete operation.
CREATE TABLE IF NOT EXISTS crm_situation_references (
    workspace_id uuid NOT NULL,
    situation_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('signal', 'suggestion')),
    source_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, situation_id, kind, source_id),
    FOREIGN KEY (workspace_id, situation_id)
        REFERENCES crm_situations(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_crm_situation_reference_source
    ON crm_situation_references(workspace_id, kind, source_id);
