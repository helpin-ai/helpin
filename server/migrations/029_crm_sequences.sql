-- CRM Phase 5: Sequences & AI Writing

-- Sequences
CREATE TABLE IF NOT EXISTS crm_sequences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    steps JSONB DEFAULT '[]',
    enrollment_count INTEGER NOT NULL DEFAULT 0,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_sequences_workspace ON crm_sequences(workspace_id);

-- Sequence enrollments
CREATE TABLE IF NOT EXISTS crm_sequence_enrollments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    sequence_id UUID NOT NULL REFERENCES crm_sequences(id) ON DELETE CASCADE,
    contact_id UUID NOT NULL,
    current_step INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    enrolled_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    exit_reason TEXT,
    last_step_executed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_enrollments_workspace ON crm_sequence_enrollments(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_enrollments_sequence ON crm_sequence_enrollments(sequence_id);
CREATE INDEX IF NOT EXISTS idx_crm_enrollments_contact ON crm_sequence_enrollments(contact_id);

-- Writing profiles
CREATE TABLE IF NOT EXISTS crm_writing_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    member_id UUID NOT NULL,
    style_attributes JSONB DEFAULT '{}',
    sample_count INTEGER NOT NULL DEFAULT 0,
    last_analyzed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_writing_profiles_workspace ON crm_writing_profiles(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_writing_profiles_member ON crm_writing_profiles(member_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_writing_profiles_unique ON crm_writing_profiles(workspace_id, member_id);
