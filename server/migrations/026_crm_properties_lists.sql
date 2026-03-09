-- CRM Phase 2: Properties, Lists, and Import
-- Idempotent migration

-- Property Definitions
CREATE TABLE IF NOT EXISTS crm_property_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    object_type VARCHAR NOT NULL,
    internal_name VARCHAR NOT NULL,
    label VARCHAR NOT NULL,
    field_type VARCHAR NOT NULL DEFAULT 'text',
    options JSONB DEFAULT '[]',
    group_name VARCHAR,
    is_required BOOLEAN NOT NULL DEFAULT FALSE,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_property_defs_workspace_id ON crm_property_definitions(workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_property_defs_unique ON crm_property_definitions(workspace_id, object_type, internal_name);

-- Property Groups
CREATE TABLE IF NOT EXISTS crm_property_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    object_type VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_property_groups_workspace_id ON crm_property_groups(workspace_id);

-- Lists
CREATE TABLE IF NOT EXISTS crm_lists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    name VARCHAR NOT NULL,
    list_type VARCHAR NOT NULL DEFAULT 'static',
    object_type VARCHAR NOT NULL,
    filter_criteria JSONB DEFAULT '{}',
    member_count INTEGER NOT NULL DEFAULT 0,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_lists_workspace_id ON crm_lists(workspace_id);

-- List Members
CREATE TABLE IF NOT EXISTS crm_list_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    list_id UUID NOT NULL REFERENCES crm_lists(id) ON DELETE CASCADE,
    object_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_list_members_list_id ON crm_list_members(list_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_list_members_unique ON crm_list_members(list_id, object_id);

-- Import Jobs
CREATE TABLE IF NOT EXISTS crm_import_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    source VARCHAR NOT NULL DEFAULT 'csv',
    status VARCHAR NOT NULL DEFAULT 'pending',
    object_type VARCHAR NOT NULL,
    file_url VARCHAR,
    column_mapping JSONB DEFAULT '{}',
    total_rows INTEGER NOT NULL DEFAULT 0,
    processed_rows INTEGER NOT NULL DEFAULT 0,
    created_rows INTEGER NOT NULL DEFAULT 0,
    updated_rows INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    error_log JSONB DEFAULT '[]',
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_import_jobs_workspace_id ON crm_import_jobs(workspace_id);
